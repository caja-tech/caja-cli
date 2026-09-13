package lsp

import (
	"testing"

	"github.com/owenrumney/go-lsp/lsp"
	"github.com/owenrumney/go-lsp/servertest"
)

// decodedToken is a semantic token with its absolute position restored, which is how a
// test can reason about it without reimplementing the delta encoding inline.
type decodedToken struct {
	line, char, length int
	tokenType          string
	modifiers          []string
}

// decodeSemanticTokens reverses the protocol's delta encoding and resolves the legend
// indices back to names, so failures read in terms of the language rather than integers.
func decodeSemanticTokens(t *testing.T, data []int) []decodedToken {
	t.Helper()

	if len(data)%5 != 0 {
		t.Fatalf("semantic token data has %d integers, which is not a multiple of 5", len(data))
	}

	var out []decodedToken
	line, char := 0, 0

	for i := 0; i < len(data); i += 5 {
		deltaLine, deltaChar, length, typeIdx, modBits := data[i], data[i+1], data[i+2], data[i+3], data[i+4]

		if deltaLine < 0 || deltaChar < 0 {
			t.Fatalf("token %d has a negative delta (%d, %d); positions must not go backwards",
				i/5, deltaLine, deltaChar)
		}
		if typeIdx < 0 || typeIdx >= len(semanticTokenTypes) {
			t.Fatalf("token %d has type index %d, which is outside the advertised legend",
				i/5, typeIdx)
		}

		line += deltaLine
		if deltaLine == 0 {
			char += deltaChar
		} else {
			char = deltaChar
		}

		var mods []string
		for bit, name := range semanticTokenModifiers {
			if modBits&(1<<bit) != 0 {
				mods = append(mods, name)
			}
		}

		out = append(out, decodedToken{
			line: line, char: char, length: length,
			tokenType: semanticTokenTypes[typeIdx], modifiers: mods,
		})
	}
	return out
}

func tokenAt(tokens []decodedToken, line, char int) (decodedToken, bool) {
	for _, tok := range tokens {
		if tok.line == line && tok.char == char {
			return tok, true
		}
	}
	return decodedToken{}, false
}

func hasModifier(tok decodedToken, want string) bool {
	for _, m := range tok.modifiers {
		if m == want {
			return true
		}
	}
	return false
}

// TestSemanticTokensClassifyBySymbol covers the whole point of the feature: the
// distinctions a TextMate grammar cannot make, because they are questions about the
// symbol table rather than about the text.
func TestSemanticTokensClassifyBySymbol(t *testing.T) {
	src := `import "math"
type Money Number
type Order struct {
    total Money
}
union Animal = Order
const rate = 5
let label = "x"
let compute = fn(price: Money) -> Money {
    return price
}
`
	_, s, uri := openInline(t, "main.caja", src)

	result, err := s.SemanticTokensFull(uri)
	if err != nil {
		t.Fatalf("SemanticTokensFull: %v", err)
	}
	tokens := decodeSemanticTokens(t, result.Data)

	cases := []struct {
		what       string
		line, char int
		wantType   string
		wantMod    string
	}{
		{"the imported module", 0, 7, "namespace", "defaultLibrary"},
		{"a type alias declaration", 1, 5, "type", "declaration"},
		{"a struct declaration", 2, 5, "struct", "declaration"},
		{"a struct field", 3, 4, "property", "declaration"},
		{"a union declaration", 5, 6, "enum", "declaration"},
		{"a const binding", 6, 6, "variable", "readonly"},
		{"a let bound to a function reads as a function", 8, 4, "function", "declaration"},
		{"a parameter", 8, 17, "parameter", "declaration"},
	}

	for _, tc := range cases {
		t.Run(tc.what, func(t *testing.T) {
			tok, found := tokenAt(tokens, tc.line, tc.char)
			if !found {
				t.Fatalf("no semantic token at %d:%d", tc.line, tc.char)
			}
			if tok.tokenType != tc.wantType {
				t.Errorf("classified as %q, want %q", tok.tokenType, tc.wantType)
			}
			if tc.wantMod != "" && !hasModifier(tok, tc.wantMod) {
				t.Errorf("modifiers %v are missing %q", tok.modifiers, tc.wantMod)
			}
		})
	}
}

// TestSemanticTokensMarkTypeAnnotations checks the classification that was impossible
// before type annotations carried positions: `Money` in an annotation is a type, not an
// ordinary capitalised word.
func TestSemanticTokensMarkTypeAnnotations(t *testing.T) {
	src := "type Money Number\nlet price: Money = 1\n"
	_, s, uri := openInline(t, "main.caja", src)

	result, err := s.SemanticTokensFull(uri)
	if err != nil {
		t.Fatalf("SemanticTokensFull: %v", err)
	}
	tokens := decodeSemanticTokens(t, result.Data)

	tok, found := tokenAt(tokens, 1, 11) // `Money` in the annotation
	if !found {
		t.Fatal("the type in the annotation produced no semantic token")
	}
	if tok.tokenType != "type" {
		t.Errorf("annotation type classified as %q, want \"type\"", tok.tokenType)
	}
	if tok.length != len("Money") {
		t.Errorf("token length %d, want %d", tok.length, len("Money"))
	}
}

// TestSemanticTokenEncodingIsWellFormed pins the protocol's delta encoding, which is easy
// to get subtly wrong in ways that silently misplace every colour after the first mistake.
func TestSemanticTokenEncodingIsWellFormed(t *testing.T) {
	src := `let a = 1
let b = 2
let c = a + b
`
	_, s, uri := openInline(t, "main.caja", src)

	result, err := s.SemanticTokensFull(uri)
	if err != nil {
		t.Fatalf("SemanticTokensFull: %v", err)
	}
	if len(result.Data) == 0 {
		t.Fatal("no semantic tokens produced for a file full of declarations")
	}

	tokens := decodeSemanticTokens(t, result.Data) // fails on negative deltas or bad indices

	// Tokens must be in source order and must not overlap.
	for i := 1; i < len(tokens); i++ {
		prev, cur := tokens[i-1], tokens[i]
		if cur.line < prev.line || (cur.line == prev.line && cur.char < prev.char) {
			t.Fatalf("token %d at %d:%d comes before token %d at %d:%d",
				i, cur.line, cur.char, i-1, prev.line, prev.char)
		}
		if cur.line == prev.line && cur.char < prev.char+prev.length {
			t.Errorf("token at %d:%d overlaps the previous token ending at %d",
				cur.line, cur.char, prev.char+prev.length)
		}
	}

	for _, tok := range tokens {
		if tok.length <= 0 {
			t.Errorf("token at %d:%d has length %d", tok.line, tok.char, tok.length)
		}
	}
}

// TestSemanticTokensUseUTF16Positions checks that token positions honour the protocol's
// encoding. A byte-based position would misplace every token after a non-ASCII character.
func TestSemanticTokensUseUTF16Positions(t *testing.T) {
	src := "let greeting = \"Olá\"\nlet name = greeting\n"
	_, s, uri := openInline(t, "main.caja", src)

	result, err := s.SemanticTokensFull(uri)
	if err != nil {
		t.Fatalf("SemanticTokensFull: %v", err)
	}
	tokens := decodeSemanticTokens(t, result.Data)

	// `greeting` on line 1 starts at UTF-16 character 11 regardless of line 0's contents.
	if _, found := tokenAt(tokens, 1, 11); !found {
		t.Errorf("no token at 1:11 for the use of `greeting`; tokens were %+v", tokens)
	}

	// The string literal's own token must cover both quotes: 5 characters for "Olá".
	if tok, found := tokenAt(tokens, 0, 15); found && tok.length != 5 {
		t.Errorf("the string literal token has length %d, want 5 (both quotes, one unit "+
			"for the accented character)", tok.length)
	}
}

// TestSemanticTokensAdvertiseALegend checks that the capability names the token types the
// server actually emits. Without a legend the client cannot decode anything.
func TestSemanticTokensAdvertiseALegend(t *testing.T) {
	h := NewCajaHandler()

	result, err := h.Initialize(t.Context(), &lsp.InitializeParams{})
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	provider := result.Capabilities.SemanticTokensProvider
	if provider == nil {
		t.Fatal("no semantic tokens capability advertised")
	}
	if len(provider.Legend.TokenTypes) != len(semanticTokenTypes) {
		t.Errorf("legend advertises %d token types, but the server emits %d",
			len(provider.Legend.TokenTypes), len(semanticTokenTypes))
	}
	if provider.Full == nil {
		t.Error("full-document semantic tokens are not advertised")
	}
}

// TestSemanticTokensOnUnopenedDocument guards the empty case.
func TestSemanticTokensOnUnopenedDocument(t *testing.T) {
	h := NewCajaHandler()

	result, err := h.SemanticTokensFull(t.Context(), &lsp.SemanticTokensParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: "file:///nowhere/missing.caja"},
	})
	if err != nil {
		t.Fatalf("SemanticTokensFull on an unopened document errored: %v", err)
	}
	if result == nil || len(result.Data) != 0 {
		t.Errorf("expected an empty token list, got %v", result)
	}
}

// TestSemanticTokensOverCorpus runs the classifier across every sample in the repo and
// validates the encoding of each result. Delta encoding fails in ways that are invisible
// on a small hand-written file but wreck a real one: a single negative delta, or two
// tokens claiming the same span, misplaces every colour after it.
func TestSemanticTokensOverCorpus(t *testing.T) {
	for _, f := range compilerSampleEntries(t) {
		t.Run(f.name, func(t *testing.T) {
			h := NewCajaHandler()
			s := servertest.New(t, h)
			openCorpusFile(t, s, f)

			result, err := h.SemanticTokensFull(t.Context(), &lsp.SemanticTokensParams{
				TextDocument: lsp.TextDocumentIdentifier{URI: f.uri()},
			})
			if err != nil {
				t.Fatalf("SemanticTokensFull: %v", err)
			}

			tokens := decodeSemanticTokens(t, result.Data)
			for i := 1; i < len(tokens); i++ {
				prev, cur := tokens[i-1], tokens[i]
				if cur.line == prev.line && cur.char < prev.char+prev.length {
					t.Errorf("token at %d:%d overlaps the previous one ending at %d",
						cur.line, cur.char, prev.char+prev.length)
				}
			}
		})
	}
}
