package generator

import (
	"strings"
	"testing"

	"github.com/restartfu/rune/internal/parser"
)

func TestGenerate_Table(t *testing.T) {
	type testCase struct {
		name        string
		input       *parser.File
		path        string
		needle      string
		expectErr   bool
		errorNeedle string
	}

	tests := []testCase{
		{
			name: "single enum with fields",
			path: "examples/enum.rn",
			input: &parser.File{
				PackageName: "examples",
				Enums: []parser.Enum{
					{
						Name: "Example",
						Variants: []parser.Variant{
							{Name: "Allow"},
							{Name: "Deny", Fields: []parser.Field{{Name: "Reason", Type: "string"}}},
						},
					},
				},
			},
			needle: strings.Join([]string{
				"type Example interface",
			}, "\n"),
		},
		{
			name: "multiple variants no fields",
			path: "nested/example.rn",
			input: &parser.File{
				PackageName: "core",
				Enums: []parser.Enum{
					{
						Name:     "State",
						Variants: []parser.Variant{{Name: "Open"}, {Name: "Closed"}},
					},
				},
			},
			needle: strings.Join([]string{
				"type State interface",
			}, "\n"),
		},
		{
			name: "rewrites enum-qualified variant literals",
			path: "examples/enum.rn",
			input: &parser.File{
				PackageName: "examples",
				Enums: []parser.Enum{
					{
						Name: "Example",
						Variants: []parser.Variant{
							{Name: "Expired"},
						},
					},
				},
				Pieces: []parser.Piece{
					{Kind: parser.PieceGo, Text: "package examples\n\nfunc make() Example {\n\treturn Example.Expired{Reason: \"x\"}\n}\n"},
					{Kind: parser.PieceEnum, Enum: &parser.Enum{
						Name:     "Example",
						Variants: []parser.Variant{{Name: "Expired"}},
					}},
				},
			},
			needle: "return ExampleExpired{Reason: \"x\"}",
		},
		{
			name: "rewrites expression switch on enum into type switch",
			path: "examples/enum.rn",
			input: &parser.File{
				PackageName: "examples",
				Enums: []parser.Enum{
					{
						Name: "Example",
						Variants: []parser.Variant{
							{Name: "Deny"},
							{Name: "Other"},
						},
					},
				},
				Pieces: []parser.Piece{
					{Kind: parser.PieceGo, Text: "package examples\n\nfunc f(e Example) {\n\tswitch e {\n\tcase Example.Deny:\n\t}\n}\n"},
				},
			},
			needle: "switch e.(type)",
		},
		{
			name: "rewrites rune bound switch variable syntax",
			path: "examples/main.rn",
			input: &parser.File{
				PackageName: "main",
				Enums: []parser.Enum{
					{
						Name: "Pipeline",
						Variants: []parser.Variant{
							{Name: "Queued", Fields: []parser.Field{{Name: "ID", Type: "string"}}},
							{Name: "Failed", Fields: []parser.Field{{Name: "ID", Type: "string"}, {Name: "Reason", Type: "string"}}},
						},
					},
				},
				Pieces: []parser.Piece{
					{Kind: parser.PieceGo, Text: "package main\n\nfunc f(request Pipeline) string {\n\tswitch value := request {\n\tcase Queued:\n\t\treturn value.ID\n\tcase Failed:\n\t\treturn value.Reason\n\tdefault:\n\t\treturn \"\"\n\t}\n}\n"},
				},
			},
			needle: "switch value := request.(type)",
		},
		{
			name: "errors on explicit type switch syntax",
			path: "examples/enum.rn",
			input: &parser.File{
				PackageName: "examples",
				Enums: []parser.Enum{
					{
						Name: "Example",
						Variants: []parser.Variant{
							{Name: "Deny"},
							{Name: "Other"},
						},
					},
				},
				Pieces: []parser.Piece{
					{Kind: parser.PieceGo, Text: "package examples\n\nfunc f(e Example) {\n\tswitch e.(type) {\n\tcase Example.Deny:\n\t}\n}\n"},
				},
			},
			expectErr:   true,
			errorNeedle: "`.(type)` is not supported in rune",
		},
		{
			name: "rewrites bare enum variant in typed switch cases",
			path: "examples/enum.rn",
			input: &parser.File{
				PackageName: "examples",
				Enums: []parser.Enum{
					{
						Name:     "Example",
						Variants: []parser.Variant{{Name: "Deny"}, {Name: "Other"}},
					},
				},
				Pieces: []parser.Piece{
					{Kind: parser.PieceGo, Text: "package examples\n\nfunc f(e Example) {\n\tswitch e {\n\tcase Deny:\n\tcase Other:\n\t}\n}\n"},
				},
			},
			needle: "switch e.(type)",
		},
		{
			name: "rewrites enum-qualified type assertions",
			path: "examples/enum.rn",
			input: &parser.File{
				PackageName: "examples",
				Enums: []parser.Enum{
					{
						Name: "Example",
						Variants: []parser.Variant{
							{Name: "Expired"},
							{Name: "Active"},
						},
					},
				},
				Pieces: []parser.Piece{
					{Kind: parser.PieceGo, Text: "package examples\n\nfunc f(e Example) {\n\tif value, ok := e.(Example.Expired); ok {\n\t\t_ = value\n\t}\n\tif _, ok := e.(Example.Active); ok {\n\t\t_ = ok\n\t}\n}\n"},
				},
			},
			needle: "e.(ExampleExpired)",
		},
		{
			name: "rewrites bare enum variant in slice of known enum",
			path: "examples/main.rn",
			input: &parser.File{
				PackageName: "main",
				Enums: []parser.Enum{
					{
						Name: "Example",
						Variants: []parser.Variant{
							{Name: "Queued", Fields: []parser.Field{{Name: "ID", Type: "string"}}},
						},
					},
				},
				Pieces: []parser.Piece{
					{Kind: parser.PieceGo, Text: "package main\n\nfunc f() []Example {\n\treturn []Example{Queued{ID: \"a\"}}\n}\n"},
				},
			},
			needle: "return []Example{ExampleQueued{ID: \"a\"}}",
		},
		{
			name: "rewrites bare enum variant in map value with known type",
			path: "examples/main.rn",
			input: &parser.File{
				PackageName: "main",
				Enums: []parser.Enum{
					{
						Name: "Example",
						Variants: []parser.Variant{
							{Name: "Queued", Fields: []parser.Field{{Name: "ID", Type: "string"}}},
						},
					},
				},
				Pieces: []parser.Piece{
					{Kind: parser.PieceGo, Text: "package main\n\nfunc f() map[string]Example {\n\treturn map[string]Example{\"q\": Queued{ID: \"a\"}}\n}\n"},
				},
			},
			needle: "map[string]Example{\"q\": ExampleQueued{ID: \"a\"}}",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Generate(tc.input, tc.path)
			if tc.expectErr {
				if err == nil {
					t.Fatal("expected parse error, got nil")
				}
				if !strings.Contains(err.Error(), tc.errorNeedle) {
					t.Fatalf("wrong error\n got: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			gotText := strings.TrimSpace(string(got))
			if !strings.Contains(gotText, tc.needle) {
				t.Fatalf("generated output missing expected block\n got:\n%s", gotText)
			}
		})
	}
}
