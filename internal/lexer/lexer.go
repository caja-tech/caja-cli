// Package lexer implements a handwritten lexer (tokenizer) for the Caja
// financial DSL. It converts raw source-code text into a flat sequence of
// tokens that can be consumed by a parser. Each token carries its type,
// literal text, and the source position (line and column) where it was found.
package lexer

import "fmt"

type TokenType string

const (
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

// Lex tokenizes the entire input string and returns two slices: the ordered
// sequence of tokens (always terminated by an EOF token) and a list of
// formatted syntax-error messages for any unrecognized characters encountered
// during scanning. The caller should check the error slice before using the
// token slice.
func Lex(input string) ([]Token, []string) {
	t := newTokenizer(input)

	var tokens []Token
	for {
		token := t.nextToken()
		tokens = append(tokens, token)

		if token.Type == EOF {
			break
		}
	}

	return tokens, t.errors
}

type tokenizer struct {
	input        string   // source code input
	position     int      // current position in input (points to current char)
	readPosition int      // current reading position in input (after current char)
	ch           byte     // current char under examination
	line         int      // current line number
	column       int      // current column number
	errors       []string // the list of formatted error messages
}

// newTokenizer creates a tokenizer for the given input string, priming it by
// reading the first character so that the first call to nextToken is ready to
// produce a token without any extra setup.
func newTokenizer(input string) *tokenizer {
	t := &tokenizer{input: input, line: 1, column: 0}
	t.readChar()

	return t
}

// readChar advances the tokenizer by one byte. If the character that was just
// consumed was a newline, the line counter is incremented and the column counter
// is reset so that the next character begins at column 1. When the end of the
// input is reached, ch is set to 0 (NUL) to signal EOF.
func (t *tokenizer) readChar() {
	if t.ch == '\n' {
		t.line++
		t.column = 0
	}

	if t.readPosition >= len(t.input) {
		t.ch = 0 // ASCII code for "NUL", representing EOF
	} else {
		t.ch = t.input[t.readPosition]
	}
	t.position = t.readPosition
	t.readPosition++
	t.column++
}

// skipWhitespace advances past any combination of spaces, tabs, newlines, and
// carriage-returns, updating line and column tracking along the way.
func (t *tokenizer) skipWhitespace() {
	for t.ch == ' ' || t.ch == '\t' || t.ch == '\n' || t.ch == '\r' {
		t.readChar()
	}
}

// readIdentifier consumes the longest sequence of letter or underscore
// characters starting at the current position and returns it as a string. The
// tokenizer is left positioned at the first character that does not belong to
// the identifier.
func (t *tokenizer) readIdentifier() string {
	position := t.position
	for t.isCurrentCharALetter() {
		t.readChar()
	}
	return t.input[position:t.position]
}

// readNumber consumes a sequence of digit characters, optionally containing a
// single decimal point, and returns it as a string. The tokenizer is left
// positioned at the first character that does not belong to the number literal.
func (t *tokenizer) readNumber() string {
	position := t.position
	for t.isCurrentCharADigit() || t.ch == '.' {
		t.readChar()
	}
	return t.input[position:t.position]
}

// isCurrentCharALetter reports whether the current character is an ASCII letter
// (a–z, A–Z) or an underscore, both of which are valid identifier constituents.
func (t *tokenizer) isCurrentCharALetter() bool {
	return 'a' <= t.ch && t.ch <= 'z' || 'A' <= t.ch && t.ch <= 'Z' || t.ch == '_'
}

// isCurrentCharADigit reports whether the current character is an ASCII decimal
// digit (0–9).
func (t *tokenizer) isCurrentCharADigit() bool {
	return '0' <= t.ch && t.ch <= '9'
}

// nextToken skips any leading whitespace and then reads the next token from the
// input. Single-character operators and delimiters are matched via a switch
// statement; identifiers and numbers are handled by their respective read
// helpers, which consume the full lexeme before returning. Unrecognized
// characters produce an ILLEGAL token and append a formatted syntax-error
// message to the tokenizer's error list.
func (t *tokenizer) nextToken() Token {
	var token Token
	t.skipWhitespace()

	startLine := t.line
	startCol := t.column

	switch t.ch {
	case '=':
		token = Token{Type: ASSIGN, Literal: string(t.ch), Line: startLine, Column: startCol}
	case '+':
		token = Token{Type: PLUS, Literal: string(t.ch), Line: startLine, Column: startCol}
	case '-':
		token = Token{Type: MINUS, Literal: string(t.ch), Line: startLine, Column: startCol}
	case '*':
		token = Token{Type: ASTERISK, Literal: string(t.ch), Line: startLine, Column: startCol}
	case '/':
		token = Token{Type: SLASH, Literal: string(t.ch), Line: startLine, Column: startCol}
	case '(':
		token = Token{Type: LPAREN, Literal: string(t.ch), Line: startLine, Column: startCol}
	case ')':
		token = Token{Type: RPAREN, Literal: string(t.ch), Line: startLine, Column: startCol}
	case 0:
		token = Token{Type: EOF, Literal: "", Line: startLine, Column: startCol}
	default:
		if t.isCurrentCharALetter() {
			identifier := t.readIdentifier()
			token = Token{Type: IDENT, Literal: identifier, Line: startLine, Column: startCol}
			return token
		} else if t.isCurrentCharADigit() {
			number := t.readNumber()
			token = Token{Type: NUMBER, Literal: number, Line: startLine, Column: startCol}
			return token
		}

		msg := fmt.Sprintf("Syntax Error at line %d, column %d: unrecognized character '%c'", startLine, startCol, t.ch)
		t.errors = append(t.errors, msg)

		token = Token{Type: ILLEGAL, Literal: string(t.ch), Line: startLine, Column: startCol}
	}

	t.readChar()
	return token
}
