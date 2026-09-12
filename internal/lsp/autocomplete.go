package lsp

import (
	"caja-cli/internal/pipeline/analyzer"
	"caja-cli/internal/pipeline/analyzer/symbol"
	"caja-cli/internal/pipeline/ast"
	"caja-cli/internal/pipeline/lexer"
	"context"
	"sort"
	"strings"

	"github.com/owenrumney/go-lsp/lsp"
)

func (h *CajaHandler) Completion(_ context.Context, params *lsp.CompletionParams) (res *lsp.CompletionList, err error) {
	defer recoverInto("completion", params.TextDocument.URI, &res, &err)

	h.mu.RLock()
	state, stateOk := h.astCache[params.TextDocument.URI]
	text, textOk := h.docs.Text(params.TextDocument.URI)
	h.mu.RUnlock()

	if !stateOk || state == nil || state.Prog == nil || !textOk {
		return &lsp.CompletionList{}, nil
	}

	var items []lsp.CompletionItem

	// Find the exact line and column in the text to see if we are in a property access (e.g. `foo.`)
	lines := strings.Split(text, "\n")
	if params.Position.Line < len(lines) {
		lineText := lines[params.Position.Line]
		col := params.Position.Character
		if col > len(lineText) {
			col = len(lineText)
		}

		// extract text before cursor
		prefix := lineText[:col]

		// If the user typed a dot, we might be completing properties of an object
		if lastDotIdx := strings.LastIndex(prefix, "."); lastDotIdx != -1 {
			// Extract the object identifier part before the dot
			// For simplicity, just read backwards to get a valid identifier
			objName := extractIdentifierBackwards(prefix[:lastDotIdx])

			if objName != "" {
				// Find this object in the AST/Analyzer
				// We can try to find the variable in the global environment or analyzer cache
				if sym, ok := resolveSymbolByName(state, objName, params.Position.Line, params.Position.Character); ok {
					if members, found := memberCompletions(sym); found {
						return &lsp.CompletionList{Items: members}, nil
					}
				}
			}
		}
	}

	// Suggest Keywords
	for _, kw := range lexer.GetKeywords() {
		items = append(items, lsp.CompletionItem{
			Label: kw,
			Kind:  kindPtr(lsp.CompletionItemKindKeyword),
		})
	}

	// Suggest Variables in Scope
	vars := GetVariablesInScope(state.Prog, state.Analyzer, params.Position.Line, params.Position.Character)
	for _, v := range vars {
		items = append(items, lsp.CompletionItem{
			Label: v,
			Kind:  kindPtr(lsp.CompletionItemKindVariable),
		})
	}

	return &lsp.CompletionList{
		IsIncomplete: false,
		Items:        items,
	}, nil
}

func extractIdentifierBackwards(text string) string {
	text = strings.TrimSpace(text)
	if len(text) == 0 {
		return ""
	}
	end := len(text)
	start := end
	for i := end - 1; i >= 0; i-- {
		c := text[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			start = i
		} else {
			break
		}
	}
	return text[start:end]
}

func resolveSymbolByName(state *DocumentState, name string, line, col int) (symbol.Symbol, bool) {
	// First check global env
	if state.Analyzer != nil {

		declNode := findDeclarationNode(state.Prog, name, line, col)
		if declNode != nil {
			if sym, ok := state.Analyzer.GetSymbol(declNode); ok {
				return sym, true
			}
		}
	}
	return nil, false
}

// scopeChainAt returns the lexical scopes enclosing a position, outermost first. Because
// inner scopes come last, a caller that simply keeps overwriting as it walks the chain
// gets shadowing for free.
//
// This replaces two near-identical hand-written walkers. Both descended only when
// containsPosition said a statement covered the cursor — but containsPosition answered
// only for terminal token nodes and returned false for every statement type, and the
// callers passed it 0-indexed coordinates it compared against 1-indexed tokens. Between
// the two, descent never happened: completion inside a function body never saw the
// function's own parameters.
//
// It deliberately does not reuse PathAt either. Completion runs on half-typed source —
// `return u.` is not yet a parseable expression — so the cursor routinely sits past the
// end of everything that parsed, and a strict containment walk would stop at the root and
// report no enclosing function at all. When no child contains the position, this descends
// into the last one that begins before it, which is where the user is still typing.
func scopeChainAt(root ast.Node, line, byteCol int) []ast.Node {
	var chain []ast.Node

	for node := root; node != nil; {
		switch node.(type) {
		case *ast.Program, *ast.BlockStatement, *ast.FunctionLiteral:
			chain = append(chain, node)
		}

		var next ast.Node
		for _, child := range ast.Children(node) {
			if !startsAtOrBefore(child, line, byteCol) {
				continue
			}
			next = child
			if spansPosition(child, line+1, byteCol+1) {
				break // a child that really contains the cursor always wins
			}
		}
		node = next
	}

	return chain
}

// statementsOf returns the statements a scope introduces bindings through.
func statementsOf(scope ast.Node) []ast.Statement {
	switch s := scope.(type) {
	case *ast.Program:
		return s.Statements
	case *ast.BlockStatement:
		return s.Statements
	default:
		return nil
	}
}

// startsAtOrBefore reports whether a node begins at or before a 0-indexed position. Only
// bindings already introduced at the cursor are in scope for it.
func startsAtOrBefore(node ast.Node, line, byteCol int) bool {
	tok := GetNodeToken(node)
	if tok.Line == 0 {
		return false
	}
	if tok.Line != line+1 {
		return tok.Line < line+1
	}
	return tok.Column <= byteCol+1
}

// findDeclarationNode locates the declaration that binds name at a position, preferring
// the innermost scope that declares it.
func findDeclarationNode(node ast.Node, name string, line, col int) ast.Node {
	var decl ast.Node

	for _, scope := range scopeChainAt(node, line, col) {
		// A function's parameters are bindings too. Resolving them is what lets
		// dot-completion work on a parameter — previously impossible, and called out as
		// such by a comment in the walker this replaces.
		if fn, ok := scope.(*ast.FunctionLiteral); ok {
			for _, param := range fn.Parameters {
				if param.Name == name {
					decl = param
				}
			}
		}

		for _, stmt := range statementsOf(scope) {
			if isDeclOf(stmt, name) && startsAtOrBefore(stmt, line, col) {
				decl = stmt
			}
		}
	}

	return decl
}

func isDeclOf(stmt ast.Statement, name string) bool {
	switch s := stmt.(type) {
	case *ast.LetStatement:
		return s.Name.Value == name
	case *ast.ConstStatement:
		return s.Name.Value == name
	case *ast.ImportStatement:
		if s.Name != nil && s.Name.Value == name {
			return true
		}
		for _, named := range s.NamedImports {
			if named.Value == name {
				return true
			}
		}
	}
	return false
}

// GetVariablesInScope returns the names visible at a position, innermost scope last so
// that a shadowed outer name is only listed once.
func GetVariablesInScope(node ast.Node, a *analyzer.Analyzer, line, col int) []string {
	var vars []string
	seen := make(map[string]bool)

	for _, scope := range scopeChainAt(node, line, col) {
		if fn, ok := scope.(*ast.FunctionLiteral); ok {
			for _, param := range fn.Parameters {
				if !seen[param.Name] {
					vars = append(vars, param.Name)
					seen[param.Name] = true
				}
			}
		}

		for _, stmt := range statementsOf(scope) {
			if startsAtOrBefore(stmt, line, col) {
				addDecl(stmt, a, &vars, seen)
			}
		}
	}

	return vars
}

func addDecl(stmt ast.Statement, a *analyzer.Analyzer, vars *[]string, seen map[string]bool) {
	switch s := stmt.(type) {
	case *ast.LetStatement:
		if s.Name != nil && !seen[s.Name.Value] {
			*vars = append(*vars, s.Name.Value)
			seen[s.Name.Value] = true
		}
	case *ast.ConstStatement:
		if s.Name != nil && !seen[s.Name.Value] {
			*vars = append(*vars, s.Name.Value)
			seen[s.Name.Value] = true
		}
	case *ast.ImportStatement:
		if s.Name != nil && !seen[s.Name.Value] {
			*vars = append(*vars, s.Name.Value)
			seen[s.Name.Value] = true
		}
		for _, named := range s.NamedImports {
			if !seen[named.Value] {
				*vars = append(*vars, named.Value)
				seen[named.Value] = true
			}
		}
		if s.IsWildcard {
			addWildcardDecls(s, a, vars, seen)
		}
	}
}

// addWildcardDecls offers the members of a `import * from mod` statement as
// bare completions. Unlike every other case in addDecl these names have no AST
// node to read, so they come from the module's symbol table instead. Names two
// wildcards both bound are skipped, since accepting one would only produce an
// ambiguity error.
func addWildcardDecls(s *ast.ImportStatement, a *analyzer.Analyzer, vars *[]string, seen map[string]bool) {
	modSym, ok := wildcardModuleSymbol(s, a)
	if !ok {
		return
	}

	add := func(name string) {
		if seen[name] || modSym.IsPrivate(name) {
			return
		}
		if a != nil {
			if _, ambiguous := a.AmbiguousWildcardModules(name); ambiguous {
				return
			}
		}
		*vars = append(*vars, name)
		seen[name] = true
	}

	for name, sym := range modSym.GetSymbols() {
		// Module aliases are never wildcard-imported (see bindWildcardImport).
		if _, isModule := sym.(*symbol.ModuleSymbol); isModule {
			continue
		}
		add(name)
	}
	for name := range modSym.GetTypes() {
		add(name)
	}
}

// wildcardModuleSymbol resolves the module a wildcard import refers to,
// preferring the analyzer's own binding (which covers user modules as well as
// builtins) and falling back to the standard-module table so completions still
// work in a buffer that has not analyzed cleanly.
func wildcardModuleSymbol(s *ast.ImportStatement, a *analyzer.Analyzer) (*symbol.ModuleSymbol, bool) {
	if a != nil && s.Name != nil {
		if entry, ok := a.GlobalScope()[s.Name.Value]; ok {
			if modSym, isMod := entry.Sym.(*symbol.ModuleSymbol); isMod {
				return modSym, true
			}
		}
	}
	if symbols, types, ok := symbol.GetStandardModule(s.Path); ok {
		return symbol.NewModuleSymbol(s.Path, symbols, types, nil, nil, nil, s.Path), true
	}
	return nil, false
}

// We also need to map CompletionItemKind
// We will use int pointers for lsp.CompletionItemKind
// lsp.CompletionItemKindKeyword = 14
// lsp.CompletionItemKindVariable = 6
// lsp.CompletionItemKindProperty = 10
// lsp.CompletionItemKindField = 5

func kindPtr(k lsp.CompletionItemKind) *lsp.CompletionItemKind {
	return &k
}

// memberCompletions returns the members reachable through `.` on a symbol, and whether
// the symbol is one that has members at all. A false result means "not a receiver",
// which the caller answers with ordinary scope completion rather than an empty list.
//
// The wrapper kinds must be unwrapped rather than rejected: a nullable or active binding
// of a struct still has that struct's fields, and stopping at the wrapper is why
// dot-completion used to go silent on them.
func memberCompletions(sym symbol.Symbol) ([]lsp.CompletionItem, bool) {
	switch s := sym.(type) {
	case *symbol.NullableSymbol:
		return memberCompletions(s.Underlying)
	case *symbol.ActiveSymbol:
		return memberCompletions(s.Underlying)
	case *symbol.ConstraintSymbol:
		// A refinement type is its base type as far as members go.
		return memberCompletions(s.BaseType)

	case *symbol.ModuleSymbol:
		return moduleMembers(s), true

	// Both the instance and the definition appear as receivers: a local bound to a struct
	// literal resolves to the instance, while a function parameter declared with a struct
	// type resolves to the definition.
	case *symbol.StructInstanceSymbol:
		return structFieldItems(s.Def), true
	case *symbol.StructDefSymbol:
		return structFieldItems(s), true

	default:
		return nil, false
	}
}

func moduleMembers(mod *symbol.ModuleSymbol) []lsp.CompletionItem {
	items := make([]lsp.CompletionItem, 0, len(mod.GetSymbols()))
	for name := range mod.GetSymbols() {
		items = append(items, lsp.CompletionItem{
			Label: name,
			Kind:  kindPtr(lsp.CompletionItemKindProperty),
		})
	}
	sortItems(items)
	return items
}

func structFieldItems(def *symbol.StructDefSymbol) []lsp.CompletionItem {
	if def == nil {
		return nil
	}
	items := make([]lsp.CompletionItem, 0, len(def.Fields))
	for name, field := range def.Fields {
		item := lsp.CompletionItem{
			Label: name,
			Kind:  kindPtr(lsp.CompletionItemKindField),
		}
		if field.Type != nil {
			item.Detail = field.Type.String()
		}
		items = append(items, item)
	}
	sortItems(items)
	return items
}

// sortItems gives the list a stable order. The symbol tables are Go maps, so without this
// the same keystroke can produce a differently-ordered list each time.
func sortItems(items []lsp.CompletionItem) {
	sort.Slice(items, func(i, j int) bool { return items[i].Label < items[j].Label })
}
