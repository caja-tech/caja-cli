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

// pushScope creates a new inner variable and type scope and pushes both onto
// their respective stacks (kept in lockstep, so a.types always has the same
// depth as a.scopes). This gives type declarations (type/define/union) real
// lexical scoping: a type declared inside a block is only visible there and
// in its children, and is cleaned up - not just shadowed - once the block
// is popped.
func (a *Analyzer) pushScope() {
	a.scopes = append(a.scopes, make(map[string]ScopeEntry))
	a.types = append(a.types, make(map[string]symbol.Symbol))
}

// popScope removes the most recently added variable and type scope from
// their respective stacks.
func (a *Analyzer) popScope() {
	a.scopes = a.scopes[:len(a.scopes)-1]
	a.types = a.types[:len(a.types)-1]
}

// pushFunctionBoundary records the current scope depth as the start of a
// new function's own scope chain. Combined with findVarSymbolInCurrentFunctionScope
// and typeDeclaredInCurrentFunctionScope, this lets redeclaration checks stop
// at a function's own boundary instead of walking into an enclosing function
// or the module scope - consistent with function purity, which already means
// a function can never read or mutate anything declared outside it, so a
// local name reusing an outer one is never actually ambiguous.
func (a *Analyzer) pushFunctionBoundary() {
	a.functionBoundaries = append(a.functionBoundaries, len(a.scopes))
}

// popFunctionBoundary removes the innermost function boundary marker.
func (a *Analyzer) popFunctionBoundary() {
	a.functionBoundaries = a.functionBoundaries[:len(a.functionBoundaries)-1]
}

// currentFunctionBoundary returns the scope-stack index a redeclaration
// check should stop at: the start of the current function's own scope
// chain, or 0 (the module scope) if not currently inside any function -
// preserving today's top-level shadowing-prevention behavior unchanged.
func (a *Analyzer) currentFunctionBoundary() int {
	if len(a.functionBoundaries) == 0 {
		return 0
	}
	return a.functionBoundaries[len(a.functionBoundaries)-1]
}

// findVarSymbolInCurrentFunctionScope searches the same scope chain as
// findVarSymbolInScope (innermost to outermost), but never looks past the
// current function's own boundary. Used only for redeclaration checks, not
// for normal identifier resolution (which must still see - and enforce
// purity around - outer variables).
func (a *Analyzer) findVarSymbolInCurrentFunctionScope(varName string) (ScopeEntry, bool) {
	boundary := a.currentFunctionBoundary()
	for i := len(a.scopes) - 1; i >= boundary; i-- {
		if entry, ok := a.scopes[i][varName]; ok {
			return entry, true
		}
	}
	return ScopeEntry{}, false
}

// typeDeclaredInCurrentFunctionScope reports whether name is already
// registered anywhere between the innermost type scope and the current
// function's own boundary (not beyond it). Used only for redeclaration
// checks; normal type-name resolution (lookupType) always searches the
// full stack, since referencing an outer-scope type isn't a purity concern.
func (a *Analyzer) typeDeclaredInCurrentFunctionScope(name string) bool {
	boundary := a.currentFunctionBoundary()
	for i := len(a.types) - 1; i >= boundary; i-- {
		if _, ok := a.types[i][name]; ok {
			return true
		}
	}
	return false
}

// declareType registers a type name in the current (innermost) type scope.
func (a *Analyzer) declareType(name string, sym symbol.Symbol) {
	a.types[len(a.types)-1][name] = sym
}

// deleteType removes a type name from the current (innermost) type scope.
// Used to clean up temporary generic type-parameter registrations.
func (a *Analyzer) deleteType(name string) {
	delete(a.types[len(a.types)-1], name)
}

// lookupType searches the type scope chain from innermost to outermost,
// returning the first match - mirroring the shadowing order findVarSymbolInScope
// uses for variables. Unrestricted by function boundaries: any function may
// reference a type declared anywhere in its enclosing scopes.
func (a *Analyzer) lookupType(name string) (symbol.Symbol, bool) {
	for i := len(a.types) - 1; i >= 0; i-- {
		if sym, ok := a.types[i][name]; ok {
			return sym, true
		}
	}
	return nil, false
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
