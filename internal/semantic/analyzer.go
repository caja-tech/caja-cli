package semantic

import (
	"caja-cli/internal/syntax"
	"fmt"
)

// Analyzer performs semantic analysis on an AST.
// It manages variable scopes and tracks semantic errors such as
// undeclared variables and variable redeclarations.
type Analyzer struct {
	scopes []map[string]bool
	errors []string
}

// New creates and returns a new Analyzer with an initial global scope.
func New() *Analyzer {
	return &Analyzer{
		scopes: []map[string]bool{make(map[string]bool)},
	}
}

// Analyze traverses the AST starting from the given node and performs
// semantic checks, populating the errors slice if any issues are found.
func (a *Analyzer) Analyze(node syntax.Node) {
	switch n := node.(type) {

	case *syntax.Program:
		for _, s := range n.Statements {
			a.Analyze(s)
		}

	case *syntax.BlockStatement:
		a.pushScope()
		for _, s := range n.Statements {
			a.Analyze(s)
		}
		a.popScope()

	case *syntax.LetStatement:
		if n.Value != nil {
			a.Analyze(n.Value)
		}
		if a.resolve(n.Name.Value) {
			a.errors = append(a.errors, fmt.Sprintf("semantic error: variable '%s' is already declared", n.Name.Value))
		}
		a.declare(n.Name.Value)

	case *syntax.AssignStatement:
		if n.Value != nil {
			a.Analyze(n.Value)
		}
		if !a.resolve(n.Name.Value) {
			a.errors = append(a.errors, fmt.Sprintf("semantic error: undeclared variable '%s'. Use 'let' to declare it.", n.Name.Value))
		}

	case *syntax.Identifier:
		if !a.resolve(n.Value) {
			a.errors = append(a.errors, fmt.Sprintf("semantic error: undeclared variable '%s'", n.Value))
		}

	case *syntax.IfExpression:
		a.Analyze(n.Condition)
		a.Analyze(n.Consequence)
		if n.Alternative != nil {
			a.Analyze(n.Alternative)
		}

	case *syntax.ReturnStatement:
		if n.ReturnValue != nil {
			a.Analyze(n.ReturnValue)
		}

	case *syntax.InfixExpression:
		a.Analyze(n.Left)
		a.Analyze(n.Right)

	case *syntax.ExpressionStatement:
		if n.Expression != nil {
			a.Analyze(n.Expression)
		}

	case *syntax.NumberLiteral:
		// Numeric literals don't require semantic analysis
	}
}

// Errors returns the list of semantic errors encountered during analysis.
func (a *Analyzer) Errors() []string {
	return a.errors
}

// pushScope creates a new inner scope and pushes it onto the scope stack.
func (a *Analyzer) pushScope() {
	a.scopes = append(a.scopes, make(map[string]bool))
}

// popScope removes the most recently added inner scope from the scope stack.
func (a *Analyzer) popScope() {
	a.scopes = a.scopes[:len(a.scopes)-1]
}

// declare registers a variable name in the current (innermost) scope.
func (a *Analyzer) declare(name string) {
	last := len(a.scopes) - 1
	a.scopes[last][name] = true
}

// resolve checks if a variable name has been declared in the current or
// any outer scope. It returns true if the variable is found, false otherwise.
func (a *Analyzer) resolve(name string) bool {
	for i := len(a.scopes) - 1; i >= 0; i-- {
		if _, ok := a.scopes[i][name]; ok {
			return true
		}
	}
	return false
}
