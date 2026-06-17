package rune_lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

import (
	"github.com/restartfu/rune/internal/generator"
	"github.com/restartfu/rune/internal/parser"
)

const (
	diagnosticErrorLevel = 1
	goLspCommand         = "gopls"
)

var parseErrorPattern = regexp.MustCompile(`parse error at line (\d+), column (\d+): (.+)$`)

type lspServer struct {
	mu          sync.Mutex
	gopls       *exec.Cmd
	goplsIn     *bufio.Writer
	goplsOut    *bufio.Reader
	clientIn    *bufio.Reader
	clientOut   *bufio.Writer
	ruByGo      map[string]string
	goByRu      map[string]string
	docStates   map[string]*docState
	runelspPath string
}

type docState struct {
	ruURI    string
	ruPath   string
	genURI   string
	genPath  string
	version  int
	opened   bool
	source   string
	hasParse bool
}

type rpcURIEvent struct {
	TextDocument struct {
		URI     string `json:"uri"`
		Version int    `json:"version"`
		Text    string `json:"text,omitempty"`
	} `json:"textDocument"`
}

type didChangeParams struct {
	TextDocument struct {
		URI     string `json:"uri"`
		Version int    `json:"version"`
	} `json:"textDocument"`
	ContentChanges []contentChange `json:"contentChanges"`
}

type contentChange struct {
	Range *lspRange `json:"range,omitempty"`
	Text  string    `json:"text"`
}

type lspRange struct {
	Start lspPos `json:"start"`
	End   lspPos `json:"end"`
}

type lspPos struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type diagnosticRange struct {
	Start lspPos `json:"start"`
	End   lspPos `json:"end"`
}

type diagnostic struct {
	Range    diagnosticRange `json:"range"`
	Severity int             `json:"severity"`
	Source   string          `json:"source"`
	Message  string          `json:"message"`
}

type publishDiagnosticsParams struct {
	URI         string       `json:"uri"`
	Version     int          `json:"version"`
	Diagnostics []diagnostic `json:"diagnostics"`
}

func Run(in io.Reader, out io.Writer) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmdPath := os.Getenv("RUNE_GOPLS_PATH")
	if cmdPath == "" {
		cmdPath = goLspCommand
	}

	cmd := exec.CommandContext(ctx, cmdPath, "-mode=stdio")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	server := &lspServer{
		gopls:     cmd,
		goplsIn:   bufio.NewWriter(stdin),
		goplsOut:  bufio.NewReader(stdout),
		clientIn:  bufio.NewReader(in),
		clientOut: bufio.NewWriter(out),
		ruByGo:    make(map[string]string),
		goByRu:    make(map[string]string),
		docStates: make(map[string]*docState),
	}

	defer func() {
		_ = server.sendExitIfRunning(cmd)
	}()

	errCh := make(chan error, 1)
	go func() {
		errCh <- cmd.Wait()
	}()

	clientErr := make(chan error, 1)
	go func() {
		clientErr <- server.proxyFromClient(ctx)
	}()

	goplsErr := make(chan error, 1)
	go func() {
		goplsErr <- server.proxyFromGopls(ctx)
	}()

	select {
	case err := <-clientErr:
		return err
	case err := <-goplsErr:
		if err == nil {
			return nil
		}
		return err
	case err := <-errCh:
		if err == nil {
			return nil
		}
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *lspServer) sendExitIfRunning(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	if err := cmd.Process.Kill(); err != nil {
		return err
	}
	return cmd.Wait()
}

func (s *lspServer) proxyFromClient(ctx context.Context) error {
	for {
		msg, err := readMessage(s.clientIn)
		if err != nil {
			return err
		}
		if msg == nil {
			continue
		}

		msgs, diag, err := s.transformClientMessage(msg)
		if err != nil {
			return err
		}
		if diag != nil {
			if err := s.sendToClient(diag); err != nil {
				return err
			}
		}
		for _, out := range msgs {
			if out == nil {
				continue
			}
			if err := s.sendToGopls(out); err != nil {
				return err
			}
		}
		select {
		case <-ctx.Done():
			return nil
		default:
		}
	}
}

func (s *lspServer) proxyFromGopls(ctx context.Context) error {
	for {
		msg, err := readMessage(s.goplsOut)
		if err != nil {
			return err
		}
		if msg == nil {
			continue
		}

		rewritten, err := s.transformGoplsMessage(msg)
		if err != nil {
			return err
		}

		if err := s.sendToClient(rewritten); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		default:
		}
	}
}

func (s *lspServer) transformClientMessage(msg *rpcMessage) ([]*rpcMessage, *rpcMessage, error) {
	if msg.Method == "" {
		return []*rpcMessage{msg}, nil, nil
	}

	switch msg.Method {
	case "textDocument/didOpen":
		return s.handleDidOpen(msg)
	case "textDocument/didChange":
		return s.handleDidChange(msg)
	case "textDocument/didClose":
		return s.handleDidClose(msg)
	case "textDocument/didSave":
		return s.handleDidSave(msg)
	default:
		return s.rewriteClientURIs(msg)
	}
}

func (s *lspServer) handleDidOpen(msg *rpcMessage) ([]*rpcMessage, *rpcMessage, error) {
	params := &rpcURIEvent{}
	if err := json.Unmarshal(msg.Params, params); err != nil {
		return nil, nil, err
	}

	if !isRuneURI(params.TextDocument.URI) {
		return []*rpcMessage{msg}, nil, nil
	}

	genText, genPath, genURI, err := s.generateGoFromSource(params.TextDocument.URI, params.TextDocument.Text)
	if err != nil {
		s.closeGeneratedDocument(params.TextDocument.URI)
		return nil, s.makeParseDiagnostic(params.TextDocument.URI, params.TextDocument.Version, err), nil
	}

	s.updateSourceState(params.TextDocument.URI, params.TextDocument.Text, genURI, genPath, params.TextDocument.Version)
	clear := s.makeParseDiagnostic(params.TextDocument.URI, params.TextDocument.Version, nil)

	goMsg := &rpcMessage{
		JSONRPC: "2.0",
		Method:  "textDocument/didOpen",
		Params: marshalMust(rpcURIEvent{
			TextDocument: struct {
				URI     string `json:"uri"`
				Version int    `json:"version"`
				Text    string `json:"text,omitempty"`
			}{
				URI:     genURI,
				Version: params.TextDocument.Version,
				Text:    string(genText),
			},
		}),
	}
	if err := s.markOpened(params.TextDocument.URI, true); err != nil {
		return nil, nil, err
	}

	return []*rpcMessage{goMsg}, clear, nil
}

func (s *lspServer) handleDidChange(msg *rpcMessage) ([]*rpcMessage, *rpcMessage, error) {
	params := &didChangeParams{}
	if err := json.Unmarshal(msg.Params, params); err != nil {
		return nil, nil, err
	}

	if !isRuneURI(params.TextDocument.URI) {
		return []*rpcMessage{msg}, nil, nil
	}

	state := s.getDocumentState(params.TextDocument.URI)
	currentText := state.source
	if len(params.ContentChanges) == 0 && len(currentText) > 0 {
		return []*rpcMessage{msg}, nil, nil
	}

	nextText := currentText
	if len(params.ContentChanges) > 0 && params.ContentChanges[len(params.ContentChanges)-1].Range == nil {
		nextText = params.ContentChanges[len(params.ContentChanges)-1].Text
	} else {
		var err error
		nextText, err = applyContentChanges(currentText, params.ContentChanges)
		if err != nil {
			nextText, err = readFileText(params.TextDocument.URI)
			if err != nil {
				return nil, nil, err
			}
		}
	}

	genText, genPath, genURI, err := s.generateGoFromSource(params.TextDocument.URI, nextText)
	if err != nil {
		s.closeGeneratedDocument(params.TextDocument.URI)
		return nil, s.makeParseDiagnostic(params.TextDocument.URI, params.TextDocument.Version, err), nil
	}

	s.updateSourceState(params.TextDocument.URI, nextText, genURI, genPath, params.TextDocument.Version)
	clear := s.makeParseDiagnostic(params.TextDocument.URI, params.TextDocument.Version, nil)

	if !state.opened {
		openMsg := &rpcMessage{
			JSONRPC: "2.0",
			Method:  "textDocument/didOpen",
			Params: marshalMust(rpcURIEvent{
				TextDocument: struct {
					URI     string `json:"uri"`
					Version int    `json:"version"`
					Text    string `json:"text,omitempty"`
				}{
					URI:     genURI,
					Version: params.TextDocument.Version,
					Text:    string(genText),
				},
			}),
		}
		if err := s.markOpened(params.TextDocument.URI, true); err != nil {
			return nil, nil, err
		}
		return []*rpcMessage{openMsg}, clear, nil
	}

	changeMsg := &rpcMessage{
		JSONRPC: "2.0",
		Method:  "textDocument/didChange",
		Params: marshalMust(map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri":     genURI,
				"version": params.TextDocument.Version,
			},
			"contentChanges": []map[string]string{
				{"text": string(genText)},
			},
		}),
	}
	return []*rpcMessage{changeMsg}, clear, nil
}

func (s *lspServer) handleDidSave(msg *rpcMessage) ([]*rpcMessage, *rpcMessage, error) {
	return s.rewriteClientURIs(msg)
}

func (s *lspServer) handleDidClose(msg *rpcMessage) ([]*rpcMessage, *rpcMessage, error) {
	params := &rpcURIEvent{}
	if err := json.Unmarshal(msg.Params, params); err != nil {
		return nil, nil, err
	}

	if !isRuneURI(params.TextDocument.URI) {
		return []*rpcMessage{msg}, nil, nil
	}

	s.closeGeneratedDocument(params.TextDocument.URI)
	return nil, nil, nil
}

func (s *lspServer) rewriteClientURIs(msg *rpcMessage) ([]*rpcMessage, *rpcMessage, error) {
	rewritten, err := rewriteURIs(msg.Params, s.ruToGoURI)
	if err != nil {
		return nil, nil, err
	}
	if len(rewritten) > 0 {
		msg.Params = rewritten
	}
	return []*rpcMessage{msg}, nil, nil
}

func (s *lspServer) transformGoplsMessage(msg *rpcMessage) (*rpcMessage, error) {
	switch msg.Method {
	case "textDocument/publishDiagnostics":
		rewritten, err := rewriteURIs(msg.Params, s.goToRuURI)
		if err != nil {
			return nil, err
		}
		if len(rewritten) > 0 {
			msg.Params = rewritten
		}
	}

	if msg.Params != nil {
		rewritten, err := rewriteURIs(msg.Params, s.goToRuURI)
		if err != nil {
			return nil, err
		}
		if len(rewritten) > 0 {
			msg.Params = rewritten
		}
	}
	if msg.Result != nil {
		rewritten, err := rewriteURIs(msg.Result, s.goToRuURI)
		if err != nil {
			return nil, err
		}
		if len(rewritten) > 0 {
			msg.Result = rewritten
		}
	}
	return msg, nil
}

func (s *lspServer) ruToGoURI(uri string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, ok := s.docStates[uri]
	if !ok {
		return "", false
	}
	return state.genURI, true
}

func (s *lspServer) goToRuURI(uri string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ru, ok := s.ruByGo[uri]
	return ru, ok
}

func (s *lspServer) updateSourceState(ruURI, source, genURI, genPath string, version int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, ok := s.docStates[ruURI]
	if !ok {
		state = &docState{ruURI: ruURI}
		s.docStates[ruURI] = state
	}

	if old := state.genURI; old != "" && old != genURI {
		delete(s.ruByGo, old)
		delete(s.goByRu, old)
	}

	state.ruPath, _ = uriToPath(ruURI)
	state.genURI = genURI
	state.genPath = genPath
	state.version = version
	state.source = source
	state.hasParse = false
	s.ruByGo[genURI] = ruURI
	s.goByRu[state.ruURI] = genURI
}

func (s *lspServer) getDocumentState(ruURI string) *docState {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.docStates[ruURI]
	if !ok {
		state = &docState{ruURI: ruURI}
		s.docStates[ruURI] = state
	}
	return state
}

func (s *lspServer) markOpened(ruURI string, open bool) error {
	s.mu.Lock()
	state, ok := s.docStates[ruURI]
	s.mu.Unlock()
	if !ok {
		return fmt.Errorf("missing doc state for %s", ruURI)
	}
	state.opened = open
	return nil
}

func (s *lspServer) closeGeneratedDocument(ruURI string) {
	s.mu.Lock()
	state, ok := s.docStates[ruURI]
	s.mu.Unlock()
	if !ok {
		return
	}

	state.hasParse = true
	state.opened = false
}

func (s *lspServer) generateGoFromSource(ruURI, source string) ([]byte, string, string, error) {
	parsed, err := parser.Parse(source)
	if err != nil {
		return nil, "", "", err
	}

	ruPath, err := uriToPath(ruURI)
	if err != nil {
		return nil, "", "", err
	}
	goPath := generatedPathFor(ruPath)

	out, err := generator.Generate(parsed, ruPath)
	if err != nil {
		return nil, "", "", err
	}
	if err := os.WriteFile(goPath, out, 0o644); err != nil {
		return nil, "", "", err
	}
	goURI, err := uriFromPath(goPath)
	if err != nil {
		return nil, "", "", err
	}
	return out, goPath, goURI, nil
}

func (s *lspServer) makeParseDiagnostic(ruURI string, version int, parseErr error) *rpcMessage {
	diagnostics := []diagnostic{}
	if parseErr != nil {
		line, col, message := parseErrorLocation(parseErr)
		diagnostics = append(diagnostics, diagnostic{
			Range: diagnosticRange{
				Start: lspPos{Line: line, Character: col},
				End:   lspPos{Line: line, Character: col + 1},
			},
			Severity: diagnosticErrorLevel,
			Source:   "rune",
			Message:  message,
		})
	}

	out, _ := json.Marshal(publishDiagnosticsParams{
		URI:         ruURI,
		Version:     version,
		Diagnostics: diagnostics,
	})

	return &rpcMessage{
		JSONRPC: "2.0",
		Method:  "textDocument/publishDiagnostics",
		Params:  out,
	}
}

func parseErrorLocation(err error) (int, int, string) {
	if err == nil {
		return 0, 0, ""
	}
	text := err.Error()
	matches := parseErrorPattern.FindStringSubmatch(text)
	if len(matches) != 4 {
		return 0, 0, text
	}

	line, _ := strconv.Atoi(matches[1])
	col, _ := strconv.Atoi(matches[2])
	return line - 1, col - 1, matches[3]
}

func parseErrorRange(msg string) (int, int) {
	matches := parseErrorPattern.FindStringSubmatch(msg)
	if len(matches) != 4 {
		return 0, 0
	}
	line, _ := strconv.Atoi(matches[1])
	col, _ := strconv.Atoi(matches[2])
	return line - 1, col - 1
}

func readFileText(uri string) (string, error) {
	path, err := uriToPath(uri)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *lspServer) sendToClient(msg *rpcMessage) error {
	return writeMessageAndFlush(s.clientOut, msg)
}

func (s *lspServer) sendToGopls(msg *rpcMessage) error {
	return writeMessageAndFlush(s.goplsIn, msg)
}

func writeMessageAndFlush(w io.Writer, msg *rpcMessage) error {
	if err := writeMessage(w, msg); err != nil {
		return err
	}
	if flusher, ok := w.(interface{ Flush() error }); ok {
		return flusher.Flush()
	}
	return nil
}

func isRuneURI(uri string) bool {
	path, err := uriToPath(uri)
	if err != nil {
		return false
	}
	return strings.HasSuffix(strings.ToLower(path), ".rn")
}

func rewriteURIs(raw json.RawMessage, rewrite func(string) (string, bool)) (json.RawMessage, error) {
	if len(raw) == 0 {
		return raw, nil
	}

	var node interface{}
	if err := json.Unmarshal(raw, &node); err != nil {
		return nil, err
	}

	changed := rewriteURITree(node, rewrite)
	if !changed {
		return raw, nil
	}

	return json.Marshal(node)
}

func rewriteURITree(node interface{}, rewrite func(string) (string, bool)) bool {
	switch v := node.(type) {
	case map[string]interface{}:
		changed := false
		for key, val := range v {
			switch key {
			case "uri", "documentUri":
				if str, ok := val.(string); ok {
					if next, ok := rewrite(str); ok {
						v[key] = next
						changed = true
					}
				}
			}
			if rewriteURITree(val, rewrite) {
				changed = true
			}
		}
		return changed
	case []interface{}:
		changed := false
		for idx, val := range v {
			if rewriteURITree(val, rewrite) {
				changed = true
			}
			_ = idx
		}
		return changed
	default:
		return false
	}
}

func applyContentChanges(source string, changes []contentChange) (string, error) {
	if len(changes) == 0 {
		return source, nil
	}

	if len(changes) == 1 && changes[0].Range == nil {
		return changes[0].Text, nil
	}

	text := source
	for _, change := range changes {
		if change.Range == nil {
			text = change.Text
			continue
		}
		text = applySingleChange(text, change)
	}
	return text, nil
}

func applySingleChange(source string, change contentChange) string {
	start := charOffsetAtLineColumn(source, change.Range.Start.Line, change.Range.Start.Character)
	end := charOffsetAtLineColumn(source, change.Range.End.Line, change.Range.End.Character)
	run := []rune(source)
	if start > len(run) {
		start = len(run)
	}
	if end > len(run) {
		end = len(run)
	}
	if start > end {
		start = end
	}

	out := append([]rune{}, run[:start]...)
	out = append(out, []rune(change.Text)...)
	out = append(out, run[end:]...)
	return string(out)
}

func charOffsetAtLineColumn(source string, line, character int) int {
	if line < 0 || character < 0 {
		return 0
	}
	lines := strings.Split(source, "\n")
	offset := 0
	for i := 0; i < line && i < len(lines)-1; i++ {
		offset += len([]rune(lines[i]))
		offset++
	}
	if line >= len(lines) {
		offset = len([]rune(source))
		return offset
	}
	return offset + min(character, len([]rune(lines[line])))
}

func uriToPath(uri string) (string, error) {
	parsed, err := url.Parse(uri)
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "file" {
		return "", fmt.Errorf("unsupported URI scheme %q", parsed.Scheme)
	}

	return url.PathUnescape(parsed.Path)
}

func uriFromPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(abs)}).String(), nil
}

func generatedPathFor(path string) string {
	ext := filepath.Ext(path)
	return strings.TrimSuffix(path, ext) + "_rune.go"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func marshalMust(v interface{}) json.RawMessage {
	raw, _ := json.Marshal(v)
	return raw
}
