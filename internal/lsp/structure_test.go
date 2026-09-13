package lsp

import (
	"testing"

	"github.com/owenrumney/go-lsp/lsp"
)

// TestDocumentSymbolOutlinesDeclarations checks the file outline: which top-level
// declarations appear, how they are classified, and that nested members are nested.
func TestDocumentSymbolOutlinesDeclarations(t *testing.T) {
	src := `import "array"
type Money Number
type Order struct {
    id Number
    total Money
}
type Cat struct {
    name String
}
type Dog struct {
    name String
}
union Animal = Cat | Dog
define Adult constraints Order with: fn(o: Order) -> Boolean { return o.total > 0 }
const rate = 5
let label = "hello"
let compute = fn(x: Number) -> Number { return x }
`
	_, s, uri := openInline(t, "main.caja", src)

	symbols, err := s.DocumentSymbol(uri)
	if err != nil {
		t.Fatalf("DocumentSymbol: %v", err)
	}

	byName := make(map[string]lsp.DocumentSymbol, len(symbols))
	for _, sym := range symbols {
		byName[sym.Name] = sym
	}

	wantKinds := map[string]lsp.SymbolKind{
		"array":   lsp.SymbolKindModule,
		"Money":   lsp.SymbolKindClass,
		"Order":   lsp.SymbolKindStruct,
		"Animal":  lsp.SymbolKindEnum,
		"Adult":   lsp.SymbolKindInterface,
		"rate":    lsp.SymbolKindConstant,
		"label":   lsp.SymbolKindVariable,
		"compute": lsp.SymbolKindFunction, // a let bound to a function reads as a function
	}

	for name, wantKind := range wantKinds {
		sym, found := byName[name]
		if !found {
			t.Errorf("outline is missing %q", name)
			continue
		}
		if sym.Kind != wantKind {
			t.Errorf("%q has kind %d, want %d", name, sym.Kind, wantKind)
		}
	}

	// Struct fields nest under the struct.
	order := byName["Order"]
	if len(order.Children) != 2 {
		t.Fatalf("Order has %d children, want its 2 fields", len(order.Children))
	}
	for _, field := range order.Children {
		if field.Kind != lsp.SymbolKindField {
			t.Errorf("field %q has kind %d, want Field", field.Name, field.Kind)
		}
	}
	if order.Children[1].Detail != "Money" {
		t.Errorf("field `total` detail = %q, want its declared type \"Money\"",
			order.Children[1].Detail)
	}

	// Union variants nest under the union.
	animal := byName["Animal"]
	if len(animal.Children) != 2 {
		t.Fatalf("Animal has %d children, want its 2 variants", len(animal.Children))
	}
	if animal.Children[0].Kind != lsp.SymbolKindEnumMember {
		t.Errorf("union variant has kind %d, want EnumMember", animal.Children[0].Kind)
	}
}

// TestDocumentSymbolRangesAreWellFormed pins the invariant the protocol requires: the
// selection range must sit inside the full range. Editors misbehave badly otherwise.
func TestDocumentSymbolRangesAreWellFormed(t *testing.T) {
	src := `type Order struct {
    id Number
}
let compute = fn(x: Number) -> Number {
    return x
}
`
	_, s, uri := openInline(t, "main.caja", src)

	symbols, err := s.DocumentSymbol(uri)
	if err != nil {
		t.Fatalf("DocumentSymbol: %v", err)
	}
	if len(symbols) == 0 {
		t.Fatal("no symbols returned")
	}

	var check func(sym lsp.DocumentSymbol)
	check = func(sym lsp.DocumentSymbol) {
		if !rangeContains(sym.Range, sym.SelectionRange) {
			t.Errorf("%q: selection range %v is not inside range %v",
				sym.Name, sym.SelectionRange, sym.Range)
		}
		if sym.Name == "" {
			t.Error("a symbol has an empty name, which the protocol forbids")
		}
		for _, child := range sym.Children {
			check(child)
		}
	}
	for _, sym := range symbols {
		check(sym)
	}

	// `compute` spans its whole body, not just its first line.
	for _, sym := range symbols {
		if sym.Name == "compute" && sym.Range.End.Line <= sym.Range.Start.Line {
			t.Errorf("compute's range %v does not span its multi-line body", sym.Range)
		}
	}
}

func rangeContains(outer, inner lsp.Range) bool {
	if inner.Start.Line < outer.Start.Line || inner.End.Line > outer.End.Line {
		return false
	}
	if inner.Start.Line == outer.Start.Line && inner.Start.Character < outer.Start.Character {
		return false
	}
	if inner.End.Line == outer.End.Line && inner.End.Character > outer.End.Character {
		return false
	}
	return true
}

// TestFoldingRangeCoversBodiesAndImports checks that the regions an editor can collapse
// line up with the constructs a reader would want collapsed.
func TestFoldingRangeCoversBodiesAndImports(t *testing.T) {
	src := `import "array"
import "math"
import "log"
type Order struct {
    id Number
    total Number
}
let compute = fn(x: Number) -> Number {
    return x
}
let inline = 5
`
	_, s, uri := openInline(t, "main.caja", src)

	ranges, err := s.FoldingRange(uri)
	if err != nil {
		t.Fatalf("FoldingRange: %v", err)
	}

	var hasImports bool
	for _, r := range ranges {
		if r.Kind != nil && *r.Kind == lsp.FoldingRangeKindImports {
			hasImports = true
			if r.StartLine != 0 || r.EndLine != 2 {
				t.Errorf("import fold covers lines %d..%d, want 0..2", r.StartLine, r.EndLine)
			}
		}
		if r.EndLine <= r.StartLine {
			t.Errorf("fold %d..%d collapses nothing", r.StartLine, r.EndLine)
		}
	}
	if !hasImports {
		t.Error("the run of imports at the top of the file is not foldable")
	}

	if !coversLines(ranges, 3, 6) {
		t.Error("the struct body is not foldable")
	}
	if !coversLines(ranges, 7, 9) {
		t.Error("the function body is not foldable")
	}
}

func coversLines(ranges []lsp.FoldingRange, start, end int) bool {
	for _, r := range ranges {
		if r.StartLine == start && r.EndLine == end {
			return true
		}
	}
	return false
}

// TestSelectionRangeExpandsOutward checks "expand selection": from a position, each step
// must grow, and every range must sit inside its parent.
func TestSelectionRangeExpandsOutward(t *testing.T) {
	src := `let compute = fn(x: Number) -> Number {
    return x + 1
}
`
	_, s, uri := openInline(t, "main.caja", src)

	// On `x` inside the return expression.
	ranges, err := s.SelectionRange(uri, []lsp.Position{{Line: 1, Character: 11}})
	if err != nil {
		t.Fatalf("SelectionRange: %v", err)
	}
	if len(ranges) != 1 {
		t.Fatalf("got %d selection ranges, want one per requested position", len(ranges))
	}

	depth := 0
	for node := &ranges[0]; node.Parent != nil; node = node.Parent {
		depth++
		if !rangeContains(node.Parent.Range, node.Range) {
			t.Fatalf("range %v is not contained by its parent %v", node.Range, node.Parent.Range)
		}
	}

	if depth < 2 {
		t.Errorf("expansion chain is %d deep; expected to grow through the expression, "+
			"the return statement and the function body", depth)
	}
}

// TestStructureRequestsOnUnopenedDocument guards the empty case: a request for a document
// the server has never analyzed must answer cleanly rather than erroring or panicking.
func TestStructureRequestsOnUnopenedDocument(t *testing.T) {
	h := NewCajaHandler()
	uri := lsp.DocumentURI("file:///nowhere/missing.caja")
	ctx := t.Context()

	if syms, err := h.DocumentSymbol(ctx, &lsp.DocumentSymbolParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
	}); err != nil || len(syms) != 0 {
		t.Errorf("DocumentSymbol on an unopened document = (%v, %v), want (empty, nil)", syms, err)
	}

	if folds, err := h.FoldingRange(ctx, &lsp.FoldingRangeParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
	}); err != nil || len(folds) != 0 {
		t.Errorf("FoldingRange on an unopened document = (%v, %v), want (empty, nil)", folds, err)
	}

	if sel, err := h.SelectionRange(ctx, &lsp.SelectionRangeParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
		Positions:    []lsp.Position{{Line: 0, Character: 0}},
	}); err != nil || len(sel) != 0 {
		t.Errorf("SelectionRange on an unopened document = (%v, %v), want (empty, nil)", sel, err)
	}
}

// TestFoldingRangesAreNotDuplicated guards against stacked chevrons in the gutter: a
// function literal and its body cover the same lines, and reporting both is redundant.
func TestFoldingRangesAreNotDuplicated(t *testing.T) {
	src := `let compute = fn(x: Number) -> Number {
    return x
}
`
	_, s, uri := openInline(t, "main.caja", src)

	ranges, err := s.FoldingRange(uri)
	if err != nil {
		t.Fatalf("FoldingRange: %v", err)
	}

	seen := make(map[[2]int]int)
	for _, r := range ranges {
		seen[[2]int{r.StartLine, r.EndLine}]++
	}
	for lines, count := range seen {
		if count > 1 {
			t.Errorf("lines %d..%d reported as %d separate folds", lines[0], lines[1], count)
		}
	}
}
