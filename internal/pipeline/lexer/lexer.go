package lexer

import "fmt"

// Lex tokenizes the entire input string and returns two slices: the ordered
// sequence of tokens (always terminated by an EOF token) and a list of
// formatted syntax-error messages for any unrecognized characters encountered
// during scanning. The caller should check the error slice before using the
// token slice.
func Lex(input string) ([]Token, []string) {
	t := New(input)
	var tokens []Token

	for {
		token := t.NextToken()
		tokens = append(tokens, token)

		if token.Type == EOF {
			break
		}
	}

	return tokens, t.Errors
}

type ErrorTracker struct {
	Line   int      // current line number
	Column int      // current column number
	Errors []string // the list of formatted error messages
}

type Lexer struct {
	input        string // source code input
	position     int    // current position in input (points to current char)
	readPosition int    // current reading position in input (after current char)
	ch           rune   // current char under examination
	ErrorTracker
}

// newTokenizer creates a Lexer for the given input string, priming it by
// reading the first character so that the first call to nextToken is ready to
// produce a token without any extra setup.
func New(input string) *Lexer {
	t := &Lexer{input: input, ErrorTracker: ErrorTracker{Line: 1, Column: 0}}
	t.readChar()

	return t
}

// nextToken skips any leading whitespace and then reads the next token from the
// input. Single-character operators and delimiters are matched via a switch
// statement; identifiers and numbers are handled by their respective read
// helpers, which consume the full lexeme before returning. Unrecognized
// characters produce an ILLEGAL token and append a formatted syntax-error
// message to the Lexer's error list.
func (l *Lexer) NextToken() Token {
	var token Token
	l.skipWhitespace()

	startLine := l.Line
	startCol := l.Column

	hasReadChar := false
	foundDecider := false
	for _, decider := range tokenDeciders {
		lastReadPosition := l.readPosition
		result := decider(l, startLine, startCol)
		hasReadChar = lastReadPosition != l.readPosition

		if result.matched {
			token = result.token
			foundDecider = true
			break
		}
	}

	if !foundDecider {
		msg := fmt.Sprintf("Syntax Error at line %d, column %d: unrecognized character '%c'", startLine, startCol, l.ch)
		l.Errors = append(l.Errors, msg)
		token = Token{Type: ILLEGAL, Literal: string(l.ch), Line: startLine, Column: startCol}
	}

	if !hasReadChar {
		l.readChar()
	}
	return token
}

// readChar advances the Lexer by one byte. If the character that was just
// consumed was a newline, the line counter is incremented and the column counter
// is reset so that the next character begins at column 1. When the end of the
// input is reached, ch is set to 0 (NUL) to signal EOF.
func (l *Lexer) readChar() {
	if l.isCurrentCharANewLine() {
		l.Line++
		l.Column = 0
	}

	l.ch = 0 // ASCII code for "NUL", representing EOF
	if !l.hasFinalizedReading() {
		l.ch = rune(l.input[l.readPosition])
	}

	l.position = l.readPosition
	l.readPosition++
	l.Column++
}

// readString consumes characters until a closing double quote or EOF is
// encountered, returning the enclosed string literal. It also consumes the
// closing quote if present.
func (l *Lexer) readString() string {
	position := l.position + 1
	for {
		l.readChar()
		if l.ch == '"' || l.ch == 0 {
			break
		}
	}
	result := l.input[position:l.position]
	if l.ch == '"' {
		l.readChar() // consume the closing quote
	}
	return result
}

// readDate consumes characters until a closing single quote or EOF is
// encountered, returning the enclosed date literal. It also consumes the
// closing quote if present.
func (l *Lexer) readDate() string {
	position := l.position + 1
	for {
		l.readChar()
		if l.ch == '\'' || l.ch == 0 {
			break
		}
	}
	result := l.input[position:l.position]
	if l.ch == '\'' {
		l.readChar() // consume the closing quote
	}
	return result
}

// skipWhitespace advances past any combination of spaces, tabs, newlines, and
// carriage-returns, updating line and column tracking along the way.
func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' || l.ch == '#' {
		if l.ch == '#' {
			// consume until newline or EOF
			for l.ch != '\n' && l.ch != '\r' && l.ch != 0 {
				l.readChar()
			}
		} else {
			l.readChar()
		}
	}
}

// readIdentifier consumes the longest sequence of letter or underscore
// characters starting at the current position and returns it as a string. The
// Lexer is left positioned at the first character that does not belong to
// the identifier.
func (l *Lexer) readIdentifier() string {
	position := l.position
	for l.isCurrentCharALetter() || l.isCurrentCharADigit() {
		l.readChar()
	}
	return l.input[position:l.position]
}

// readNumber consumes a sequence of digit characters, optionally containing a
// single decimal point, and returns it as a string. The Lexer is left
// positioned at the first character that does not belong to the number literal.
func (l *Lexer) readNumber() string {
	position := l.position
	for l.isCurrentCharADigit() || l.ch == '.' {
		l.readChar()
	}
	return l.input[position:l.position]
}

// isCurrentCharALetter reports whether the current character is an ASCII letter
// (a–z, A–Z) or an underscore, both of which are valid identifier constituents.
func (l *Lexer) isCurrentCharALetter() bool {
	return 'a' <= l.ch && l.ch <= 'z' || 'A' <= l.ch && l.ch <= 'Z' || l.ch == '_'
}

// isCurrentCharADigit reports whether the current character is an ASCII decimal
// digit (0–9).
func (l *Lexer) isCurrentCharADigit() bool {
	return '0' <= l.ch && l.ch <= '9'
}

// isCurrentCharANewLine reports whether the current character represents a
// newline. It returns true for '\n' and for a standalone '\r' that is not
// followed by '\n', so that a "\r\n" pair is counted as a single line break.
func (l *Lexer) isCurrentCharANewLine() bool {
	return l.ch == '\n' || (l.ch == '\r' && (l.hasFinalizedReading() || l.input[l.readPosition] != '\n'))
}

// peekChar returns the next character in the input without advancing the
// Lexer's position. It returns 0 (NUL) if the end of input has been reached.
func (l *Lexer) peekChar() rune {
	if l.hasFinalizedReading() {
		return 0
	}
	return rune(l.input[l.readPosition])
}

// hasFinalizedReading reports whether the Lexer has consumed all bytes in
// the input, meaning the next readChar call would produce EOF.
func (l *Lexer) hasFinalizedReading() bool {
	return l.readPosition >= len(l.input)
}
