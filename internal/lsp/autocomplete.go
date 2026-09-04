package lsp

import (
	"caja-cli/internal/pipeline/analyzer/symbol"
	"caja-cli/internal/pipeline/ast"
	"caja-cli/internal/pipeline/lexer"
	"context"
	"fmt"
	"strings"

	"github.com/owenrumney/go-lsp/lsp"
)

func (h *CajaHandler) Completion(_ context.Context, params *lsp.CompletionParams) (*lsp.CompletionList, error) {
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
					if modSym, isMod := sym.(*symbol.ModuleSymbol); isMod {
						for expName := range modSym.GetSymbols() {
							items = append(items, lsp.CompletionItem{
								Label: expName,
								Kind:  kindPtr(lsp.CompletionItemKindProperty),
							})
						}
						return &lsp.CompletionList{Items: items}, nil
					} else if structSym, isStruct := sym.(*symbol.StructInstanceSymbol); isStruct {
						for fieldName := range structSym.Def.Fields {
							items = append(items, lsp.CompletionItem{
								Label: fieldName,
								Kind:  kindPtr(lsp.CompletionItemKindField),
							})
						}
						return &lsp.CompletionList{Items: items}, nil
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
	vars := GetVariablesInScope(state.Prog, params.Position.Line, params.Position.Character)
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
				fmt.Printf("resolved symbol: %T\n", sym)
				return sym, true
			}
		}
	}
	return nil, false
}

func findDeclarationNode(node ast.Node, name string, line, col int) ast.Node {
	var decl ast.Node
	var walk func(n ast.Node)
	walk = func(n ast.Node) {
		if n == nil {
			return
		}

		switch nodeType := n.(type) {
		case *ast.Program:
			for _, stmt := range nodeType.Statements {
				if isDeclOf(stmt, name) {
					if stmtToken := GetNodeToken(stmt); stmtToken.Line <= line+1 { // 1-indexed
						decl = stmt
					}
				}
				if containsPosition(stmt, line, col) {
					walk(stmt)
				}
			}
		case *ast.BlockStatement:
			for _, stmt := range nodeType.Statements {
				if isDeclOf(stmt, name) {
					if stmtToken := GetNodeToken(stmt); stmtToken.Line <= line+1 { // 1-indexed
						decl = stmt
					}
				}
				if containsPosition(stmt, line, col) {
					walk(stmt)
				}
			}
		case *ast.FunctionLiteral:
			// No way to get param symbol directly from nodeSymbols since it's just a string,
			// but we can return the function node itself maybe? No.
			if containsPosition(nodeType.Body, line, col) {
				walk(nodeType.Body)
			}
		case *ast.LetStatement:
			if containsPosition(nodeType.Value, line, col) {
				walk(nodeType.Value)
			}
		case *ast.ConstStatement:
			if containsPosition(nodeType.Value, line, col) {
				walk(nodeType.Value)
			}
		case *ast.IfExpression:
			if containsPosition(nodeType.Condition, line, col) {
				walk(nodeType.Condition)
			} else if containsPosition(nodeType.Consequence, line, col) {
				walk(nodeType.Consequence)
			} else if containsPosition(nodeType.Alternative, line, col) {
				walk(nodeType.Alternative)
			}
		}
	}
	walk(node)
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

func GetVariablesInScope(node ast.Node, line, col int) []string {
	var vars []string

	// Helper to track uniqueness
	seen := make(map[string]bool)

	var walk func(n ast.Node)
	walk = func(n ast.Node) {
		if n == nil {
			return
		}

		switch nodeType := n.(type) {
		case *ast.Program:
			for _, stmt := range nodeType.Statements {
				stmtToken := GetNodeToken(stmt)
				// 1-indexed vs 0-indexed: line is 0-indexed, stmtToken.Line is 1-indexed
				if stmtToken.Line < line+1 || (stmtToken.Line == line+1 && stmtToken.Column <= col+1) {
					addDecl(stmt, &vars, seen)
				}
				if containsPosition(stmt, line, col) {
					walk(stmt)
				}
			}
		case *ast.BlockStatement:
			for _, stmt := range nodeType.Statements {
				stmtToken := GetNodeToken(stmt)
				if stmtToken.Line < line+1 || (stmtToken.Line == line+1 && stmtToken.Column <= col+1) {
					addDecl(stmt, &vars, seen)
				}
				if containsPosition(stmt, line, col) {
					walk(stmt)
				}
			}
		case *ast.FunctionLiteral:
			for _, param := range nodeType.Parameters {
				if !seen[param.Name] {
					vars = append(vars, param.Name)
					seen[param.Name] = true
				}
			}
			if containsPosition(nodeType.Body, line, col) {
				walk(nodeType.Body)
			}
		case *ast.LetStatement:
			if containsPosition(nodeType.Value, line, col) {
				walk(nodeType.Value)
			}
		case *ast.ConstStatement:
			if containsPosition(nodeType.Value, line, col) {
				walk(nodeType.Value)
			}
		case *ast.IfExpression:
			if containsPosition(nodeType.Condition, line, col) {
				walk(nodeType.Condition)
			} else if containsPosition(nodeType.Consequence, line, col) {
				walk(nodeType.Consequence)
			} else if containsPosition(nodeType.Alternative, line, col) {
				walk(nodeType.Alternative)
			}
		}
	}
	walk(node)
	return vars
}

func addDecl(stmt ast.Statement, vars *[]string, seen map[string]bool) {
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
	}
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
