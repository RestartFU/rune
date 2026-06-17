package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateTargets_Table(t *testing.T) {
	t.Run("direct file", func(t *testing.T) {
		tmp := t.TempDir()
		target := filepath.Join(tmp, "example.ru")
		if err := os.WriteFile(target, []byte("package examples\n\ntype Example enum { Allow }"), 0o644); err != nil {
			t.Fatal(err)
		}

		if err := generate([]string{target}); err != nil {
			t.Fatalf("generate file: %v", err)
		}

		if _, err := os.Stat(outputPathForTest(target)); err != nil {
			t.Fatalf("expected generated file: %v", err)
		}
	})

	t.Run("directory only", func(t *testing.T) {
		tmp := t.TempDir()
		root := filepath.Join(tmp, "root")
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
		nested := filepath.Join(root, "nested")
		if err := os.MkdirAll(nested, 0o755); err != nil {
			t.Fatal(err)
		}

		rootInput := filepath.Join(root, "root.ru")
		nestedInput := filepath.Join(nested, "nested.ru")
		if err := os.WriteFile(rootInput, []byte("package root\n\ntype R enum { A }"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(nestedInput, []byte("package nested\n\ntype N enum { B }"), 0o644); err != nil {
			t.Fatal(err)
		}

		if err := generate([]string{root}); err != nil {
			t.Fatalf("generate directory: %v", err)
		}

		if _, err := os.Stat(outputPathForTest(rootInput)); err != nil {
			t.Fatalf("expected root output: %v", err)
		}
		if _, err := os.Stat(outputPathForTest(nestedInput)); !os.IsNotExist(err) {
			t.Fatalf("did not expect nested output, got: %v", err)
		}
	})

	t.Run("./... recursive", func(t *testing.T) {
		tmp := t.TempDir()
		root := filepath.Join(tmp, "root")
		nested := filepath.Join(root, "nested")
		if err := os.MkdirAll(nested, 0o755); err != nil {
			t.Fatal(err)
		}

		rootInput := filepath.Join(root, "root.ru")
		nestedInput := filepath.Join(nested, "nested.ru")
		if err := os.WriteFile(rootInput, []byte("package root\n\ntype R enum { A }"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(nestedInput, []byte("package nested\n\ntype N enum { B }"), 0o644); err != nil {
			t.Fatal(err)
		}

		if err := generate([]string{filepath.Join(root, "...")}); err != nil {
			t.Fatalf("generate recursive: %v", err)
		}

		if _, err := os.Stat(outputPathForTest(rootInput)); err != nil {
			t.Fatalf("expected root output: %v", err)
		}
		if _, err := os.Stat(outputPathForTest(nestedInput)); err != nil {
			t.Fatalf("expected nested output: %v", err)
		}
	})

	t.Run("invalid target", func(t *testing.T) {
		tmp := t.TempDir()
		target := filepath.Join(tmp, "bad.txt")
		if err := os.WriteFile(target, []byte("bad"), 0o644); err != nil {
			t.Fatal(err)
		}

		err := generate([]string{target})
		if err == nil {
			t.Fatalf("expected unsupported target error")
		}
		if !strings.Contains(err.Error(), "unsupported target") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func outputPathForTest(path string) string {
	ext := filepath.Ext(path)
	return strings.TrimSuffix(path, ext) + "_rune.go"
}

func TestRunCommand_DirectoryMain(t *testing.T) {
	tmp := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module example\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	enumPath := filepath.Join(tmp, "enum.ru")
	mainPath := filepath.Join(tmp, "main.ru")

	if err := os.WriteFile(enumPath, []byte(`package main

type Example enum {
	Deny
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(mainPath, []byte(`package main

import "fmt"

func main() {
	example := Example.Deny{}
	fmt.Println(example)
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	origStdout := os.Stdout
	defer func() { os.Stdout = origStdout }()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	runErr := runCommand([]string{tmp})

	if closeErr := w.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	output, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}

	if runErr != nil {
		t.Fatalf("runCommand(%q): %v\nstdout: %s", tmp, runErr, string(output))
	}
	if !strings.Contains(string(output), "{}") {
		t.Fatalf("expected generated program output in stdout, got:\n%s", string(output))
	}

	binPath := filepath.Join(tmp, "target", "bin", filepath.Base(tmp))
	if _, err := os.Stat(binPath); err != nil {
		t.Fatalf("expected binary at %s", binPath)
	}
}

func TestRunCommand_NoMainError(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module example\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(tmp, "main.ru"), []byte(`package main

type Example enum {
	Deny
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runCommand([]string{tmp}); err == nil {
		t.Fatal("expected no main function error")
	}
}
