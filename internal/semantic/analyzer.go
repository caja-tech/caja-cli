package semantic

import (
	"caja-cli/internal/environment"
	"caja-cli/internal/lexer"
	"caja-cli/internal/syntax"
	"fmt"
	"strings"
)

// Analyzer performs semantic analysis on an AST.
// It manages variable scopes and tracks semantic errors such as
// undeclared variables and variable redeclarations.
type Analyzer struct {
	scopes []map[string]Symbol
	types  map[string]Symbol
	errors []string
}

// New creates and returns a new Analyzer with an initial global scope.
func New() *Analyzer {
	globalScope := make(map[string]Symbol)
	analyzer := &Analyzer{
		scopes: []map[string]Symbol{globalScope},
		types:  make(map[string]Symbol),
	}

	analyzer.declare("len", Symbol{Type: environment.BUILTIN_OBJ, Arity: 1})
	analyzer.declare("append", Symbol{Type: environment.BUILTIN_OBJ, Arity: 2})
	analyzer.declare("head", Symbol{Type: environment.BUILTIN_OBJ, Arity: 1})
	analyzer.declare("tail", Symbol{Type: environment.BUILTIN_OBJ, Arity: 1})
	analyzer.declare("last", Symbol{Type: environment.BUILTIN_OBJ, Arity: 1})
	analyzer.declare("copy", Symbol{Type: environment.BUILTIN_OBJ, Arity: 1})
	analyzer.declare("slice", Symbol{Type: environment.BUILTIN_OBJ, Arity: 3})
	analyzer.declare("join", Symbol{Type: environment.BUILTIN_OBJ, Arity: 2})
	analyzer.declare("charAt", Symbol{Type: environment.BUILTIN_OBJ, Arity: 2})
	analyzer.declare("substring", Symbol{Type: environment.BUILTIN_OBJ, Arity: 3})
	analyzer.declare("concat", Symbol{Type: environment.BUILTIN_OBJ, Arity: 2})
	analyzer.declare("split", Symbol{Type: environment.BUILTIN_OBJ, Arity: 2})
	analyzer.declare("contains", Symbol{Type: environment.BUILTIN_OBJ, Arity: 2})
	analyzer.declare("startsWith", Symbol{Type: environment.BUILTIN_OBJ, Arity: 2})
	analyzer.declare("endsWith", Symbol{Type: environment.BUILTIN_OBJ, Arity: 2})
	analyzer.declare("replace", Symbol{Type: environment.BUILTIN_OBJ, Arity: 3})
	analyzer.declare("toUpper", Symbol{Type: environment.BUILTIN_OBJ, Arity: 1})
	analyzer.declare("toLower", Symbol{Type: environment.BUILTIN_OBJ, Arity: 1})
	analyzer.declare("trim", Symbol{Type: environment.BUILTIN_OBJ, Arity: 1})
	analyzer.declare("strlen", Symbol{Type: environment.BUILTIN_OBJ, Arity: 1})
	analyzer.declare("year", Symbol{Type: environment.BUILTIN_OBJ, Arity: 1})
	analyzer.declare("month", Symbol{Type: environment.BUILTIN_OBJ, Arity: 1})
	analyzer.declare("day", Symbol{Type: environment.BUILTIN_OBJ, Arity: 1})
	analyzer.declare("weekday", Symbol{Type: environment.BUILTIN_OBJ, Arity: 1})
	analyzer.declare("today", Symbol{Type: environment.BUILTIN_OBJ, Arity: 0})
	analyzer.declare("parseDate", Symbol{Type: environment.BUILTIN_OBJ, Arity: 1})
	analyzer.declare("addDays", Symbol{Type: environment.BUILTIN_OBJ, Arity: 2})
	analyzer.declare("diffDays", Symbol{Type: environment.BUILTIN_OBJ, Arity: 2})
	analyzer.declare("newDate", Symbol{Type: environment.BUILTIN_OBJ, Arity: 3})

	return analyzer
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
	case *syntax.TypeAliasStatement:
		return a.analyzeTypeAliasStatement(n)
	case *syntax.FunctionLiteral:
		return a.analyzeFunctionLiteral(n)
	case *syntax.CallExpression:
		return a.analyzeCallExpression(n)
	case *syntax.ArrayLiteral:
		return a.analyzeArrayLiteral(n)
	case *syntax.IndexExpression:
		return a.analyzeIndexExpression(n)
	case *syntax.NumberLiteral:
		return Symbol{Type: environment.NUMBER_OBJ}
	case *syntax.StringLiteral:
		return Symbol{Type: environment.STRING_OBJ}
	case *syntax.DateLiteral:
		return Symbol{Type: environment.DATE_OBJ}
	case *syntax.BooleanLiteral:
		return Symbol{Type: environment.BOOLEAN_OBJ}
	}

	return anySymbol
}

// reportError formats and appends a semantic error with the token's line and column.
func (a *Analyzer) reportError(token lexer.Token, msg string) {
	a.errors = append(a.errors, fmt.Sprintf("[Line %d, Column %d] %s", token.Line, token.Column, msg))
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
	if _, exists := a.resolve(n.Name.Value); exists {
		a.reportError(n.Token, fmt.Sprintf("semantic error: variable '%s' is already declared", n.Name.Value))
	}

	if fnNode, ok := n.Value.(*syntax.FunctionLiteral); ok {
		var paramTypes []Symbol
		for _, param := range fnNode.Parameters {
			paramTypes = append(paramTypes, a.resolveTypeName(param.Type, n.Token))
		}

		var retType *Symbol
		if fnNode.ReturnType != "" {
			resolved := a.resolveTypeName(fnNode.ReturnType, n.Token)
			retType = &resolved
		}

		fnSymbol := Symbol{
			Type:       environment.FUNCTION_OBJ,
			Arity:      len(fnNode.Parameters),
			ParamTypes: paramTypes,
			ReturnType: retType,
		}

		a.declare(n.Name.Value, fnSymbol)

		if guaranteesRecursiveCall(fnNode.Body, n.Name.Value) {
			a.reportError(n.Token, fmt.Sprintf("semantic error: function '%s' contains unconditional recursion and will infinitely loop", n.Name.Value))
		}
	}

	valType := anySymbol
	if n.Value != nil {
		valType = a.Analyze(n.Value)
	}

	a.declare(n.Name.Value, valType)
	return valType
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
		a.reportError(n.Token, fmt.Sprintf("semantic error: undeclared variable '%s'. Use 'let' to declare it.", n.Name.Value))
	} else {
		if expectedType.Type != sym.Type && expectedType.Type != environment.ANY_OBJ && sym.Type != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: cannot assign %s to variable '%s' of type %s", sym.Type, n.Name.Value, expectedType.Type))
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
	a.reportError(n.Token, fmt.Sprintf("semantic error: undeclared variable '%s'", n.Value))
	return anySymbol
}

// analyzeIfExpression recursively analyzes its condition and both branches.
// It returns the type of the consequence branch.
func (a *Analyzer) analyzeIfExpression(n *syntax.IfExpression) Symbol {
	condType := a.Analyze(n.Condition)
	if condType.Type != environment.BOOLEAN_OBJ && condType.Type != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: condition must be a BOOLEAN, got %s", condType.Type))
	}
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
		a.reportError(n.Token, fmt.Sprintf("type error: cannot add %s and %s", leftType.Type, rightType.Type))
		return Symbol{Type: environment.ANY_OBJ}

	case "-", "*", "/", "%", "^":
		if leftType.Type != environment.NUMBER_OBJ || rightType.Type != environment.NUMBER_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: operator '%s' requires two NUMBERs, got %s and %s", n.Operator, leftType.Type, rightType.Type))
			return Symbol{Type: environment.ANY_OBJ}
		}
		return Symbol{Type: environment.NUMBER_OBJ}
	case "<", ">", "<=", ">=":
		if leftType.Type != environment.NUMBER_OBJ || rightType.Type != environment.NUMBER_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: operator '%s' requires two NUMBERs, got %s and %s", n.Operator, leftType.Type, rightType.Type))
		}
		return Symbol{Type: environment.BOOLEAN_OBJ}
	case "==", "!=":
		if leftType.Type != rightType.Type {
			a.reportError(n.Token, fmt.Sprintf("type error: cannot compare %s and %s using '%s'", leftType.Type, rightType.Type, n.Operator))
		}
		return Symbol{Type: environment.BOOLEAN_OBJ}
	}

	return anySymbol
}

// analyzeArrayLiteral ensures all elements in the array match the type of the first element,
// and returns an ARRAY_OBJ symbol with the inferred ElementType.
func (a *Analyzer) analyzeArrayLiteral(n *syntax.ArrayLiteral) Symbol {
	if len(n.Elements) == 0 {
		return Symbol{Type: environment.ARRAY_OBJ}
	}

	firstType := a.Analyze(n.Elements[0])
	for i := 1; i < len(n.Elements); i++ {
		elType := a.Analyze(n.Elements[i])
		if !firstType.Equals(elType) && firstType.Type != environment.ANY_OBJ && elType.Type != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: array elements must have the same type, expected %s, got %s", firstType.Type, elType.Type))
		}
	}

	return Symbol{
		Type:        environment.ARRAY_OBJ,
		ElementType: &firstType,
	}
}

// analyzeIndexExpression ensures the left side is an array and the index is a number.
func (a *Analyzer) analyzeIndexExpression(n *syntax.IndexExpression) Symbol {
	leftType := a.Analyze(n.Left)
	if leftType.Type != environment.ARRAY_OBJ && leftType.Type != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: index operator not supported for %s", leftType.Type))
	}

	indexType := a.Analyze(n.Index)
	if indexType.Type != environment.NUMBER_OBJ && indexType.Type != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: array index expected NUMBER, got %s", indexType.Type))
	}

	if leftType.ElementType != nil {
		return *leftType.ElementType
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

// analyzeTypeAliasStatement resolves the parameter and return types for a type alias
// and registers the resulting function signature in the analyzer's type registry.
func (a *Analyzer) analyzeTypeAliasStatement(n *syntax.TypeAliasStatement) Symbol {
	var paramTypes []Symbol
	for _, pt := range n.Signature.ParamTypes {
		paramTypes = append(paramTypes, a.resolveTypeName(pt, n.Token))
	}

	var retType *Symbol
	if n.Signature.ReturnType != "" {
		resolved := a.resolveTypeName(n.Signature.ReturnType, n.Token)
		retType = &resolved
	}

	a.types[n.Name.Value] = Symbol{
		Type:       environment.FUNCTION_OBJ,
		Arity:      len(n.Signature.ParamTypes),
		ParamTypes: paramTypes,
		ReturnType: retType,
	}

	return anySymbol
}

// analyzeFunctionLiteral analyzes a function definition within a new scope,
// registers its parameters, checks its body's return type against the declared
// return type, and verifies that the function guarantees a return if needed.
func (a *Analyzer) analyzeFunctionLiteral(n *syntax.FunctionLiteral) Symbol {
	a.pushScope()

	var paramTypes []Symbol

	for _, param := range n.Parameters {
		sym := a.resolveTypeName(param.Type, n.Token)
		paramTypes = append(paramTypes, sym)
		a.declare(param.Name, sym)
	}

	actualReturnSymbol := a.Analyze(n.Body)
	a.popScope()

	var expectedReturnSymbol *Symbol
	if n.ReturnType != "" {
		if !guaranteesReturn(n.Body) {
			a.reportError(n.Token, "semantic error: function is missing a guaranteed return statement. All code paths must return a value.")
		}

		resolved := a.resolveTypeName(n.ReturnType, n.Token)
		expectedReturnSymbol = &resolved

		if !expectedReturnSymbol.Equals(actualReturnSymbol) {
			a.reportError(n.Token, fmt.Sprintf("type error: function declared to return %s, but body returns %s", expectedReturnSymbol.Type, actualReturnSymbol.Type))
		}
	}

	return Symbol{
		Type:       environment.FUNCTION_OBJ,
		Arity:      len(n.Parameters),
		ParamTypes: paramTypes,
		ReturnType: expectedReturnSymbol,
	}
}

// analyzeBuiltinCall intercepts calls to builtin functions (like len, append, head, tail)
// to provide custom, compile-time polymorphic type-checking and inference.
func (a *Analyzer) analyzeBuiltinCall(name string, n *syntax.CallExpression) (Symbol, bool) {
	switch name {
	case "len":
		if len(n.Arguments) != 1 {
			a.reportError(n.Token, fmt.Sprintf("arity error: expected 1 arguments for 'len', got %d", len(n.Arguments)))
			return anySymbol, true
		}
		argSymbol := a.Analyze(n.Arguments[0])
		if argSymbol.Type != environment.ARRAY_OBJ && argSymbol.Type != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: first argument to 'len' must be an ARRAY, got %s", argSymbol.Type))
		}
		return Symbol{Type: environment.NUMBER_OBJ}, true

	case "append":
		if len(n.Arguments) != 2 {
			a.reportError(n.Token, fmt.Sprintf("arity error: expected 2 arguments for 'append', got %d", len(n.Arguments)))
			return anySymbol, true
		}
		arrSymbol := a.Analyze(n.Arguments[0])
		elSymbol := a.Analyze(n.Arguments[1])

		if arrSymbol.Type != environment.ARRAY_OBJ && arrSymbol.Type != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: first argument to 'append' must be an ARRAY, got %s", arrSymbol.Type))
			return anySymbol, true
		}

		if arrSymbol.Type == environment.ARRAY_OBJ && arrSymbol.ElementType != nil && !arrSymbol.ElementType.Equals(elSymbol) && arrSymbol.ElementType.Type != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: cannot append %s to array of %s", elSymbol.Type, arrSymbol.ElementType.Type))
		}
		return arrSymbol, true

	case "head":
		if len(n.Arguments) != 1 {
			a.reportError(n.Token, fmt.Sprintf("arity error: expected 1 arguments for 'head', got %d", len(n.Arguments)))
			return anySymbol, true
		}
		arrSymbol := a.Analyze(n.Arguments[0])
		if arrSymbol.Type != environment.ARRAY_OBJ && arrSymbol.Type != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: first argument to 'head' must be an ARRAY, got %s", arrSymbol.Type))
			return anySymbol, true
		}
		if arrSymbol.ElementType != nil {
			return *arrSymbol.ElementType, true
		}
		return anySymbol, true

	case "tail":
		if len(n.Arguments) != 1 {
			a.reportError(n.Token, fmt.Sprintf("arity error: expected 1 arguments for 'tail', got %d", len(n.Arguments)))
			return anySymbol, true
		}
		arrSymbol := a.Analyze(n.Arguments[0])
		if arrSymbol.Type != environment.ARRAY_OBJ && arrSymbol.Type != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: first argument to 'tail' must be an ARRAY, got %s", arrSymbol.Type))
			return anySymbol, true
		}
		return arrSymbol, true

	case "last":
		if len(n.Arguments) != 1 {
			a.reportError(n.Token, fmt.Sprintf("arity error: expected 1 arguments for 'last', got %d", len(n.Arguments)))
			return anySymbol, true
		}
		arrSymbol := a.Analyze(n.Arguments[0])
		if arrSymbol.Type != environment.ARRAY_OBJ && arrSymbol.Type != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: first argument to 'last' must be an ARRAY, got %s", arrSymbol.Type))
			return anySymbol, true
		}
		if arrSymbol.ElementType != nil {
			return *arrSymbol.ElementType, true
		}
		return anySymbol, true

	case "copy":
		if len(n.Arguments) != 1 {
			a.reportError(n.Token, fmt.Sprintf("arity error: expected 1 arguments for 'copy', got %d", len(n.Arguments)))
			return anySymbol, true
		}
		arrSymbol := a.Analyze(n.Arguments[0])
		if arrSymbol.Type != environment.ARRAY_OBJ && arrSymbol.Type != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: first argument to 'copy' must be an ARRAY, got %s", arrSymbol.Type))
			return anySymbol, true
		}
		return arrSymbol, true

	case "slice":
		if len(n.Arguments) != 3 {
			a.reportError(n.Token, fmt.Sprintf("arity error: expected 3 arguments for 'slice', got %d", len(n.Arguments)))
			return anySymbol, true
		}
		arrSymbol := a.Analyze(n.Arguments[0])
		startSymbol := a.Analyze(n.Arguments[1])
		endSymbol := a.Analyze(n.Arguments[2])

		if arrSymbol.Type != environment.ARRAY_OBJ && arrSymbol.Type != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: first argument to 'slice' must be an ARRAY, got %s", arrSymbol.Type))
		}
		if startSymbol.Type != environment.NUMBER_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: second argument to 'slice' must be NUMBER, got %s", startSymbol.Type))
		}
		if endSymbol.Type != environment.NUMBER_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: third argument to 'slice' must be NUMBER, got %s", endSymbol.Type))
		}
		return arrSymbol, true

	case "join":
		if len(n.Arguments) != 2 {
			a.reportError(n.Token, fmt.Sprintf("arity error: expected 2 arguments for 'join', got %d", len(n.Arguments)))
			return anySymbol, true
		}
		arr1Symbol := a.Analyze(n.Arguments[0])
		arr2Symbol := a.Analyze(n.Arguments[1])

		if arr1Symbol.Type != environment.ARRAY_OBJ && arr1Symbol.Type != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: first argument to 'join' must be an ARRAY, got %s", arr1Symbol.Type))
			return anySymbol, true
		}
		if arr2Symbol.Type != environment.ARRAY_OBJ && arr2Symbol.Type != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: second argument to 'join' must be an ARRAY, got %s", arr2Symbol.Type))
			return anySymbol, true
		}

		if arr1Symbol.Type == environment.ARRAY_OBJ && arr2Symbol.Type == environment.ARRAY_OBJ {
			if arr1Symbol.ElementType != nil && arr2Symbol.ElementType != nil && !arr1Symbol.ElementType.Equals(*arr2Symbol.ElementType) && arr1Symbol.ElementType.Type != environment.ANY_OBJ && arr2Symbol.ElementType.Type != environment.ANY_OBJ {
				a.reportError(n.Token, fmt.Sprintf("type error: cannot join array of %s with array of %s", arr1Symbol.ElementType.Type, arr2Symbol.ElementType.Type))
			}
		}
		return arr1Symbol, true

	case "charAt":
		if len(n.Arguments) != 2 {
			a.reportError(n.Token, fmt.Sprintf("arity error: expected 2 arguments for 'charAt', got %d", len(n.Arguments)))
			return anySymbol, true
		}
		strSymbol := a.Analyze(n.Arguments[0])
		idxSymbol := a.Analyze(n.Arguments[1])

		if strSymbol.Type != environment.STRING_OBJ && strSymbol.Type != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: first argument to 'charAt' must be STRING, got %s", strSymbol.Type))
		}
		if idxSymbol.Type != environment.NUMBER_OBJ && idxSymbol.Type != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: second argument to 'charAt' must be NUMBER, got %s", idxSymbol.Type))
		}
		return Symbol{Type: environment.STRING_OBJ}, true

	case "substring":
		if len(n.Arguments) != 3 {
			a.reportError(n.Token, fmt.Sprintf("arity error: expected 3 arguments for 'substring', got %d", len(n.Arguments)))
			return anySymbol, true
		}
		strSymbol := a.Analyze(n.Arguments[0])
		startSymbol := a.Analyze(n.Arguments[1])
		endSymbol := a.Analyze(n.Arguments[2])

		if strSymbol.Type != environment.STRING_OBJ && strSymbol.Type != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: first argument to 'substring' must be STRING, got %s", strSymbol.Type))
		}
		if startSymbol.Type != environment.NUMBER_OBJ && startSymbol.Type != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: second argument to 'substring' must be NUMBER, got %s", startSymbol.Type))
		}
		if endSymbol.Type != environment.NUMBER_OBJ && endSymbol.Type != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: third argument to 'substring' must be NUMBER, got %s", endSymbol.Type))
		}
		return Symbol{Type: environment.STRING_OBJ}, true

	case "concat":
		if len(n.Arguments) != 2 {
			a.reportError(n.Token, fmt.Sprintf("arity error: expected 2 arguments for 'concat', got %d", len(n.Arguments)))
			return anySymbol, true
		}
		str1Symbol := a.Analyze(n.Arguments[0])
		str2Symbol := a.Analyze(n.Arguments[1])

		if str1Symbol.Type != environment.STRING_OBJ && str1Symbol.Type != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: first argument to 'concat' must be STRING, got %s", str1Symbol.Type))
		}
		if str2Symbol.Type != environment.STRING_OBJ && str2Symbol.Type != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: second argument to 'concat' must be STRING, got %s", str2Symbol.Type))
		}
		return Symbol{Type: environment.STRING_OBJ}, true

	case "split":
		if len(n.Arguments) != 2 {
			a.reportError(n.Token, fmt.Sprintf("arity error: expected 2 arguments for 'split', got %d", len(n.Arguments)))
			return anySymbol, true
		}
		strSymbol := a.Analyze(n.Arguments[0])
		delimSymbol := a.Analyze(n.Arguments[1])

		if strSymbol.Type != environment.STRING_OBJ && strSymbol.Type != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: first argument to 'split' must be STRING, got %s", strSymbol.Type))
		}
		if delimSymbol.Type != environment.STRING_OBJ && delimSymbol.Type != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: second argument to 'split' must be STRING, got %s", delimSymbol.Type))
		}
		
		stringType := Symbol{Type: environment.STRING_OBJ}
		return Symbol{Type: environment.ARRAY_OBJ, ElementType: &stringType}, true

	case "contains", "startsWith", "endsWith":
		if len(n.Arguments) != 2 {
			a.reportError(n.Token, fmt.Sprintf("arity error: expected 2 arguments for '%s', got %d", name, len(n.Arguments)))
			return anySymbol, true
		}
		strSymbol := a.Analyze(n.Arguments[0])
		subSymbol := a.Analyze(n.Arguments[1])

		if strSymbol.Type != environment.STRING_OBJ && strSymbol.Type != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: first argument to '%s' must be STRING, got %s", name, strSymbol.Type))
		}
		if subSymbol.Type != environment.STRING_OBJ && subSymbol.Type != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: second argument to '%s' must be STRING, got %s", name, subSymbol.Type))
		}
		return Symbol{Type: environment.BOOLEAN_OBJ}, true

	case "replace":
		if len(n.Arguments) != 3 {
			a.reportError(n.Token, fmt.Sprintf("arity error: expected 3 arguments for 'replace', got %d", len(n.Arguments)))
			return anySymbol, true
		}
		strSymbol := a.Analyze(n.Arguments[0])
		oldSymbol := a.Analyze(n.Arguments[1])
		newSymbol := a.Analyze(n.Arguments[2])

		if strSymbol.Type != environment.STRING_OBJ && strSymbol.Type != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: first argument to 'replace' must be STRING, got %s", strSymbol.Type))
		}
		if oldSymbol.Type != environment.STRING_OBJ && oldSymbol.Type != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: second argument to 'replace' must be STRING, got %s", oldSymbol.Type))
		}
		if newSymbol.Type != environment.STRING_OBJ && newSymbol.Type != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: third argument to 'replace' must be STRING, got %s", newSymbol.Type))
		}
		return Symbol{Type: environment.STRING_OBJ}, true

	case "toUpper", "toLower", "trim":
		if len(n.Arguments) != 1 {
			a.reportError(n.Token, fmt.Sprintf("arity error: expected 1 arguments for '%s', got %d", name, len(n.Arguments)))
			return anySymbol, true
		}
		strSymbol := a.Analyze(n.Arguments[0])

		if strSymbol.Type != environment.STRING_OBJ && strSymbol.Type != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: first argument to '%s' must be STRING, got %s", name, strSymbol.Type))
		}
		return Symbol{Type: environment.STRING_OBJ}, true

	case "strlen":
		if len(n.Arguments) != 1 {
			a.reportError(n.Token, fmt.Sprintf("arity error: expected 1 arguments for 'strlen', got %d", len(n.Arguments)))
			return anySymbol, true
		}
		strSymbol := a.Analyze(n.Arguments[0])

		if strSymbol.Type != environment.STRING_OBJ && strSymbol.Type != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: first argument to 'strlen' must be STRING, got %s", strSymbol.Type))
		}
		return Symbol{Type: environment.NUMBER_OBJ}, true

	case "year", "month", "day", "weekday":
		if len(n.Arguments) != 1 {
			a.reportError(n.Token, fmt.Sprintf("arity error: expected 1 arguments for '%s', got %d", name, len(n.Arguments)))
			return anySymbol, true
		}
		dateSymbol := a.Analyze(n.Arguments[0])

		if dateSymbol.Type != environment.DATE_OBJ && dateSymbol.Type != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: first argument to '%s' must be DATE, got %s", name, dateSymbol.Type))
		}
		return Symbol{Type: environment.NUMBER_OBJ}, true

	case "today":
		if len(n.Arguments) != 0 {
			a.reportError(n.Token, fmt.Sprintf("arity error: expected 0 arguments for 'today', got %d", len(n.Arguments)))
			return anySymbol, true
		}
		return Symbol{Type: environment.DATE_OBJ}, true

	case "parseDate":
		if len(n.Arguments) != 1 {
			a.reportError(n.Token, fmt.Sprintf("arity error: expected 1 arguments for 'parseDate', got %d", len(n.Arguments)))
			return anySymbol, true
		}
		strSymbol := a.Analyze(n.Arguments[0])

		if strSymbol.Type != environment.STRING_OBJ && strSymbol.Type != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: first argument to 'parseDate' must be STRING, got %s", strSymbol.Type))
		}
		return Symbol{Type: environment.DATE_OBJ}, true

	case "addDays":
		if len(n.Arguments) != 2 {
			a.reportError(n.Token, fmt.Sprintf("arity error: expected 2 arguments for 'addDays', got %d", len(n.Arguments)))
			return anySymbol, true
		}
		dateSymbol := a.Analyze(n.Arguments[0])
		numSymbol := a.Analyze(n.Arguments[1])

		if dateSymbol.Type != environment.DATE_OBJ && dateSymbol.Type != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: first argument to 'addDays' must be DATE, got %s", dateSymbol.Type))
		}
		if numSymbol.Type != environment.NUMBER_OBJ && numSymbol.Type != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: second argument to 'addDays' must be NUMBER, got %s", numSymbol.Type))
		}
		return Symbol{Type: environment.DATE_OBJ}, true

	case "diffDays":
		if len(n.Arguments) != 2 {
			a.reportError(n.Token, fmt.Sprintf("arity error: expected 2 arguments for 'diffDays', got %d", len(n.Arguments)))
			return anySymbol, true
		}
		date1Symbol := a.Analyze(n.Arguments[0])
		date2Symbol := a.Analyze(n.Arguments[1])

		if date1Symbol.Type != environment.DATE_OBJ && date1Symbol.Type != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: first argument to 'diffDays' must be DATE, got %s", date1Symbol.Type))
		}
		if date2Symbol.Type != environment.DATE_OBJ && date2Symbol.Type != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: second argument to 'diffDays' must be DATE, got %s", date2Symbol.Type))
		}
		return Symbol{Type: environment.NUMBER_OBJ}, true

	case "newDate":
		if len(n.Arguments) != 3 {
			a.reportError(n.Token, fmt.Sprintf("arity error: expected 3 arguments for 'newDate', got %d", len(n.Arguments)))
			return anySymbol, true
		}
		yearSymbol := a.Analyze(n.Arguments[0])
		monthSymbol := a.Analyze(n.Arguments[1])
		daySymbol := a.Analyze(n.Arguments[2])

		if yearSymbol.Type != environment.NUMBER_OBJ && yearSymbol.Type != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: first argument to 'newDate' must be NUMBER, got %s", yearSymbol.Type))
		}
		if monthSymbol.Type != environment.NUMBER_OBJ && monthSymbol.Type != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: second argument to 'newDate' must be NUMBER, got %s", monthSymbol.Type))
		}
		if daySymbol.Type != environment.NUMBER_OBJ && daySymbol.Type != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: third argument to 'newDate' must be NUMBER, got %s", daySymbol.Type))
		}
		return Symbol{Type: environment.DATE_OBJ}, true

	default:
		return Symbol{}, false
	}
}

// analyzeCallExpression analyzes a function call to ensure the target is callable,
// verifies the number of arguments matches the function's arity, and checks
// the types of the provided arguments against the function's parameters.
func (a *Analyzer) analyzeCallExpression(n *syntax.CallExpression) Symbol {
	fnSymbol := a.Analyze(n.Function)

	if ident, ok := n.Function.(*syntax.Identifier); ok {
		if fnSymbol.Type == environment.BUILTIN_OBJ {
			if sym, handled := a.analyzeBuiltinCall(ident.Value, n); handled {
				return sym
			}
		}
	}

	if fnSymbol.Type != environment.FUNCTION_OBJ && fnSymbol.Type != environment.ANY_OBJ && fnSymbol.Type != environment.BUILTIN_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: cannot call a non-function (got %s)", fnSymbol.Type))
	}

	if fnSymbol.Type != environment.ANY_OBJ && len(n.Arguments) != fnSymbol.Arity {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected %d arguments, got %d", fnSymbol.Arity, len(n.Arguments)))
	}

	for i, arg := range n.Arguments {
		argSymbol := a.Analyze(arg)

		if i < len(fnSymbol.ParamTypes) {
			expectedType := fnSymbol.ParamTypes[i]

			if !expectedType.Equals(argSymbol) {
				a.reportError(n.Token, fmt.Sprintf("type error: argument %d expected %s, got %s", i+1, expectedType.Type, argSymbol.Type))
			}
		}
	}

	if fnSymbol.ReturnType != nil {
		return *fnSymbol.ReturnType
	}

	return anySymbol
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

// resolveTypeName converts a type name string into its corresponding Symbol representation,
// checking built-in types first, and then looking up custom types in the registry.
func (a *Analyzer) resolveTypeName(typeName string, token lexer.Token) Symbol {
	if typeName == "" {
		return anySymbol
	}

	if strings.HasPrefix(typeName, "[") && strings.HasSuffix(typeName, "]") {
		innerTypeStr := typeName[1 : len(typeName)-1]
		innerSymbol := a.resolveTypeName(innerTypeStr, token)

		return Symbol{
			Type:        environment.ARRAY_OBJ,
			ElementType: &innerSymbol,
		}
	}

	switch typeName {
	case "Any":
		return anySymbol
	case "Number":
		return Symbol{Type: environment.NUMBER_OBJ}
	case "String":
		return Symbol{Type: environment.STRING_OBJ}
	case "Boolean":
		return Symbol{Type: environment.BOOLEAN_OBJ}
	case "Date":
		return Symbol{Type: environment.DATE_OBJ}
	}

	if sym, ok := a.types[typeName]; ok {
		return sym
	}

	a.reportError(token, fmt.Sprintf("unknown type '%s'", typeName))
	return anySymbol
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

// guaranteesRecursiveCall determines if evaluating the given AST node guarantees
// that the function named targetName will be recursively called unconditionally
// on all of its code execution paths.
func guaranteesRecursiveCall(node syntax.Node, targetName string) bool {
	if node == nil {
		return false
	}

	switch n := node.(type) {
	case *syntax.BlockStatement:
		for _, stmt := range n.Statements {
			if guaranteesRecursiveCall(stmt, targetName) {
				return true
			}
		}
		return false

	case *syntax.CallExpression:
		if ident, ok := n.Function.(*syntax.Identifier); ok && ident.Value == targetName {
			return true
		}

		for _, arg := range n.Arguments {
			if guaranteesRecursiveCall(arg, targetName) {
				return true
			}
		}
		return guaranteesRecursiveCall(n.Function, targetName)

	case *syntax.IfExpression:
		if guaranteesRecursiveCall(n.Condition, targetName) {
			return true
		}
		if n.Alternative == nil {
			return false
		}
		return guaranteesRecursiveCall(n.Consequence, targetName) &&
			guaranteesRecursiveCall(n.Alternative, targetName)

	case *syntax.ReturnStatement:
		return guaranteesRecursiveCall(n.ReturnValue, targetName)

	case *syntax.LetStatement:
		return guaranteesRecursiveCall(n.Value, targetName)

	case *syntax.ExpressionStatement:
		return guaranteesRecursiveCall(n.Expression, targetName)

	case *syntax.InfixExpression:
		return guaranteesRecursiveCall(n.Left, targetName) || guaranteesRecursiveCall(n.Right, targetName)

	case *syntax.FunctionLiteral:
		return false

	case *syntax.ArrayLiteral:
		for _, el := range n.Elements {
			if guaranteesRecursiveCall(el, targetName) {
				return true
			}
		}
		return false

	case *syntax.IndexExpression:
		return guaranteesRecursiveCall(n.Left, targetName) || guaranteesRecursiveCall(n.Index, targetName)

	default:
		return false
	}
}
