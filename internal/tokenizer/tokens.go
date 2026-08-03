package tokenizer

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
	POWER    TokenType = "POWER"
	MODULO   TokenType = "MODULO"

	// Delimiters
	LPAREN TokenType = "LPAREN"
	RPAREN TokenType = "RPAREN"

	// Keywords
	RETURN TokenType = "RETURN"
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
}

// LookupIdent checks whether ident is a reserved keyword and returns the
// matching TokenType. If it is not a keyword, IDENT is returned.
func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}

	return IDENT
}
