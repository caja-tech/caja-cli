package parser

import (
	"caja-cli/internal/ast"
	"caja-cli/internal/lexer"
)

const (
	_ int = iota
	LOWEST
	ASSIGN
	SUM
	PRODUCT
)

var precedences = map[lexer.TokenType]int{
	lexer.ASSIGN:   ASSIGN,
	lexer.PLUS:     SUM,
	lexer.MINUS:    SUM,
	lexer.ASTERISK: PRODUCT,
	lexer.SLASH:    PRODUCT,
}

type prefixParseFunc func() ast.Expression
type infixParseFunc func(ast.Expression) ast.Expression

type Parser struct {
	tknzr lexer.Tokenizer

	currToken lexer.Token
	peekToken lexer.Token

	prefixParseFuncs map[lexer.TokenType]prefixParseFunc
	infixParseFuncs  map[lexer.TokenType]infixParseFunc
}
