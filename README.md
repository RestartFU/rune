# Rune

Rune is a Go preprocessor that adds enum-like ADT syntax for sum types and rewrites it into plain Go during generation.

## Requirements

- Go 1.26.3+

## Install

```bash
go install github.com/restartfu/rune/cmd/rune@latest
go install github.com/restartfu/rune/cmd/rune-lsp@latest
```

Binary names:

- `rune` (compiler/transpiler)
- `rune-lsp` (Language Server Protocol proxy for editor integrations)

## Usage

```bash
rune <command> <target> [<target>...]
```

Targets:

- `path/to/file.rn`
- `path/to/dir`
- `./...` (recursive)

### Commands

`generate`

```bash
rune generate path/to/file.rn
rune generate path/to/dir
rune generate ./...
```

Generates `<name>_rune.go` for each `.rn` file.

`run`

```bash
rune run path/to/main.rn
rune run path/to/dir
```

Collects enums in directory, generates outputs into `target/`, builds with `go build`, then runs.
If building from a directory target, stale `*_rune.go` files in `target/` are removed before generation.

`build`

```bash
rune build path/to/main.rn
rune build -o ./bin/app path/to/main.rn
```

Generates in `target/` and runs `go build`, outputting a binary next to the generated package by default.

## Example

Source `main.rn`:

```go
package main

type Pipeline enum {
    Queued { ID string }
    Running { ID string, Service string, Attempts int }
    Succeeded { ID string, Artifact string }
    Failed { Reason string }
}

func isTerminal(p Pipeline) bool {
    switch p {
    case Succeeded, Failed:
        return true
    default:
        return false
    }
}
```

Generated Go uses generated interfaces/types plus type switches, e.g. `PipelineQueued`, `NewPipelineQueued()`, and `switch request.(type)` where relevant.

## Testing

```bash
make tests
```

## LSP / editor

`rune-lsp` is a thin wrapper that forwards to `gopls` and maps diagnostics/edits for `.rn` sources. The repo includes a Zed extension under `editors/zed`.
