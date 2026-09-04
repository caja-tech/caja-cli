package analyzer

import (
	"caja-cli/internal/pipeline/analyzer/symbol"
	"caja-cli/internal/pipeline/lexer"
)

// ScopeEntry represents a symbol declared in a scope and tracks if it is a constant.
type ScopeEntry struct {
	Sym             symbol.Symbol
	IsConstant      bool
	FunctionDepth   int
	DefinitionToken lexer.Token
	IsImport        bool
	FilePath        string
	IsMoved         bool
}

// GlobalScope returns the top-level scope of this analyzer
func (a *Analyzer) GlobalScope() map[string]ScopeEntry {
	if len(a.scopes) > 0 {
		return a.scopes[0]
	}
	return make(map[string]ScopeEntry)
}

// pushScope creates a new inner scope and pushes it onto the scope stack.
func (a *Analyzer) pushScope() {
	a.scopes = append(a.scopes, make(map[string]ScopeEntry))
}

// popScope removes the most recently added inner scope from the scope stack.
func (a *Analyzer) popScope() {
	a.scopes = a.scopes[:len(a.scopes)-1]
}

// declare registers a variable name in the current (innermost) scope.
func (a *Analyzer) declare(name string, sym symbol.Symbol, isConstant bool, defToken lexer.Token) {
	last := len(a.scopes) - 1
	filePath := ""
	if a.globalEnv != nil {
		filePath = a.globalEnv.FileName
	}
	a.scopes[last][name] = ScopeEntry{Sym: sym, IsConstant: isConstant, FunctionDepth: a.functionDepth, DefinitionToken: defToken, FilePath: filePath}
}

// declareImport registers an imported variable name in the current scope.
func (a *Analyzer) declareImport(name string, sym symbol.Symbol, isConstant bool, defToken lexer.Token, filePath string) {
	last := len(a.scopes) - 1
    
	a.scopes[last][name] = ScopeEntry{Sym: sym, IsConstant: isConstant, FunctionDepth: a.functionDepth, DefinitionToken: defToken, IsImport: true, FilePath: filePath}
}

// GetGlobalType retrieves a type from the global type registry
func (a *Analyzer) GetGlobalType(name string) (symbol.Symbol, bool) {
	return a.findTypeSymbolInTypes(name)
}

// markVarMoved marks a variable as moved in its declaration scope.
func (a *Analyzer) markVarMoved(varName string) {
	for i := len(a.scopes) - 1; i >= 0; i-- {
		if entry, ok := a.scopes[i][varName]; ok {
			entry.IsMoved = true
			a.scopes[i][varName] = entry
			return
		}
	}
}

func (a *Analyzer) snapshotMovedVars() map[string]bool {
	snapshot := make(map[string]bool)
	for i := range a.scopes {
		for k, v := range a.scopes[i] {
			if v.IsMoved {
				snapshot[k] = true
			}
		}
	}
	return snapshot
}

func (a *Analyzer) restoreMovedVars(snapshot map[string]bool) {
	for i := range a.scopes {
		for k, v := range a.scopes[i] {
			_, shouldBeMoved := snapshot[k]
			v.IsMoved = shouldBeMoved
			a.scopes[i][k] = v
		}
	}
}

func (a *Analyzer) unionMovedVars(snapshot map[string]bool) {
	for i := range a.scopes {
		for k, v := range a.scopes[i] {
			if snapshot[k] {
				v.IsMoved = true
				a.scopes[i][k] = v
			}
		}
	}
}
