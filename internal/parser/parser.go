package parser

import (
	"fmt"
	"strings"

	"github.com/restartfu/rune/internal/lexer"
	"github.com/restartfu/rune/internal/token"
)

type Parser struct {
	lexer  *lexer.Lexer
	source []rune

	cur   token.Token
	peek  token.Token
	peek2 token.Token

	curStart   int
	peekStart  int
	peek2Start int

	parenDepth int
}

func Parse(src string) (*File, error) {
	p := &Parser{lexer: lexer.New(src), source: []rune(src)}
	p.advance()
	p.advance()
	p.advance()

	file := &File{}

	if err := p.parsePackage(file); err != nil {
		return nil, err
	}

	cursor := 0
	for p.cur.Type != token.EOF {
		if p.isEnumDeclaration() && p.parenDepth == 0 {
			enumStart := p.curStart
			if enumStart > cursor {
				file.Pieces = append(file.Pieces, Piece{Kind: PieceGo, Text: string(p.source[cursor:enumStart])})
			}
			enum, err := p.parseEnumDecl()
			if err != nil {
				return nil, err
			}
			file.Enums = append(file.Enums, *enum)
			file.Pieces = append(file.Pieces, Piece{Kind: PieceEnum, Enum: enum})
			cursor = p.curStart
			continue
		}

		p.trackBraces()
		p.advance()
	}

	if cursor < len(p.source) {
		file.Pieces = append(file.Pieces, Piece{Kind: PieceGo, Text: string(p.source[cursor:])})
	}

	return file, nil
}

func (p *Parser) parsePackage(file *File) error {
	p.skipNewlines()
	if err := p.expect(token.PACKAGE, "expected 'package' declaration"); err != nil {
		return err
	}

	pkgTok, err := p.consume(token.IDENT, "expected package name")
	if err != nil {
		return err
	}

	file.PackageName = pkgTok.Literal
	return nil
}

func (p *Parser) parseEnumDecl() (*Enum, error) {
	p.skipNewlines()
	if err := p.expect(token.TYPE, "expected 'type' before enum declaration"); err != nil {
		return nil, err
	}

	nameTok, err := p.consume(token.IDENT, "expected enum name")
	if err != nil {
		return nil, err
	}

	p.skipNewlines()
	if err := p.expect(token.ENUM, "expected 'enum'"); err != nil {
		return nil, err
	}

	p.skipNewlines()
	if err := p.expect(token.LBRACE, "expected '{' after enum name"); err != nil {
		return nil, err
	}

	variants, err := p.parseVariantList()
	if err != nil {
		return nil, err
	}

	if err := p.expect(token.RBRACE, "expected '}' after enum variants"); err != nil {
		return nil, err
	}

	enum := &Enum{
		Name:     nameTok.Literal,
		Variants: variants,
	}

	return enum, nil
}

func (p *Parser) parseVariantList() ([]Variant, error) {
	if p.cur.Type == token.RBRACE {
		return nil, p.errorAt(p.cur, "enum must have at least one variant")
	}

	variants := []Variant{}

	first, err := p.parseVariant()
	if err != nil {
		return nil, err
	}
	variants = append(variants, first)

	for {
		p.skipNewlines()
		if !p.match(token.COMMA) {
			break
		}
		p.skipNewlines()
		if p.cur.Type == token.RBRACE {
			break
		}
		variant, err := p.parseVariant()
		if err != nil {
			return nil, err
		}
		variants = append(variants, variant)
	}

	return variants, nil
}

func (p *Parser) parseVariant() (Variant, error) {
	p.skipNewlines()
	nameTok, err := p.consume(token.IDENT, "expected variant name")
	if err != nil {
		return Variant{}, err
	}

	variant := Variant{Name: nameTok.Literal}

	if p.cur.Type == token.LBRACE {
		fields, err := p.parseFieldBlock()
		if err != nil {
			return Variant{}, err
		}
		variant.Fields = fields
	}

	return variant, nil
}

func (p *Parser) parseFieldBlock() ([]Field, error) {
	if err := p.expect(token.LBRACE, "expected '{' to start variant field block"); err != nil {
		return nil, err
	}

	fields := []Field{}
	for p.cur.Type != token.RBRACE {
		p.skipNewlines()
		if p.cur.Type == token.EOF {
			return nil, p.errorAt(p.cur, "unterminated variant field block")
		}
		if p.cur.Type != token.IDENT {
			return nil, p.errorf("expected field name")
		}

		nameTok, err := p.consume(token.IDENT, "expected field name")
		if err != nil {
			return nil, err
		}

		fieldType, err := p.parseFieldType()
		if err != nil {
			return nil, err
		}

		fields = append(fields, Field{Name: nameTok.Literal, Type: fieldType})

		p.skipNewlines()
		if p.cur.Type == token.COMMA {
			p.advance()
			p.skipNewlines()
		}
	}

	p.advance()
	return fields, nil
}

type fieldTypeDepth struct {
	paren int
	brack int
	brace int
}

func (d fieldTypeDepth) empty() bool {
	return d.paren == 0 && d.brack == 0 && d.brace == 0
}

func (p *Parser) parseFieldType() (string, error) {
	p.skipNewlines()
	if p.cur.Type == token.RBRACE || p.cur.Type == token.EOF || p.cur.Type == token.COMMA {
		return "", p.errorf("expected field type")
	}

	start := p.curStart
	depth := fieldTypeDepth{}
	prev := token.Token{}

	for {
		switch p.cur.Type {
		case token.EOF:
			return "", p.errorf("unterminated field type")
		case token.NEWLINE:
			if depth.empty() && !p.fieldTypeCanContinue(prev.Type) {
				typeText := strings.TrimSpace(string(p.source[start:p.curStart]))
				if typeText == "" {
					return "", p.errorf("expected field type")
				}
				return typeText, nil
			}
			p.advance()
			prev = token.Token{Type: token.NEWLINE}
			continue
		case token.COMMA:
			if depth.empty() {
				typeText := strings.TrimSpace(string(p.source[start:p.curStart]))
				if typeText == "" {
					return "", p.errorf("expected field type")
				}
				return typeText, nil
			}
		case token.RBRACE:
			if depth.empty() {
				typeText := strings.TrimSpace(string(p.source[start:p.curStart]))
				if typeText == "" {
					return "", p.errorf("expected field type")
				}
				return typeText, nil
			}
		case token.IDENT:
			if depth.empty() && prev.Type != "" && !p.fieldTypeCanContinue(prev.Type) && prev.Line == p.cur.Line {
				typeText := strings.TrimSpace(string(p.source[start:p.curStart]))
				if typeText == "" {
					return "", p.errorf("expected field type")
				}
				return typeText, nil
			}
		}

		switch p.cur.Type {
		case token.LPAREN:
			depth.paren++
		case token.RPAREN:
			if depth.paren > 0 {
				depth.paren--
			}
		case token.LBRACK:
			depth.brack++
		case token.RBRACK:
			if depth.brack > 0 {
				depth.brack--
			}
		case token.LBRACE:
			depth.brace++
		case token.RBRACE:
			if depth.brace > 0 {
				depth.brace--
			}
		}

		prev = p.cur
		p.advance()
	}
}

func (p *Parser) fieldTypeCanContinue(tokType token.Type) bool {
	switch tokType {
	case token.DOT, token.STAR, token.LBRACK, token.LPAREN, token.RPAREN, token.RBRACK, token.LBRACE:
		return true
	}
	return false
}

func (p *Parser) skipNewlines() {
	for p.cur.Type == token.NEWLINE {
		p.advance()
	}
}

func (p *Parser) isEnumDeclaration() bool {
	return p.cur.Type == token.TYPE && p.peek.Type == token.IDENT && p.peek2.Type == token.ENUM
}

func (p *Parser) expect(expected token.Type, message string) error {
	if p.cur.Type != expected {
		return p.errorAt(p.cur, message)
	}
	p.advance()
	return nil
}

func (p *Parser) consume(expected token.Type, message string) (token.Token, error) {
	if p.cur.Type != expected {
		return token.Token{}, p.errorAt(p.cur, message)
	}
	tok := p.cur
	p.advance()
	return tok, nil
}

func (p *Parser) match(expected token.Type) bool {
	if p.cur.Type != expected {
		return false
	}
	p.advance()
	return true
}

func (p *Parser) errorf(message string) error {
	return p.errorAt(p.cur, message)
}

func (p *Parser) errorAt(tok token.Token, message string) error {
	if tok.Type == token.EOF {
		return fmt.Errorf("parse error at end of file: %s", message)
	}
	return fmt.Errorf("parse error at line %d, column %d: %s (got %q)", tok.Line, tok.Column, message, tok.Literal)
}

func (p *Parser) advance() {
	p.cur = p.peek
	p.curStart = p.peekStart

	p.peek = p.peek2
	p.peekStart = p.peek2Start

	p.peek2, p.peek2Start, _ = p.lexer.NextTokenWithRange()
}

func (p *Parser) trackBraces() {
	switch p.cur.Type {
	case token.LBRACE:
		p.parenDepth++
	case token.RBRACE:
		if p.parenDepth > 0 {
			p.parenDepth--
		}
	}
}
