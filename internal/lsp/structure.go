package lsp

import (
	"context"

	"caja-cli/internal/lsp/posmap"
	"caja-cli/internal/pipeline/ast"

	"github.com/owenrumney/go-lsp/lsp"
)

// DocumentSymbol builds the outline shown in the editor's breadcrumb bar and symbol list.
//
// Only top-level declarations become symbols. Locals inside a function body are
// deliberately left out: an outline that lists every `let` in every body stops being a
// map of the file, and Caja has no loops, so bodies are often long chains of bindings.
func (h *CajaHandler) DocumentSymbol(_ context.Context, params *lsp.DocumentSymbolParams) (res []lsp.DocumentSymbol, err error) {
	defer recoverInto("documentSymbol", params.TextDocument.URI, &res, &err)

	state, ix, ok := h.documentFor(params.TextDocument.URI)
	if !ok {
		return nil, nil
	}

	symbols := make([]lsp.DocumentSymbol, 0, len(state.Prog.Statements))
	for _, stmt := range state.Prog.Statements {
		if sym, produced := declarationSymbol(ix, stmt); produced {
			symbols = append(symbols, sym)
		}
	}
	return symbols, nil
}

// declarationSymbol converts one top-level statement into an outline entry.
func declarationSymbol(ix *posmap.LineIndex, stmt ast.Statement) (lsp.DocumentSymbol, bool) {
	switch s := stmt.(type) {
	case *ast.LetStatement:
		// A binding whose value is a function reads as a function in the outline, which
		// is how it reads in the source too — Caja has no separate `func` declaration.
		kind := lsp.SymbolKindVariable
		if _, isFn := s.Value.(*ast.FunctionLiteral); isFn {
			kind = lsp.SymbolKindFunction
		}
		return newSymbol(ix, s.Name.Value, declarationDetail(s.ValueType, s.Value), kind, s, s.Name), true

	case *ast.ConstStatement:
		return newSymbol(ix, s.Name.Value, declarationDetail(s.ValueType, s.Value), lsp.SymbolKindConstant, s, s.Name), true

	case *ast.TypeAliasStatement:
		sym := newSymbol(ix, s.Name.Value, s.String(), typeAliasKind(s), s, s.Name)
		sym.Children = structFieldSymbols(ix, s)
		return sym, true

	case *ast.UnionStatement:
		sym := newSymbol(ix, s.Name.Value, s.String(), lsp.SymbolKindEnum, s, s.Name)
		for _, variant := range s.Variants {
			sym.Children = append(sym.Children,
				newSymbol(ix, variant.Value, "", lsp.SymbolKindEnumMember, variant, variant))
		}
		return sym, true

	case *ast.TypeConstraintStatement:
		// A `define ... constraints ... with:` is a refinement of another type, which is
		// closest in spirit to an interface: a named set of values a type must satisfy.
		return newSymbol(ix, s.Name.Value, s.String(), lsp.SymbolKindInterface, s, s.Name), true

	case *ast.ImportStatement:
		if s.Name == nil {
			return lsp.DocumentSymbol{}, false // a wildcard import binds no single name
		}
		return newSymbol(ix, s.Name.Value, s.String(), lsp.SymbolKindModule, s, s.Name), true

	default:
		return lsp.DocumentSymbol{}, false
	}
}

func typeAliasKind(s *ast.TypeAliasStatement) lsp.SymbolKind {
	switch {
	case s.StructDefinition != nil:
		return lsp.SymbolKindStruct
	case s.Signature != nil:
		return lsp.SymbolKindFunction
	default:
		return lsp.SymbolKindClass // a plain alias, e.g. `type Money Number`
	}
}

func structFieldSymbols(ix *posmap.LineIndex, s *ast.TypeAliasStatement) []lsp.DocumentSymbol {
	if s.StructDefinition == nil {
		return nil
	}

	fields := make([]lsp.DocumentSymbol, 0, len(s.StructDefinition.Fields))
	for i := range s.StructDefinition.Fields {
		field := &s.StructDefinition.Fields[i]
		if field.Name == nil {
			continue
		}
		fields = append(fields,
			newSymbol(ix, field.Name.Value, field.Type.Text(), lsp.SymbolKindField, field.Name, field.Name))
	}
	return fields
}

// declarationDetail is the grey text beside a name in the outline: the declared type when
// there is one, and otherwise the inferred shape is left to hover rather than guessed at.
func declarationDetail(declared *ast.TypeExpr, value ast.Expression) string {
	if text := declared.Text(); text != "" {
		return text
	}
	if fn, ok := value.(*ast.FunctionLiteral); ok {
		return fn.String()
	}
	return ""
}

// newSymbol builds an outline entry. Range covers the whole declaration so the editor can
// tell when the cursor is inside it; SelectionRange covers just the name, which is what
// gets revealed and highlighted when the symbol is picked.
func newSymbol(ix *posmap.LineIndex, name, detail string, kind lsp.SymbolKind, whole, nameNode ast.Node) lsp.DocumentSymbol {
	start, end := ast.Span(whole)

	return lsp.DocumentSymbol{
		Name:           name,
		Detail:         detail,
		Kind:           kind,
		Range:          ix.SpanRange(start, end),
		SelectionRange: ix.TokenRange(ast.StartToken(nameNode)),
	}
}

// FoldingRange reports the regions an editor can collapse.
func (h *CajaHandler) FoldingRange(_ context.Context, params *lsp.FoldingRangeParams) (res []lsp.FoldingRange, err error) {
	defer recoverInto("foldingRange", params.TextDocument.URI, &res, &err)

	state, _, ok := h.documentFor(params.TextDocument.URI)
	if !ok {
		return nil, nil
	}

	ranges := make([]lsp.FoldingRange, 0)
	// Nested constructs routinely share a span — a function literal and its body cover
	// exactly the same lines — and duplicate folds show up as stacked, redundant chevrons
	// in the gutter.
	seen := make(map[[2]int]bool)

	ast.Inspect(state.Prog, func(n ast.Node) bool {
		if !foldable(n) {
			return true
		}
		start, end := ast.Span(n)
		// A region confined to one line has nothing to collapse.
		if start.Line == 0 || end.Line <= start.Line {
			return true
		}

		lines := [2]int{start.Line - 1, end.Line - 1}
		if seen[lines] {
			return true
		}
		seen[lines] = true

		ranges = append(ranges, lsp.FoldingRange{
			StartLine: lines[0],
			EndLine:   lines[1],
			Kind:      foldKind(lsp.FoldingRangeKindRegion),
		})
		return true
	})

	if imports := importFoldingRange(state.Prog); imports != nil {
		ranges = append(ranges, *imports)
	}
	return ranges, nil
}

// foldable reports whether a node delimits a region worth collapsing. Anything with a
// body or a bracketed list qualifies; expressions that merely happen to wrap do not,
// because a fold marker on every multi-line infix expression is noise.
func foldable(n ast.Node) bool {
	switch n.(type) {
	case *ast.BlockStatement, *ast.FunctionLiteral, *ast.TypeAliasStatement,
		*ast.UnionStatement, *ast.TypeConstraintStatement,
		*ast.ArrayLiteral, *ast.MapLiteral, *ast.StructLiteral,
		*ast.CallExpression, *ast.StreamPipeExpression, *ast.JoinGroupExpression:
		return true
	default:
		return false
	}
}

// importFoldingRange collapses the run of imports at the top of a file into one region.
// Caja requires imports to be first and contiguous, so a single span always covers them.
func importFoldingRange(prog *ast.Program) *lsp.FoldingRange {
	first, last := -1, -1
	for _, stmt := range prog.Statements {
		if _, isImport := stmt.(*ast.ImportStatement); !isImport {
			break
		}
		tok := ast.StartToken(stmt)
		if tok.Line == 0 {
			continue
		}
		if first == -1 {
			first = tok.Line
		}
		last = tok.Line
	}

	if first == -1 || last <= first {
		return nil
	}
	return &lsp.FoldingRange{
		StartLine: first - 1,
		EndLine:   last - 1,
		Kind:      foldKind(lsp.FoldingRangeKindImports),
	}
}

func foldKind(k lsp.FoldingRangeKind) *lsp.FoldingRangeKind { return &k }

// SelectionRange powers "expand selection": each step grows the selection to the next
// enclosing syntactic construct. The chain of nodes enclosing a position is exactly what
// PathAt already computes.
func (h *CajaHandler) SelectionRange(_ context.Context, params *lsp.SelectionRangeParams) (res []lsp.SelectionRange, err error) {
	defer recoverInto("selectionRange", params.TextDocument.URI, &res, &err)

	state, ix, ok := h.documentFor(params.TextDocument.URI)
	if !ok {
		return nil, nil
	}

	ranges := make([]lsp.SelectionRange, 0, len(params.Positions))
	for _, pos := range params.Positions {
		path := PathAt(state.Prog, pos.Line, ix.ByteColumn(pos))

		// Build outermost-first so each range can point at its parent, then hand back
		// the innermost, which is where the selection starts.
		var current *lsp.SelectionRange
		for _, node := range path {
			start, end := ast.Span(node)
			if start.Line == 0 {
				continue
			}
			current = &lsp.SelectionRange{Range: ix.SpanRange(start, end), Parent: current}
		}

		if current == nil {
			// No syntax covers the position; report the caret itself so the client has a
			// well-formed answer rather than a missing entry.
			current = &lsp.SelectionRange{Range: lsp.Range{Start: pos, End: pos}}
		}
		ranges = append(ranges, *current)
	}
	return ranges, nil
}

// documentFor fetches the cached analysis and coordinate index for a document, reporting
// false when the document has not been analyzed yet.
func (h *CajaHandler) documentFor(uri lsp.DocumentURI) (*DocumentState, *posmap.LineIndex, bool) {
	h.mu.RLock()
	state, ok := h.astCache[uri]
	h.mu.RUnlock()

	if !ok || state == nil || state.Prog == nil {
		return nil, nil, false
	}
	return state, h.indexFor(uri), true
}
