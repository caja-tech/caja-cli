package semantic

import (
	"caja-cli/internal/semantic/symbol"
)

// globalScope returns the top-level scope of this analyzer
func (a *Analyzer) globalScope() map[string]symbol.Symbol {
	if len(a.scopes) > 0 {
		return a.scopes[0]
	}
	return make(map[string]symbol.Symbol)
}

// pushScope creates a new inner scope and pushes it onto the scope stack.
func (a *Analyzer) pushScope() {
	a.scopes = append(a.scopes, make(map[string]symbol.Symbol))
}

// popScope removes the most recently added inner scope from the scope stack.
func (a *Analyzer) popScope() {
	a.scopes = a.scopes[:len(a.scopes)-1]
}

// declare registers a variable name in the current (innermost) scope.
func (a *Analyzer) declare(name string, sym symbol.Symbol) {
	last := len(a.scopes) - 1
	a.scopes[last][name] = sym
}
