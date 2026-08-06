package lexer

type TokenType string

const (
	NONE    TokenType = "NONE"
	ILLEGAL TokenType = "ILLEGAL"
	EOF     TokenType = "EOF"

	// Identifiers and Literals
	IDENT  TokenType = "IDENT"
	NUMBER TokenType = "NUMBER"
	STRING TokenType = "STRING"
	DATE   TokenType = "DATE"

	// Operators
	ASSIGN   TokenType = "ASSIGN"
	PLUS     TokenType = "PLUS"
	MINUS    TokenType = "MINUS"
	ASTERISK TokenType = "ASTERISK"
	SLASH    TokenType = "SLASH"
	POWER    TokenType = "POWER"
	MODULO   TokenType = "MODULO"
	LT       TokenType = "LT"
	GT       TokenType = "GT"
	LTEQ     TokenType = "LTEQ"
	GTEQ     TokenType = "GTEQ"
	EQ       TokenType = "EQ"
	NEQ      TokenType = "NEQ"

	// Delimiters
	LPAREN TokenType = "LPAREN"
	RPAREN TokenType = "RPAREN"
	LBRACE TokenType = "LBRACE"
	RBRACE TokenType = "RBRACE"

	// Keywords
	RETURN TokenType = "RETURN"
	IF     TokenType = "IF"
	ELSE   TokenType = "ELSE"
	LET    TokenType = "LET"
	FN     TokenType = "FN"
	COMMA  TokenType = "COMMA"
	COLON  TokenType = "COLON"
	TRUE   TokenType = "TRUE"
	FALSE  TokenType = "FALSE"
	TYPE   TokenType = "TYPE"
)

type Token struct {
	Type    TokenType
	Literal string
	Line    int
	Column  int
}

// keywords maps reserved word literals to their corresponding TokenType.
// Any identifier matching a keyword is emitted as that keyword token instead
// of a generic IDENT token.
var keywords = map[string]TokenType{
	"return": RETURN,
	"if":     IF,
	"else":   ELSE,
	"let":    LET,
	"fn":     FN,
	"true":   TRUE,
	"false":  FALSE,
	"type":   TYPE,
}

// lookupIdent checks whether ident is a reserved keyword and returns the
// matching TokenType. If it is not a keyword, IDENT is returned.
func lookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}

	return IDENT
}
