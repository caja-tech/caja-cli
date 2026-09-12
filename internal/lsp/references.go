package lsp

import (
	"context"
	"fmt"
	"slices"

	"caja-cli/internal/pipeline/ast"
	"caja-cli/internal/pipeline/lexer"

	"github.com/owenrumney/go-lsp/lsp"
)

// bindingKey identifies the thing a name refers to, by where it was declared. The
// analyzer records a definition token per referencing node but keeps no reverse index, so
// grouping nodes by their resolved definition is what turns "go to definition" into "find
// every other name that resolves to the same place".
type bindingKey struct {
	file   string
	line   int
	column int
}

func (k bindingKey) valid() bool { return k.line != 0 }

// bindingAt returns the binding the name under the cursor refers to, and the node itself.
func bindingAt(state *DocumentState, prog ast.Node, line, byteCol int) (bindingKey, ast.Node, bool) {
	node := FindNodeAtPosition(prog, line, byteCol)
	if node == nil || !nameable(node) {
		return bindingKey{}, nil, false
	}

	key, ok := bindingOf(state, node)
	return key, node, ok
}

// bindingOf resolves a node to the declaration it refers to.
func bindingOf(state *DocumentState, node ast.Node) (bindingKey, bool) {
	tok, file, ok := state.Analyzer.GetDefinition(node)
	if !ok || tok.Line == 0 {
		return bindingKey{}, false
	}
	return bindingKey{file: file, line: tok.Line, column: tok.Column}, true
}

// nameable reports whether a node is the kind of thing that has references at all.
// Literals and operators are not; identifiers and type references are.
func nameable(node ast.Node) bool {
	switch node.(type) {
	case *ast.Identifier, *ast.TypeRef:
		return true
	default:
		return false
	}
}

// occurrencesOf collects every node in the document that refers to the same binding.
func occurrencesOf(state *DocumentState, key bindingKey) []ast.Node {
	if !key.valid() {
		return nil
	}

	var found []ast.Node
	ast.Inspect(state.Prog, func(n ast.Node) bool {
		if !nameable(n) {
			return true
		}
		if other, ok := bindingOf(state, n); ok && other == key {
			found = append(found, n)
		}
		return true
	})
	return found
}

// References answers "find all references". Results are confined to the current document:
// the server keeps no workspace index, so a name used in another file that imports this
// one cannot be seen, and reporting a partial cross-file answer as if it were complete
// would be worse than scoping it honestly to one file.
func (h *CajaHandler) References(_ context.Context, params *lsp.ReferenceParams) (res []lsp.Location, err error) {
	defer recoverInto("references", params.TextDocument.URI, &res, &err)

	state, ix, ok := h.documentFor(params.TextDocument.URI)
	if !ok || state.Analyzer == nil {
		return nil, nil
	}

	key, _, found := bindingAt(state, state.Prog, params.Position.Line, ix.ByteColumn(params.Position))
	if !found {
		return nil, nil
	}

	locations := make([]lsp.Location, 0)
	for _, node := range occurrencesOf(state, key) {
		if !params.Context.IncludeDeclaration && isDeclarationOf(node, key) {
			continue
		}
		locations = append(locations, lsp.Location{
			URI:   params.TextDocument.URI,
			Range: ix.TokenRange(ast.StartToken(node)),
		})
	}
	return locations, nil
}

// isDeclarationOf reports whether a node is the declaring occurrence rather than a use.
// A declaration's recorded definition is its own token, which is what distinguishes it.
func isDeclarationOf(node ast.Node, key bindingKey) bool {
	tok := ast.StartToken(node)
	return tok.Line == key.line && tok.Column == key.column
}

// DocumentHighlight marks the other occurrences of the name under the cursor, which is
// what makes every use of a variable light up when you put the caret on one of them.
func (h *CajaHandler) DocumentHighlight(_ context.Context, params *lsp.DocumentHighlightParams) (res []lsp.DocumentHighlight, err error) {
	defer recoverInto("documentHighlight", params.TextDocument.URI, &res, &err)

	state, ix, ok := h.documentFor(params.TextDocument.URI)
	if !ok || state.Analyzer == nil {
		return nil, nil
	}

	key, _, found := bindingAt(state, state.Prog, params.Position.Line, ix.ByteColumn(params.Position))
	if !found {
		return nil, nil
	}

	writes := writeTargets(state.Prog)

	highlights := make([]lsp.DocumentHighlight, 0)
	for _, node := range occurrencesOf(state, key) {
		kind := lsp.DocumentHighlightKindRead
		if writes[node] {
			kind = lsp.DocumentHighlightKindWrite
		}
		highlights = append(highlights, lsp.DocumentHighlight{
			Range: ix.TokenRange(ast.StartToken(node)),
			Kind:  &kind,
		})
	}
	return highlights, nil
}

// writeTargets collects the name nodes that are being assigned to rather than read, so
// the editor can shade writes differently from reads. A declaration counts as a write:
// it is where the value first arrives.
func writeTargets(prog ast.Node) map[ast.Node]bool {
	writes := make(map[ast.Node]bool)

	ast.Inspect(prog, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.LetStatement:
			writes[s.Name] = true
		case *ast.ConstStatement:
			writes[s.Name] = true
		case *ast.AssignStatement:
			writes[s.Name] = true
		case *ast.PropertyAssignmentStatement:
			writes[s.Property] = true
		}
		return true
	})
	return writes
}

// PrepareRename validates that the thing under the cursor can be renamed and tells the
// editor which span to pre-fill, so the rename box opens on the name rather than on
// whatever text happens to surround the caret.
func (h *CajaHandler) PrepareRename(_ context.Context, params *lsp.PrepareRenameParams) (res *lsp.PrepareRenameResult, err error) {
	defer recoverInto("prepareRename", params.TextDocument.URI, &res, &err)

	state, ix, ok := h.documentFor(params.TextDocument.URI)
	if !ok || state.Analyzer == nil {
		return nil, nil
	}

	key, node, found := bindingAt(state, state.Prog, params.Position.Line, ix.ByteColumn(params.Position))
	if !found {
		return nil, nil
	}

	// A name declared in another file cannot be renamed from here: the edits would have
	// to reach a document this server has not indexed, and a rename that silently updates
	// only some uses is worse than one that declines.
	if declaredElsewhere(key, params.TextDocument.URI) {
		return nil, fmt.Errorf("cannot rename %q: it is declared in another file", nameOf(node))
	}

	tok := ast.StartToken(node)
	return &lsp.PrepareRenameResult{
		Range:       ix.TokenRange(tok),
		Placeholder: nameOf(node),
	}, nil
}

// Rename rewrites every occurrence of a binding in the current document.
func (h *CajaHandler) Rename(_ context.Context, params *lsp.RenameParams) (res *lsp.WorkspaceEdit, err error) {
	defer recoverInto("rename", params.TextDocument.URI, &res, &err)

	state, ix, ok := h.documentFor(params.TextDocument.URI)
	if !ok || state.Analyzer == nil {
		return nil, nil
	}

	key, node, found := bindingAt(state, state.Prog, params.Position.Line, ix.ByteColumn(params.Position))
	if !found {
		return nil, nil
	}
	if declaredElsewhere(key, params.TextDocument.URI) {
		return nil, fmt.Errorf("cannot rename %q: it is declared in another file", nameOf(node))
	}
	if err := validateNewName(state, node, params.NewName); err != nil {
		return nil, err
	}

	occurrences := occurrencesOf(state, key)
	if len(occurrences) == 0 {
		return nil, nil
	}

	edits := make([]lsp.TextEdit, 0, len(occurrences))
	for _, occurrence := range occurrences {
		edits = append(edits, lsp.TextEdit{
			Range:   ix.TokenRange(ast.StartToken(occurrence)),
			NewText: params.NewName,
		})
	}

	return &lsp.WorkspaceEdit{
		Changes: map[lsp.DocumentURI][]lsp.TextEdit{params.TextDocument.URI: edits},
	}, nil
}

// declaredElsewhere reports whether a binding's declaration lives outside this document.
func declaredElsewhere(key bindingKey, uri lsp.DocumentURI) bool {
	return key.file != "" && key.file != uriToPath(string(uri))
}

func nameOf(node ast.Node) string {
	switch n := node.(type) {
	case *ast.Identifier:
		return n.Value
	case *ast.TypeRef:
		return n.Name
	default:
		return ast.StartToken(node).Literal
	}
}

// validateNewName rejects replacements that would not parse back as the same kind of
// name. Letting a rename introduce a keyword or a lowercase type would turn a refactor
// into a syntax or semantic error across every use at once.
func validateNewName(state *DocumentState, node ast.Node, newName string) error {
	if newName == "" {
		return fmt.Errorf("the new name is empty")
	}
	if !isIdentifier(newName) {
		return fmt.Errorf("%q is not a valid Caja identifier", newName)
	}
	if slices.Contains(lexer.GetKeywords(), newName) {
		return fmt.Errorf("%q is a keyword", newName)
	}
	if isTypeName(state, node) && !startsUpper(newName) {
		return fmt.Errorf("type names must start with a capital letter, so %q will not do", newName)
	}
	return nil
}

// isTypeName reports whether a node names a type, whether it is a reference inside an
// annotation or the declaring occurrence itself.
func isTypeName(state *DocumentState, node ast.Node) bool {
	if _, isRef := node.(*ast.TypeRef); isRef {
		return true
	}
	ident, isIdent := node.(*ast.Identifier)
	return isIdent && state.Analyzer != nil && state.Analyzer.IsDeclaredType(ident.Value)
}

func isIdentifier(s string) bool {
	for i := range len(s) {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c == '_':
		case c >= '0' && c <= '9' && i > 0:
		default:
			return false
		}
	}
	return true
}

func startsUpper(s string) bool { return s != "" && s[0] >= 'A' && s[0] <= 'Z' }
