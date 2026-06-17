package transpile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/restartfu/rune/internal/parser"
)

func TestOutputPathFor(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		outputDir string
		out       string
	}{
		{name: "ru extension", in: "/tmp/example/example.rn", outputDir: "/tmp/example", out: "/tmp/example/example_rune.go"},
		{name: "path without extension", in: "/tmp/example/example", outputDir: "/tmp/example", out: "/tmp/example/example_rune.go"},
		{name: "custom target dir", in: "/tmp/example/example.rn", outputDir: "/tmp/example/target", out: "/tmp/example/target/example_rune.go"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got, want := outputPathFor(tc.in, tc.outputDir), tc.out; got != want {
				t.Fatalf("outputPathFor(%q) = %q, want %q", tc.in, got, want)
			}
		})
	}
}

func TestOutputPathForInTarget(t *testing.T) {
	got := OutputPathForInTarget("/tmp/example/example.rn")
	want := filepath.Join(filepath.Dir("/tmp/example/example.rn"), "target", "example_rune.go")
	if got != want {
		t.Fatalf("OutputPathForInTarget(%q) = %q, want %q", "/tmp/example/example.rn", got, want)
	}
}

func TestFileInTargetWithEnums_RewritesAcrossFiles(t *testing.T) {
	tmp := t.TempDir()
	srcPath := filepath.Join(tmp, "main.rn")
	enumPath := filepath.Join(tmp, "enum.rn")

	if err := os.WriteFile(enumPath, []byte(`package examples

type Example enum {
	Deny
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(srcPath, []byte(`package examples

func main() {
	example := Example.Deny{}
	_ = example
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	parsed, err := parser.Parse(`package examples

type Example enum {
	Deny
}`)
	if err != nil {
		t.Fatal(err)
	}

	generatedPath, err := FileInTargetWithEnums(srcPath, parsed.Enums)
	if err != nil {
		t.Fatalf("FileInTargetWithEnums: %v", err)
	}

	out, err := os.ReadFile(generatedPath)
	if err != nil {
		t.Fatalf("expected generated file: %v", err)
	}

	got := string(out)
	if !strings.Contains(got, "example := ExampleDeny{}") {
		t.Fatalf("expected cross-file enum constructor rewrite, got:\n%s", got)
	}
	if !strings.Contains(got, "package examples") {
		t.Fatalf("expected original package in generated output, got:\n%s", got)
	}
}

func TestFile_Table(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		sourcePath  string
		errContains string
	}{
		{
			name:       "generates sibling file",
			input:      "package examples\ntype Example enum { Allow, Deny { Reason string } }",
			sourcePath: "examples/input.rn",
		},
		{
			name: "keeps go code and inserts enums",
			input: `package examples

import "time"

type Example enum {
	Allow,
	Deny { Timestamp *time.Time },
}

func NewExample() Example { return nil }`,
			sourcePath: "examples/mixed.rn",
		},
		{
			name:        "returns parse error",
			input:       "type Example enum { Allow }",
			sourcePath:  "examples/bad.rn",
			errContains: "expected 'package' declaration",
		},
		{
			name:       "keeps go code without enums",
			input:      "package examples\n\nfunc main() {}",
			sourcePath: "examples/main.rn",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			inputPath := filepath.Join(tmp, tc.sourcePath)
			if err := os.MkdirAll(filepath.Dir(inputPath), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(inputPath, []byte(tc.input), 0644); err != nil {
				t.Fatal(err)
			}

			err := File(inputPath)
			if tc.errContains != "" {
				if err == nil {
					t.Fatalf("expected error containing %q", tc.errContains)
				}
				if !strings.Contains(err.Error(), tc.errContains) {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			outPath := outputPathFor(inputPath, filepath.Dir(inputPath))
			out, err := os.ReadFile(outPath)
			if err != nil {
				t.Fatalf("expected generated file: %v", err)
			}
			got := string(out)
			if tc.name != "keeps go code without enums" && !strings.Contains(got, "type Example interface {") {
				t.Fatalf("generated file missing enum interface\n%s", got)
			}
			if tc.name == "keeps go code and inserts enums" {
				if !strings.Contains(got, "import \"time\"") {
					t.Fatalf("generated file missing original import block\n%s", got)
				}
				if !strings.Contains(got, "func NewExample() Example") {
					t.Fatalf("generated file missing original go code\n%s", got)
				}
			} else if tc.name == "keeps go code without enums" && !strings.Contains(got, "func main() {}") {
				t.Fatalf("expected original go code in generated file\n%s", got)
			}
			if !strings.Contains(got, "//go:generate rune generate "+filepath.ToSlash(inputPath)) {
				t.Fatalf("expected go:generate path %q in output", inputPath)
			}
		})
	}
}
