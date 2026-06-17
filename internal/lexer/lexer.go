package lexer

import (
	"unicode"

	"github.com/restartfu/rune/internal/token"
)

type Lexer struct {
	src      []rune
	position int
	line     int
	column   int
}

func New(input string) *Lexer {
	return &Lexer{
		src:    []rune(input),
		line:   1,
		column: 1,
	}
}

func (l *Lexer) NextToken() token.Token {
	tok, _, _ := l.nextTokenWithRange()
	return tok
}

func (l *Lexer) NextTokenWithRange() (token.Token, int, int) {
	return l.nextTokenWithRange()
}

func (l *Lexer) nextTokenWithRange() (token.Token, int, int) {
	for {
		if l.position >= len(l.src) {
			return token.New(token.EOF, "", l.line, l.column), l.position, l.position
		}

		start := l.position
		r := l.src[l.position]
		if r == ' ' || r == '\t' || r == '\r' {
			l.advanceRune()
			continue
		}
		if r == '\n' {
			startLine := l.line
			startCol := l.column
			l.advanceRune()
			return token.New(token.NEWLINE, string(r), startLine, startCol), start, l.position
		}

		startLine := l.line
		startCol := l.column
		if isIdentStart(r) {
			return l.readIdent(startLine, startCol, start)
		}

		l.advanceRune()

		switch r {
		case '(':
			return token.New(token.LPAREN, string(r), startLine, startCol), start, l.position
		case ')':
			return token.New(token.RPAREN, string(r), startLine, startCol), start, l.position
		case '[':
			return token.New(token.LBRACK, string(r), startLine, startCol), start, l.position
		case ']':
			return token.New(token.RBRACK, string(r), startLine, startCol), start, l.position
		case '.':
			return token.New(token.DOT, string(r), startLine, startCol), start, l.position
		case '*':
			return token.New(token.STAR, string(r), startLine, startCol), start, l.position
		case '{':
			return token.New(token.LBRACE, string(r), startLine, startCol), start, l.position
		case '}':
			return token.New(token.RBRACE, string(r), startLine, startCol), start, l.position
		case ',':
			return token.New(token.COMMA, string(r), startLine, startCol), start, l.position
		case '/':
			if l.position < len(l.src) {
				next := l.src[l.position]
				if next == '/' {
					for l.position < len(l.src) && l.src[l.position] != '\n' {
						l.advanceRune()
					}
					continue
				}
				if next == '*' {
					l.advanceRune()
					for l.position < len(l.src)-1 {
						if l.src[l.position] == '*' && l.src[l.position+1] == '/' {
							l.advanceRune()
							l.advanceRune()
							break
						}
						l.advanceRune()
					}
					continue
				}
			}
			return token.New(token.ILLEGAL, string(r), startLine, startCol), start, l.position
		case '"':
			return l.readQuote('"', startLine, startCol, start, token.STRING)
		case '\'':
			return l.readQuote('\'', startLine, startCol, start, token.CHAR)
		case '`':
			return l.readQuote('`', startLine, startCol, start, token.STRING)
		default:
			if unicode.IsSpace(r) {
				continue
			}
			return token.New(token.ILLEGAL, string(r), startLine, startCol), start, l.position
		}
	}
}

func (l *Lexer) readQuote(quote rune, startLine, startCol, start int, tokenType token.Type) (token.Token, int, int) {
	l.advanceRune()
	for l.position < len(l.src) {
		r := l.src[l.position]
		if r == '\\' && quote != '`' && l.position+1 < len(l.src) {
			l.advanceRune()
			l.advanceRune()
			continue
		}
		if r == quote {
			l.advanceRune()
			break
		}
		l.advanceRune()
	}

	return token.New(tokenType, string(l.src[start:l.position]), startLine, startCol), start, l.position
}

func (l *Lexer) readIdent(startLine, startCol, start int) (token.Token, int, int) {
	l.advanceRune()
	for l.position < len(l.src) {
		r := l.src[l.position]
		if !isIdentPart(r) {
			break
		}
		l.advanceRune()
	}

	lit := string(l.src[start:l.position])

	switch lit {
	case "package":
		return token.New(token.PACKAGE, lit, startLine, startCol), start, l.position
	case "type":
		return token.New(token.TYPE, lit, startLine, startCol), start, l.position
	case "enum":
		return token.New(token.ENUM, lit, startLine, startCol), start, l.position
	default:
		return token.New(token.IDENT, lit, startLine, startCol), start, l.position
	}
}

func (l *Lexer) advanceRune() {
	r := l.src[l.position]
	l.position++
	if r == '\n' {
		l.line++
		l.column = 1
		return
	}
	l.column++
}

func isIdentStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func isIdentPart(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}
