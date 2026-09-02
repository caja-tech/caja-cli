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
	ARROW    TokenType = "ARROW"
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
	BANG     TokenType = "BANG"
	CONST    TokenType = "CONST"

	// Delimiters
	LPAREN       TokenType = "LPAREN"
	RPAREN       TokenType = "RPAREN"
	LBRACE       TokenType = "LBRACE"
	RBRACE       TokenType = "RBRACE"
	LBRACKET     TokenType = "LBRACKET"
	RBRACKET     TokenType = "RBRACKET"
	DOT          TokenType = "DOT"
	QUESTION     TokenType = "QUESTION"
	QUESTIONDOT  TokenType = "QUESTIONDOT"
	COLON        TokenType = "COLON"
	DOUBLE_COLON TokenType = "DOUBLE_COLON"

	// Keywords
	RETURN  TokenType = "RETURN"
	IF      TokenType = "IF"
	ELSE    TokenType = "ELSE"
	LET     TokenType = "LET"
	FN      TokenType = "FN"
	COMMA   TokenType = "COMMA"
	TRUE    TokenType = "TRUE"
	FALSE   TokenType = "FALSE"
	TYPE    TokenType = "TYPE"
	IMPORT  TokenType = "IMPORT"
	AS      TokenType = "AS"
	AND     TokenType = "AND"
	OR      TokenType = "OR"
	XOR     TokenType = "XOR"
	PIPE    TokenType = "PIPE"
	SAFE_PIPE TokenType = "SAFE_PIPE"
	PRIVATE TokenType = "PRIVATE"
	STRUCT  TokenType = "STRUCT"
	DEFINE  TokenType = "DEFINE"
	CONSTRAINTS TokenType = "CONSTRAINTS"
	WITH    TokenType = "WITH"
	NIL     TokenType = "NIL"
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
	"return":  RETURN,
	"if":      IF,
	"else":    ELSE,
	"let":     LET,
	"fn":      FN,
	"true":    TRUE,
	"false":   FALSE,
	"type":    TYPE,
	"const":   CONST,
	"import":  IMPORT,
	"as":      AS,
	"and":     AND,
	"or":      OR,
	"xor":     XOR,
	"private": PRIVATE,
	"struct":  STRUCT,
	"define":  DEFINE,
	"constraints": CONSTRAINTS,
	"with":   WITH,
	"nil":     NIL,
}

// lookupIdent checks whether ident is a reserved keyword and returns the
// matching TokenType. If it is not a keyword, IDENT is returned.
func lookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}

	return IDENT
}

// isKeyword checks if the given token type is a reserved keyword in the language.
func IsKeyword(tokenType TokenType) bool {
	switch tokenType {
	case RETURN, IF, ELSE, LET, FN, TRUE, FALSE, TYPE, IMPORT, AS, AND, OR, XOR, PRIVATE, STRUCT, CONST, DEFINE, CONSTRAINTS, WITH, NIL:
		return true
	}
	return false
}
