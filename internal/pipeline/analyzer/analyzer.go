package analyzer

import (
	"caja-cli/internal/pipeline/analyzer/symbol"
	"caja-cli/internal/pipeline/ast"
	"caja-cli/internal/pipeline/environment"
	"caja-cli/internal/pipeline/lexer"
	"caja-cli/internal/pipeline/modules"
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

// Run initiates the semantic analysis process starting from the given AST node.
func (a *Analyzer) Run(node ast.Node) {
	a.analyze(node)
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

// analyze traverses the AST starting from the given node and performs
// semantic checks, populating the errors slice if any issues are found.
func (a *Analyzer) analyze(node ast.Node) symbol.Symbol {
	switch n := node.(type) {
	case *ast.Program:
		return a.analyzeProgram(n)
	case *ast.BlockStatement:
		return a.analyzeBlockStatement(n)
	case *ast.LetStatement:
		return a.analyzeLetStatement(n)
	case *ast.ConstStatement:
		return a.analyzeConstStatement(n)
	case *ast.AssignStatement:
		return a.analyzeAssignStatement(n)
	case *ast.IndexAssignmentStatement:
		return a.analyzeIndexAssignmentStatement(n)
	case *ast.PropertyAssignmentStatement:
		return a.analyzePropertyAssignmentStatement(n)
	case *ast.Identifier:
		return a.analyzeIdentifier(n)
	case *ast.IfExpression:
		return a.analyzeIfExpression(n)
	case *ast.ReturnStatement:
		return a.analyzeReturnStatement(n)
	case *ast.InfixExpression:
		return a.analyzeInfixExpression(n)
	case *ast.ExpressionStatement:
		return a.analyzeExpressionStatement(n)
	case *ast.TypeAliasStatement:
		return a.analyzeTypeAliasStatement(n)
	case *ast.FunctionLiteral:
		return a.analyzeFunctionLiteral(n)
	case *ast.CallExpression:
		return a.analyzeCallExpression(n)
	case *ast.ArrayLiteral:
		return a.analyzeArrayLiteral(n)
	case *ast.IndexExpression:
		return a.analyzeIndexExpression(n)
	case *ast.PrefixExpression:
		return a.analyzePrefixExpression(n)
	case *ast.NumberLiteral:
		return symbol.NewBasicSymbol(environment.NUMBER_OBJ)
	case *ast.StringLiteral:
		return symbol.NewBasicSymbol(environment.STRING_OBJ)
	case *ast.DateLiteral:
		return symbol.NewBasicSymbol(environment.DATE_OBJ)
	case *ast.BooleanLiteral:
		return symbol.NewBasicSymbol(environment.BOOLEAN_OBJ)
	case *ast.NilLiteral:
		return &symbol.NullSymbol{}
	case *ast.ImportStatement:
		return a.analyzeImportStatement(n)
	case *ast.PropertyExpression:
		return a.analyzePropertyExpression(n)
	case *ast.StructLiteral:
		return a.analyzeStructLiteral(n)
	case *ast.MapLiteral:
		return a.analyzeMapLiteral(n)
	}

	return symbol.AnySymbol()
}

// analyzeProgram iterates over all statements in the program and analyzes them.
func (a *Analyzer) analyzeProgram(n *ast.Program) symbol.Symbol {
	seenNonImport := false
	for _, s := range n.Statements {
		if importStmt, isImport := s.(*ast.ImportStatement); isImport {
			if seenNonImport {
				a.reportError(importStmt.Token, "semantic error: import statements must appear at the beginning of the file")
			}
		} else {
			seenNonImport = true
		}
		a.analyze(s)
	}

	exports := make(map[string]symbol.Symbol)
	types := make(map[string]symbol.Symbol)
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
	for name, sym := range a.types {
		types[name] = sym
		if a.privates[name] {
			privates[name] = true
		}
	}

	return symbol.NewModuleSymbol("module", exports, types, privates, constants)
}

// analyzeArrayLiteral ensures all elements in the array match the type of the first element,
// and returns an ARRAY_OBJ symbol with the inferred ElementType.
func (a *Analyzer) analyzeArrayLiteral(n *ast.ArrayLiteral) symbol.Symbol {
	if len(n.Elements) == 0 {
		return symbol.NewArraySymbol(symbol.AnySymbol())
	}

	firstElSymbol := a.analyze(n.Elements[0])
	for _, el := range n.Elements[1:] {
		elSymbol := a.analyze(el)
		if !firstElSymbol.Equals(elSymbol) {
			a.reportError(n.Token, fmt.Sprintf("type error: array elements must have the same type, expected %s, got %s", firstElSymbol.Type(), elSymbol.Type()))
		}
	}

	return symbol.NewArraySymbol(firstElSymbol)
}

func (a *Analyzer) analyzeMapLiteral(n *ast.MapLiteral) symbol.Symbol {
	if len(n.Pairs) == 0 {
		return symbol.NewMapSymbol(symbol.AnySymbol(), symbol.AnySymbol())
	}

	var firstKeySymbol symbol.Symbol
	var firstValueSymbol symbol.Symbol
	first := true

	for keyNode, valueNode := range n.Pairs {
		keySym := a.analyze(keyNode)
		valSym := a.analyze(valueNode)

		if first {
			firstKeySymbol = keySym
			firstValueSymbol = valSym

			// Validate key type
			isValidKey := false
			if keySym.Type() == environment.STRING_OBJ || keySym.Type() == environment.NUMBER_OBJ || keySym.Type() == environment.ANY_OBJ {
				isValidKey = true
			} else if structSym, ok := keySym.(*symbol.StructInstanceSymbol); ok {
				if keyField, exists := structSym.Def.Fields["key"]; exists {
					if fnSym, ok := keyField.Type.(*symbol.FunctionSymbol); ok {
						if fnSym.ReturnType().Type() == environment.STRING_OBJ {
							isValidKey = true
						}
					}
				}
			}
			if !isValidKey {
				a.reportError(n.Token, "semantic error: map key must be String, Number, or a Struct with a 'key fn(): String' property")
			}

			first = false
		} else {
			if !firstKeySymbol.Equals(keySym) {
				a.reportError(n.Token, fmt.Sprintf("type error: map literal has mixed key types, expected %s, got %s", firstKeySymbol.Type(), keySym.Type()))
			}
			if !firstValueSymbol.Equals(valSym) {
				a.reportError(n.Token, fmt.Sprintf("type error: map literal has mixed value types, expected %s, got %s", firstValueSymbol.Type(), valSym.Type()))
			}
		}
	}

	return symbol.NewMapSymbol(firstKeySymbol, firstValueSymbol)
}

// analyzeFunctionLiteral analyzes a function definition within a new scope,
// registers its parameters, checks its body's return type against the declared
// return type, and verifies that the function guarantees a return if needed.
func (a *Analyzer) analyzeFunctionLiteral(n *ast.FunctionLiteral) symbol.Symbol {
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
		if !ast.GuaranteesReturn(n.Body) {
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

// analyzeStructLiteral validates the fields of a struct instantiation
// against its struct definition, checking for missing or mismatched fields.
func (a *Analyzer) analyzeStructLiteral(n *ast.StructLiteral) symbol.Symbol {
	defSym, ok := a.findTypeSymbolInTypes(n.StructName)
	if !ok {
		a.reportError(n.Token, fmt.Sprintf("semantic error: undefined struct '%s'", n.StructName))
		return symbol.AnySymbol()
	}

	structDef, ok := defSym.(*symbol.StructDefSymbol)
	if !ok {
		a.reportError(n.Token, fmt.Sprintf("type error: '%s' is not a struct", n.StructName))
		return symbol.AnySymbol()
	}

	// Check missing fields
	for fieldName := range structDef.Fields {
		if _, exists := n.Fields[fieldName]; !exists {
			a.reportError(n.Token, fmt.Sprintf("semantic error: missing required field '%s' in struct literal", fieldName))
		}
	}

	// Check excess fields and types
	for fieldName, expr := range n.Fields {
		fieldDef, exists := structDef.Fields[fieldName]
		if !exists {
			a.reportError(n.Token, fmt.Sprintf("semantic error: undefined field '%s' in struct literal", fieldName))
			continue
		}

		valSym := a.analyze(expr)
		if !fieldDef.Type.Equals(valSym) {
			a.reportError(n.Token, fmt.Sprintf("type error: field '%s' expects %s, got %s", fieldName, fieldDef.Type.Type(), valSym.Type()))
		}
	}

	return symbol.NewStructInstanceSymbol(structDef)
}

// analyzeLetStatement checks for variable redeclarations and registers the
// newly declared variable in the current scope with its analyzed type.
func (a *Analyzer) analyzeLetStatement(n *ast.LetStatement) symbol.Symbol {
	if _, exists := a.findVarSymbolInScope(n.Name.Value); exists {
		a.reportError(n.Token, fmt.Sprintf("semantic error: variable '%s' is already declared", n.Name.Value))
	}

	if n.ValueType != "" {
		if _, ok := a.findTypeSymbolInTypes(n.ValueType); !ok {
			a.reportError(n.Token, fmt.Sprintf("semantic error: variable '%s' type is not declared: '%s'", n.Name.Value, n.ValueType))
		}
	}

	if fnNode, ok := n.Value.(*ast.FunctionLiteral); ok {
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

		if ast.GuaranteesRecursiveCall(fnNode.Body, n.Name.Value) {
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
func (a *Analyzer) analyzeConstStatement(n *ast.ConstStatement) symbol.Symbol {
	if _, exists := a.findVarSymbolInScope(n.Name.Value); exists {
		a.reportError(n.Token, fmt.Sprintf("semantic error: variable '%s' is already declared", n.Name.Value))
	}

	if n.ValueType != "" {
		if _, ok := a.findTypeSymbolInTypes(n.ValueType); !ok {
			a.reportError(n.Token, fmt.Sprintf("semantic error: variable '%s' type is not declared: '%s'", n.Name.Value, n.ValueType))
		}
	}

	if fnNode, ok := n.Value.(*ast.FunctionLiteral); ok {
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

		if ast.GuaranteesRecursiveCall(fnNode.Body, n.Name.Value) {
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

// analyzeImportStatement uses the ImportLoader to statically analyze the imported module
func (a *Analyzer) analyzeImportStatement(n *ast.ImportStatement) symbol.Symbol {
	modPath := n.Path
	modName := n.Name.Value

	if symbols, exportedTypes, ok := symbol.GetStandardModule(modPath); ok {
		modSymbol := symbol.NewModuleSymbol(modName, symbols, exportedTypes, nil, nil)
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

// analyzeReturnStatement recursively analyzes the return value expression, if any.
func (a *Analyzer) analyzeReturnStatement(n *ast.ReturnStatement) symbol.Symbol {
	if a.globalEnv.IsModule && a.functionDepth == 0 {
		a.reportError(n.Token, "semantic error: top-level return statements are forbidden inside modules")
	}

	if n.ReturnValue != nil {
		return a.analyze(n.ReturnValue)
	}
	return symbol.AnySymbol()
}

// analyzeAssignStatement ensures the assigned variable has been declared
// and that the type of the assigned value matches the declared type.
func (a *Analyzer) analyzeAssignStatement(n *ast.AssignStatement) symbol.Symbol {
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

// analyzeBlockStatement analyzes a block of statements within a new scope,
// returning the type of the last statement in the block.
func (a *Analyzer) analyzeBlockStatement(n *ast.BlockStatement) symbol.Symbol {
	a.pushScope()
	var lastSymbol symbol.Symbol = symbol.NewBasicSymbol(environment.ANY_OBJ)
	for _, s := range n.Statements {
		if importStmt, isImport := s.(*ast.ImportStatement); isImport {
			a.reportError(importStmt.Token, "semantic error: import statements are only allowed at the top-level of a file")
		}
		lastSymbol = a.analyze(s)
	}
	a.popScope()
	return lastSymbol
}

// analyzeTypeAliasStatement resolves the parameter and return types for a type alias
// and registers the resulting function signature in the analyzer's type registry.
func (a *Analyzer) analyzeTypeAliasStatement(n *ast.TypeAliasStatement) symbol.Symbol {
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
	} else if n.StructDefinition != nil {
		fields := make(map[string]symbol.StructFieldSymbol)
		aliasedSymbol = symbol.NewStructDefSymbol(n.Name.Value, fields)

		// Pre-register the struct in the type registry to allow recursive definitions
		a.types[n.Name.Value] = aliasedSymbol

		for _, field := range n.StructDefinition.Fields {
			fieldSym, ok := a.findTypeSymbolInTypes(field.Type)
			if !ok {
				a.reportError(n.Token, fmt.Sprintf("type error: cannot resolve type name for %s", field.Type))
			}
			fields[field.Name.Value] = symbol.StructFieldSymbol{
				Type:       fieldSym,
				IsConstant: field.IsConstant,
			}
		}
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

// analyzeExpressionStatement wraps the analysis of the inner expression.
func (a *Analyzer) analyzeExpressionStatement(n *ast.ExpressionStatement) symbol.Symbol {
	if n.Expression != nil {
		return a.analyze(n.Expression)
	}
	return symbol.AnySymbol()
}

// analyzeIdentifier resolves the identifier in the current and outer scopes,
// logging an error if it has not been declared.
func (a *Analyzer) analyzeIdentifier(n *ast.Identifier) symbol.Symbol {
	if entry, ok := a.findVarSymbolInScope(n.Value); ok {
		return entry.Sym
	}

	a.reportError(n.Token, fmt.Sprintf("semantic error: undeclared variable '%s'. Use 'let' to declare it.", n.Value))
	return symbol.AnySymbol()
}

// analyzePrefixExpression ensures the right side of a prefix operator
// matches the operator's expected type.
func (a *Analyzer) analyzePrefixExpression(n *ast.PrefixExpression) symbol.Symbol {
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

// analyzeInfixExpression ensures that both the left and right operands
// are of appropriate types for the given operator.
func (a *Analyzer) analyzeInfixExpression(n *ast.InfixExpression) symbol.Symbol {
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
		// Allow comparison if they are the same type, or if one is NULL_OBJ and the other is a NullableSymbol, ANY_OBJ, Struct, Array, or Function.
		_, leftNullable := leftSymbol.(*symbol.NullableSymbol)
		_, rightNullable := rightSymbol.(*symbol.NullableSymbol)

		isValid := leftSymbol.Type() == rightSymbol.Type()
		if !isValid {
			if leftSymbol.Type() == environment.NULL_OBJ && (rightNullable || rightSymbol.Type() == environment.ANY_OBJ || environment.IsReferenceType(rightSymbol.Type())) {
				isValid = true
			} else if rightSymbol.Type() == environment.NULL_OBJ && (leftNullable || leftSymbol.Type() == environment.ANY_OBJ || environment.IsReferenceType(leftSymbol.Type())) {
				isValid = true
			}
		}

		if !isValid {
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

// analyzeIfExpression recursively analyzes its condition and both branches.
// It returns the type of the consequence branch.
func (a *Analyzer) analyzeIfExpression(n *ast.IfExpression) symbol.Symbol {
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

// analyzeCallExpression analyzes a function call to ensure the target is callable,
// verifies the number of arguments matches the function's arity, and checks
// the types of the provided arguments against the function's parameters.
func (a *Analyzer) analyzeCallExpression(n *ast.CallExpression) symbol.Symbol {
	sym := a.analyze(n.Function)

	_, isBuiltin := sym.(*symbol.BuiltinSymbol)
	if isBuiltin {
		if ident, ok := n.Function.(*ast.Identifier); ok {
			if builtinSym, handled := a.analyzeBuiltinCall("", ident.Value, n); handled {
				return builtinSym
			}
		}

		if prop, ok := n.Function.(*ast.PropertyExpression); ok {
			var modName string
			if objIdent, ok := prop.Object.(*ast.Identifier); ok {
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

// analyzeIndexExpression ensures the left side is an array or map and the index is valid.
func (a *Analyzer) analyzeIndexExpression(n *ast.IndexExpression) symbol.Symbol {
	leftSym := a.analyze(n.Left)
	indexSym := a.analyze(n.Index)

	if leftSym.Type() == environment.ARRAY_OBJ {
		if indexSym.Type() != environment.NUMBER_OBJ && indexSym.Type() != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: array index expected NUMBER, got %s", indexSym.Type()))
		}
		if arrSym, ok := leftSym.(*symbol.ArraySymbol); ok {
			return arrSym.ElementSymbol()
		}
	} else if leftSym.Type() == environment.MAP_OBJ {
		if mapSym, ok := leftSym.(*symbol.MapSymbol); ok {
			if !mapSym.Key.Equals(indexSym) && indexSym.Type() != environment.ANY_OBJ {
				a.reportError(n.Token, fmt.Sprintf("type error: map index must be %s, got %s", mapSym.Key.Type(), indexSym.Type()))
			}
			return mapSym.Value
		}
	} else if leftSym.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: index operator not supported for %s", leftSym.Type()))
	}

	return symbol.AnySymbol()
}

// analyzeIndexAssignmentStatement ensures that the indexed target is an array,
// the index is a number, and then evaluates the assigned value.
func (a *Analyzer) analyzeIndexAssignmentStatement(n *ast.IndexAssignmentStatement) symbol.Symbol {
	leftSym := a.analyze(n.Left)
	idxSym := a.analyze(n.Index)
	valSym := a.analyze(n.Value)

	if leftSym.Type() == environment.ARRAY_OBJ {
		if idxSym.Type() != environment.NUMBER_OBJ && idxSym.Type() != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: array index must be NUMBER, got %s", idxSym.Type()))
		}
		if arrSym, ok := leftSym.(*symbol.ArraySymbol); ok {
			if !arrSym.ElementSymbol().Equals(valSym) && valSym.Type() != environment.ANY_OBJ {
				a.reportError(n.Token, fmt.Sprintf("type error: cannot assign %s to array of %s", valSym.Type(), arrSym.ElementSymbol().Type()))
			}
		}
	} else if leftSym.Type() == environment.MAP_OBJ {
		if mapSym, ok := leftSym.(*symbol.MapSymbol); ok {
			if !mapSym.Key.Equals(idxSym) && idxSym.Type() != environment.ANY_OBJ {
				a.reportError(n.Token, fmt.Sprintf("type error: map index must be %s, got %s", mapSym.Key.Type(), idxSym.Type()))
			}
			if !mapSym.Value.Equals(valSym) && valSym.Type() != environment.ANY_OBJ {
				a.reportError(n.Token, fmt.Sprintf("type error: cannot assign %s to map with value type %s", valSym.Type(), mapSym.Value.Type()))
			}
		}
	} else if leftSym.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: index assignment not supported for %s", leftSym.Type()))
	}

	return valSym
}

// analyzePropertyExpression ensures that the object is a module or struct,
// checks that the property exists, and handles nullable object safe navigation.
func (a *Analyzer) analyzePropertyExpression(n *ast.PropertyExpression) symbol.Symbol {
	leftSymbol := a.analyze(n.Object)

	if leftSymbol.Type() == environment.ANY_OBJ {
		// Only short-circuit if it's the actual AnySymbol (BasicSymbol with ANY_OBJ), not StructDefSymbol which might return ANY_OBJ.
		if basic, isBasic := leftSymbol.(*symbol.BasicSymbol); isBasic && basic.Type() == environment.ANY_OBJ {
			return symbol.AnySymbol()
		}
	}

	isNullable := false
	if nullableSym, ok := leftSymbol.(*symbol.NullableSymbol); ok {
		isNullable = true
		leftSymbol = nullableSym.Underlying
	}

	if isNullable && !n.Safe {
		a.reportError(n.Token, fmt.Sprintf("semantic error: property access on nullable type requires safe navigation operator '?.' (property: %s)", n.Property.Value))
		return symbol.AnySymbol()
	} else if !isNullable && n.Safe {
		a.reportError(n.Token, fmt.Sprintf("semantic error: unnecessary safe navigation on non-nullable type (property: %s)", n.Property.Value))
		return symbol.AnySymbol()
	}

	var structDef *symbol.StructDefSymbol
	if inst, ok := leftSymbol.(*symbol.StructInstanceSymbol); ok {
		structDef = inst.Def
	} else if def, ok := leftSymbol.(*symbol.StructDefSymbol); ok {
		structDef = def
	}

	if leftSymbol.Type() != environment.MODULE_OBJ && structDef == nil {
		a.reportError(n.Token, fmt.Sprintf("type error: property access not supported for %s", leftSymbol.Type()))
		return symbol.AnySymbol()
	}

	var propType symbol.Symbol

	if modSymbol, ok := leftSymbol.(*symbol.ModuleSymbol); ok {
		if propertySymbol, ok := modSymbol.GetSymbol(n.Property.Value); ok {
			propType = propertySymbol
		} else {
			a.reportError(n.Token, fmt.Sprintf("semantic error: property '%s' not found on module", n.Property.Value))
			return symbol.AnySymbol()
		}
	} else if structDef != nil {
		if fieldSym, exists := structDef.Fields[n.Property.Value]; exists {
			propType = fieldSym.Type
		} else {
			a.reportError(n.Token, fmt.Sprintf("semantic error: property '%s' not found on struct '%s'", n.Property.Value, structDef.Name))
			return symbol.AnySymbol()
		}
	} else {
		return symbol.AnySymbol()
	}

	if isNullable {
		// If the object is nullable, and we safely navigated, the resulting property access is also nullable.
		if _, alreadyNullable := propType.(*symbol.NullableSymbol); !alreadyNullable {
			propType = &symbol.NullableSymbol{Underlying: propType}
		}
	}
	return propType
}

// analyzePropertyAssignmentStatement ensures that the object is a module,
// checks that the property exists, is not private and is not constant,
// and evaluates the assigned value.
func (a *Analyzer) analyzePropertyAssignmentStatement(n *ast.PropertyAssignmentStatement) symbol.Symbol {
	leftSym := a.analyze(n.Object)

	isNullable := false
	if nullableSym, ok := leftSym.(*symbol.NullableSymbol); ok {
		isNullable = true
		leftSym = nullableSym.Underlying
	}

	if isNullable && !n.Safe {
		a.reportError(n.Token, fmt.Sprintf("semantic error: property assignment on nullable type requires safe navigation operator '?.' (property: %s)", n.Property.Value))
		return symbol.AnySymbol()
	} else if !isNullable && n.Safe {
		a.reportError(n.Token, fmt.Sprintf("semantic error: unnecessary safe navigation on non-nullable type (property: %s)", n.Property.Value))
		return symbol.AnySymbol()
	}

	var structDef *symbol.StructDefSymbol
	if inst, ok := leftSym.(*symbol.StructInstanceSymbol); ok {
		structDef = inst.Def
	} else if def, ok := leftSym.(*symbol.StructDefSymbol); ok {
		structDef = def
	}

	if leftSym.Type() != environment.MODULE_OBJ && leftSym.Type() != environment.ANY_OBJ && structDef == nil {
		a.reportError(n.Token, fmt.Sprintf("type error: property assignment not supported for %s", leftSym.Type()))
	}

	if modSymbol, ok := leftSym.(*symbol.ModuleSymbol); ok {
		// Module symbol is available (e.g., standard modules)
		if modSymbol.IsConstant(n.Property.Value) {
			a.reportError(n.Token, fmt.Sprintf("semantic error: cannot assign to constant property '%s'", n.Property.Value))
		}
	} else if structDef != nil {
		fieldSym, exists := structDef.Fields[n.Property.Value]
		if !exists {
			a.reportError(n.Token, fmt.Sprintf("semantic error: property '%s' not found on struct '%s'", n.Property.Value, structDef.Name))
		} else if fieldSym.IsConstant {
			a.reportError(n.Token, fmt.Sprintf("semantic error: cannot assign to constant property '%s' on struct '%s'", n.Property.Value, structDef.Name))
		} else {
			// type check the assignment
			valSym := a.analyze(n.Value)
			if !fieldSym.Type.Equals(valSym) {
				a.reportError(n.Token, fmt.Sprintf("type error: cannot assign %s to property '%s' of type %s", valSym.Type(), n.Property.Value, fieldSym.Type.Type()))
			}
			return symbol.AnySymbol()
		}
	}

	a.analyze(n.Value)
	return symbol.AnySymbol()
}

// reportError formats and appends a semantic error with the token's line and column.
func (a *Analyzer) reportError(token lexer.Token, msg string) {
	a.errors = append(a.errors, fmt.Sprintf("[Line %d, Column %d] %s", token.Line, token.Column, msg))
}
