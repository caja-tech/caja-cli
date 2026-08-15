package lexer

// tokenDeciderFunc is the signature shared by all token-decision functions.
// Each function inspects the Lexer's current character and returns true
// together with a fully populated Token when it recognizes the character, or
// false with a NONE token otherwise.
type tokenDeciderFunc func(*Lexer, int, int) deciderResult

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
	decideBangToken,
	decideLeftParenToken,
	decideRightParenToken,
	decideLeftBraceToken,
	decideRightBraceToken,
	decideLeftBracketToken,
	decideRightBracketToken,
	decideEOFToken,
	decideLetterToken,
	decideDigitToken,
	decideCommaToken,
	decideColonToken,
	decideStringToken,
	decideDateToken,
	decideDotToken,
	decideQuestionToken,
}

// decideAssignToken matches the '=' character and produces an ASSIGN token.
func decideAssignToken(t *Lexer, line int, column int) deciderResult {
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
func decidePlusToken(t *Lexer, line int, column int) deciderResult {
	if t.ch == '+' {
		return deciderResult{true, Token{Type: PLUS, Literal: string(t.ch), Line: line, Column: column}}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}}
}

// decideMinusToken matches the '-' character and produces a MINUS token, or '->' for an ARROW token.
func decideMinusToken(t *Lexer, line int, column int) deciderResult {
	if t.ch == '-' {
		if t.peekChar() == '>' {
			ch := t.ch
			t.readChar()
			literal := string(ch) + string(t.ch)
			t.readChar()
			return deciderResult{true, Token{Type: ARROW, Literal: literal, Line: line, Column: column}}
		}
		return deciderResult{true, Token{Type: MINUS, Literal: string(t.ch), Line: line, Column: column}}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}}
}

// decideAsteriskToken matches the '*' character and produces an ASTERISK token.
func decideAsteriskToken(t *Lexer, line int, column int) deciderResult {
	if t.ch == '*' {
		return deciderResult{true, Token{Type: ASTERISK, Literal: string(t.ch), Line: line, Column: column}}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}}
}

// decideSlashToken matches the '/' character and produces a SLASH token.
func decideSlashToken(t *Lexer, line int, column int) deciderResult {
	if t.ch == '/' {
		return deciderResult{true, Token{Type: SLASH, Literal: string(t.ch), Line: line, Column: column}}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}}
}

// decidePowerToken matches the '^' character and produces a POWER token.
func decidePowerToken(t *Lexer, line int, column int) deciderResult {
	if t.ch == '^' {
		return deciderResult{true, Token{Type: POWER, Literal: string(t.ch), Line: line, Column: column}}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}}
}

// decideModuloToken matches the '%' character and produces a MODULO token.
func decideModuloToken(t *Lexer, line int, column int) deciderResult {
	if t.ch == '%' {
		return deciderResult{true, Token{Type: MODULO, Literal: string(t.ch), Line: line, Column: column}}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}}
}

// decideLessThanToken matches the '<' character and produces an LT token,
// or an LTEQ token if it is followed by an '=' character.
func decideLessThanToken(t *Lexer, line int, column int) deciderResult {
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
func decideGreaterThanToken(t *Lexer, line int, column int) deciderResult {
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

// decideBangToken matches the '!' character and produces an NEQ token
// if it is followed by an '=' character, otherwise it produces a BANG token.
func decideBangToken(t *Lexer, line int, column int) deciderResult {
	if t.ch == '!' {
		if t.peekChar() == '=' {
			ch := t.ch
			t.readChar()
			literal := string(ch) + string(t.ch)
			t.readChar()
			return deciderResult{true, Token{Type: NEQ, Literal: literal, Line: line, Column: column}}
		}
		t.readChar()
		return deciderResult{true, Token{Type: BANG, Literal: "!", Line: line, Column: column}}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}}
}

// decideLeftParenToken matches the '(' character and produces an LPAREN token.
func decideLeftParenToken(t *Lexer, line int, column int) deciderResult {
	if t.ch == '(' {
		return deciderResult{true, Token{Type: LPAREN, Literal: string(t.ch), Line: line, Column: column}}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}}
}

// decideRightParenToken matches the ')' character and produces an RPAREN token.
func decideRightParenToken(t *Lexer, line int, column int) deciderResult {
	if t.ch == ')' {
		return deciderResult{true, Token{Type: RPAREN, Literal: string(t.ch), Line: line, Column: column}}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}}
}

// decideLeftBraceToken matches the '{' character and produces an LBRACE token.
func decideLeftBraceToken(t *Lexer, line int, column int) deciderResult {
	if t.ch == '{' {
		return deciderResult{true, Token{Type: LBRACE, Literal: string(t.ch), Line: line, Column: column}}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}}
}

// decideRightBraceToken matches the '}' character and produces an RBRACE token.
func decideRightBraceToken(t *Lexer, line int, column int) deciderResult {
	if t.ch == '}' {
		return deciderResult{true, Token{Type: RBRACE, Literal: string(t.ch), Line: line, Column: column}}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}}
}

// decideLeftBracketToken matches the '[' character and produces an LBRACKET token.
func decideLeftBracketToken(t *Lexer, line int, column int) deciderResult {
	if t.ch == '[' {
		return deciderResult{true, Token{Type: LBRACKET, Literal: string(t.ch), Line: line, Column: column}}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}}
}

// decideRightBracketToken matches the ']' character and produces an RBRACKET token.
func decideRightBracketToken(t *Lexer, line int, column int) deciderResult {
	if t.ch == ']' {
		return deciderResult{true, Token{Type: RBRACKET, Literal: string(t.ch), Line: line, Column: column}}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}}
}

// decideEOFToken matches the NUL byte (0) that signals end-of-input and
// produces an EOF token.
func decideEOFToken(t *Lexer, line int, column int) deciderResult {
	if t.ch == 0 {
		return deciderResult{true, Token{Type: EOF, Literal: "", Line: line, Column: column}}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}}
}

// decideLetterToken matches any letter or underscore, reads the full
// identifier via readIdentifier, and produces an IDENT token or a KEYWORD token.
func decideLetterToken(t *Lexer, line int, column int) deciderResult {
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
func decideDigitToken(t *Lexer, line int, column int) deciderResult {
	if t.isCurrentCharADigit() {
		number := t.readNumber()
		token := Token{Type: NUMBER, Literal: number, Line: line, Column: column}
		return deciderResult{true, token}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}}
}

// decideCommaToken matches the ',' character and produces a COMMA token.
func decideCommaToken(t *Lexer, line int, column int) deciderResult {
	if t.ch == ',' {
		return deciderResult{true, Token{Type: COMMA, Literal: string(t.ch), Line: line, Column: column}}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}}
}

// decideColonToken matches the ':' character and produces a COLON token.
func decideColonToken(t *Lexer, line int, column int) deciderResult {
	if t.ch == ':' {
		return deciderResult{true, Token{Type: COLON, Literal: string(t.ch), Line: line, Column: column}}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}}
}

// decideStringToken matches the " character, reads the full string literal,
// and produces a STRING token.
func decideStringToken(t *Lexer, line int, column int) deciderResult {
	if t.ch == '"' {
		return deciderResult{true, Token{Type: STRING, Literal: t.readString(), Line: line, Column: column}}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}}
}

// decideDateToken matches the '\ character, reads the full date literal,
// and produces a DATE token.
func decideDateToken(t *Lexer, line int, column int) deciderResult {
	if t.ch == '\'' {
		return deciderResult{true, Token{Type: DATE, Literal: t.readDate(), Line: line, Column: column}}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}}
}

// decideDotToken matches the '.' character and produces a DOT token.
func decideDotToken(t *Lexer, line int, column int) deciderResult {
	if t.ch == '.' {
		return deciderResult{true, Token{Type: DOT, Literal: string(t.ch), Line: line, Column: column}}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}}
}

// decideQuestionToken matches the '?' character and produces a QUESTION token,
// or a QUESTIONDOT token if it is followed by a '.' character.
func decideQuestionToken(t *Lexer, line int, column int) deciderResult {
	if t.ch == '?' {
		if t.peekChar() == '.' {
			ch := t.ch
			t.readChar()
			literal := string(ch) + string(t.ch)
			t.readChar()
			return deciderResult{true, Token{Type: QUESTIONDOT, Literal: literal, Line: line, Column: column}}
		}
		return deciderResult{true, Token{Type: QUESTION, Literal: string(t.ch), Line: line, Column: column}}
	}

	return deciderResult{false, Token{Type: NONE, Literal: string(t.ch), Line: line, Column: column}}
}
