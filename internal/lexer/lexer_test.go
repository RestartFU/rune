package lexer

import (
	"reflect"
	"testing"

	"github.com/restartfu/rune/internal/token"
)

func TestNextToken_Table(t *testing.T) {
	type testCase struct {
		name  string
		input string
		want  []token.Token
	}

	tests := []testCase{
		{
			name:  "keywords and separators",
			input: "package p type E enum { A, B { C string } }",
			want: []token.Token{
				{Type: token.PACKAGE, Literal: "package", Line: 1, Column: 1},
				{Type: token.IDENT, Literal: "p", Line: 1, Column: 9},
				{Type: token.TYPE, Literal: "type", Line: 1, Column: 11},
				{Type: token.IDENT, Literal: "E", Line: 1, Column: 16},
				{Type: token.ENUM, Literal: "enum", Line: 1, Column: 18},
				{Type: token.LBRACE, Literal: "{", Line: 1, Column: 23},
				{Type: token.IDENT, Literal: "A", Line: 1, Column: 25},
				{Type: token.COMMA, Literal: ",", Line: 1, Column: 26},
				{Type: token.IDENT, Literal: "B", Line: 1, Column: 28},
				{Type: token.LBRACE, Literal: "{", Line: 1, Column: 30},
				{Type: token.IDENT, Literal: "C", Line: 1, Column: 32},
				{Type: token.IDENT, Literal: "string", Line: 1, Column: 34},
				{Type: token.RBRACE, Literal: "}", Line: 1, Column: 41},
				{Type: token.RBRACE, Literal: "}", Line: 1, Column: 43},
				{Type: token.EOF, Literal: "", Line: 1, Column: 44},
			},
		},
		{
			name:  "multiline fields with positions",
			input: "package p\ntype Example enum {\n  A,\n  B {\n    Reason string\n    Code int\n  }\n}\n",
			want: []token.Token{
				{Type: token.PACKAGE, Literal: "package", Line: 1, Column: 1},
				{Type: token.IDENT, Literal: "p", Line: 1, Column: 9},
				{Type: token.NEWLINE, Literal: "\n", Line: 1, Column: 10},
				{Type: token.TYPE, Literal: "type", Line: 2, Column: 1},
				{Type: token.IDENT, Literal: "Example", Line: 2, Column: 6},
				{Type: token.ENUM, Literal: "enum", Line: 2, Column: 14},
				{Type: token.LBRACE, Literal: "{", Line: 2, Column: 19},
				{Type: token.NEWLINE, Literal: "\n", Line: 2, Column: 20},
				{Type: token.IDENT, Literal: "A", Line: 3, Column: 3},
				{Type: token.COMMA, Literal: ",", Line: 3, Column: 4},
				{Type: token.NEWLINE, Literal: "\n", Line: 3, Column: 5},
				{Type: token.IDENT, Literal: "B", Line: 4, Column: 3},
				{Type: token.LBRACE, Literal: "{", Line: 4, Column: 5},
				{Type: token.NEWLINE, Literal: "\n", Line: 4, Column: 6},
				{Type: token.IDENT, Literal: "Reason", Line: 5, Column: 5},
				{Type: token.IDENT, Literal: "string", Line: 5, Column: 12},
				{Type: token.NEWLINE, Literal: "\n", Line: 5, Column: 18},
				{Type: token.IDENT, Literal: "Code", Line: 6, Column: 5},
				{Type: token.IDENT, Literal: "int", Line: 6, Column: 10},
				{Type: token.NEWLINE, Literal: "\n", Line: 6, Column: 13},
				{Type: token.RBRACE, Literal: "}", Line: 7, Column: 3},
				{Type: token.NEWLINE, Literal: "\n", Line: 7, Column: 4},
				{Type: token.RBRACE, Literal: "}", Line: 8, Column: 1},
				{Type: token.NEWLINE, Literal: "\n", Line: 8, Column: 2},
				{Type: token.EOF, Literal: "", Line: 9, Column: 1},
			},
		},
		{
			name:  "illegal token has location",
			input: "package p\n@",
			want: []token.Token{
				{Type: token.PACKAGE, Literal: "package", Line: 1, Column: 1},
				{Type: token.IDENT, Literal: "p", Line: 1, Column: 9},
				{Type: token.NEWLINE, Literal: "\n", Line: 1, Column: 10},
				{Type: token.ILLEGAL, Literal: "@", Line: 2, Column: 1},
				{Type: token.EOF, Literal: "", Line: 2, Column: 2},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l := New(tc.input)
			got := []token.Token{}

			for {
				tok := l.NextToken()
				got = append(got, tok)
				if tok.Type == token.EOF {
					break
				}
			}

			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("tokens mismatch\n got: %#v\nwant: %#v", got, tc.want)
			}
		})
	}
}
