package parser

// File is a parsed Rune file.
type File struct {
	PackageName string
	Pieces      []Piece
	Enums       []Enum
}

type PieceKind int

const (
	PieceGo PieceKind = iota
	PieceEnum
)

type Piece struct {
	Kind PieceKind
	Text string
	Enum *Enum
}

// Enum represents a named enum declaration.
type Enum struct {
	Name     string
	Variants []Variant
}

// Variant is one enum case, optionally with fields.
type Variant struct {
	Name   string
	Fields []Field
}

// Field is an enum variant field.
type Field struct {
	Name string
	Type string
}
