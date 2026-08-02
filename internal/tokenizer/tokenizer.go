package tokenizer

import "fmt"

type ErrorTracker struct {
	Line   int      // current line number
	Column int      // current column number
	Errors []string // the list of formatted error messages
}

type Tokenizer struct {
	input        string // source code input
	position     int    // current position in input (points to current char)
	readPosition int    // current reading position in input (after current char)
	ch           rune   // current char under examination
	ErrorTracker
}

// newTokenizer creates a Tokenizer for the given input string, priming it by
// reading the first character so that the first call to nextToken is ready to
// produce a token without any extra setup.
func New(input string) *Tokenizer {
	t := &Tokenizer{input: input, ErrorTracker: ErrorTracker{Line: 1, Column: 0}}
	t.readChar()

	return t
}

// nextToken skips any leading whitespace and then reads the next token from the
// input. Single-character operators and delimiters are matched via a switch
// statement; identifiers and numbers are handled by their respective read
// helpers, which consume the full lexeme before returning. Unrecognized
// characters produce an ILLEGAL token and append a formatted syntax-error
// message to the Tokenizer's error list.
func (t *Tokenizer) NextToken() Token {
	var token Token
	t.skipWhitespace()

	startLine := t.Line
	startCol := t.Column

	hasReadChar := false
	foundDecider := false
	for _, decider := range tokenDeciders {
		lastReadPosition := t.readPosition
		result := decider(t, startLine, startCol)
		hasReadChar = lastReadPosition != t.readPosition

		if result.matched {
			token = result.token
			foundDecider = true
			break
		}
	}

	if !foundDecider {
		msg := fmt.Sprintf("Syntax Error at line %d, column %d: unrecognized character '%c'", startLine, startCol, t.ch)
		t.Errors = append(t.Errors, msg)
		token = Token{Type: ILLEGAL, Literal: string(t.ch), Line: startLine, Column: startCol}
	}

	if !hasReadChar {
		t.readChar()
	}
	return token
}

// readChar advances the Tokenizer by one byte. If the character that was just
// consumed was a newline, the line counter is incremented and the column counter
// is reset so that the next character begins at column 1. When the end of the
// input is reached, ch is set to 0 (NUL) to signal EOF.
func (t *Tokenizer) readChar() {
	if t.isCurrentCharANewLine() {
		t.Line++
		t.Column = 0
	}

	t.ch = 0 // ASCII code for "NUL", representing EOF
	if !t.hasFinalizedReading() {
		t.ch = rune(t.input[t.readPosition])
	}

	t.position = t.readPosition
	t.readPosition++
	t.Column++
}

// skipWhitespace advances past any combination of spaces, tabs, newlines, and
// carriage-returns, updating line and column tracking along the way.
func (t *Tokenizer) skipWhitespace() {
	for t.ch == ' ' || t.ch == '\t' || t.ch == '\n' || t.ch == '\r' {
		t.readChar()
	}
}

// readIdentifier consumes the longest sequence of letter or underscore
// characters starting at the current position and returns it as a string. The
// Tokenizer is left positioned at the first character that does not belong to
// the identifier.
func (t *Tokenizer) readIdentifier() string {
	position := t.position
	for t.isCurrentCharALetter() {
		t.readChar()
	}
	return t.input[position:t.position]
}

// readNumber consumes a sequence of digit characters, optionally containing a
// single decimal point, and returns it as a string. The Tokenizer is left
// positioned at the first character that does not belong to the number literal.
func (t *Tokenizer) readNumber() string {
	position := t.position
	for t.isCurrentCharADigit() || t.ch == '.' {
		t.readChar()
	}
	return t.input[position:t.position]
}

// isCurrentCharALetter reports whether the current character is an ASCII letter
// (a–z, A–Z) or an underscore, both of which are valid identifier constituents.
func (t *Tokenizer) isCurrentCharALetter() bool {
	return 'a' <= t.ch && t.ch <= 'z' || 'A' <= t.ch && t.ch <= 'Z' || t.ch == '_'
}

// isCurrentCharADigit reports whether the current character is an ASCII decimal
// digit (0–9).
func (t *Tokenizer) isCurrentCharADigit() bool {
	return '0' <= t.ch && t.ch <= '9'
}

// isCurrentCharANewLine reports whether the current character represents a
// newline. It returns true for '\n' and for a standalone '\r' that is not
// followed by '\n', so that a "\r\n" pair is counted as a single line break.
func (t *Tokenizer) isCurrentCharANewLine() bool {
	return t.ch == '\n' || (t.ch == '\r' && (t.hasFinalizedReading() || t.input[t.readPosition] != '\n'))
}

// hasFinalizedReading reports whether the Tokenizer has consumed all bytes in
// the input, meaning the next readChar call would produce EOF.
func (t *Tokenizer) hasFinalizedReading() bool {
	return t.readPosition >= len(t.input)
}
