package lsp

import (
	"context"
	"testing"

	"github.com/owenrumney/go-lsp/lsp"
)

// completionAt runs dot-completion at a position and returns the offered labels.
func completionAt(t *testing.T, h *CajaHandler, uri lsp.DocumentURI, line, char int) []string {
	t.Helper()

	res, err := h.Completion(context.Background(), &lsp.CompletionParams{
		TextDocumentPositionParams: textDocPos(uri, lsp.Position{Line: line, Character: char}),
	})
	if err != nil {
		t.Fatalf("Completion at %d:%d: %v", line, char, err)
	}
	if res == nil {
		return nil
	}
	return labels(res.Items)
}

func contains(haystack []string, needle string) bool {
	for _, item := range haystack {
		if item == needle {
			return true
		}
	}
	return false
}

// TestUFCSCompletionOffersMethodsOnAValueReceiver covers the point of the dot-call form.
//
// `array.push(list, 4)` was always writable; what `list.push(4)` adds is finding `push` by
// typing a dot. Before this, a non-struct, non-module receiver offered nothing at all, so
// the one thing the syntax exists for was the one thing completion could not do.
func TestUFCSCompletionOffersMethodsOnAValueReceiver(t *testing.T) {
	src := `import "array"
let arr = [1, 2, 3]
let a1 = arr.
`
	h, _, uri := openInline(t, "main.caja", src)

	offered := completionAt(t, h, uri, 2, 13)
	for _, want := range []string{"push", "len", "pop", "head", "tail"} {
		if !contains(offered, want) {
			t.Errorf("dot-completion on an array receiver did not offer %q; got %v", want, offered)
		}
	}
}

// TestUFCSCompletionRespectsTheReceiverType checks that the offer is filtered by what
// would actually resolve. Offering a name the analyzer then rejects is worse than not
// offering it.
func TestUFCSCompletionRespectsTheReceiverType(t *testing.T) {
	src := `let onlyForText = fn(s: String, n: Number) -> String { return s }
let onlyForNumbers = fn(n: Number) -> Number { return n }
let count = 5
let result = count.
`
	h, _, uri := openInline(t, "main.caja", src)

	offered := completionAt(t, h, uri, 3, 19)
	if !contains(offered, "onlyForNumbers") {
		t.Errorf("a function whose first parameter is a Number was not offered on a Number "+
			"receiver; got %v", offered)
	}
	if contains(offered, "onlyForText") {
		t.Errorf("a function whose first parameter is a String was offered on a Number "+
			"receiver; got %v", offered)
	}
}

// TestUFCSCompletionOnStructOffersFieldsAndMethods checks that a struct receiver keeps its
// own fields while also gaining the functions callable on it — which is exactly how the
// language resolves such a call, the two coexisting and overloading.
func TestUFCSCompletionOnStructOffersFieldsAndMethods(t *testing.T) {
	src := `type Point struct {
    x Number
    y Number
}
let describe = fn(p: Point) -> Number { return p.x }
let origin = Point { x: 0, y: 0 }
let out = origin.
`
	h, _, uri := openInline(t, "main.caja", src)

	offered := completionAt(t, h, uri, 6, 17)
	for _, want := range []string{"x", "y"} {
		if !contains(offered, want) {
			t.Errorf("the struct's own field %q is no longer offered; got %v", want, offered)
		}
	}
	if !contains(offered, "describe") {
		t.Errorf("a function taking the struct as its first parameter was not offered as a "+
			"method; got %v", offered)
	}
}

// TestUFCSDefinitionResolvesToTheFunction covers go-to-definition on a method name. The
// analyzer recorded which function the call resolved to but not where it was declared, so
// this answered with nothing even for a function declared a line above.
func TestUFCSDefinitionResolvesToTheFunction(t *testing.T) {
	src := `let double = fn(x: Number) -> Number { return x * 2 }
let n = 5
let d = n.double()
`
	_, s, uri := openInline(t, "main.caja", src)

	locs, err := s.Definition(uri, 2, 10) // on `double` after the dot
	if err != nil {
		t.Fatalf("Definition: %v", err)
	}
	if len(locs) == 0 {
		t.Fatal("go-to-definition on a dot-called function resolved to nothing")
	}
	if locs[0].Range.Start.Line != 0 {
		t.Errorf("expected the declaration on line 0, got line %d", locs[0].Range.Start.Line)
	}
}

// TestUFCSDefinitionOnBuiltinIsEmpty pins deliberate behaviour rather than an oversight: a
// builtin has no source location, so `list.push(4)` gives the same answer that
// `array.push(list, 4)` already gives.
func TestUFCSDefinitionOnBuiltinIsEmpty(t *testing.T) {
	src := `import "array"
let arr = [1, 2, 3]
let a1 = arr.push(4)
`
	_, s, uri := openInline(t, "main.caja", src)

	locs, err := s.Definition(uri, 2, 13) // on `push`
	if err != nil {
		t.Fatalf("Definition: %v", err)
	}
	if len(locs) != 0 {
		t.Errorf("expected no definition for a builtin, got %v", locs)
	}
}

// TestUFCSHoverAndSignatureHelp guards what already worked, so a later change to the
// resolution path cannot quietly take it away.
func TestUFCSHoverAndSignatureHelp(t *testing.T) {
	src := `import "array"
let arr = [1, 2, 3]
let a1 = arr.push(4)
`
	_, s, uri := openInline(t, "main.caja", src)

	if got := hoverText(t, s, uri, 2, 13); got == "" {
		t.Error("hover on a dot-called function resolved to nothing")
	}

	sig, err := s.SignatureHelp(uri, 2, 18) // inside push(...)
	if err != nil {
		t.Fatalf("SignatureHelp: %v", err)
	}
	if sig == nil || len(sig.Signatures) == 0 {
		t.Error("no signature help inside a dot-called function's arguments")
	}
}

// TestUFCSMethodIsClassifiedAsAFunction covers the highlighting. The name after a dot is
// usually a field, but under the dot-call form it is a function, and colouring it as a
// field misreports what the dot means.
func TestUFCSMethodIsClassifiedAsAFunction(t *testing.T) {
	src := `type Point struct {
    x Number
}
let double = fn(n: Number) -> Number { return n * 2 }
let p = Point { x: 1 }
let field = p.x
let called = 5.double()
`
	_, s, uri := openInline(t, "main.caja", src)

	result, err := s.SemanticTokensFull(uri)
	if err != nil {
		t.Fatalf("SemanticTokensFull: %v", err)
	}
	tokens := decodeSemanticTokens(t, result.Data)

	if tok, ok := tokenAt(tokens, 5, 14); ok && tok.tokenType != "property" {
		t.Errorf("a genuine struct field is classified as %q, want \"property\"", tok.tokenType)
	}
	tok, ok := tokenAt(tokens, 6, 15) // `double` after the dot
	if !ok {
		t.Fatal("the dot-called function produced no semantic token")
	}
	if tok.tokenType != "function" {
		t.Errorf("a dot-called function is classified as %q, want \"function\"", tok.tokenType)
	}
}
