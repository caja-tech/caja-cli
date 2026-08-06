package semantic

import (
	"caja-cli/internal/environment"
	"caja-cli/internal/syntax"
	"fmt"
)

// Analyzer performs semantic analysis on an AST.
// It manages variable scopes and tracks semantic errors such as
// undeclared variables and variable redeclarations.
type Analyzer struct {
	scopes []map[string]Symbol
	errors []string
}

// Symbol represents a semantic entity (like a variable or function) tracked during
// analysis, containing its type information and function signature details if applicable.
type Symbol struct {
	Type       environment.ObjectType
	Arity      int
	ParamTypes []environment.ObjectType
	ReturnType environment.ObjectType
}

var anySymbol = Symbol{Type: environment.ANY_OBJ, Arity: 0}

// New creates and returns a new Analyzer with an initial global scope.
func New() *Analyzer {
	globalScope := make(map[string]Symbol)
	return &Analyzer{
		scopes: []map[string]Symbol{globalScope},
	}
}

// Analyze traverses the AST starting from the given node and performs
// semantic checks, populating the errors slice if any issues are found.
func (a *Analyzer) Analyze(node syntax.Node) Symbol {
	switch n := node.(type) {
	case *syntax.Program:
		return a.analyzeProgram(n)
	case *syntax.BlockStatement:
		return a.analyzeBlockStatement(n)
	case *syntax.LetStatement:
		return a.analyzeLetStatement(n)
	case *syntax.AssignStatement:
		return a.analyzeAssignStatement(n)
	case *syntax.Identifier:
		return a.analyzeIdentifier(n)
	case *syntax.IfExpression:
		return a.analyzeIfExpression(n)
	case *syntax.ReturnStatement:
		return a.analyzeReturnStatement(n)
	case *syntax.InfixExpression:
		return a.analyzeInfixExpression(n)
	case *syntax.ExpressionStatement:
		return a.analyzeExpressionStatement(n)
	case *syntax.FunctionLiteral:
		return a.analyzeFunctionLiteral(n)
	case *syntax.CallExpression:
		return a.analyzeCallExpression(n)
	case *syntax.NumberLiteral:
		return a.analyzeNumberLiteral(n)
	case *syntax.StringLiteral:
		return a.analyzeStringLiteral(n)
	case *syntax.Boolean:
		return a.analyzeBoolean(n)
	}

	return anySymbol
}

// Errors returns the list of semantic errors encountered during analysis.
func (a *Analyzer) Errors() []string {
	return a.errors
}

// analyzeProgram iterates over all statements in the program and analyzes them.
func (a *Analyzer) analyzeProgram(n *syntax.Program) Symbol {
	for _, s := range n.Statements {
		a.Analyze(s)
	}
	return anySymbol
}

// analyzeBlockStatement analyzes a block of statements within a new scope,
// returning the type of the last statement in the block.
func (a *Analyzer) analyzeBlockStatement(n *syntax.BlockStatement) Symbol {
	a.pushScope()
	var lastType = environment.ANY_OBJ
	for _, s := range n.Statements {
		lastType = a.Analyze(s).Type
	}
	a.popScope()
	return Symbol{Type: lastType}
}

// analyzeLetStatement checks for variable redeclarations and registers the
// newly declared variable in the current scope with its analyzed type.
func (a *Analyzer) analyzeLetStatement(n *syntax.LetStatement) Symbol {
	sym := anySymbol
	if n.Value != nil {
		sym = a.Analyze(n.Value)
	}

	if _, exists := a.resolve(n.Name.Value); exists {
		a.errors = append(a.errors, fmt.Sprintf("semantic error: variable '%s' is already declared", n.Name.Value))
	}

	a.declare(n.Name.Value, sym)
	return sym
}

// analyzeAssignStatement ensures the assigned variable has been declared
// and that the type of the assigned value matches the declared type.
func (a *Analyzer) analyzeAssignStatement(n *syntax.AssignStatement) Symbol {
	sym := anySymbol
	if n.Value != nil {
		sym = a.Analyze(n.Value)
	}

	expectedType, exists := a.resolve(n.Name.Value)
	if !exists {
		a.errors = append(a.errors, fmt.Sprintf("semantic error: undeclared variable '%s'. Use 'let' to declare it.", n.Name.Value))
	} else {
		if expectedType.Type != sym.Type && expectedType.Type != environment.ANY_OBJ && sym.Type != environment.ANY_OBJ {
			a.errors = append(a.errors, fmt.Sprintf("type error: cannot assign %s to variable '%s' of type %s", sym.Type, n.Name.Value, expectedType.Type))
		}
	}
	return sym
}

// analyzeIdentifier resolves the identifier in the current and outer scopes,
// logging an error if it has not been declared.
func (a *Analyzer) analyzeIdentifier(n *syntax.Identifier) Symbol {
	if sym, ok := a.resolve(n.Value); ok {
		return sym
	}
	a.errors = append(a.errors, fmt.Sprintf("semantic error: undeclared variable '%s'", n.Value))
	return anySymbol
}

// analyzeIfExpression recursively analyzes its condition and both branches.
// It returns the type of the consequence branch.
func (a *Analyzer) analyzeIfExpression(n *syntax.IfExpression) Symbol {
	a.Analyze(n.Condition)
	trueType := a.Analyze(n.Consequence)
	if n.Alternative != nil {
		a.Analyze(n.Alternative)
	}
	return trueType
}

// analyzeReturnStatement recursively analyzes the return value expression, if any.
func (a *Analyzer) analyzeReturnStatement(n *syntax.ReturnStatement) Symbol {
	if n.ReturnValue != nil {
		return a.Analyze(n.ReturnValue)
	}
	return anySymbol
}

// analyzeInfixExpression ensures that both the left and right operands
// are of appropriate types for the given operator.
func (a *Analyzer) analyzeInfixExpression(n *syntax.InfixExpression) Symbol {
	leftType := a.Analyze(n.Left)
	rightType := a.Analyze(n.Right)

	if leftType.Type == environment.ANY_OBJ || rightType.Type == environment.ANY_OBJ {
		return Symbol{Type: environment.ANY_OBJ}
	}

	switch n.Operator {
	case "+":
		if leftType.Type == environment.NUMBER_OBJ && rightType.Type == environment.NUMBER_OBJ {
			return Symbol{Type: environment.NUMBER_OBJ}
		}
		if leftType.Type == environment.STRING_OBJ && rightType.Type == environment.STRING_OBJ {
			return Symbol{Type: environment.STRING_OBJ}
		}
		a.errors = append(a.errors, fmt.Sprintf("type error: cannot add %s and %s", leftType.Type, rightType.Type))
		return Symbol{Type: environment.ANY_OBJ}

	case "-", "*", "/", "%", "^":
		if leftType.Type != environment.NUMBER_OBJ || rightType.Type != environment.NUMBER_OBJ {
			a.errors = append(a.errors, fmt.Sprintf("type error: operator '%s' requires two NUMBERs, got %s and %s", n.Operator, leftType.Type, rightType.Type))
			return Symbol{Type: environment.ANY_OBJ}
		}
		return Symbol{Type: environment.NUMBER_OBJ}
	case "<", ">", "<=", ">=":
		if leftType.Type != environment.NUMBER_OBJ || rightType.Type != environment.NUMBER_OBJ {
			a.errors = append(a.errors, fmt.Sprintf("type error: operator '%s' requires two NUMBERs, got %s and %s", n.Operator, leftType.Type, rightType.Type))
		}
		return Symbol{Type: environment.BOOLEAN_OBJ}
	case "==", "!=":
		if leftType.Type != rightType.Type {
			a.errors = append(a.errors, fmt.Sprintf("type error: cannot compare %s and %s using '%s'", leftType.Type, rightType.Type, n.Operator))
		}
		return Symbol{Type: environment.BOOLEAN_OBJ}
	}

	return anySymbol
}

// analyzeExpressionStatement wraps the analysis of the inner expression.
func (a *Analyzer) analyzeExpressionStatement(n *syntax.ExpressionStatement) Symbol {
	if n.Expression != nil {
		return a.Analyze(n.Expression)
	}
	return anySymbol
}

// analyzeFunctionLiteral analyzes a function definition within a new scope,
// registers its parameters, checks its body's return type against the declared
// return type, and verifies that the function guarantees a return if needed.
func (a *Analyzer) analyzeFunctionLiteral(n *syntax.FunctionLiteral) Symbol {
	a.pushScope()

	var paramTypes []environment.ObjectType

	for _, param := range n.Parameters {
		objType := stringToObjectType(param.Type)
		paramTypes = append(paramTypes, objType)

		a.declare(param.Name, Symbol{Type: objType})
	}

	actualReturnSymbol := a.Analyze(n.Body)
	a.popScope()

	expectedReturnType := stringToObjectType(n.ReturnType)
	if n.ReturnType != "" && actualReturnSymbol.Type != expectedReturnType && actualReturnSymbol.Type != environment.ANY_OBJ {
		a.errors = append(a.errors, fmt.Sprintf("type error: function declared to return %s, but body returns %s", expectedReturnType, actualReturnSymbol.Type))
	}

	if n.ReturnType != "" {
		if !guaranteesReturn(n.Body) {
			a.errors = append(a.errors, "semantic error: function is missing a guaranteed return statement. All code paths must return a value.")
		}
	}

	return Symbol{
		Type:       environment.FUNCTION_OBJ,
		Arity:      len(n.Parameters),
		ParamTypes: paramTypes,
		ReturnType: expectedReturnType,
	}
}

// analyzeCallExpression analyzes a function call to ensure the target is callable,
// verifies the number of arguments matches the function's arity, and checks
// the types of the provided arguments against the function's parameters.
func (a *Analyzer) analyzeCallExpression(n *syntax.CallExpression) Symbol {
	fnSymbol := a.Analyze(n.Function)

	if fnSymbol.Type != environment.FUNCTION_OBJ && fnSymbol.Type != "ANY" {
		a.errors = append(a.errors, fmt.Sprintf("type error: cannot call a non-function (got %s)", fnSymbol.Type))
	}

	if fnSymbol.Type != environment.ANY_OBJ && len(n.Arguments) != fnSymbol.Arity {
		a.errors = append(a.errors, fmt.Sprintf("arity error: expected %d arguments, got %d", fnSymbol.Arity, len(n.Arguments)))
	}

	for i, arg := range n.Arguments {
		argSymbol := a.Analyze(arg)

		if i < len(fnSymbol.ParamTypes) {
			expectedType := fnSymbol.ParamTypes[i]

			if argSymbol.Type != expectedType && argSymbol.Type != "ANY" && expectedType != "ANY" {
				a.errors = append(a.errors, fmt.Sprintf("type error: argument %d expected %s, got %s", i+1, expectedType, argSymbol.Type))
			}
		}
	}

	if fnSymbol.ReturnType != "" {
		return Symbol{Type: fnSymbol.ReturnType}
	}

	return anySymbol
}

// analyzeNumberLiteral produces the semantic symbol for a number literal.
func (a *Analyzer) analyzeNumberLiteral(n *syntax.NumberLiteral) Symbol {
	return Symbol{Type: environment.NUMBER_OBJ}
}

// analyzeStringLiteral produces the semantic symbol for a string literal.
func (a *Analyzer) analyzeStringLiteral(n *syntax.StringLiteral) Symbol {
	return Symbol{Type: environment.STRING_OBJ}
}

// analyzeBoolean produces the semantic symbol for a boolean literal.
func (a *Analyzer) analyzeBoolean(n *syntax.Boolean) Symbol {
	return Symbol{Type: environment.BOOLEAN_OBJ}
}

// pushScope creates a new inner scope and pushes it onto the scope stack.
func (a *Analyzer) pushScope() {
	a.scopes = append(a.scopes, make(map[string]Symbol))
}

// popScope removes the most recently added inner scope from the scope stack.
func (a *Analyzer) popScope() {
	a.scopes = a.scopes[:len(a.scopes)-1]
}

// declare registers a variable name in the current (innermost) scope.
func (a *Analyzer) declare(name string, sym Symbol) {
	last := len(a.scopes) - 1
	a.scopes[last][name] = sym
}

// resolve checks if a variable name has been declared in the current or
// any outer scope. It returns true if the variable is found, false otherwise.
func (a *Analyzer) resolve(name string) (Symbol, bool) {
	for i := len(a.scopes) - 1; i >= 0; i-- {
		if sym, ok := a.scopes[i][name]; ok {
			return sym, true
		}
	}

	return anySymbol, false
}

// stringToObjectType converts a string representation of a type (e.g., "Number")
// into its corresponding environment.ObjectType.
func stringToObjectType(t string) environment.ObjectType {
	switch t {
	case "Number":
		return environment.NUMBER_OBJ
	case "String":
		return environment.STRING_OBJ
	case "Boolean":
		return environment.BOOLEAN_OBJ
	case "Date":
		return environment.DATE_OBJ
	default:
		return environment.ANY_OBJ
	}
}

// guaranteesReturn checks if a given AST node is guaranteed to execute a return
// statement on all of its code paths.
func guaranteesReturn(node syntax.Node) bool {
	switch n := node.(type) {
	case *syntax.ReturnStatement:
		return true

	case *syntax.BlockStatement:
		if len(n.Statements) == 0 {
			return false
		}
		lastStatement := n.Statements[len(n.Statements)-1]
		return guaranteesReturn(lastStatement)

	case *syntax.ExpressionStatement:
		return guaranteesReturn(n.Expression)

	case *syntax.IfExpression:
		if n.Alternative != nil {
			return guaranteesReturn(n.Consequence) && guaranteesReturn(n.Alternative)
		}
		return false

	default:
		return false
	}
}
