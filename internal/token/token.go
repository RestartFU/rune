package token

// Type identifies a lexical token kind in Rune source files.
type Type string

const (
	ILLEGAL Type = "ILLEGAL"
	EOF     Type = "EOF"
	NEWLINE Type = "NEWLINE"

	IDENT  Type = "IDENT"
	COMMA  Type = "COMMA"
	DOT    Type = "DOT"
	LPAREN Type = "LPAREN"
	RPAREN Type = "RPAREN"
	LBRACK Type = "LBRACK"
	RBRACK Type = "RBRACK"
	STAR   Type = "STAR"
	LBRACE Type = "LBRACE"
	RBRACE Type = "RBRACE"
	STRING Type = "STRING"
	CHAR   Type = "CHAR"

	PACKAGE Type = "PACKAGE"
	TYPE    Type = "TYPE"
	ENUM    Type = "ENUM"
)

// Token is a single lexical token with its location in the source.
type Token struct {
	Type    Type
	Literal string
	Line    int
	Column  int
}

// New returns a basic token value.
func New(t Type, literal string, line, column int) Token {
	return Token{
		Type:    t,
		Literal: literal,
		Line:    line,
		Column:  column,
	}
}
