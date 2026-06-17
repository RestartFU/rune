package rune_lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func readMessage(r *bufio.Reader) (*rpcMessage, error) {
	var contentLength int

	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			break
		}

		const prefix = "Content-Length:"
		if strings.HasPrefix(strings.ToLower(line), strings.ToLower(prefix)) {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				trimmed := strings.TrimSpace(parts[1])
				if n, parseErr := strconv.Atoi(trimmed); parseErr == nil {
					contentLength = n
				}
			}
		}
	}

	if contentLength <= 0 {
		return nil, fmt.Errorf("invalid lsp content length")
	}

	payload := make([]byte, contentLength)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}

	msg := &rpcMessage{}
	if err := json.Unmarshal(payload, msg); err != nil {
		return nil, err
	}

	return msg, nil
}

func writeMessage(w io.Writer, msg *rpcMessage) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n", len(payload)); err != nil {
		return err
	}
	_, err = w.Write(payload)
	return err
}
