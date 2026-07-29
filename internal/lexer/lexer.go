package lexer

import "fmt"

type TokenType string

const (
	ILLEGAL TokenType = "ILLEGAL" // Unknown character
	EOF     TokenType = "EOF"     // End of File

	// Identifiers and Literals
	IDENT  TokenType = "IDENT"  // Variables like: x, y, rate
	NUMBER TokenType = "NUMBER" // Numbers like: 5, 10, 3.14

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

func Lex(input string) ([]Token, []string) {
	t := &tokenizer{input: input, line: 1, column: 0}
	t.readChar()

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

func (t *tokenizer) skipWhitespace() {
	for t.ch == ' ' || t.ch == '\t' || t.ch == '\n' || t.ch == '\r' {
		t.readChar()
	}
}

func (t *tokenizer) readIdentifier() string {
	position := t.position
	for t.isCurrentCharALetter() {
		t.readChar()
	}
	return t.input[position:t.position]
}

func (t *tokenizer) readNumber() string {
	position := t.position
	for t.isCurrentCharADigit() || t.ch == '.' {
		t.readChar()
	}
	return t.input[position:t.position]
}

func (t *tokenizer) isCurrentCharALetter() bool {
	return 'a' <= t.ch && t.ch <= 'z' || 'A' <= t.ch && t.ch <= 'Z' || t.ch == '_'
}

func (t *tokenizer) isCurrentCharADigit() bool {
	return '0' <= t.ch && t.ch <= '9'
}

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
