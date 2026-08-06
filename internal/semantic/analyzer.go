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

type Symbol struct {
	Type       environment.ObjectType
	Arity      int
	ParamTypes []environment.ObjectType
	ReturnType environment.ObjectType
}

var ANY_SYMBOL = Symbol{Type: environment.ANY_OBJ, Arity: 0}

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
		for _, s := range n.Statements {
			a.Analyze(s)
		}

	case *syntax.BlockStatement:
		a.pushScope()
		var lastType = environment.ANY_OBJ
		for _, s := range n.Statements {
			lastType = a.Analyze(s).Type
		}
		a.popScope()
		return Symbol{Type: lastType}

	case *syntax.LetStatement:
		sym := ANY_SYMBOL
		if n.Value != nil {
			sym = a.Analyze(n.Value)
		}

		if _, exists := a.resolve(n.Name.Value); exists {
			a.errors = append(a.errors, fmt.Sprintf("semantic error: variable '%s' is already declared", n.Name.Value))
		}

		a.declare(n.Name.Value, sym)
		return sym

	case *syntax.AssignStatement:
		sym := ANY_SYMBOL
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

	case *syntax.Identifier:
		if sym, ok := a.resolve(n.Value); ok {
			return sym
		}
		a.errors = append(a.errors, fmt.Sprintf("semantic error: undeclared variable '%s'", n.Value))

	case *syntax.IfExpression:
		a.Analyze(n.Condition)
		trueType := a.Analyze(n.Consequence)
		if n.Alternative != nil {
			a.Analyze(n.Alternative)
		}
		return trueType

	case *syntax.ReturnStatement:
		if n.ReturnValue != nil {
			return a.Analyze(n.ReturnValue)
		}

	case *syntax.InfixExpression:
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

	case *syntax.ExpressionStatement:
		if n.Expression != nil {
			return a.Analyze(n.Expression)
		}

	case *syntax.FunctionLiteral:
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

	case *syntax.CallExpression:
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

		return ANY_SYMBOL

	case *syntax.NumberLiteral:
		return Symbol{Type: environment.NUMBER_OBJ}
	case *syntax.StringLiteral:
		return Symbol{Type: environment.STRING_OBJ}
	case *syntax.Boolean:
		return Symbol{Type: environment.BOOLEAN_OBJ}
	}

	return ANY_SYMBOL
}

// Errors returns the list of semantic errors encountered during analysis.
func (a *Analyzer) Errors() []string {
	return a.errors
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

	return ANY_SYMBOL, false
}

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
