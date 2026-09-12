// Package posmap converts between Caja's internal source coordinates and LSP positions.
//
// The two systems disagree on both axes. lexer.Token uses 1-based lines and a 1-based
// *byte* column — lexer.readChar advances the column once per byte, not per rune. LSP
// uses 0-based lines and a 0-based character offset counted in *UTF-16 code units*. For
// pure ASCII the only difference is the off-by-one, which is why the mismatch went
// unnoticed; the moment a line contains a non-ASCII character, every position after it on
// that line is wrong by the extra UTF-8 bytes.
//
// Keeping every conversion here means the arithmetic exists once. The rule this package
// exists to enforce: no lsp.Position or lsp.Range should be built anywhere else.
package posmap

import (
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"caja-cli/internal/pipeline/lexer"

	"github.com/owenrumney/go-lsp/lsp"
)

// LineIndex maps positions for one document version. Build it once per text; it is
// immutable and safe to share.
type LineIndex struct {
	lines []string
	// ascii records, per line, whether the line is pure ASCII. That is the overwhelmingly
	// common case in Caja source (identifiers are ASCII-only by construction, so only
	// string literals, date literals and comments can carry anything else), and it lets
	// the conversions short-circuit to plain integer arithmetic.
	ascii []bool
}

// New builds an index over text. Line splitting matches the lexer's: \n terminates a
// line and a preceding \r belongs to the line ending, not to the content.
func New(text string) *LineIndex {
	raw := strings.Split(text, "\n")

	ix := &LineIndex{
		lines: make([]string, len(raw)),
		ascii: make([]bool, len(raw)),
	}
	for i, line := range raw {
		line = strings.TrimSuffix(line, "\r")
		ix.lines[i] = line
		ix.ascii[i] = isASCII(line)
	}
	return ix
}

func isASCII(s string) bool {
	for i := range len(s) {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// Line returns the content of a 0-based line, without its line ending. Out-of-range lines
// return "" so callers need no bounds checks of their own.
func (ix *LineIndex) Line(line int) string {
	if line < 0 || line >= len(ix.lines) {
		return ""
	}
	return ix.lines[line]
}

// ByteColumn converts an LSP position to the 0-based byte column the lexer would report.
// A character offset past the end of the line clamps to the end, which is what an editor
// sends when the cursor sits in the virtual space beyond the last character.
func (ix *LineIndex) ByteColumn(pos lsp.Position) int {
	line := ix.Line(pos.Line)
	if pos.Character <= 0 {
		return 0
	}
	if ix.lineIsASCII(pos.Line) {
		return min(pos.Character, len(line))
	}

	units := 0
	for offset, r := range line {
		if units >= pos.Character {
			return offset
		}
		units += utf16.RuneLen(r)
	}
	return len(line)
}

// UTF16Column converts a 0-based byte column into the LSP character offset.
func (ix *LineIndex) UTF16Column(line, byteCol int) int {
	text := ix.Line(line)
	if byteCol <= 0 {
		return 0
	}
	if ix.lineIsASCII(line) {
		return min(byteCol, len(text))
	}
	if byteCol > len(text) {
		byteCol = len(text)
	}
	return len(utf16.Encode([]rune(text[:byteCol])))
}

func (ix *LineIndex) lineIsASCII(line int) bool {
	return line >= 0 && line < len(ix.ascii) && ix.ascii[line]
}

// Position converts a token's start into an LSP position.
func (ix *LineIndex) Position(tok lexer.Token) lsp.Position {
	line := max(tok.Line-1, 0)
	byteCol := max(tok.Column-1, 0)

	return lsp.Position{Line: line, Character: ix.UTF16Column(line, byteCol)}
}

// TokenRange returns the source range a token covers, in LSP coordinates.
func (ix *LineIndex) TokenRange(tok lexer.Token) lsp.Range {
	start := ix.Position(tok)
	line := max(tok.Line-1, 0)

	end := lsp.Position{
		Line:      line,
		Character: ix.UTF16Column(line, max(tok.Column-1, 0)+TokenByteLen(tok)),
	}
	if end.Character <= start.Character {
		end.Character = start.Character + 1
	}
	return lsp.Range{Start: start, End: end}
}

// TokenByteLen returns how many bytes of source a token actually occupies.
//
// This is not len(tok.Literal). The lexer stores a quoted literal's *inner* text — for
// `"abc"` the literal is `abc` — while Column points at the opening quote. Using the
// literal's length directly makes every string and date range two characters short, so
// the closing quote and the character before it fall outside the token.
func TokenByteLen(tok lexer.Token) int {
	switch tok.Type {
	case lexer.STRING, lexer.DATE:
		return len(tok.Literal) + 2 // the delimiters the literal omits
	case lexer.EOF:
		return 0
	default:
		return len(tok.Literal)
	}
}
