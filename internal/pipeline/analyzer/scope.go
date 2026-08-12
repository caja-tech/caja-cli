package analyzer

import "caja-cli/internal/pipeline/analyzer/symbol"

// ScopeEntry represents a symbol declared in a scope and tracks if it is a constant.
type ScopeEntry struct {
	Sym        symbol.Symbol
	IsConstant bool
}

// globalScope returns the top-level scope of this analyzer
func (a *Analyzer) globalScope() map[string]ScopeEntry {
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
func (a *Analyzer) declare(name string, sym symbol.Symbol, isConstant bool) {
	last := len(a.scopes) - 1
	a.scopes[last][name] = ScopeEntry{Sym: sym, IsConstant: isConstant}
}
