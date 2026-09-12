package posmap

import (
	"testing"
	"unicode/utf16"

	"caja-cli/internal/pipeline/lexer"

	"github.com/owenrumney/go-lsp/lsp"
)

func TestByteColumnFromUTF16(t *testing.T) {
	// "Olá" — á is one UTF-16 unit but two UTF-8 bytes, so every column after it differs
	// between the two encodings. "日本" are one UTF-16 unit each but three bytes each.
	const text = "let a = 1\n" +
		"let g = \"Olá\" + name\n" +
		"let j = \"日本\" + tail\n"

	ix := New(text)

	cases := []struct {
		name      string
		line      int
		utf16Char int
		wantByte  int
	}{
		// `let g = "Olá" + name` — the accent sits at UTF-16 index 11 and occupies two
		// bytes, so everything from index 12 onwards is shifted by one.
		{"ascii line is identity", 0, 8, 8},
		{"before the accent", 1, 10, 10},
		{"at the accent", 1, 11, 11},
		{"closing quote, just after the accent", 1, 12, 13},
		{"identifier after the accent", 1, 16, 17},
		// `let j = "日本" + tail` — two 3-byte runes at UTF-16 indices 9 and 10.
		{"after two 3-byte runes", 2, 11, 15},
		{"past end of line clamps", 0, 999, 9},
		{"negative clamps to zero", 0, -5, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ix.ByteColumn(lsp.Position{Line: tc.line, Character: tc.utf16Char})
			if got != tc.wantByte {
				t.Errorf("ByteColumn(line %d, char %d) = %d, want %d",
					tc.line, tc.utf16Char, got, tc.wantByte)
			}
		})
	}
}

// TestColumnConversionsRoundTrip checks the two directions agree at every rune boundary,
// which is the property the rest of the server depends on: a position that goes out to
// the editor must come back as the same byte column.
func TestColumnConversionsRoundTrip(t *testing.T) {
	lines := []string{
		"let x = 1",
		"let s = \"café au lait\"",
		"let e = \"\U0001F600 emoji\"", // outside the BMP: two UTF-16 units, four bytes
		"",
		"   ",
	}

	for li, line := range lines {
		ix := New(line)
		for byteCol := 0; byteCol <= len(line); byteCol++ {
			if byteCol < len(line) && !utf8StartsRune(line, byteCol) {
				continue // mid-rune offsets are not addressable positions
			}
			char := ix.UTF16Column(0, byteCol)
			back := ix.ByteColumn(lsp.Position{Line: 0, Character: char})
			if back != byteCol {
				t.Errorf("line %d %q: byte %d -> char %d -> byte %d", li, line, byteCol, char, back)
			}
		}
	}
}

func utf8StartsRune(s string, i int) bool { return s[i]&0xC0 != 0x80 }

// TestUTF16ColumnMatchesStdlib pins our conversion against the standard library's own
// UTF-16 encoder, so the two cannot drift.
func TestUTF16ColumnMatchesStdlib(t *testing.T) {
	const line = "let v = \"é\U0001F600日\" + z"
	ix := New(line)

	for byteCol := 0; byteCol <= len(line); byteCol++ {
		if byteCol < len(line) && !utf8StartsRune(line, byteCol) {
			continue
		}
		want := len(utf16.Encode([]rune(line[:byteCol])))
		if got := ix.UTF16Column(0, byteCol); got != want {
			t.Errorf("UTF16Column(0, %d) = %d, want %d", byteCol, got, want)
		}
	}
}

// TestTokenByteLenCountsDelimiters covers the defect this helper exists for: the lexer
// strips the quotes from string and date literals, so the literal is shorter than the
// source it came from.
func TestTokenByteLenCountsDelimiters(t *testing.T) {
	cases := []struct {
		name string
		tok  lexer.Token
		want int
	}{
		{"identifier", lexer.Token{Type: lexer.IDENT, Literal: "total"}, 5},
		{"number", lexer.Token{Type: lexer.NUMBER, Literal: "3.14"}, 4},
		{"string includes both quotes", lexer.Token{Type: lexer.STRING, Literal: "abc"}, 5},
		{"empty string is just quotes", lexer.Token{Type: lexer.STRING, Literal: ""}, 2},
		{"date includes both quotes", lexer.Token{Type: lexer.DATE, Literal: "2024-01-01"}, 12},
		{"eof occupies nothing", lexer.Token{Type: lexer.EOF, Literal: ""}, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := TokenByteLen(tc.tok); got != tc.want {
				t.Errorf("TokenByteLen(%s) = %d, want %d", tc.name, got, tc.want)
			}
		})
	}
}

// TestTokenRangeSpansTheWholeLiteral is the end-to-end shape the editor sees: a range
// that covers every character the user can put a cursor on.
func TestTokenRangeSpansTheWholeLiteral(t *testing.T) {
	const text = "let greeting = \"abc\"\n"
	ix := New(text)

	// The lexer reports 1-based line/column; the literal starts at the opening quote,
	// which is byte 15 -> column 16.
	tok := lexer.Token{Type: lexer.STRING, Literal: "abc", Line: 1, Column: 16}

	got := ix.TokenRange(tok)
	want := lsp.Range{
		Start: lsp.Position{Line: 0, Character: 15},
		End:   lsp.Position{Line: 0, Character: 20},
	}
	if got != want {
		t.Errorf("TokenRange = %+v, want %+v (the range must cover `\"abc\"` including both quotes)", got, want)
	}
}

// TestTokenRangeIsNeverEmpty guarantees a caret-width range even for zero-length tokens,
// because an empty range is invisible in the editor.
func TestTokenRangeIsNeverEmpty(t *testing.T) {
	ix := New("let x = 1\n")

	got := ix.TokenRange(lexer.Token{Type: lexer.EOF, Literal: "", Line: 1, Column: 10})
	if got.End.Character <= got.Start.Character {
		t.Errorf("zero-length token produced an empty range %+v", got)
	}
}

// TestLineStripsCarriageReturn covers CRLF documents: the \r belongs to the line ending,
// not to the content, so column arithmetic must not count it.
func TestLineStripsCarriageReturn(t *testing.T) {
	ix := New("let x = 1\r\nlet y = 2\r\n")

	if got := ix.Line(0); got != "let x = 1" {
		t.Errorf("Line(0) = %q, want the content without the carriage return", got)
	}
	if got := ix.ByteColumn(lsp.Position{Line: 0, Character: 99}); got != 9 {
		t.Errorf("clamping past the end of a CRLF line = %d, want 9", got)
	}
}
