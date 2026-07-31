package lexer

// tokenDeciderFunc is the signature shared by all token-decision functions.
// Each function inspects the Tokenizer's current character and returns true
// together with a fully populated Token when it recognises the character, or
// false with a NONE token otherwise.
type tokenDeciderFunc func(*Tokenizer, int, int) deciderResult

type deciderResult struct {
	matched     bool
	token       Token
	hasReadChar bool
}

var tokenDeciders = []tokenDeciderFunc{
	decideAssignToken,
	decidePlusToken,
	decideMinusToken,
	decideAsteriskToken,
	decideSlashToken,
	decideLeftParenToken,
	decideRightParenToken,
	decideEOFToken,
	decideLetterToken,
	decideDigitToken,
}

// decideAssignToken matches the '=' character and produces an ASSIGN token.
func decideAssignToken(t *Tokenizer, line int, column int) deciderResult {
	if t.ch == '=' {
		return deciderResult{true, Token{Type: ASSIGN, Literal: string(t.ch), Line: line, Column: column}, false}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}, false}
}

// decidePlusToken matches the '+' character and produces a PLUS token.
func decidePlusToken(t *Tokenizer, line int, column int) deciderResult {
	if t.ch == '+' {
		return deciderResult{true, Token{Type: PLUS, Literal: string(t.ch), Line: line, Column: column}, false}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}, false}
}

// decideMinusToken matches the '-' character and produces a MINUS token.
func decideMinusToken(t *Tokenizer, line int, column int) deciderResult {
	if t.ch == '-' {
		return deciderResult{true, Token{Type: MINUS, Literal: string(t.ch), Line: line, Column: column}, false}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}, false}
}

// decideAsteriskToken matches the '*' character and produces an ASTERISK token.
func decideAsteriskToken(t *Tokenizer, line int, column int) deciderResult {
	if t.ch == '*' {
		return deciderResult{true, Token{Type: ASTERISK, Literal: string(t.ch), Line: line, Column: column}, false}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}, false}
}

// decideSlashToken matches the '/' character and produces a SLASH token.
func decideSlashToken(t *Tokenizer, line int, column int) deciderResult {
	if t.ch == '/' {
		return deciderResult{true, Token{Type: SLASH, Literal: string(t.ch), Line: line, Column: column}, false}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}, false}
}

// decideLeftParenToken matches the '(' character and produces an LPAREN token.
func decideLeftParenToken(t *Tokenizer, line int, column int) deciderResult {
	if t.ch == '(' {
		return deciderResult{true, Token{Type: LPAREN, Literal: string(t.ch), Line: line, Column: column}, false}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}, false}
}

// decideRightParenToken matches the ')' character and produces an RPAREN token.
func decideRightParenToken(t *Tokenizer, line int, column int) deciderResult {
	if t.ch == ')' {
		return deciderResult{true, Token{Type: RPAREN, Literal: string(t.ch), Line: line, Column: column}, false}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}, false}
}

// decideEOFToken matches the NUL byte (0) that signals end-of-input and
// produces an EOF token.
func decideEOFToken(t *Tokenizer, line int, column int) deciderResult {
	if t.ch == 0 {
		return deciderResult{true, Token{Type: EOF, Literal: "", Line: line, Column: column}, false}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}, false}
}

// decideLetterToken matches any letter or underscore, reads the full
// identifier via readIdentifier, and produces an IDENT token.
func decideLetterToken(t *Tokenizer, line int, column int) deciderResult {
	if t.isCurrentCharALetter() {
		identifier := t.readIdentifier()
		token := Token{Type: IDENT, Literal: identifier, Line: line, Column: column}
		return deciderResult{true, token, true}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}, false}
}

// decideDigitToken matches any digit, reads the full numeric literal via
// readNumber, and produces a NUMBER token.
func decideDigitToken(t *Tokenizer, line int, column int) deciderResult {
	if t.isCurrentCharADigit() {
		number := t.readNumber()
		token := Token{Type: NUMBER, Literal: number, Line: line, Column: column}
		return deciderResult{true, token, true}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}, false}
}
