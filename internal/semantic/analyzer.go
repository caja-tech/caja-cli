package semantic

import (
	"caja-cli/internal/environment"
	"caja-cli/internal/lexer"
	"caja-cli/internal/modules"
	"caja-cli/internal/semantic/symbol"
	"caja-cli/internal/syntax"
	"fmt"
)

// Analyzer performs semantic analysis on an AST.
// It manages variable scopes and tracks semantic errors such as
// undeclared variables and variable redeclarations.
type Analyzer struct {
	scopes        []map[string]ScopeEntry
	types         map[string]symbol.Symbol
	errors        []string
	functionDepth int
	globalEnv     *environment.Environment
	cache         map[string]*Analyzer
	loading       map[string]bool
	privates      map[string]bool
}

// New creates and returns a new Analyzer with an initial global scope.
func New(globalEnv *environment.Environment) *Analyzer {
	globalScope := make(map[string]ScopeEntry)
	analyzer := &Analyzer{
		scopes:    []map[string]ScopeEntry{globalScope},
		types:     make(map[string]symbol.Symbol),
		errors:    make([]string, 0),
		globalEnv: globalEnv,
		cache:     make(map[string]*Analyzer),
		loading:   make(map[string]bool),
		privates:  make(map[string]bool),
	}

	return analyzer
}

// New creates a new Analyzer with a default environment for standalone usage.
//func New() *Analyzer {
//	defaultEnv := environment.NewEnvironment("", "", false)
//	return New(defaultEnv)
//}

// Run initiates the semantic analysis process starting from the given AST node.
func (a *Analyzer) Run(node syntax.Node) {
	a.analyze(node)
}

// analyze traverses the AST starting from the given node and performs
// semantic checks, populating the errors slice if any issues are found.
func (a *Analyzer) analyze(node syntax.Node) symbol.Symbol {
	switch n := node.(type) {
	case *syntax.Program:
		return a.analyzeProgram(n)
	case *syntax.BlockStatement:
		return a.analyzeBlockStatement(n)
	case *syntax.LetStatement:
		return a.analyzeLetStatement(n)
	case *syntax.ConstStatement:
		return a.analyzeConstStatement(n)
	case *syntax.AssignStatement:
		return a.analyzeAssignStatement(n)
	case *syntax.IndexAssignmentStatement:
		return a.analyzeIndexAssignmentStatement(n)
	case *syntax.PropertyAssignmentStatement:
		return a.analyzePropertyAssignmentStatement(n)
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
	case *syntax.PrefixExpression:
		return a.analyzePrefixExpression(n)
	case *syntax.NumberLiteral:
		return symbol.NewBasicSymbol(environment.NUMBER_OBJ)
	case *syntax.StringLiteral:
		return symbol.NewBasicSymbol(environment.STRING_OBJ)
	case *syntax.DateLiteral:
		return symbol.NewBasicSymbol(environment.DATE_OBJ)
	case *syntax.BooleanLiteral:
		return symbol.NewBasicSymbol(environment.BOOLEAN_OBJ)
	case *syntax.ImportStatement:
		return a.analyzeImportStatement(n)
	case *syntax.PropertyExpression:
		return a.analyzePropertyExpression(n)
	}

	return symbol.AnySymbol()
}

// PrintErrors prints all encountered semantic errors to standard output.
func (a *Analyzer) PrintErrors() {
	if a.HasErrors() {
		fmt.Println("Semantic errors found:")
		for _, msg := range a.Errors() {
			fmt.Printf("\t- %s\n", msg)
		}
	}
}

// Errors returns the list of semantic errors found during analysis.
func (a *Analyzer) Errors() []string {
	return a.errors
}

// HasErrors returns true if any semantic errors were found.
func (a *Analyzer) HasErrors() bool {
	return len(a.errors) > 0
}

// analyzeProgram iterates over all statements in the program and analyzes them.
func (a *Analyzer) analyzeProgram(n *syntax.Program) symbol.Symbol {
	seenNonImport := false
	for _, s := range n.Statements {
		if importStmt, isImport := s.(*syntax.ImportStatement); isImport {
			if seenNonImport {
				a.reportError(importStmt.Token, "semantic error: import statements must appear at the beginning of the file")
			}
		} else {
			seenNonImport = true
		}
		a.analyze(s)
	}

	exports := make(map[string]symbol.Symbol)
	privates := make(map[string]bool)
	constants := make(map[string]bool)
	
	// Assuming top-level declarations are in the first scope (index 0)
	if len(a.scopes) > 0 {
		for name, entry := range a.scopes[0] {
			exports[name] = entry.Sym
			if a.privates[name] {
				privates[name] = true
			}
			if entry.IsConstant {
				constants[name] = true
			}
		}
	}

	return symbol.NewModuleSymbol("module", exports, privates, constants)
}

// analyzeIndexAssignmentStatement ensures that the indexed target is an array,
// the index is a number, and then evaluates the assigned value.
func (a *Analyzer) analyzeIndexAssignmentStatement(n *syntax.IndexAssignmentStatement) symbol.Symbol {
	leftSym := a.analyze(n.Left)
	if leftSym.Type() != environment.ARRAY_OBJ && leftSym.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: index assignment not supported for %s", leftSym.Type()))
	}

	idxSym := a.analyze(n.Index)
	if idxSym.Type() != environment.NUMBER_OBJ && idxSym.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: array index must be NUMBER, got %s", idxSym.Type()))
	}

	a.analyze(n.Value)
	return symbol.AnySymbol()
}

// analyzePropertyAssignmentStatement ensures that the object is a module,
// checks that the property exists, is not private and is not constant,
// and evaluates the assigned value.
func (a *Analyzer) analyzePropertyAssignmentStatement(n *syntax.PropertyAssignmentStatement) symbol.Symbol {
	leftSym := a.analyze(n.Object)
	
	if leftSym.Type() != environment.MODULE_OBJ && leftSym.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: property assignment not supported for %s", leftSym.Type()))
	}
	
	if modSymbol, ok := leftSym.(*symbol.ModuleSymbol); ok {
		// Module symbol is available (e.g., standard modules)
		if modSymbol.IsConstant(n.Property.Value) {
			a.reportError(n.Token, fmt.Sprintf("semantic error: cannot assign to constant property '%s'", n.Property.Value))
		}
	}
	
	a.analyze(n.Value)
	return symbol.AnySymbol()
}

// analyzeBlockStatement analyzes a block of statements within a new scope,
// returning the type of the last statement in the block.
func (a *Analyzer) analyzeBlockStatement(n *syntax.BlockStatement) symbol.Symbol {
	a.pushScope()
	var lastSymbol symbol.Symbol = symbol.NewBasicSymbol(environment.ANY_OBJ)
	for _, s := range n.Statements {
		if importStmt, isImport := s.(*syntax.ImportStatement); isImport {
			a.reportError(importStmt.Token, "semantic error: import statements are only allowed at the top-level of a file")
		}
		lastSymbol = a.analyze(s)
	}
	a.popScope()
	return lastSymbol
}

// analyzeLetStatement checks for variable redeclarations and registers the
// newly declared variable in the current scope with its analyzed type.
func (a *Analyzer) analyzeLetStatement(n *syntax.LetStatement) symbol.Symbol {
	if _, exists := a.findVarSymbolInScope(n.Name.Value); exists {
		a.reportError(n.Token, fmt.Sprintf("semantic error: variable '%s' is already declared", n.Name.Value))
	}

	if fnNode, ok := n.Value.(*syntax.FunctionLiteral); ok {
		var paramTypes []symbol.Symbol
		for _, param := range fnNode.Parameters {
			typeName, ok := a.findTypeSymbolInTypes(param.Type)
			if !ok {
				a.reportError(n.Token, fmt.Sprintf("semantic error: variable '%s' type is not declared: '%s'", param.Name, param.Type))
			}
			paramTypes = append(paramTypes, typeName)
		}

		var returnType symbol.Symbol
		if fnNode.ReturnType != "" {
			resolvedReturnType, ok := a.findTypeSymbolInTypes(fnNode.ReturnType)
			if !ok {
				a.reportError(n.Token, fmt.Sprintf("semantic error: function return type is not declared: '%s'", fnNode.ReturnType))
			}
			returnType = resolvedReturnType
		}

		fnSymbol := symbol.NewFunctionSymbol(len(fnNode.Parameters), paramTypes, returnType)
		a.declare(n.Name.Value, fnSymbol, false)

		if guaranteesRecursiveCall(fnNode.Body, n.Name.Value) {
			a.reportError(n.Token, fmt.Sprintf("semantic error: function '%s' contains unconditional recursion and will infinitely loop", n.Name.Value))
		}
	}

	var valType symbol.Symbol
	valType = symbol.AnySymbol()
	if n.Value != nil {
		valType = a.analyze(n.Value)
	}

	a.declare(n.Name.Value, valType, false)
	
	if n.IsPrivate {
		if len(a.scopes) > 1 {
			a.reportError(n.Token, "semantic error: 'private' modifier is only allowed at the top-level of a module")
		} else {
			a.privates[n.Name.Value] = true
		}
	}

	return valType
}

// analyzeConstStatement checks for variable redeclarations and registers the
// newly declared constant in the current scope with its analyzed type.
func (a *Analyzer) analyzeConstStatement(n *syntax.ConstStatement) symbol.Symbol {
	if _, exists := a.findVarSymbolInScope(n.Name.Value); exists {
		a.reportError(n.Token, fmt.Sprintf("semantic error: variable '%s' is already declared", n.Name.Value))
	}

	if fnNode, ok := n.Value.(*syntax.FunctionLiteral); ok {
		var paramTypes []symbol.Symbol
		for _, param := range fnNode.Parameters {
			typeName, ok := a.findTypeSymbolInTypes(param.Type)
			if !ok {
				a.reportError(n.Token, fmt.Sprintf("semantic error: variable '%s' type is not declared: '%s'", param.Name, param.Type))
			}
			paramTypes = append(paramTypes, typeName)
		}

		var returnType symbol.Symbol
		if fnNode.ReturnType != "" {
			resolvedReturnType, ok := a.findTypeSymbolInTypes(fnNode.ReturnType)
			if !ok {
				a.reportError(n.Token, fmt.Sprintf("semantic error: function return type is not declared: '%s'", fnNode.ReturnType))
			}
			returnType = resolvedReturnType
		}

		fnSymbol := symbol.NewFunctionSymbol(len(fnNode.Parameters), paramTypes, returnType)
		a.declare(n.Name.Value, fnSymbol, true)

		if guaranteesRecursiveCall(fnNode.Body, n.Name.Value) {
			a.reportError(n.Token, fmt.Sprintf("semantic error: function '%s' contains unconditional recursion and will infinitely loop", n.Name.Value))
		}
	}

	var valType symbol.Symbol
	valType = symbol.AnySymbol()
	if n.Value != nil {
		valType = a.analyze(n.Value)
	}

	a.declare(n.Name.Value, valType, true)
	
	if n.IsPrivate {
		if len(a.scopes) > 1 {
			a.reportError(n.Token, "semantic error: 'private' modifier is only allowed at the top-level of a module")
		} else {
			a.privates[n.Name.Value] = true
		}
	}

	return valType
}

// analyzeAssignStatement ensures the assigned variable has been declared
// and that the type of the assigned value matches the declared type.
func (a *Analyzer) analyzeAssignStatement(n *syntax.AssignStatement) symbol.Symbol {
	var sym symbol.Symbol
	sym = symbol.AnySymbol()
	if n.Value != nil {
		sym = a.analyze(n.Value)
	}

	entry, exists := a.findVarSymbolInScope(n.Name.Value)
	if !exists {
		a.reportError(n.Token, fmt.Sprintf("semantic error: undeclared variable '%s'. Use 'let' to declare it.", n.Name.Value))
	} else if entry.IsConstant {
		a.reportError(n.Token, fmt.Sprintf("semantic error: cannot assign to constant variable '%s'", n.Name.Value))
	} else {
		expectedType := entry.Sym
		if expectedType.Type() != sym.Type() && expectedType.Type() != environment.ANY_OBJ && sym.Type() != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: cannot assign %s to variable '%s' of type %s", sym.Type(), n.Name.Value, expectedType.Type()))
		}
	}

	return sym
}

// analyzeIdentifier resolves the identifier in the current and outer scopes,
// logging an error if it has not been declared.
func (a *Analyzer) analyzeIdentifier(n *syntax.Identifier) symbol.Symbol {
	if entry, ok := a.findVarSymbolInScope(n.Value); ok {
		return entry.Sym
	}

	a.reportError(n.Token, fmt.Sprintf("semantic error: undeclared variable '%s'. Use 'let' to declare it.", n.Value))
	return symbol.AnySymbol()
}

// analyzeIfExpression recursively analyzes its condition and both branches.
// It returns the type of the consequence branch.
func (a *Analyzer) analyzeIfExpression(n *syntax.IfExpression) symbol.Symbol {
	condSymbol := a.analyze(n.Condition)
	if condSymbol.Type() != environment.BOOLEAN_OBJ && condSymbol.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: condition must be a BOOLEAN, got %s", condSymbol.Type()))
	}

	trueType := a.analyze(n.Consequence)
	if n.Alternative != nil {
		a.analyze(n.Alternative)
	}
	return trueType
}

// analyzePrefixExpression ensures the right side of a prefix operator
// matches the operator's expected type.
func (a *Analyzer) analyzePrefixExpression(n *syntax.PrefixExpression) symbol.Symbol {
	rightSymbol := a.analyze(n.Right)
	switch n.Operator {
	case "!":
		if rightSymbol.Type() != environment.BOOLEAN_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: operator '!' requires a BOOLEAN, got %s", rightSymbol.Type()))
		}
		return symbol.NewBasicSymbol(environment.BOOLEAN_OBJ)
	case "-":
		if rightSymbol.Type() != environment.NUMBER_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: operator '-' requires a NUMBER, got %s", rightSymbol.Type()))
		}
		return symbol.NewBasicSymbol(environment.NUMBER_OBJ)
	default:
		a.reportError(n.Token, fmt.Sprintf("semantic error: unknown prefix operator '%s'", n.Operator))
		return symbol.AnySymbol()
	}
}

// analyzeReturnStatement recursively analyzes the return value expression, if any.
func (a *Analyzer) analyzeReturnStatement(n *syntax.ReturnStatement) symbol.Symbol {
	if a.globalEnv.IsModule && a.functionDepth == 0 {
		a.reportError(n.Token, "semantic error: top-level return statements are forbidden inside modules")
	}

	if n.ReturnValue != nil {
		return a.analyze(n.ReturnValue)
	}
	return symbol.AnySymbol()
}

// analyzeInfixExpression ensures that both the left and right operands
// are of appropriate types for the given operator.
func (a *Analyzer) analyzeInfixExpression(n *syntax.InfixExpression) symbol.Symbol {
	leftSymbol := a.analyze(n.Left)
	rightSymbol := a.analyze(n.Right)

	if leftSymbol.Type() == environment.ANY_OBJ || rightSymbol.Type() == environment.ANY_OBJ {
		return symbol.NewBasicSymbol(environment.ANY_OBJ)
	}

	switch n.Operator {
	case "+":
		if leftSymbol.Type() == environment.NUMBER_OBJ && rightSymbol.Type() == environment.NUMBER_OBJ {
			return symbol.NewBasicSymbol(environment.NUMBER_OBJ)
		}
		if leftSymbol.Type() == environment.STRING_OBJ && rightSymbol.Type() == environment.STRING_OBJ {
			return symbol.NewBasicSymbol(environment.STRING_OBJ)
		}
		a.reportError(n.Token, fmt.Sprintf("type error: cannot add %s and %s", leftSymbol.Type(), rightSymbol.Type()))
		return symbol.NewBasicSymbol(environment.ANY_OBJ)

	case "-", "*", "/", "%", "^":
		if leftSymbol.Type() != environment.NUMBER_OBJ || rightSymbol.Type() != environment.NUMBER_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: operator '%s' requires two NUMBERs, got %s and %s", n.Operator, leftSymbol.Type(), rightSymbol.Type()))
			return symbol.NewBasicSymbol(environment.ANY_OBJ)
		}
		return symbol.NewBasicSymbol(environment.NUMBER_OBJ)

	case "<", ">", "<=", ">=":
		if leftSymbol.Type() != environment.NUMBER_OBJ || rightSymbol.Type() != environment.NUMBER_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: operator '%s' requires two NUMBERs, got %s and %s", n.Operator, leftSymbol.Type(), rightSymbol.Type()))
		}
		return symbol.NewBasicSymbol(environment.BOOLEAN_OBJ)

	case "==", "!=":
		if leftSymbol.Type() != rightSymbol.Type() {
			a.reportError(n.Token, fmt.Sprintf("type error: cannot compare %s and %s using '%s'", leftSymbol.Type(), rightSymbol.Type(), n.Operator))
		}
		return symbol.NewBasicSymbol(environment.BOOLEAN_OBJ)

	case "and", "or", "xor":
		if leftSymbol.Type() != environment.BOOLEAN_OBJ || rightSymbol.Type() != environment.BOOLEAN_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: operator '%s' requires two BOOLEANs, got %s and %s", n.Operator, leftSymbol.Type(), rightSymbol.Type()))
		}
		return symbol.NewBasicSymbol(environment.BOOLEAN_OBJ)
	}

	return symbol.AnySymbol()
}

// analyzeArrayLiteral ensures all elements in the array match the type of the first element,
// and returns an ARRAY_OBJ symbol with the inferred ElementType.
func (a *Analyzer) analyzeArrayLiteral(n *syntax.ArrayLiteral) symbol.Symbol {
	if len(n.Elements) == 0 {
		return symbol.NewArraySymbol(symbol.AnySymbol())
	}

	firstElSymbol := a.analyze(n.Elements[0])
	for i := 1; i < len(n.Elements); i++ {
		elSymbol := a.analyze(n.Elements[i])
		if !firstElSymbol.Equals(elSymbol) && firstElSymbol.Type() != environment.ANY_OBJ && elSymbol.Type() != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: array elements must have the same type, expected %s, got %s", firstElSymbol.Type(), elSymbol.Type()))
		}
	}

	return symbol.NewArraySymbol(firstElSymbol)
}

// analyzeIndexExpression ensures the left side is an array and the index is a number.
func (a *Analyzer) analyzeIndexExpression(n *syntax.IndexExpression) symbol.Symbol {
	leftSymbol := a.analyze(n.Left)

	if leftSymbol.Type() != environment.ARRAY_OBJ && leftSymbol.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: index operator not supported for %s", leftSymbol.Type()))
	}

	indexSymbol := a.analyze(n.Index)
	if indexSymbol.Type() != environment.NUMBER_OBJ && indexSymbol.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: array index expected NUMBER, got %s", indexSymbol.Type()))
	}

	leftSymbolArray, ok := leftSymbol.(*symbol.ArraySymbol)
	if !ok {
		return symbol.AnySymbol()
	}

	if leftSymbolArray.ElementSymbol() != nil {
		return leftSymbolArray.ElementSymbol()
	}

	return symbol.AnySymbol()
}

// analyzeExpressionStatement wraps the analysis of the inner expression.
func (a *Analyzer) analyzeExpressionStatement(n *syntax.ExpressionStatement) symbol.Symbol {
	if n.Expression != nil {
		return a.analyze(n.Expression)
	}
	return symbol.AnySymbol()
}

// analyzeTypeAliasStatement resolves the parameter and return types for a type alias
// and registers the resulting function signature in the analyzer's type registry.
func (a *Analyzer) analyzeTypeAliasStatement(n *syntax.TypeAliasStatement) symbol.Symbol {
	var aliasedSymbol symbol.Symbol

	if n.Signature != nil {
		var paramTypes []symbol.Symbol
		for _, pt := range n.Signature.ParamTypes {
			paramSymbol, ok := a.findTypeSymbolInTypes(pt)
			if !ok {
				a.reportError(n.Token, fmt.Sprintf("type error: cannot resolve type name for %s", pt))
			}
			paramTypes = append(paramTypes, paramSymbol)
		}
	
		var returnType symbol.Symbol
		if n.Signature.ReturnType != "" {
			resolvedReturn, ok := a.findTypeSymbolInTypes(n.Signature.ReturnType)
			if !ok {
				a.reportError(n.Token, fmt.Sprintf("type error: cannot resolve type name for %s", n.Signature.ReturnType))
			}
			returnType = resolvedReturn
		}
		aliasedSymbol = symbol.NewFunctionSymbol(len(n.Signature.ParamTypes), paramTypes, returnType)
	} else if n.TargetType != "" {
		resolvedSymbol, ok := a.findTypeSymbolInTypes(n.TargetType)
		if !ok {
			a.reportError(n.Token, fmt.Sprintf("type error: cannot resolve type name for %s", n.TargetType))
		}
		aliasedSymbol = resolvedSymbol
	}

	a.types[n.Name.Value] = aliasedSymbol

	if n.IsPrivate {
		if len(a.scopes) > 1 {
			a.reportError(n.Token, "semantic error: 'private' modifier is only allowed at the top-level of a module")
		} else {
			a.privates[n.Name.Value] = true
		}
	}

	return symbol.AnySymbol()
}

// analyzeFunctionLiteral analyzes a function definition within a new scope,
// registers its parameters, checks its body's return type against the declared
// return type, and verifies that the function guarantees a return if needed.
func (a *Analyzer) analyzeFunctionLiteral(n *syntax.FunctionLiteral) symbol.Symbol {
	a.pushScope()
	a.functionDepth++

	var paramTypes []symbol.Symbol

	for _, param := range n.Parameters {
		paramSymbol, ok := a.findTypeSymbolInTypes(param.Type)
		if !ok {
			a.reportError(n.Token, fmt.Sprintf("type error: cannot resolve type name for %s", param.Type))
		}
		paramTypes = append(paramTypes, paramSymbol)
		a.declare(param.Name, paramSymbol, true)
	}

	actualReturnSymbol := a.analyze(n.Body)
	a.functionDepth--
	a.popScope()

	var expectedReturnSymbol symbol.Symbol
	if n.ReturnType != "" {
		if !guaranteesReturn(n.Body) {
			a.reportError(n.Token, "semantic error: function is missing a guaranteed return statement. All code paths must return a value.")
		}

		resolvedReturn, ok := a.findTypeSymbolInTypes(n.ReturnType)
		if !ok {
			a.reportError(n.Token, fmt.Sprintf("type error: cannot resolve type name for %s", n.ReturnType))
		}
		expectedReturnSymbol = resolvedReturn

		if !expectedReturnSymbol.Equals(actualReturnSymbol) {
			a.reportError(n.Token, fmt.Sprintf("type error: function declared to return %s, but body returns %s", expectedReturnSymbol.Type(), actualReturnSymbol.Type()))
		}
	}

	return symbol.NewFunctionSymbol(len(n.Parameters), paramTypes, expectedReturnSymbol)
}

// analyzeBuiltinCall intercepts calls to builtin functions (like len, append, head, tail)
// to provide custom, compile-time polymorphic type-checking and inference.
func (a *Analyzer) analyzeBuiltinCall(moduleName string, functionName string, n *syntax.CallExpression) (symbol.Symbol, bool) {
	fullName := functionName
	if moduleName != "" {
		fullName = moduleName + "." + functionName
	}

	switch fullName {
	case "array.len":
		return a.analyzeArrayLenFunction(n), true
	case "array.append":
		return a.analyzeArrayAppendFunction(n), true
	case "array.head":
		return a.analyzeArrayHeadFunction(n), true
	case "array.tail":
		return a.analyzeArrayTailFunction(n), true
	case "array.last":
		return a.analyzeArrayLastFunction(n), true
	case "array.copy":
		return a.analyzeArrayCopyFunction(n), true
	case "array.slice":
		return a.analyzeArraySliceFunction(n), true
	case "array.join":
		return a.analyzeArrayJoinFunction(n), true
	case "string.charAt":
		return a.analyzeStringCharAtFunction(n), true
	case "string.substring":
		return a.analyzeStringSubstringFunction(n), true
	case "string.concat":
		return a.analyzeStringConcatFunction(n), true
	case "string.split":
		return a.analyzeStringSplitFunction(n), true
	case "string.contains", "string.startsWith", "string.endsWith":
		return a.analyzeStringMatchFunction(functionName, n), true
	case "string.replace":
		return a.analyzeStringReplaceFunction(n), true
	case "string.toUpper", "string.toLower", "string.trim":
		return a.analyzeStringTransformFunction(functionName, n), true
	case "string.len":
		return a.analyzeStringLenFunction(n), true
	case "date.year", "date.month", "date.day", "date.weekday":
		return a.analyzeDateComponentFunction(functionName, n), true
	case "date.today":
		return a.analyzeDateTodayFunction(n), true
	case "date.parseDate":
		return a.analyzeDateParseDateFunction(n), true
	case "date.addDays":
		return a.analyzeDateAddDaysFunction(n), true
	case "date.diffDays":
		return a.analyzeDateDiffDaysFunction(n), true
	case "date.newDate":
		return a.analyzeDateNewDateFunction(n), true
	case "math.abs", "math.sqrt", "math.floor", "math.ceil", "math.round":
		return a.analyzeMathOneArgFunction(functionName, n), true
	case "math.pow", "math.min", "math.max", "math.log":
		return a.analyzeMathTwoArgFunction(functionName, n), true
	case "log.info", "log.warn", "log.error":
		return a.analyzeLogFunction(functionName, n), true
	case "log.export":
		return a.analyzeLogExportFunction(functionName, n), true
	default:
		return symbol.AnySymbol(), false

	}
}

// analyzeCallExpression analyzes a function call to ensure the target is callable,
// verifies the number of arguments matches the function's arity, and checks
// the types of the provided arguments against the function's parameters.
func (a *Analyzer) analyzeCallExpression(n *syntax.CallExpression) symbol.Symbol {
	sym := a.analyze(n.Function)

	_, isBuiltin := sym.(*symbol.BuiltinSymbol)
	if isBuiltin {
		if ident, ok := n.Function.(*syntax.Identifier); ok {
			if builtinSym, handled := a.analyzeBuiltinCall("", ident.Value, n); handled {
				return builtinSym
			}
		}

		if prop, ok := n.Function.(*syntax.PropertyExpression); ok {
			var modName string
			if objIdent, ok := prop.Object.(*syntax.Identifier); ok {
				modName = objIdent.Value
			}
			if builtinSym, handled := a.analyzeBuiltinCall(modName, prop.Property.Value, n); handled {
				return builtinSym
			}
		}
	}

	fnSymbol, isFunction := sym.(*symbol.FunctionSymbol)
	if !isFunction {
		if sym.Type() != environment.ANY_OBJ && sym.Type() != environment.BUILTIN_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: cannot call a non-function (got %s)", sym.Type()))
		}
		return symbol.AnySymbol()
	}

	if fnSymbol.Type() != environment.ANY_OBJ && len(n.Arguments) != fnSymbol.Arity() {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected %d arguments, got %d", fnSymbol.Arity(), len(n.Arguments)))
	}

	for i, arg := range n.Arguments {
		argSymbol := a.analyze(arg)

		if i < len(fnSymbol.ParamTypes()) {
			expectedType := fnSymbol.ParamTypes()[i]

			if !expectedType.Equals(argSymbol) {
				a.reportError(n.Token, fmt.Sprintf("type error: argument %d expected %s, got %s", i+1, expectedType.Type(), argSymbol.Type()))
			}
		}
	}

	if fnSymbol.ReturnType() != nil {
		return fnSymbol.ReturnType()
	}

	return symbol.AnySymbol()
}

// analyzeImportStatement uses the ImportLoader to statically analyze the imported module
func (a *Analyzer) analyzeImportStatement(n *syntax.ImportStatement) symbol.Symbol {
	modPath := n.Path
	modName := n.Name.Value

	if symbols, ok := symbol.GetStandardModule(modPath); ok {
		modSymbol := symbol.NewModuleSymbol(modName, symbols, nil, nil)
		a.declare(modName, modSymbol, true)
		return modSymbol
	}

	// Check circular dependencies
	if a.loading[modPath] {
		a.reportError(n.Token, fmt.Sprintf("circular import detected: '%s'", modPath))
		return symbol.AnySymbol()
	}

	a.loading[modPath] = true
	defer func() { a.loading[modPath] = false }()

	modProgram, err := modules.Load(a.globalEnv.BaseDir, modPath)
	if err != nil {
		a.reportError(n.Token, fmt.Sprintf("semantic error: failed to import '%s': %v", modPath, err))
		return symbol.AnySymbol()
	}

	// Cache the parsed AST for the evaluator to reuse
	if a.globalEnv != nil {
		a.globalEnv.ModuleASTs[modPath] = modProgram
	}

	modEnv := environment.NewEnvironment(a.globalEnv.BaseDir, modPath, true)
	modEnv.ModuleASTs = a.globalEnv.ModuleASTs
	modAnalyzer := New(modEnv)
	modAnalyzer.loading = a.loading // Share loading state to detect circular dependencies
	modSymbol := modAnalyzer.analyze(modProgram)
	if len(modAnalyzer.errors) > 0 {
		a.reportError(n.Token, fmt.Sprintf("semantic error: failed to analyze module %s", modPath))
		a.errors = append(a.errors, modAnalyzer.errors...)
		return symbol.AnySymbol()
	}

	a.declare(modName, modSymbol, true)
	return modSymbol
}

// analyzePropertyExpression ensures the left side is a module and retrieves the property type
func (a *Analyzer) analyzePropertyExpression(n *syntax.PropertyExpression) symbol.Symbol {
	leftSymbol := a.analyze(n.Object)

	if leftSymbol.Type() == environment.ANY_OBJ {
		return symbol.AnySymbol()
	}

	if leftSymbol.Type() != environment.MODULE_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: property access not supported for %s", leftSymbol.Type()))
		return symbol.AnySymbol()
	}

	modSymbol, ok := leftSymbol.(*symbol.ModuleSymbol)
	if !ok {
		a.reportError(n.Token, fmt.Sprintf("type error: could not parse symbol as module %s", leftSymbol.Type()))
		return symbol.AnySymbol()
	}

	if propertySymbol, ok := modSymbol.GetSymbol(n.Property.Value); ok {
		return propertySymbol
	}

	a.reportError(n.Token, fmt.Sprintf("semantic error: property '%s' not found on module", n.Property.Value))
	return symbol.AnySymbol()
}

// reportError formats and appends a semantic error with the token's line and column.
func (a *Analyzer) reportError(token lexer.Token, msg string) {
	a.errors = append(a.errors, fmt.Sprintf("[Line %d, Column %d] %s", token.Line, token.Column, msg))
}
