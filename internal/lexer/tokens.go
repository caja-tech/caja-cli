package lexer

type TokenType string

const (
	NONE    TokenType = "NONE"
	ILLEGAL TokenType = "ILLEGAL"
	EOF     TokenType = "EOF"

	// Identifiers and Literals
	IDENT  TokenType = "IDENT"
	NUMBER TokenType = "NUMBER"

	// Operators
	ASSIGN   TokenType = "ASSIGN"
	PLUS     TokenType = "PLUS"
	MINUS    TokenType = "MINUS"
	ASTERISK TokenType = "ASTERISK"
	SLASH    TokenType = "SLASH"

	// Delimiters
	LPAREN TokenType = "LPAREN"
	RPAREN TokenType = "RPAREN"
)

type Token struct {
	Type    TokenType
	Literal string
	Line    int
	Column  int
}
