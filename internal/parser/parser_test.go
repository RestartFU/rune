package parser

import (
	"reflect"
	"strings"
	"testing"
)

func TestParse_Table(t *testing.T) {
	type testCase struct {
		name         string
		input        string
		want         *File
		errContains  string
		hasLineError bool
	}

	tests := []testCase{
		{
			name:  "single enum with trailing comma and field",
			input: "package examples\n\n type Example enum {\n  Allow,\n  Deny { Reason string Code int },\n }",
			want: &File{
				PackageName: "examples",
				Enums: []Enum{
					{
						Name: "Example",
						Variants: []Variant{
							{Name: "Allow"},
							{Name: "Deny", Fields: []Field{{Name: "Reason", Type: "string"}, {Name: "Code", Type: "int"}}},
						},
					},
				},
			},
		},
		{
			name:  "multiple enum declarations",
			input: "package ex\ntype A enum { X }\ntype B enum { Y { V string } }",
			want: &File{
				PackageName: "ex",
				Enums: []Enum{
					{Name: "A", Variants: []Variant{{Name: "X"}}},
					{Name: "B", Variants: []Variant{{Name: "Y", Fields: []Field{{Name: "V", Type: "string"}}}}},
				},
			},
		},
		{
			name:  "multiline field block",
			input: "package ex\ntype E enum {\n  A {\n    Name string\n    Count int\n  }\n}",
			want: &File{
				PackageName: "ex",
				Enums: []Enum{
					{
						Name: "E",
						Variants: []Variant{
							{Name: "A", Fields: []Field{{Name: "Name", Type: "string"}, {Name: "Count", Type: "int"}}},
						},
					},
				},
			},
		},
		{
			name:        "missing package",
			input:       "type E enum { A }",
			errContains: "expected 'package' declaration",
		},
		{
			name:        "empty enum",
			input:       "package ex\ntype E enum { }",
			errContains: "enum must have at least one variant",
		},
		{
			name:         "invalid field syntax",
			input:        "package ex\ntype E enum { A { Name } }",
			errContains:  "expected field type",
			hasLineError: true,
		},
		{
			name: "no enum declarations",
			input: "package examples\n\nfunc main() {}",
			want: &File{
				PackageName: "examples",
			},
		},
		{
			name: "string literal with braces",
			input: `package ex

func template() string { return "{ }" }

type E enum { A }
`,
			want: &File{
				PackageName: "ex",
				Enums: []Enum{
					{Name: "E", Variants: []Variant{{Name: "A"}}},
				},
			},
		},
		{
			name: "go code with enum",
			input: `package core

import "time"

type Example enum {
	Allow,
	Deny { At *time.Time, Code int },
}

func helper() string { return "" }
`,
			want: &File{
				PackageName: "core",
				Enums: []Enum{
					{
						Name: "Example",
						Variants: []Variant{
							{Name: "Allow"},
							{Name: "Deny", Fields: []Field{{Name: "At", Type: "*time.Time"}, {Name: "Code", Type: "int"}}},
						},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(tc.input)
			if tc.errContains != "" {
				if err == nil {
					t.Fatalf("expected error containing %q", tc.errContains)
				}
				if !strings.Contains(err.Error(), tc.errContains) {
					t.Fatalf("unexpected error: %q", err)
				}
				if tc.hasLineError && !strings.Contains(err.Error(), "line 1,") && !strings.Contains(err.Error(), "line 2,") {
					t.Fatalf("expected error with line/column, got: %q", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got.Enums, tc.want.Enums) || got.PackageName != tc.want.PackageName {
				t.Fatalf("parsed AST mismatch\n got: %#v\nwant: %#v", got, tc.want)
			}
		})
	}
}
