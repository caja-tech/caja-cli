package analyzer

import (
	"caja-cli/internal/pipeline/analyzer/symbol"
	"caja-cli/internal/pipeline/ast"
	"caja-cli/internal/pipeline/environment"
	"caja-cli/internal/pipeline/lexer"
	"caja-cli/internal/pipeline/modules"
	"context"

	"fmt"

)

// Analyzer performs semantic analysis on an AST.
// It manages variable scopes and tracks semantic errors such as
// undeclared variables and variable redeclarations.
type Analyzer struct {
	ctx                 context.Context
	scopes              []map[string]ScopeEntry
	types               []map[string]symbol.Symbol
	functionBoundaries  []int
	diagnosticErrors    []ast.DiagnosticError
	nodeSymbols         map[ast.Node]symbol.Symbol
	nodeDefinitions     map[ast.Node]lexer.Token
	nodeDefinitionFiles map[ast.Node]string
	nodeImportedFiles   map[ast.Node]string
	functionDepth       int
	globalEnv           *environment.Environment
	cache               map[string]*Analyzer
	loading             map[string]bool
	privates            map[string]bool
	unwrappedPipeArgs   map[ast.Node]symbol.Symbol
	expectedTypeStack   []symbol.Symbol
	streamStageTypes    map[*ast.StreamPipeExpression]symbol.Symbol
	memoBindingAllowed  bool
	topLevelAwaitAsync map[ast.Node]bool
}

// New creates and returns a new Analyzer with an initial global scope.
func New(globalEnv *environment.Environment) *Analyzer {
	globalScope := make(map[string]ScopeEntry)
	analyzer := &Analyzer{
		scopes:              []map[string]ScopeEntry{globalScope},
		types:               []map[string]symbol.Symbol{make(map[string]symbol.Symbol)},
		diagnosticErrors:    make([]ast.DiagnosticError, 0),
		nodeSymbols:         make(map[ast.Node]symbol.Symbol),
		nodeDefinitions:     make(map[ast.Node]lexer.Token),
		nodeDefinitionFiles: make(map[ast.Node]string),
		nodeImportedFiles:   make(map[ast.Node]string),
		globalEnv:           globalEnv,
		cache:               make(map[string]*Analyzer),
		loading:             make(map[string]bool),
		privates:            make(map[string]bool),
		unwrappedPipeArgs:   make(map[ast.Node]symbol.Symbol),
		expectedTypeStack:   make([]symbol.Symbol, 0),
		streamStageTypes:    make(map[*ast.StreamPipeExpression]symbol.Symbol),
		topLevelAwaitAsync:  make(map[ast.Node]bool),
	}

	// Inject Nothing as a global builtin type
	analyzer.types[0]["Nothing"] = symbol.NewStructDefSymbol("Nothing", nil, make(map[string]symbol.StructFieldSymbol), "")

	return analyzer
}

func (a *Analyzer) pushExpectedType(sym symbol.Symbol) {
	a.expectedTypeStack = append(a.expectedTypeStack, sym)
}

func (a *Analyzer) popExpectedType() {
	if len(a.expectedTypeStack) > 0 {
		a.expectedTypeStack = a.expectedTypeStack[:len(a.expectedTypeStack)-1]
	}
}

func (a *Analyzer) peekExpectedType() symbol.Symbol {
	if len(a.expectedTypeStack) > 0 {
		return a.expectedTypeStack[len(a.expectedTypeStack)-1]
	}
	return nil
}

func (a *Analyzer) WithContext(ctx context.Context) *Analyzer {
	a.ctx = ctx
	return a
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
	var errs []string
	for _, e := range a.diagnosticErrors {
		errs = append(errs, e.String())
	}
	return errs
}

// DiagnosticErrors returns the structured diagnostic errors.
func (a *Analyzer) DiagnosticErrors() []ast.DiagnosticError {
	return a.diagnosticErrors
}

// GetSymbol retrieves the semantic symbol evaluated for the given AST node.
func (a *Analyzer) GetSymbol(node ast.Node) (symbol.Symbol, bool) {
	sym, ok := a.nodeSymbols[node]
	return sym, ok
}

// GetStreamStageType retrieves the resolved per-item output type for a
// stream pipeline stage (the type of a single element flowing out of that
// |>>/?>> stage), as computed by analyzeStreamPipeExpression. Unlike
// GetSymbol on the same node (which, for the outermost stage, reflects the
// whole expression's array type), this always returns the unwrapped element
// type at that stage boundary — used by the transpiler to type the Go
// channel at each point in the generated pipeline.
func (a *Analyzer) GetStreamStageType(node *ast.StreamPipeExpression) (symbol.Symbol, bool) {
	sym, ok := a.streamStageTypes[node]
	return sym, ok
}

// GetDefinition retrieves the token where the symbol in the given AST node was declared, and the file path.
func (a *Analyzer) GetDefinition(node ast.Node) (lexer.Token, string, bool) {
	tok, ok := a.nodeDefinitions[node]
	file := a.nodeDefinitionFiles[node]
	if file == "" && a.globalEnv != nil {
		file = a.globalEnv.FileName
	}
	return tok, file, ok
}

// GetImportedModule returns the module path from which an identifier was imported, if any.
func (a *Analyzer) GetImportedModule(node ast.Node) string {
	return a.nodeImportedFiles[node]
}

// HasErrors returns true if any semantic errors were found.
func (a *Analyzer) HasErrors() bool {
	return len(a.diagnosticErrors) > 0
}

// analyze traverses the AST starting from the given node and performs
// semantic checks, populating the errors slice if any issues are found.
func (a *Analyzer) analyze(node ast.Node) symbol.Symbol {
	if sym, ok := a.unwrappedPipeArgs[node]; ok {
		return sym
	}
	if a.ctx != nil && a.ctx.Err() != nil {
		return symbol.AnySymbol()
	}
	if node == nil {
		return symbol.AnySymbol()
	}
	sym := a.analyzeNode(node)
	a.nodeSymbols[node] = sym
	return sym
}

// analyzeTopLevelValue analyzes an expression that appears directly as the
// value of a let/const statement or as a bare expression statement — the
// only positions where `async`/`await` are legal. `async`/`await` are parsed
// at the lowest precedence so they can greedily capture a trailing pipe
// chain, which makes them ambiguous if nested inside a larger expression, so
// that position is rejected as a semantic error (see the *ast.AsyncExpression
// / *ast.UnwrapExpression cases in analyzeNode). Marking the node here, right
// before delegating to the normal a.analyze dispatch, is what distinguishes
// "legal top-level use" from "illegal nested use" without needing to thread
// a context flag through every other recursive analysis call site.
func (a *Analyzer) analyzeTopLevelValue(expr ast.Expression) symbol.Symbol {
	switch expr.(type) {
	case *ast.AsyncExpression, *ast.UnwrapExpression:
		a.topLevelAwaitAsync[expr] = true
	}
	return a.analyze(expr)
}

func (a *Analyzer) analyzeNode(node ast.Node) symbol.Symbol {
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
	case *ast.TypeConstraintStatement:
		return a.analyzeTypeConstraintStatement(n)
	case *ast.UnionStatement:
		return a.analyzeUnionStatement(n)
	case *ast.IsExpression:
		return a.analyzeIsExpression(n)
	case *ast.FunctionLiteral:
		return a.analyzeFunctionLiteral(n)
	case *ast.SafePipeExpression:
		return a.analyzeSafePipeExpression(n)
	case *ast.StreamPipeExpression:
		return a.analyzeStreamPipeExpression(n)
	case *ast.JoinGroupExpression:
		a.reportError(n.Token, "semantic error: a parallel join group (f & g & h) can only be used as a |>>/?>> stage, immediately followed by another stage that consumes its results")
		return symbol.AnySymbol()
	case *ast.AsyncExpression:
		if !a.topLevelAwaitAsync[n] {
			a.reportError(n.Token, "semantic error: 'async' can only be used as the value of a let/const statement or as a bare statement")
			return symbol.AnySymbol()
		}
		return a.analyzeAsyncExpression(n)
	case *ast.UnwrapExpression:
		if !a.topLevelAwaitAsync[n] {
			a.reportError(n.Token, "semantic error: 'unwrap' can only be used as the value of a let/const statement or as a bare statement")
			return symbol.AnySymbol()
		}
		return a.analyzeUnwrapExpression(n)
	case *ast.AwaitStatement:
		return a.analyzeAwaitStatement(n)
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
	definitions := make(map[string]lexer.Token)

	// Assuming top-level declarations are in the first scope (index 0)
	if len(a.scopes) > 0 {
		for name, entry := range a.scopes[0] {
			exports[name] = entry.Sym
			definitions[name] = entry.DefinitionToken
			if a.privates[name] {
				privates[name] = true
			}
			if entry.IsConstant {
				constants[name] = true
			}
		}
	}
	for name, sym := range a.types[0] {
		types[name] = sym
		if a.privates[name] {
			privates[name] = true
		}
	}

	return symbol.NewModuleSymbol("module", exports, types, privates, constants, definitions, a.globalEnv.FileName)
}

// analyzeArrayLiteral ensures all elements in the array match the type of the first element,
// and returns an ARRAY_OBJ symbol with the inferred ElementType.
func (a *Analyzer) analyzeArrayLiteral(n *ast.ArrayLiteral) symbol.Symbol {
	if len(n.Elements) == 0 {
		// An empty array literal has no elements to infer an element type
		// from; fall back to whatever element type the surrounding context
		// expects (e.g. a call argument typed [SomeStruct]), the same way
		// analyzeFunctionLiteral infers untyped anonymous-function params.
		if expected, ok := a.peekExpectedType().(*symbol.ArraySymbol); ok {
			return symbol.NewArraySymbol(expected.ElementSymbol())
		}
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
	// Captured immediately: analyzing the body below may recurse into nested
	// let/const statements that flip a.memoBindingAllowed for their own
	// values, so it must not be read again after the body has been analyzed.
	wasBoundAsMemo := a.memoBindingAllowed
	a.memoBindingAllowed = false

	expectedType := a.peekExpectedType()
	var expectedFnType *symbol.FunctionSymbol
	if expectedType != nil {
		if fs, ok := expectedType.(*symbol.FunctionSymbol); ok {
			expectedFnType = fs
		}
	}

	a.pushFunctionBoundary()
	a.pushScope()
	a.functionDepth++

	for _, tParam := range n.TypeParameters {
		a.declareType(tParam, symbol.NewGenericSymbol(tParam))
	}

	var paramTypes []symbol.Symbol

	for i, param := range n.Parameters {
		var paramSymbol symbol.Symbol
		if param.Type == "" {
			if expectedFnType != nil && i < len(expectedFnType.ParamTypes()) {
				paramSymbol = expectedFnType.ParamTypes()[i]
			} else {
				a.reportError(n.Token, fmt.Sprintf("type error: cannot infer type for parameter '%s'. Provide an explicit type or context.", param.Name))
				paramSymbol = symbol.AnySymbol()
			}
		} else {
			var ok bool
			paramSymbol, ok = a.findTypeSymbolInTypes(param.Type)
			if !ok {
				a.reportError(n.Token, fmt.Sprintf("type error: cannot resolve type name for %s", param.Type))
				paramSymbol = symbol.AnySymbol()
			}
		}

		paramTypes = append(paramTypes, paramSymbol)
		a.declare(param.Name, paramSymbol, true, n.Token)
		a.nodeSymbols[param] = paramSymbol
	}

	actualReturnSymbol := a.analyze(n.Body)

	// Resolve the return type (which may reference n.TypeParameters, e.g.
	// fn<T>(x: T) -> T) before popping the scope those type parameters were
	// registered into - popScope() also clears that scope's type registry.
	var expectedReturnSymbol symbol.Symbol
	if n.ReturnType != "" {
		resolvedReturn, ok := a.findTypeSymbolInTypes(n.ReturnType)
		if !ok {
			a.reportError(n.Token, fmt.Sprintf("type error: cannot resolve type name for %s", n.ReturnType))
		}
		expectedReturnSymbol = resolvedReturn
	} else if expectedFnType != nil {
		expectedReturnSymbol = expectedFnType.ReturnType()
	}

	a.functionDepth--
	a.popScope()
	a.popFunctionBoundary()

	if expectedReturnSymbol == nil {
		if actualReturnSymbol != nil && actualReturnSymbol.Type() != environment.RETURN_VALUE_OBJ {
			expectedReturnSymbol = actualReturnSymbol
		} else {
			expectedReturnSymbol = symbol.NewBasicSymbol(environment.NULL_OBJ)
		}
	}

	if expectedReturnSymbol != nil {
		isNothing := false
		if def, ok := expectedReturnSymbol.(*symbol.StructDefSymbol); ok && def.Name == "Nothing" {
			isNothing = true
		}

		if !isNothing && !ast.GuaranteesReturn(n.Body) {
			a.reportError(n.Token, "semantic error: function is missing a guaranteed return statement. All code paths must return a value.")
		}

		// If it's Nothing, and we returned Any (empty return) or Nothing, it's fine.
		// If it's Nothing, and we returned something else, it's an error.
		if !expectedReturnSymbol.Equals(actualReturnSymbol) {
			condition1 := isNothing && !ast.GuaranteesReturn(n.Body)
			condition2 := isNothing && actualReturnSymbol.Type() == environment.RETURN_VALUE_OBJ
			if !condition1 && !condition2 {
				a.reportError(n.Token, fmt.Sprintf("type error: function declared to return %s, but body returns %s", expectedReturnSymbol.Type(), actualReturnSymbol.Type()))
			}
		}
	}

	var paramNames []string
	for _, p := range n.Parameters {
		paramNames = append(paramNames, p.Name)
	}
	fnSymbol := symbol.NewFunctionSymbol("", "", paramNames, n.TypeParameters, len(n.Parameters), paramTypes, expectedReturnSymbol)

	if n.IsMemo {
		fnSymbol.IsMemo = true
		a.validateMemoFunction(n, wasBoundAsMemo, paramNames, paramTypes, expectedReturnSymbol)
	}

	return fnSymbol
}

// validateMemoFunction enforces the semantic rules for a 'memo'-modified
// function literal: it must not be generic, must return a value, every
// parameter must be usable as a cache key (function-typed parameters are
// rejected — fmt formats a func value as its address, which the transpiler's
// hashed-key fallback can't use meaningfully; nullable struct parameters are
// fine, since Go's fmt dereferences struct pointers by content), and 'memo'
// must appear directly as the value of a let/const binding.
func (a *Analyzer) validateMemoFunction(n *ast.FunctionLiteral, boundAsLetOrConst bool, paramNames []string, paramTypes []symbol.Symbol, returnType symbol.Symbol) {
	if !boundAsLetOrConst {
		a.reportError(n.Token, "semantic error: 'memo' is currently only supported directly on a let/const binding, e.g. let name = memo fn(...) {...}")
	}

	if len(n.TypeParameters) > 0 {
		a.reportError(n.Token, "semantic error: 'memo' does not support generic functions")
	}

	isNothing := returnType == nil
	if def, ok := returnType.(*symbol.StructDefSymbol); ok && def.Name == "Nothing" {
		isNothing = true
	}
	if isNothing {
		a.reportError(n.Token, "semantic error: memoized function must have a return type")
	}

	for i, pt := range paramTypes {
		name := ""
		if i < len(paramNames) {
			name = paramNames[i]
		}
		if _, isFn := pt.(*symbol.FunctionSymbol); isFn {
			a.reportError(n.Token, fmt.Sprintf("semantic error: memoized function parameter '%s' has type '%s' which cannot be used as a cache key", name, pt.String()))
		}
	}
}

// analyzeStructLiteral validates the fields of a struct instantiation
// against its struct definition, checking for missing or mismatched fields.
func (a *Analyzer) analyzeStructLiteral(n *ast.StructLiteral) symbol.Symbol {
	defSym, ok := a.findTypeSymbolInTypesRaw(n.StructName)
	if !ok {
		a.reportError(n.Token, fmt.Sprintf("semantic error: undefined struct '%s'", n.StructName))
		return symbol.AnySymbol()
	}

	structDef, ok := defSym.(*symbol.StructDefSymbol)
	if !ok {
		a.reportError(n.Token, fmt.Sprintf("type error: '%s' is not a struct", n.StructName))
		return symbol.AnySymbol()
	}

	if len(n.TypeArguments) > 0 {
		if len(n.TypeArguments) != len(structDef.TypeParameters) {
			a.reportError(n.Token, fmt.Sprintf("type error: expected %d type arguments for struct '%s', got %d", len(structDef.TypeParameters), n.StructName, len(n.TypeArguments)))
		} else {
			inferred := make(map[string]symbol.Symbol)
			for i, typeArg := range n.TypeArguments {
				resolvedArg, ok := a.findTypeSymbolInTypes(typeArg)
				if !ok {
					a.reportError(n.Token, fmt.Sprintf("type error: cannot resolve type name for %s", typeArg))
					resolvedArg = symbol.AnySymbol()
				}
				inferred[structDef.TypeParameters[i]] = resolvedArg
			}
			structDef = substituteTypes(structDef, inferred).(*symbol.StructDefSymbol)
		}
	} else if len(structDef.TypeParameters) > 0 {
		a.reportError(n.Token, fmt.Sprintf("type error: missing type arguments for generic struct '%s'", n.StructName))
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
			a.reportError(n.Token, fmt.Sprintf("type error: field '%s' expects %s, got %s", fieldName, fieldDef.Type.String(), valSym.String()))
		}
	}

	return symbol.NewStructInstanceSymbol(structDef)
}

// checkNameAvailable reports a semantic error if name is already bound as a
// type (declared via type/define/union) or as a variable/constant/import
// within the current function's own scope chain (or, at the top level, the
// whole module). It's called by every declaration kind (let, const, type,
// define, union) so that no declaration can silently redeclare or shadow a
// name already claimed by any of the others - types and variables share
// one namespace. The search deliberately stops at the current function's
// boundary rather than walking into an enclosing function or the module
// scope: since Cajá functions are pure and can never read or mutate
// anything declared outside them, reusing an outer name inside a function
// is never actually ambiguous. Returns true if name is free to declare.
func (a *Analyzer) checkNameAvailable(tok lexer.Token, name string) bool {
	if a.typeDeclaredInCurrentFunctionScope(name) {
		a.reportError(tok, fmt.Sprintf("semantic error: '%s' is already declared as a type", name))
		return false
	}
	if entry, exists := a.findVarSymbolInCurrentFunctionScope(name); exists {
		if entry.IsImport {
			a.reportError(tok, fmt.Sprintf("import conflict: variable '%s' is already declared. Suggestion: create an alias for the module", name))
		} else {
			a.reportError(tok, fmt.Sprintf("semantic error: variable '%s' is already declared", name))
		}
		return false
	}
	return true
}

// analyzeLetStatement checks for variable redeclarations and registers the
// newly declared variable in the current scope with its analyzed type.
func (a *Analyzer) analyzeLetStatement(n *ast.LetStatement) symbol.Symbol {
	a.checkNameAvailable(n.Token, n.Name.Value)

	var valType symbol.Symbol
	var hasExplicitType bool
	var explicitType symbol.Symbol

	valType = symbol.AnySymbol()

	if n.ValueType != "" {
		if t, ok := a.findTypeSymbolInTypes(n.ValueType); !ok {
			a.reportError(n.Token, fmt.Sprintf("semantic error: variable '%s' type is not declared: '%s'", n.Name.Value, n.ValueType))
		} else {
			explicitType = t
			hasExplicitType = true
			valType = explicitType
		}
	}

	if fnNode, ok := n.Value.(*ast.FunctionLiteral); ok {
		for _, tParam := range fnNode.TypeParameters {
			a.declareType(tParam, symbol.NewGenericSymbol(tParam))
		}

		var expectedFnType *symbol.FunctionSymbol
		if hasExplicitType {
			if fs, ok := explicitType.(*symbol.FunctionSymbol); ok {
				expectedFnType = fs
			}
		}

		var paramTypes []symbol.Symbol
		for i, param := range fnNode.Parameters {
			if param.Type == "" && expectedFnType != nil && i < len(expectedFnType.ParamTypes()) {
				paramTypes = append(paramTypes, expectedFnType.ParamTypes()[i])
			} else if param.Type == "" {
				// Type cannot be inferred from context, handled in function body analysis
				paramTypes = append(paramTypes, symbol.AnySymbol())
			} else {
				typeName, ok := a.findTypeSymbolInTypes(param.Type)
				if !ok {
					a.reportError(n.Token, fmt.Sprintf("semantic error: variable '%s' type is not declared: '%s'", param.Name, param.Type))
				}
				paramTypes = append(paramTypes, typeName)
			}
		}

		var returnType symbol.Symbol
		if fnNode.ReturnType != "" {
			resolvedReturnType, ok := a.findTypeSymbolInTypes(fnNode.ReturnType)
			if !ok {
				a.reportError(n.Token, fmt.Sprintf("semantic error: function return type is not declared: '%s'", fnNode.ReturnType))
			}
			returnType = resolvedReturnType
		} else if expectedFnType != nil {
			returnType = expectedFnType.ReturnType()
		}

		var paramNames []string
		for _, p := range fnNode.Parameters {
			paramNames = append(paramNames, p.Name)
		}
		fnSymbol := symbol.NewFunctionSymbol("", n.Name.Value, paramNames, fnNode.TypeParameters, len(fnNode.Parameters), paramTypes, returnType)
		a.declare(n.Name.Value, fnSymbol, false, n.Name.Token)

		if ast.GuaranteesRecursiveCall(fnNode.Body, n.Name.Value) {
			a.reportError(n.Token, fmt.Sprintf("semantic error: function '%s' contains unconditional recursion and will infinitely loop", n.Name.Value))
		}

		for _, tParam := range fnNode.TypeParameters {
			a.deleteType(tParam)
		}
	}

	if n.Value != nil {
		if hasExplicitType {
			a.pushExpectedType(explicitType)
		}
		if fnLit, ok := n.Value.(*ast.FunctionLiteral); ok && fnLit.IsMemo {
			a.memoBindingAllowed = true
		}
		rhsType := a.analyzeTopLevelValue(n.Value)
		a.memoBindingAllowed = false
		if hasExplicitType {
			a.popExpectedType()
		}
		if hasExplicitType {
			if !explicitType.Equals(rhsType) && rhsType.Type() != environment.NULL_OBJ && rhsType.Type() != environment.ANY_OBJ {
				a.reportError(n.Token, fmt.Sprintf("type error: cannot assign %s to %s", rhsType.String(), explicitType.String()))
			}
		} else {
			valType = rhsType
		}
	}

	if fnSym, ok := valType.(*symbol.FunctionSymbol); ok && fnSym.Name == "" {
		fnSym.Name = n.Name.Value
	}

	a.declare(n.Name.Value, valType, false, n.Name.Token)

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
	a.checkNameAvailable(n.Token, n.Name.Value)

	var expectedFnType *symbol.FunctionSymbol
	if n.ValueType != "" {
		if explicitType, ok := a.findTypeSymbolInTypes(n.ValueType); ok {
			if fs, ok := explicitType.(*symbol.FunctionSymbol); ok {
				expectedFnType = fs
			}
		}
	}

	if fnNode, ok := n.Value.(*ast.FunctionLiteral); ok {
		for _, tParam := range fnNode.TypeParameters {
			a.declareType(tParam, symbol.NewGenericSymbol(tParam))
		}

		var paramTypes []symbol.Symbol
		for i, param := range fnNode.Parameters {
			if param.Type == "" && expectedFnType != nil && i < len(expectedFnType.ParamTypes()) {
				paramTypes = append(paramTypes, expectedFnType.ParamTypes()[i])
			} else if param.Type == "" {
				paramTypes = append(paramTypes, symbol.AnySymbol())
			} else {
				typeName, ok := a.findTypeSymbolInTypes(param.Type)
				if !ok {
					a.reportError(n.Token, fmt.Sprintf("semantic error: variable '%s' type is not declared: '%s'", param.Name, param.Type))
				}
				paramTypes = append(paramTypes, typeName)
			}
		}

		var returnType symbol.Symbol
		if fnNode.ReturnType != "" {
			resolvedReturnType, ok := a.findTypeSymbolInTypes(fnNode.ReturnType)
			if !ok {
				a.reportError(n.Token, fmt.Sprintf("semantic error: function return type is not declared: '%s'", fnNode.ReturnType))
			}
			returnType = resolvedReturnType
		}

		var paramNames []string
		for _, p := range fnNode.Parameters {
			paramNames = append(paramNames, p.Name)
		}
		fnSymbol := symbol.NewFunctionSymbol("", n.Name.Value, paramNames, fnNode.TypeParameters, len(fnNode.Parameters), paramTypes, returnType)
		a.declare(n.Name.Value, fnSymbol, true, n.Name.Token)

		if ast.GuaranteesRecursiveCall(fnNode.Body, n.Name.Value) {
			a.reportError(n.Token, fmt.Sprintf("semantic error: function '%s' contains unconditional recursion and will infinitely loop", n.Name.Value))
		}

		for _, tParam := range fnNode.TypeParameters {
			a.deleteType(tParam)
		}
	}

	var valType symbol.Symbol
	var hasExplicitType bool
	var explicitType symbol.Symbol

	valType = symbol.AnySymbol()

	if n.ValueType != "" {
		if t, ok := a.findTypeSymbolInTypes(n.ValueType); !ok {
			a.reportError(n.Token, fmt.Sprintf("semantic error: variable '%s' type is not declared: '%s'", n.Name.Value, n.ValueType))
		} else {
			explicitType = t
			hasExplicitType = true
			valType = explicitType
		}
	}

	if n.Value != nil {
		if fnLit, ok := n.Value.(*ast.FunctionLiteral); ok && fnLit.IsMemo {
			a.memoBindingAllowed = true
		}
		rhsType := a.analyzeTopLevelValue(n.Value)
		a.memoBindingAllowed = false
		if hasExplicitType {
			if !explicitType.Equals(rhsType) && rhsType.Type() != environment.NULL_OBJ && rhsType.Type() != environment.ANY_OBJ {
				a.reportError(n.Token, fmt.Sprintf("type error: cannot assign %s to %s", rhsType.String(), explicitType.String()))
			}
		} else {
			valType = rhsType
		}
	}

	if fnSym, ok := valType.(*symbol.FunctionSymbol); ok && fnSym.Name == "" {
		fnSym.Name = n.Name.Value
	}

	a.declare(n.Name.Value, valType, true, n.Name.Token)

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

	var modSymbol symbol.Symbol
	if symbols, exportedTypes, ok := symbol.GetStandardModule(modPath); ok {
		modSymbol = symbol.NewModuleSymbol(modName, symbols, exportedTypes, nil, nil, nil, modPath)
	} else {
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
		modSymbol = modAnalyzer.analyze(modProgram)

		if a.globalEnv != nil {
			if a.globalEnv.ModuleAnalyzers == nil {
				a.globalEnv.ModuleAnalyzers = make(map[string]interface{})
			}
			a.globalEnv.ModuleAnalyzers[modPath] = modAnalyzer
		}

		if len(modAnalyzer.diagnosticErrors) > 0 {
			a.reportError(n.Token, fmt.Sprintf("semantic error: failed to analyze module %s", modPath))
			a.diagnosticErrors = append(a.diagnosticErrors, modAnalyzer.diagnosticErrors...)
			return symbol.AnySymbol()
		}
	}

	a.declare(modName, modSymbol, true, n.Name.Token)

	if len(n.NamedImports) > 0 {
		modSym, ok := modSymbol.(*symbol.ModuleSymbol)
		if !ok {
			a.reportError(n.Token, fmt.Sprintf("semantic error: '%s' is not a module", modPath))
			return symbol.AnySymbol()
		}

		for _, named := range n.NamedImports {
			if sym, exists := modSym.GetSymbol(named.Value); exists {
				a.nodeSymbols[named] = sym
				if defTok, ok := modSym.Definitions[named.Value]; ok {
					a.nodeDefinitions[named] = defTok
				}
				if modSym.FilePath != "" {
					if a.nodeImportedFiles == nil {
						a.nodeImportedFiles = make(map[ast.Node]string)
					}
					a.nodeImportedFiles[named] = modSym.FilePath
				}
				if _, alreadyDeclared := a.findVarSymbolInScope(named.Value); alreadyDeclared {
					a.reportError(named.Token, fmt.Sprintf("import conflict: variable '%s' is already declared. Suggestion: create an alias for the module", named.Value))
				} else {
					a.declareImport(named.Value, sym, true, named.Token, modPath)
				}
			} else {
				a.reportError(named.Token, fmt.Sprintf("semantic error: module '%s' has no exported member '%s'", modPath, named.Value))
			}
		}
	}

	return modSymbol
}

// analyzeReturnStatement recursively analyzes the return value expression, if any.
func (a *Analyzer) analyzeReturnStatement(n *ast.ReturnStatement) symbol.Symbol {
	if a.globalEnv.IsModule && a.functionDepth == 0 {
		a.reportError(n.Token, "semantic error: top-level return statements are forbidden inside modules")
	}

	if n.ReturnValue != nil {
		return a.analyzeTopLevelValue(n.ReturnValue)
	}
	// Return a special token for empty returns so it doesn't match normal types using AnySymbol
	return symbol.NewBasicSymbol(environment.RETURN_VALUE_OBJ)
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
		a.enforcePurity(n.Name, n.Token)
		expectedType := entry.Sym
		if !expectedType.Equals(sym) && expectedType.Type() != environment.ANY_OBJ && sym.Type() != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: cannot assign %s to variable '%s' of type %s", sym.String(), n.Name.Value, expectedType.String()))
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
// analyzeTypeAliasStatement resolves the parameter and return types for a type alias
// and registers the resulting function signature in the analyzer's type registry.
func (a *Analyzer) analyzeTypeConstraintStatement(n *ast.TypeConstraintStatement) symbol.Symbol {
	a.requireTopLevel(n.Token, "define")
	a.checkNameAvailable(n.Token, n.Name.Value)

	baseType, ok := a.findTypeSymbolInTypes(n.BaseType.Value)
	if !ok {
		a.reportError(n.BaseType.Token, fmt.Sprintf("semantic error: base type '%s' is not declared", n.BaseType.Value))
		return symbol.AnySymbol()
	}

	constraint := symbol.NewConstraintSymbol(n.Name.Value, baseType, n.Predicate)
	a.declareType(n.Name.Value, constraint)
	a.nodeSymbols[n.BaseType] = baseType
	a.nodeSymbols[n.Name] = constraint

	// Verify predicate has correct type signature fn(BaseType) -> Boolean
	fnType := a.analyze(n.Predicate)
	if fnSym, ok := fnType.(*symbol.FunctionSymbol); ok {
		if len(fnSym.ParamTypes()) != 1 || !fnSym.ParamTypes()[0].Equals(baseType) {
			a.reportError(n.Token, fmt.Sprintf("type error: constraint predicate must accept a single argument of type %s", n.BaseType.Value))
		}
		if fnSym.ReturnType().Type() != environment.BOOLEAN_OBJ {
			a.reportError(n.Token, "type error: constraint predicate must return Boolean")
		}
	} else if fnType.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, "type error: constraint predicate must be a function")
	}

	return constraint
}

// requireTopLevel reports a semantic error if a type-introducing statement
// (type/define/union) appears anywhere other than a module's top level.
// Standardized across all three: a function-scoped type would only ever be
// usable within that one function, which cuts against the very reason
// these declarations exist in Cajá - building DSL trees whose types are
// shared between a builder function and a separate evaluator function.
// Union has the added, harder constraint that it cannot work locally at
// all (it compiles to a Go interface plus per-variant methods, and Go does
// not allow method declarations inside a function body), so this rule
// keeps every type-introducing declaration to one consistent shape rather
// than carving out an exception per statement kind.
func (a *Analyzer) requireTopLevel(tok lexer.Token, keyword string) {
	if len(a.scopes) > 1 {
		a.reportError(tok, fmt.Sprintf("semantic error: '%s' can only be declared at the top level of a module", keyword))
	}
}

// analyzeUnionStatement resolves each listed variant to an already-declared
// struct type and registers a UnionSymbol under the union's own name. A
// union is a compile-time-only label - it has no runtime representation of
// its own, unlike a struct definition.
func (a *Analyzer) analyzeUnionStatement(n *ast.UnionStatement) symbol.Symbol {
	a.requireTopLevel(n.Token, "union")
	a.checkNameAvailable(n.Token, n.Name.Value)

	variants := make(map[string]*symbol.StructDefSymbol)
	for _, variantIdent := range n.Variants {
		variantSym, ok := a.findTypeSymbolInTypesRaw(variantIdent.Value)
		if !ok {
			a.reportError(variantIdent.Token, fmt.Sprintf("semantic error: undefined type '%s'", variantIdent.Value))
			continue
		}
		structDef, ok := variantSym.(*symbol.StructDefSymbol)
		if !ok {
			a.reportError(variantIdent.Token, fmt.Sprintf("type error: union variant '%s' must be a struct type", variantIdent.Value))
			continue
		}
		if len(structDef.TypeParameters) > 0 {
			a.reportError(variantIdent.Token, fmt.Sprintf("type error: union variant '%s' cannot be a generic struct type", variantIdent.Value))
			continue
		}
		if _, dup := variants[structDef.Name]; dup {
			a.reportError(variantIdent.Token, fmt.Sprintf("semantic error: duplicate union variant '%s'", structDef.Name))
			continue
		}
		variants[structDef.Name] = structDef
		a.nodeSymbols[variantIdent] = structDef
	}

	union := symbol.NewUnionSymbol(n.Name.Value, variants, a.globalEnv.FileName)
	a.declareType(n.Name.Value, union)
	a.nodeSymbols[n.Name] = union

	if n.IsPrivate {
		if len(a.scopes) > 1 {
			a.reportError(n.Token, "semantic error: 'private' modifier is only allowed at the top-level of a module")
		} else {
			a.privates[n.Name.Value] = true
		}
	}

	return union
}

// analyzeIsExpression narrows a union-typed value to one of its listed
// variants, e.g. `animal is Cat`, resolving to Cat? (nil at runtime if the
// value isn't actually a Cat).
func (a *Analyzer) analyzeIsExpression(n *ast.IsExpression) symbol.Symbol {
	leftSym := a.analyze(n.Left)

	variantSym, ok := a.findTypeSymbolInTypesRaw(n.TypeName)
	if !ok {
		a.reportError(n.Token, fmt.Sprintf("semantic error: undefined type '%s'", n.TypeName))
		return symbol.AnySymbol()
	}
	variantStructDef, ok := variantSym.(*symbol.StructDefSymbol)
	if !ok {
		a.reportError(n.Token, fmt.Sprintf("type error: 'is' target '%s' must be a struct type", n.TypeName))
		return symbol.AnySymbol()
	}

	if leftSym.Type() != environment.ANY_OBJ {
		unionSym, ok := leftSym.(*symbol.UnionSymbol)
		if !ok {
			a.reportError(n.Token, fmt.Sprintf("type error: 'is' can only be used on a union type, got %s", leftSym.String()))
			return &symbol.NullableSymbol{Underlying: variantStructDef}
		}
		// Key by the resolved struct's own (unqualified) name, not the raw
		// TypeName string - n.TypeName may be module-qualified (e.g.
		// "animals.Cat") while UnionSymbol.Variants is always keyed by the
		// bare struct name, matching how StructDefSymbol.Name is stored.
		if _, isVariant := unionSym.Variants[variantStructDef.Name]; !isVariant {
			a.reportError(n.Token, fmt.Sprintf("type error: '%s' is not a variant of union '%s'", n.TypeName, unionSym.Name))
		}
	}

	return &symbol.NullableSymbol{Underlying: variantStructDef}
}

func (a *Analyzer) analyzeTypeAliasStatement(n *ast.TypeAliasStatement) symbol.Symbol {
	a.requireTopLevel(n.Token, "type")
	a.checkNameAvailable(n.Token, n.Name.Value)

	var aliasedSymbol symbol.Symbol

	// Temporarily register TypeParameters into the registry to support recursive lookups (e.g. fn(T) -> T)
	for _, tp := range n.TypeParameters {
		a.declareType(tp, symbol.NewGenericSymbol(tp))
	}

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

		fnSym := symbol.NewFunctionSymbol("", n.Name.Value, nil, nil, len(n.Signature.ParamTypes), paramTypes, returnType)
		if len(n.TypeParameters) > 0 {
			fnSym.TypeParameters = n.TypeParameters
		}
		aliasedSymbol = fnSym
	} else if n.StructDefinition != nil {
		fields := make(map[string]symbol.StructFieldSymbol)
		aliasedSymbol = symbol.NewStructDefSymbol(n.Name.Value, n.TypeParameters, fields, a.globalEnv.FileName)

		// Pre-register the struct in the type registry to allow recursive definitions
		a.declareType(n.Name.Value, aliasedSymbol)

		for _, field := range n.StructDefinition.Fields {
			if _, exists := fields[field.Name.Value]; exists {
				a.reportError(n.Token, fmt.Sprintf("semantic error: duplicate field '%s' in struct '%s'", field.Name.Value, n.Name.Value))
				continue
			}
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

	// Clean up temporary TypeParameters
	for _, tp := range n.TypeParameters {
		a.deleteType(tp)
	}

	a.declareType(n.Name.Value, aliasedSymbol)

	if n.IsPrivate {
		if len(a.scopes) > 1 {
			a.reportError(n.Token, "semantic error: 'private' modifier is only allowed at the top-level of a module")
		} else {
			a.privates[n.Name.Value] = true
		}
	}

	return aliasedSymbol
}

// analyzeExpressionStatement wraps the analysis of the inner expression.
func (a *Analyzer) analyzeExpressionStatement(n *ast.ExpressionStatement) symbol.Symbol {
	if n.Expression != nil {
		return a.analyzeTopLevelValue(n.Expression)
	}
	return symbol.AnySymbol()
}

// analyzeIdentifier resolves the identifier in the current and outer scopes,
// logging an error if it has not been declared.
func (a *Analyzer) analyzeIdentifier(n *ast.Identifier) symbol.Symbol {
	if entry, ok := a.findVarSymbolInScope(n.Value); ok {
		if entry.IsMoved {
			a.reportError(n.Token, fmt.Sprintf("semantic error: use of moved variable '%s'", n.Value))
		}
		if a.functionDepth > 0 && entry.FunctionDepth == 0 {
			if !entry.IsConstant && !entry.IsImport && entry.Sym.Type() != environment.FUNCTION_OBJ && entry.Sym.Type() != environment.MODULE_OBJ {
				a.reportError(n.Token, fmt.Sprintf("semantic error: pure functions cannot capture global/module variable '%s'", n.Value))
			}
		}
		a.nodeDefinitions[n] = entry.DefinitionToken
		if entry.IsImport && entry.FilePath != "" {
			if a.nodeDefinitionFiles == nil {
				a.nodeImportedFiles = make(map[ast.Node]string)
			}
			a.nodeImportedFiles[n] = entry.FilePath
		}
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
			a.reportError(n.Token, fmt.Sprintf("type error: operator '!' requires a Boolean, got %s", rightSymbol.Type()))
		}
		return symbol.NewBasicSymbol(environment.BOOLEAN_OBJ)
	case "-":
		if rightSymbol.Type() != environment.NUMBER_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: operator '-' requires a Number, got %s", rightSymbol.Type()))
		}
		return symbol.NewBasicSymbol(environment.NUMBER_OBJ)
	case "move":
		if ident, ok := n.Right.(*ast.Identifier); ok {
			entry, exists := a.findVarSymbolInScope(ident.Value)
			if exists && entry.IsConstant {
				a.reportError(n.Token, fmt.Sprintf("semantic error: cannot move constant variable '%s'", ident.Value))
			} else {
				a.markVarMoved(ident.Value)
			}
		}
		return rightSymbol
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
			a.reportError(n.Token, fmt.Sprintf("type error: operator '%s' requires two Numbers, got %s and %s", n.Operator, leftSymbol.Type(), rightSymbol.Type()))
			return symbol.NewBasicSymbol(environment.ANY_OBJ)
		}
		return symbol.NewBasicSymbol(environment.NUMBER_OBJ)

	case "<", ">", "<=", ">=":
		if leftSymbol.Type() != environment.NUMBER_OBJ || rightSymbol.Type() != environment.NUMBER_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: operator '%s' requires two Numbers, got %s and %s", n.Operator, leftSymbol.Type(), rightSymbol.Type()))
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
			a.reportError(n.Token, fmt.Sprintf("type error: operator '%s' requires two Booleans, got %s and %s", n.Operator, leftSymbol.Type(), rightSymbol.Type()))
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
		a.reportError(n.Token, fmt.Sprintf("type error: condition must be a Boolean, got %s", condSymbol.Type()))
	}

	snapshot := a.snapshotMovedVars()

	trueType := a.analyze(n.Consequence)

	movedAfterIf := a.snapshotMovedVars()
	a.restoreMovedVars(snapshot)

	if n.Alternative != nil {
		a.analyze(n.Alternative)
	}

	a.unionMovedVars(movedAfterIf)
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
			modName := a.GetImportedModule(ident)
			if builtinSym, handled := a.analyzeBuiltinCall(modName, ident.Value, n); handled {
				return builtinSym
			}
		}

		if prop, ok := n.Function.(*ast.PropertyExpression); ok {
			modName := ""
			if builtinSym, ok := sym.(*symbol.BuiltinSymbol); ok {
				modName = builtinSym.ModuleName
			}
			if modName == "" {
				if objIdent, ok := prop.Object.(*ast.Identifier); ok {
					modName = objIdent.Value
				}
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

	inferredTypes := make(map[string]symbol.Symbol)
	argSymbols := make([]symbol.Symbol, len(n.Arguments))

	for i, arg := range n.Arguments {
		var expectedArgType symbol.Symbol
		if fnSymbol != nil && i < len(fnSymbol.ParamTypes()) {
			expectedArgType = fnSymbol.ParamTypes()[i]
			a.pushExpectedType(expectedArgType)
		}

		argSymbols[i] = a.analyze(arg)

		if expectedArgType != nil {
			a.popExpectedType()
		}
	}

	// First pass: resolve explicit type arguments or infer generic type parameters
	if len(fnSymbol.TypeParameters) > 0 {
		if len(n.TypeArguments) > 0 {
			if len(n.TypeArguments) != len(fnSymbol.TypeParameters) {
				a.reportError(n.Token, fmt.Sprintf("arity error: expected %d generic type arguments, got %d", len(fnSymbol.TypeParameters), len(n.TypeArguments)))
			} else {
				for i, typeArgName := range n.TypeArguments {
					resolvedArg, ok := a.findTypeSymbolInTypes(typeArgName)
					if !ok {
						a.reportError(n.Token, fmt.Sprintf("type error: cannot resolve type name for %s", typeArgName))
						resolvedArg = symbol.AnySymbol()
					}
					tParam := fnSymbol.TypeParameters[i]
					inferredTypes[tParam] = resolvedArg
				}
			}
		} else {
			for i := range n.Arguments {
				if i < len(fnSymbol.ParamTypes()) {
					expectedType := fnSymbol.ParamTypes()[i]
					err := inferTypes(expectedType, argSymbols[i], inferredTypes)
					if err != nil {
						a.reportError(n.Token, fmt.Sprintf("type inference error: %v", err))
					}
				}
			}
		}
	} else if len(n.TypeArguments) > 0 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 0 generic type arguments, got %d", len(n.TypeArguments)))
	}

	for i := range n.Arguments {
		argSymbol := argSymbols[i]

		if i < len(fnSymbol.ParamTypes()) {
			expectedType := fnSymbol.ParamTypes()[i]
			// Substitute inferred types
			finalExpected := substituteTypes(expectedType, inferredTypes)

			if !finalExpected.Equals(argSymbol) {
				a.reportError(n.Token, fmt.Sprintf("type error: argument %d expected %s, got %s", i+1, finalExpected.String(), argSymbol.String()))
			}
		}
	}

	if fnSymbol.ReturnType() != nil {
		return substituteTypes(fnSymbol.ReturnType(), inferredTypes)
	}

	return symbol.AnySymbol()
}

// analyzeIndexExpression ensures the left side is an array or map and the index is valid.
func (a *Analyzer) analyzeIndexExpression(n *ast.IndexExpression) symbol.Symbol {
	leftSym := a.analyze(n.Left)
	indexSym := a.analyze(n.Index)

	if leftSym.Type() == environment.ARRAY_OBJ {
		if indexSym.Type() != environment.NUMBER_OBJ && indexSym.Type() != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: array index expected Number, got %s", indexSym.Type()))
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
	a.enforcePurity(n.Left, n.Token)
	leftSym := a.analyze(n.Left)
	idxSym := a.analyze(n.Index)
	valSym := a.analyze(n.Value)

	if leftSym.Type() == environment.ARRAY_OBJ {
		if idxSym.Type() != environment.NUMBER_OBJ && idxSym.Type() != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: array index must be Number, got %s", idxSym.Type()))
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

	if constraintSym, ok := leftSymbol.(*symbol.ConstraintSymbol); ok {
		leftSymbol = constraintSym.BaseType
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
			if modSymbol.IsPrivate(n.Property.Value) {
				a.reportError(n.Token, fmt.Sprintf("semantic error: property '%s' is private and cannot be accessed from outside the module", n.Property.Value))
				return symbol.AnySymbol()
			}
			propType = propertySymbol
			a.nodeSymbols[n.Property] = propertySymbol
			if defToken, ok := modSymbol.Definitions[n.Property.Value]; ok {
				a.nodeDefinitions[n.Property] = defToken
				a.nodeDefinitionFiles[n.Property] = modSymbol.FilePath
			}
		} else {
			a.reportError(n.Token, fmt.Sprintf("semantic error: property '%s' not found on module", n.Property.Value))
			return symbol.AnySymbol()
		}
	} else if structDef != nil {
		if fieldSym, exists := structDef.Fields[n.Property.Value]; exists {
			propType = fieldSym.Type
			a.nodeSymbols[n.Property] = propType
			// We could also store definition if StructDefSymbol tracked it, but we don't have it yet.
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
	a.enforcePurity(n.Object, n.Token)
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
		if modSymbol.IsPrivate(n.Property.Value) {
			a.reportError(n.Token, fmt.Sprintf("semantic error: property '%s' is private and cannot be assigned from outside the module", n.Property.Value))
		}
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
				a.reportError(n.Token, fmt.Sprintf("type error: cannot assign %s to property '%s' of type %s", valSym.String(), n.Property.Value, fieldSym.Type.String()))
			}
			return symbol.AnySymbol()
		}
	}

	a.analyze(n.Value)
	return symbol.AnySymbol()
}

// reportError formats and appends a semantic error with the token's line and column.
func (a *Analyzer) reportError(token lexer.Token, msg string) {
	a.diagnosticErrors = append(a.diagnosticErrors, ast.DiagnosticError{Token: token, Message: msg})
}

// enforcePurity checks if the given expression is mutating an outer scope variable.
// It recursively traverses down PropertyExpression and IndexExpression to find the root Identifier.
func (a *Analyzer) enforcePurity(node ast.Node, token lexer.Token) {
	switch n := node.(type) {
	case *ast.Identifier:
		entry, exists := a.findVarSymbolInScope(n.Value)
		if exists {
			if entry.FunctionDepth < a.functionDepth {
				a.reportError(token, fmt.Sprintf("semantic error: cannot mutate outer scope variable '%s' inside a function", n.Value))
			}
			if entry.IsConstant {
				a.reportError(token, fmt.Sprintf("semantic error: cannot mutate property/index of constant variable '%s'", n.Value))
			}
		}
	case *ast.PropertyExpression:
		a.enforcePurity(n.Object, token)
	case *ast.IndexExpression:
		a.enforcePurity(n.Left, token)
	}
}

// analyzeAsyncExpression type-checks `async <expr>`, wrapping the analyzed
// type of Right in an AsyncSymbol. Only reachable (without a semantic error)
// when n was registered via analyzeTopLevelValue.
func (a *Analyzer) analyzeAsyncExpression(n *ast.AsyncExpression) symbol.Symbol {
	underlying := a.analyze(n.Right)
	return &symbol.AsyncSymbol{Underlying: underlying}
}

// analyzeUnwrapExpression type-checks `unwrap <expr>`, requiring Right to be
// an async value and extracting it to its underlying type. This is the only
// construct that produces a plain value out of an async handle — `await` is
// a pure synchronization barrier and never does (see analyzeAwaitStatement).
// Only reachable (without a semantic error) when n was registered via
// analyzeTopLevelValue.
func (a *Analyzer) analyzeUnwrapExpression(n *ast.UnwrapExpression) symbol.Symbol {
	rightSym := a.analyze(n.Right)
	if asyncSym, ok := rightSym.(*symbol.AsyncSymbol); ok {
		return asyncSym.Underlying
	}
	if rightSym.Type() == environment.ANY_OBJ {
		return symbol.AnySymbol()
	}
	a.reportError(n.Token, fmt.Sprintf("semantic error: 'unwrap' requires an async value, got %s", rightSym.String()))
	return rightSym
}

// analyzeAwaitStatement type-checks `await p1 & p2 & ... & pn`, a
// synchronization barrier requiring every operand to be an async value. It
// produces no result of its own (the barrier is not assignable) — individual
// results are retrieved afterward via separate `unwrap <name>` expressions.
func (a *Analyzer) analyzeAwaitStatement(n *ast.AwaitStatement) symbol.Symbol {
	for _, p := range n.Pipelines {
		sym := a.analyze(p)
		if _, ok := sym.(*symbol.AsyncSymbol); ok {
			continue
		}
		if sym.Type() == environment.ANY_OBJ {
			continue
		}
		a.reportError(n.Token, fmt.Sprintf("semantic error: 'await' operand is not an async value, got %s", sym.String()))
	}
	return symbol.AnySymbol()
}

func (a *Analyzer) analyzeSafePipeExpression(n *ast.SafePipeExpression) symbol.Symbol {
	leftSymbol := a.analyze(n.Left)

	var underlying symbol.Symbol
	if nullable, ok := leftSymbol.(*symbol.NullableSymbol); ok {
		underlying = nullable.Underlying
	} else if leftSymbol.Type() == environment.ANY_OBJ {
		underlying = symbol.AnySymbol()
	} else {
		a.reportError(n.Token, fmt.Sprintf("semantic error: unnecessary safe pipe on non-nullable type"))
		underlying = leftSymbol
	}

	a.unwrappedPipeArgs[n.Left] = underlying
	resultSym := a.analyzeCallExpression(n.Call)
	delete(a.unwrappedPipeArgs, n.Left)

	if resultSym.Type() == environment.ANY_OBJ || resultSym.Type() == environment.NULL_OBJ {
		return resultSym
	}
	if _, isNullable := resultSym.(*symbol.NullableSymbol); isNullable {
		return resultSym
	}
	return &symbol.NullableSymbol{Underlying: resultSym}
}

// analyzeStreamPipeExpression type-checks a streaming pipeline chain
// (|>>/?>>). Unlike |>/?>, where the whole collection flows as the next
// call's first argument, each stream stage operates per-item: the chain is
// flattened first, the source expression's array element type is threaded
// through each stage as the expected first-argument type (reusing the same
// unwrappedPipeArgs override that analyzeSafePipeExpression uses for a
// single pipe), and the overall expression's type is an array of whatever
// element type the final stage produces.
func (a *Analyzer) analyzeStreamPipeExpression(n *ast.StreamPipeExpression) symbol.Symbol {
	if n.Call == nil {
		a.reportError(n.Token, "semantic error: a parallel join group (f & g & h) must be followed by another stage that consumes its results")
		return symbol.NewArraySymbol(symbol.AnySymbol())
	}

	var stages []*ast.StreamPipeExpression
	for cur := n; ; {
		stages = append(stages, cur)
		next, ok := cur.Left.(*ast.StreamPipeExpression)
		if !ok {
			break
		}
		cur = next
	}
	// stages was collected sink-to-source; reverse it into source-to-sink order.
	for i, j := 0, len(stages)-1; i < j; i, j = i+1, j-1 {
		stages[i], stages[j] = stages[j], stages[i]
	}

	sourceExpr := stages[0].Left
	sourceSym := a.analyze(sourceExpr)

	var currentElem symbol.Symbol
	switch sym := sourceSym.(type) {
	case *symbol.ArraySymbol:
		currentElem = sym.ElementSymbol()
	default:
		if sourceSym.Type() == environment.ANY_OBJ {
			currentElem = symbol.AnySymbol()
		} else {
			a.reportError(n.Token, fmt.Sprintf("semantic error: stream pipe source must be an array, got %s", sourceSym.Type()))
			currentElem = symbol.AnySymbol()
		}
	}

	for _, stage := range stages {
		expectedElem := currentElem
		if stage.Safe {
			if nullable, ok := currentElem.(*symbol.NullableSymbol); ok {
				expectedElem = nullable.Underlying
			} else if currentElem.Type() == environment.ANY_OBJ {
				expectedElem = symbol.AnySymbol()
			} else {
				a.reportError(stage.Token, "semantic error: unnecessary safe stream pipe on non-nullable element type")
			}
		}

		a.unwrappedPipeArgs[stage.Left] = expectedElem

		var resultSym symbol.Symbol
		if stage.Join != nil {
			resultSym = a.analyzeJoinStage(stage)
		} else {
			resultSym = a.analyzeCallExpression(stage.Call)
		}

		delete(a.unwrappedPipeArgs, stage.Left)

		a.streamStageTypes[stage] = resultSym
		currentElem = resultSym
	}

	return symbol.NewArraySymbol(currentElem)
}

// analyzeJoinStage type-checks a fixed-size parallel join stage. Each call in
// stage.Join.Calls independently receives the stage's upstream element (via
// the unwrappedPipeArgs override the caller already set on stage.Left,
// shared by every call in the group since they all read the same upstream
// item), and their N result types are threaded into stage.Call's reserved
// leading argument slots (see parseStreamPipeExpressionCommon's fusion
// logic) before delegating to the normal call-analysis path, which validates
// them against stage.Call's function signature exactly like any other
// argument.
func (a *Analyzer) analyzeJoinStage(stage *ast.StreamPipeExpression) symbol.Symbol {
	n := len(stage.Join.Calls)
	placeholders := make([]*ast.Identifier, n)
	for i, joinCall := range stage.Join.Calls {
		// Analyzed via a.analyze (not a.analyzeCallExpression directly) so the
		// result is cached in nodeSymbols/GetSymbol for the transpiler to
		// retrieve later when typing each join call's temp variable.
		resultSym := a.analyze(joinCall)
		ph := &ast.Identifier{Value: fmt.Sprintf("_join%d", i)}
		a.unwrappedPipeArgs[ph] = resultSym
		placeholders[i] = ph
		stage.Call.Arguments[i] = ph
	}

	resultSym := a.analyzeCallExpression(stage.Call)

	for i, ph := range placeholders {
		delete(a.unwrappedPipeArgs, ph)
		stage.Call.Arguments[i] = nil
	}

	return resultSym
}

func (a *Analyzer) GlobalEnv() *environment.Environment {
	return a.globalEnv
}
