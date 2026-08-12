package syntax

import "caja-cli/internal/lexer"

const (
	_ int = iota
	LOWEST
	ASSIGN
	LOGICAL
	COMPARISON
	SUM
	PRODUCT
	EXPONENT
	PREFIX
	CALL
	INDEX
)

var precedences = map[lexer.TokenType]int{
	lexer.ASSIGN:   ASSIGN,
	lexer.LT:       COMPARISON,
	lexer.GT:       COMPARISON,
	lexer.LTEQ:     COMPARISON,
	lexer.GTEQ:     COMPARISON,
	lexer.EQ:       COMPARISON,
	lexer.NEQ:      COMPARISON,
	lexer.PLUS:     SUM,
	lexer.MINUS:    SUM,
	lexer.ASTERISK: PRODUCT,
	lexer.SLASH:    PRODUCT,
	lexer.MODULO:   PRODUCT,
	lexer.POWER:    EXPONENT,
	lexer.LPAREN:   CALL,
	lexer.LBRACE:   CALL,
	lexer.LBRACKET:    INDEX,
	lexer.DOT:         INDEX,
	lexer.QUESTIONDOT: INDEX,
	lexer.AND:         LOGICAL,
	lexer.OR:       LOGICAL,
	lexer.XOR:      LOGICAL,
}

// verifyPrecedenceLevel returns the precedence level associated with the given
// token type. If the token type has no registered precedence, LOWEST is returned.
func verifyPrecedenceLevel(t lexer.TokenType) int {
	if p, ok := precedences[t]; ok {
		return p
	}

	return LOWEST
}
