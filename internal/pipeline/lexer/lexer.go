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

// NewAt is like New, but seeds the starting Line/Column instead of always
// beginning at (1, 0). Used to lex a substring extracted from a larger
// source file (e.g. an interpolated string's "${...}" segment) while still
// reporting positions relative to the original file, not the substring.
func NewAt(input string, startLine, startColumn int) *Lexer {
	t := &Lexer{input: input, ErrorTracker: ErrorTracker{Line: startLine, Column: startColumn}}
	t.readChar()

	return t
}

// Clone creates a shallow copy of the Lexer without sharing the Errors slice,
// allowing safe lookahead without modifying the original lexer's error state.
func (l *Lexer) Clone() *Lexer {
	clone := *l
	clone.Errors = nil
	return &clone
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
// encountered, returning the enclosed string literal RAW (escape sequences
// like "\n"/"\"" are left undecoded here — decoding happens later in the
// parser, once "${...}" interpolation boundaries are already fixed; see
// parseStringLiteral). It also consumes the closing quote if present.
//
// Two things this scan must not be fooled by, both confined to this one
// function (no state spans multiple NextToken calls):
//   - An escaped quote ("\"") must not end the string early — readChar past
//     any "\" unconditionally consumes the following character without
//     testing it against the closing delimiter.
//   - A "${...}" interpolation's own embedded expression can itself contain
//     a nested string/date literal (e.g. "${concat(x, "-")}"). A depth
//     counter tracks whether the scan is currently inside a "${" that
//     hasn't been closed by its matching "}" yet; a '"'/'\'' seen at depth 0
//     ends the outer string as always, but one seen at depth > 0 starts a
//     nested literal that gets its own escape-aware skip-to-delimiter scan
//     (skipNestedDelimited) rather than being mistaken for the outer
//     string's own terminator.
func (l *Lexer) readString() string {
	position := l.position + 1
	interpDepth := 0
	for {
		l.readChar()
		if l.ch == 0 {
			break
		}
		if l.ch == '\\' {
			l.readChar() // consume the escaped character verbatim
			if l.ch == 0 {
				break
			}
			continue
		}
		if interpDepth == 0 && l.ch == '"' {
			break
		}
		if l.ch == '$' && l.peekChar() == '{' {
			interpDepth++
			l.readChar() // also consume the '{' so it isn't double-counted below
			continue
		}
		if interpDepth > 0 {
			switch l.ch {
			case '{':
				interpDepth++
			case '}':
				interpDepth--
			case '"':
				l.skipNestedDelimited('"')
			case '\'':
				l.skipNestedDelimited('\'')
			}
			// skipNestedDelimited advances via its own readChar calls and
			// can land on EOF (an unterminated nested literal, or simply an
			// unterminated outer/interpolation with no more input left) —
			// check immediately rather than looping back to call readChar
			// again, which would advance position/readPosition past the
			// end of input with no bounds check (readChar itself doesn't
			// guard against being called again after EOF).
			if l.ch == 0 {
				break
			}
		}
	}
	result := l.input[position:l.position]
	switch l.ch {
	case 0:
		l.Errors = append(l.Errors, fmt.Sprintf("Syntax Error at line %d, column %d: unterminated string literal", l.Line, l.Column))
	case '"':
		l.readChar() // consume the closing quote
	}
	return result
}

// skipNestedDelimited advances past a nested "..."/'...' literal that starts
// at the current character — used when readString finds one inside an
// unclosed "${...}" interpolation. A small escape-aware scan to the closing
// delimiter or EOF, mirroring readString/readDate's own rule, so neither an
// escaped quote nor a quote belonging to this nested literal gets mistaken
// for the outer string's own terminator.
func (l *Lexer) skipNestedDelimited(closing rune) {
	for {
		l.readChar()
		if l.ch == closing || l.ch == 0 {
			return
		}
		if l.ch == '\\' {
			l.readChar()
			if l.ch == 0 {
				return
			}
		}
	}
}

// readDate consumes characters until a closing single quote or EOF is
// encountered, returning the enclosed date literal RAW (undecoded — see
// readString's doc comment; the same escape table applies). It also
// consumes the closing quote if present. An escaped quote ("\'") does not
// end the literal early, mirroring readString's own escape-skip.
func (l *Lexer) readDate() string {
	position := l.position + 1
	for {
		l.readChar()
		if l.ch == 0 {
			break
		}
		if l.ch == '\\' {
			l.readChar()
			if l.ch == 0 {
				break
			}
			continue
		}
		if l.ch == '\'' {
			break
		}
	}
	result := l.input[position:l.position]
	switch l.ch {
	case 0:
		l.Errors = append(l.Errors, fmt.Sprintf("Syntax Error at line %d, column %d: unterminated date literal", l.Line, l.Column))
	case '\'':
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

// readNumber consumes a sequence of digit characters, optionally followed by
// a decimal point and more digits, and returns it as a string. The decimal
// point is only consumed when a digit follows it — a trailing dot with no
// fractional digits (e.g. the "." in "5.abs()") is left for the next token,
// so a numeric literal can be immediately followed by a dot-call. The Lexer
// is left positioned at the first character that does not belong to the
// number literal.
func (l *Lexer) readNumber() string {
	position := l.position
	for l.isCurrentCharADigit() {
		l.readChar()
	}
	if l.ch == '.' && isASCIIDigit(l.peekChar()) {
		l.readChar()
		for l.isCurrentCharADigit() {
			l.readChar()
		}
	}
	return l.input[position:l.position]
}

// isASCIIDigit reports whether ch is an ASCII decimal digit (0-9).
func isASCIIDigit(ch rune) bool {
	return '0' <= ch && ch <= '9'
}

// isCurrentCharALetter reports whether the current character is an ASCII letter
// (a–z, A–Z) or an underscore, both of which are valid identifier constituents.
func (l *Lexer) isCurrentCharALetter() bool {
	return 'a' <= l.ch && l.ch <= 'z' || 'A' <= l.ch && l.ch <= 'Z' || l.ch == '_'
}

// isCurrentCharADigit reports whether the current character is an ASCII decimal
// digit (0–9).
func (l *Lexer) isCurrentCharADigit() bool {
	return isASCIIDigit(l.ch)
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
