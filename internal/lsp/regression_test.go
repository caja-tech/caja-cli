package lsp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/owenrumney/go-lsp/lsp"
	"github.com/owenrumney/go-lsp/servertest"
)

// openInline writes text to a real file (module resolution and Definition both read from
// disk) and opens it, returning the handler, harness and URI.
func openInline(t *testing.T, name, text string) (*CajaHandler, *servertest.Harness, lsp.DocumentURI) {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}

	h := NewCajaHandler()
	s := servertest.New(t, h)
	uri := lsp.DocumentURI("file://" + path)

	if err := s.DidOpen(uri, "caja", text); err != nil {
		t.Fatalf("DidOpen: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err := s.WaitForDiagnostics(ctx, uri); err != nil {
		t.Fatalf("no diagnostics published within timeout: %v", err)
	}

	return h, s, uri
}

func hoverText(t *testing.T, s *servertest.Harness, uri lsp.DocumentURI, line, char int) string {
	t.Helper()

	hv, err := s.Hover(uri, line, char)
	if err != nil {
		t.Fatalf("Hover at %d:%d: %v", line, char, err)
	}
	if hv == nil || hv.Contents.Markup == nil {
		return ""
	}
	return hv.Contents.Markup.Value
}

// TestHoverInIfWithoutElse pins the typed-nil crash: IfExpression.Alternative is a nil
// *ast.BlockStatement inside a non-nil ast.Node interface when there is no else branch,
// which used to make every position lookup in the file dereference nil.
func TestHoverInIfWithoutElse(t *testing.T) {
	src := `let classify = fn(n: Number) -> Number {
    if (n > 10) {
        return 1
    }
    return 0
}
let result = classify(42)
`
	_, s, uri := openInline(t, "main.caja", src)

	// The assertion is simply that no position in the file crashes the lookup. Before
	// isNilNode, every one of these dereferenced a nil *ast.BlockStatement.
	for line := range 7 {
		for char := range 30 {
			if _, err := s.Hover(uri, line, char); err != nil {
				t.Fatalf("Hover at %d:%d errored: %v", line, char, err)
			}
		}
	}
}

// TestHoverOnDeclarationName covers hovering the name being bound, rather than a later
// use of it: `let x = 5` with the cursor on `x`. The analyzer records nodeSymbols for
// identifier references but not for the Name node of a let/const binding, so the single
// most common hover target in a declaration answers with nothing.
func TestHoverOnDeclarationName(t *testing.T) {
	cases := []struct {
		name       string
		src        string
		line, char int
		want       string
	}{
		{"let bound to a literal", "let x = 5\n", 0, 4, "Number"},
		{"const bound to a literal", "const rate = 100\n", 0, 6, "Number"},
		{"let bound to a function", "let add = fn(x: Number) -> Number { return x }\n", 0, 4, "Number"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, s, uri := openInline(t, "main.caja", tc.src)

			got := hoverText(t, s, uri, tc.line, tc.char)
			if !strings.Contains(got, tc.want) {
				t.Errorf("hover on the declared name returned %q, want it to mention %q",
					got, tc.want)
			}
		})
	}
}

// TestDefinitionOnDeclarationName is the same gap seen through go-to-definition: invoking
// it on the declared name should resolve to the declaration itself rather than nothing,
// which is what makes "jump to definition" idempotent.
func TestDefinitionOnDeclarationName(t *testing.T) {
	_, s, uri := openInline(t, "main.caja", "let total = 42\nlet copy = total\n")

	locs, err := s.Definition(uri, 0, 4) // on `total` in its own declaration
	if err != nil {
		t.Fatalf("Definition: %v", err)
	}
	if len(locs) == 0 {
		t.Fatal("go-to-definition on a declared name resolved to nothing")
	}
	if locs[0].Range.Start.Line != 0 {
		t.Errorf("expected the declaration on line 0, got line %d", locs[0].Range.Start.Line)
	}
}

// TestPositionsAreUTF16 encodes the LSP contract: Position.Character counts UTF-16 code
// units, not bytes. lexer.Token.Column is a byte column (readChar increments per byte),
// and nothing converts between them, so every position after a non-ASCII character on the
// same line is wrong.
func TestPositionsAreUTF16(t *testing.T) {
	// `á` is two UTF-8 bytes but one UTF-16 unit, so `name` begins at UTF-16 character 23
	// and byte column 24 — the discrepancy this test exists to catch.
	src := "let name = \"Jose\"\n" +
		"let greeting = \"Olá\" + name\n"

	const (
		line          = 1
		nameUTF16Char = 23
	)

	if got := []rune(strings.Split(src, "\n")[line])[nameUTF16Char]; got != 'n' {
		t.Fatalf("test fixture drifted: expected UTF-16 char %d to be 'n', got %q", nameUTF16Char, got)
	}

	_, s, uri := openInline(t, "main.caja", src)

	got := hoverText(t, s, uri, line, nameUTF16Char)
	if !strings.Contains(got, "String") {
		t.Errorf("hover over `name` at UTF-16 character %d returned %q, want the String type.\n"+
			"Positions are being compared as byte offsets, so everything after a non-ASCII "+
			"character on the line is off by the extra UTF-8 bytes.", nameUTF16Char, got)
	}
}

// TestCRLFDocumentCompletion covers documents with Windows line endings. Completion
// splits the buffer on "\n" and leaves the trailing "\r" on every line, which corrupts
// the prefix it extracts to decide what is being completed.
func TestCRLFDocumentCompletion(t *testing.T) {
	src := "type User struct {\r\n    name String\r\n}\r\n" +
		"let u = User { name: \"a\" }\r\n" +
		"let n = u.\r\n"

	h, _, uri := openInline(t, "main.caja", src)

	// Cursor sits immediately after the `.` on the last line.
	res, err := h.Completion(context.Background(), &lsp.CompletionParams{
		TextDocumentPositionParams: textDocPos(uri, lsp.Position{Line: 4, Character: 10}),
	})
	if err != nil {
		t.Fatalf("Completion: %v", err)
	}
	if res == nil {
		t.Fatal("Completion returned nil")
	}

	if !hasLabel(res.Items, "name") {
		t.Errorf("dot-completion on a CRLF document did not offer the struct field `name`; got %v",
			labels(res.Items))
	}
}

// TestStringLiteralHoverRange covers the token-length defect for quoted literals:
// Token.Column points at the opening quote but Token.Literal holds only the inner text,
// so the computed range is two characters short and the last character of the literal
// falls outside it.
func TestStringLiteralHoverRange(t *testing.T) {
	src := "let greeting = \"abc\"\n"
	_, s, uri := openInline(t, "main.caja", src)

	// `"abc"` spans characters 15..19 inclusive. Every one of them is inside the literal.
	for char := 15; char <= 19; char++ {
		got := hoverText(t, s, uri, 0, char)
		if got == "" {
			t.Errorf("hover at character %d (inside the string literal `\"abc\"`) returned nothing; "+
				"the literal's range is computed from len(Token.Literal), which excludes the quotes", char)
		}
	}
}

// TestSignatureHelpInsideContainers pins the gaps in findCallExpression, which never
// descends into several very common expression positions. Each of these is a call the
// user can put their cursor inside and reasonably expect signature help for.
func TestSignatureHelpInsideContainers(t *testing.T) {
	prelude := "let add = fn(x: Number, y: Number) -> Number {\n    return x + y\n}\n"

	cases := []struct {
		name string
		code string
		line int // relative to the end of the prelude
		char int
	}{
		{"const statement", "const total = add(1, 2)\n", 0, 19},
		{"array literal", "let xs = [add(1, 2)]\n", 0, 15},
		{"nested call argument", "let n = add(add(1, 2), 3)\n", 0, 17},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, s, uri := openInline(t, "main.caja", prelude+tc.code)

			sig, err := s.SignatureHelp(uri, 3+tc.line, tc.char)
			if err != nil {
				t.Fatalf("SignatureHelp: %v", err)
			}
			if sig == nil || len(sig.Signatures) == 0 {
				t.Errorf("no signature help inside a call in %s; findCallExpression does not "+
					"descend into this expression position", tc.name)
			}
		})
	}
}

// TestMapLiteralLookupIsDeterministic guards against ast_util.go ranging over
// MapLiteral.Pairs, which is a Go map: iteration order is randomized per run, so the node
// chosen for a position can differ between two identical requests.
func TestMapLiteralLookupIsDeterministic(t *testing.T) {
	src := "let config = {\"alpha\": 1, \"beta\": 2, \"gamma\": 3, \"delta\": 4}\n"
	_, s, uri := openInline(t, "main.caja", src)

	const line, char = 0, 15 // inside the "alpha" key

	first := hoverText(t, s, uri, line, char)
	for i := range 200 {
		if got := hoverText(t, s, uri, line, char); got != first {
			t.Fatalf("hover at %d:%d is not deterministic: iteration %d returned %q, first call returned %q.\n"+
				"MapLiteral.Pairs is a Go map and is iterated in randomized order.",
				line, char, i, got, first)
		}
	}
}

func labels(items []lsp.CompletionItem) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.Label)
	}
	return out
}

func hasLabel(items []lsp.CompletionItem, want string) bool {
	for _, it := range items {
		if it.Label == want {
			return true
		}
	}
	return false
}
