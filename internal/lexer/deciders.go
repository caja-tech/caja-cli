package lexer

// tokenDeciderFunc is the signature shared by all token-decision functions.
// Each function inspects the Tokenizer's current character and returns true
// together with a fully populated Token when it recognizes the character, or
// false with a NONE token otherwise.
type tokenDeciderFunc func(*Tokenizer, int, int) deciderResult

type deciderResult struct {
	matched bool
	token   Token
}

var tokenDeciders = []tokenDeciderFunc{
	decideAssignToken,
	decidePlusToken,
	decideMinusToken,
	decideAsteriskToken,
	decideSlashToken,
	decidePowerToken,
	decideModuloToken,
	decideLessThanToken,
	decideGreaterThanToken,
	decideNotEqualToken,
	decideLeftParenToken,
	decideRightParenToken,
	decideLeftBraceToken,
	decideRightBraceToken,
	decideEOFToken,
	decideLetterToken,
	decideDigitToken,
	decideCommaToken,
	decideColonToken,
	decideStringToken,
}

// decideAssignToken matches the '=' character and produces an ASSIGN token.
func decideAssignToken(t *Tokenizer, line int, column int) deciderResult {
	if t.ch == '=' {
		if t.peekChar() == '=' {
			ch := t.ch
			t.readChar()
			literal := string(ch) + string(t.ch)
			t.readChar()
			return deciderResult{true, Token{Type: EQ, Literal: literal, Line: line, Column: column}}
		}

		return deciderResult{true, Token{Type: ASSIGN, Literal: string(t.ch), Line: line, Column: column}}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}}
}

// decidePlusToken matches the '+' character and produces a PLUS token.
func decidePlusToken(t *Tokenizer, line int, column int) deciderResult {
	if t.ch == '+' {
		return deciderResult{true, Token{Type: PLUS, Literal: string(t.ch), Line: line, Column: column}}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}}
}

// decideMinusToken matches the '-' character and produces a MINUS token.
func decideMinusToken(t *Tokenizer, line int, column int) deciderResult {
	if t.ch == '-' {
		return deciderResult{true, Token{Type: MINUS, Literal: string(t.ch), Line: line, Column: column}}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}}
}

// decideAsteriskToken matches the '*' character and produces an ASTERISK token.
func decideAsteriskToken(t *Tokenizer, line int, column int) deciderResult {
	if t.ch == '*' {
		return deciderResult{true, Token{Type: ASTERISK, Literal: string(t.ch), Line: line, Column: column}}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}}
}

// decideSlashToken matches the '/' character and produces a SLASH token.
func decideSlashToken(t *Tokenizer, line int, column int) deciderResult {
	if t.ch == '/' {
		return deciderResult{true, Token{Type: SLASH, Literal: string(t.ch), Line: line, Column: column}}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}}
}

// decidePowerToken matches the '^' character and produces a POWER token.
func decidePowerToken(t *Tokenizer, line int, column int) deciderResult {
	if t.ch == '^' {
		return deciderResult{true, Token{Type: POWER, Literal: string(t.ch), Line: line, Column: column}}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}}
}

// decideModuloToken matches the '%' character and produces a MODULO token.
func decideModuloToken(t *Tokenizer, line int, column int) deciderResult {
	if t.ch == '%' {
		return deciderResult{true, Token{Type: MODULO, Literal: string(t.ch), Line: line, Column: column}}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}}
}

// decideLessThanToken matches the '<' character and produces an LT token,
// or an LTEQ token if it is followed by an '=' character.
func decideLessThanToken(t *Tokenizer, line int, column int) deciderResult {
	if t.ch == '<' {
		if t.peekChar() == '=' {
			ch := t.ch
			t.readChar()
			literal := string(ch) + string(t.ch)
			t.readChar()
			return deciderResult{true, Token{Type: LTEQ, Literal: literal, Line: line, Column: column}}
		}

		return deciderResult{true, Token{Type: LT, Literal: string(t.ch), Line: line, Column: column}}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}}
}

// decideGreaterThanToken matches the '>' character and produces a GT token,
// or a GTEQ token if it is followed by an '=' character.
func decideGreaterThanToken(t *Tokenizer, line int, column int) deciderResult {
	if t.ch == '>' {
		if t.peekChar() == '=' {
			ch := t.ch
			t.readChar()
			literal := string(ch) + string(t.ch)
			t.readChar()
			return deciderResult{true, Token{Type: GTEQ, Literal: literal, Line: line, Column: column}}
		}
		return deciderResult{true, Token{Type: GT, Literal: string(t.ch), Line: line, Column: column}}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}}
}

// decideNotEqualToken matches the '!' character and produces an NEQ token
// if it is followed by an '=' character.
func decideNotEqualToken(t *Tokenizer, line int, column int) deciderResult {
	if t.ch == '!' {
		if t.peekChar() == '=' {
			ch := t.ch
			t.readChar()
			literal := string(ch) + string(t.ch)
			t.readChar()
			return deciderResult{true, Token{Type: NEQ, Literal: literal, Line: line, Column: column}}
		}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}}
}

// decideLeftParenToken matches the '(' character and produces an LPAREN token.
func decideLeftParenToken(t *Tokenizer, line int, column int) deciderResult {
	if t.ch == '(' {
		return deciderResult{true, Token{Type: LPAREN, Literal: string(t.ch), Line: line, Column: column}}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}}
}

// decideRightParenToken matches the ')' character and produces an RPAREN token.
func decideRightParenToken(t *Tokenizer, line int, column int) deciderResult {
	if t.ch == ')' {
		return deciderResult{true, Token{Type: RPAREN, Literal: string(t.ch), Line: line, Column: column}}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}}
}

// decideLeftBraceToken matches the '{' character and produces an LBRACE token.
func decideLeftBraceToken(t *Tokenizer, line int, column int) deciderResult {
	if t.ch == '{' {
		return deciderResult{true, Token{Type: LBRACE, Literal: string(t.ch), Line: line, Column: column}}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}}
}

// decideRightBraceToken matches the '}' character and produces an RBRACE token.
func decideRightBraceToken(t *Tokenizer, line int, column int) deciderResult {
	if t.ch == '}' {
		return deciderResult{true, Token{Type: RBRACE, Literal: string(t.ch), Line: line, Column: column}}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}}
}

// decideEOFToken matches the NUL byte (0) that signals end-of-input and
// produces an EOF token.
func decideEOFToken(t *Tokenizer, line int, column int) deciderResult {
	if t.ch == 0 {
		return deciderResult{true, Token{Type: EOF, Literal: "", Line: line, Column: column}}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}}
}

// decideLetterToken matches any letter or underscore, reads the full
// identifier via readIdentifier, and produces an IDENT token or a KEYWORD token.
func decideLetterToken(t *Tokenizer, line int, column int) deciderResult {
	if t.isCurrentCharALetter() {
		identifier := t.readIdentifier()
		tokenType := lookupIdent(identifier)
		token := Token{Type: tokenType, Literal: identifier, Line: line, Column: column}
		return deciderResult{true, token}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}}
}

// decideDigitToken matches any digit, reads the full numeric literal via
// readNumber, and produces a NUMBER token.
func decideDigitToken(t *Tokenizer, line int, column int) deciderResult {
	if t.isCurrentCharADigit() {
		number := t.readNumber()
		token := Token{Type: NUMBER, Literal: number, Line: line, Column: column}
		return deciderResult{true, token}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}}
}

func decideCommaToken(t *Tokenizer, line int, column int) deciderResult {
	if t.ch == ',' {
		return deciderResult{true, Token{Type: COMMA, Literal: string(t.ch), Line: line, Column: column}}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}}
}

func decideColonToken(t *Tokenizer, line int, column int) deciderResult {
	if t.ch == ':' {
		return deciderResult{true, Token{Type: COLON, Literal: string(t.ch), Line: line, Column: column}}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}}
}

func decideStringToken(t *Tokenizer, line int, column int) deciderResult {
	if t.ch == '"' {
		return deciderResult{true, Token{Type: STRING, Literal: t.readString(), Line: line, Column: column}}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}}
}
