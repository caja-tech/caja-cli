package compiler

import (
	"bytes"
	"caja-cli/internal/pipeline/analyzer"
	"caja-cli/internal/pipeline/analyzer/symbol"
	"caja-cli/internal/pipeline/ast"
	"caja-cli/internal/pipeline/environment"

	"fmt"
	"strings"
)

type transpileContext struct {
	analyzer              *analyzer.Analyzer
	currentFuncName       string
	currentFunctionParams []string
	hasTailCall           bool
	usedModules           map[string]bool
	CurrentModulePath     string
	packageLevelCode      *bytes.Buffer
	inFunction            bool
}

// ctx.mapSymbolToGoType converts a semantic symbol to a static Go type string.
func (ctx *transpileContext) mapSymbolToGoType(sym symbol.Symbol) string {
	if sym == nil {
		return "any"
	}

	if constraint, ok := sym.(*symbol.ConstraintSymbol); ok {
		return "*" + constraint.Name
	}

	// Interface checks must happen BEFORE sym.Type() because some symbols
	// (like NullableSymbol) forward their Type() to their underlying symbol!
	if nullableSym, ok := sym.(*symbol.NullableSymbol); ok {
		underlyingType := ctx.mapSymbolToGoType(nullableSym.Underlying)
		if strings.HasPrefix(underlyingType, "*") || strings.HasPrefix(underlyingType, "map[") ||
			strings.HasPrefix(underlyingType, "[]") || strings.HasPrefix(underlyingType, "func(") ||
			underlyingType == "any" {
			return underlyingType
		}
		return "*" + underlyingType
	}
	if arrSym, ok := sym.(*symbol.ArraySymbol); ok {
		elType := ctx.mapSymbolToGoType(arrSym.ElementSymbol())
		if elType == "" {
			elType = "any"
		}
		return "[]" + elType
	}
	if mapSym, ok := sym.(*symbol.MapSymbol); ok {
		kType := ctx.mapSymbolToGoType(mapSym.Key)
		vType := ctx.mapSymbolToGoType(mapSym.Value)
		if kType == "" {
			kType = "any"
		} else if strings.HasPrefix(kType, "*") {
			kType = "string"
		}
		if vType == "" {
			vType = "any"
		}
		return "map[" + kType + "]" + vType
	}
	if fnSym, ok := sym.(*symbol.FunctionSymbol); ok {
		var paramTypes []string
		for _, p := range fnSym.ParamTypes() {
			pt := ctx.mapSymbolToGoType(p)
			if pt == "" {
				pt = "any"
			}
			paramTypes = append(paramTypes, pt)
		}

		retType := ""
		if fnSym.ReturnType() != nil && fnSym.ReturnType().Type() != environment.NULL_OBJ {
			retType = ctx.mapSymbolToGoType(fnSym.ReturnType())
		}

		goFunc := fmt.Sprintf("func(%s)", strings.Join(paramTypes, ", "))
		if retType != "" {
			goFunc += " " + retType
		}
		return goFunc
	}
	if structDef, ok := sym.(*symbol.StructDefSymbol); ok {
		baseName := structDef.Name
		if structDef.FilePath != "" && ctx != nil && structDef.FilePath != ctx.analyzer.GlobalEnv().FileName {
			baseName = sanitizeIdentifier(structDef.FilePath) + "_" + baseName
		}
		if len(structDef.InstantiatedTypes) > 0 {
			var typeArgs []string
			for _, t := range structDef.InstantiatedTypes {
				typeArgs = append(typeArgs, ctx.mapSymbolToGoType(t))
			}
			return fmt.Sprintf("*%s[%s]", baseName, strings.Join(typeArgs, ", "))
		}
		return "*" + baseName
	}
	if structInst, ok := sym.(*symbol.StructInstanceSymbol); ok {
		baseName := structInst.Def.Name
		if structInst.Def.FilePath != "" && ctx != nil && structInst.Def.FilePath != ctx.analyzer.GlobalEnv().FileName {
			baseName = sanitizeIdentifier(structInst.Def.FilePath) + "_" + baseName
		}
		if len(structInst.Def.InstantiatedTypes) > 0 {
			var typeArgs []string
			for _, t := range structInst.Def.InstantiatedTypes {
				typeArgs = append(typeArgs, ctx.mapSymbolToGoType(t))
			}
			return fmt.Sprintf("*%s[%s]", baseName, strings.Join(typeArgs, ", "))
		}
		return "*" + baseName
	}

	switch sym.Type() {
	case environment.ANY_OBJ:
		if genSym, ok := sym.(*symbol.GenericSymbol); ok {
			return genSym.Name
		}
		return "any"
	case environment.NUMBER_OBJ:
		return "float64"
	case environment.STRING_OBJ:
		return "string"
	case environment.BOOLEAN_OBJ:
		return "bool"
	case environment.DATE_OBJ:
		return "time.Time"
	default:
		return ""
	}
}

// Transpile walks the AST and returns the equivalent Go source code.
func Transpile(program *ast.Program, a *analyzer.Analyzer) (string, error) {
	var bodyBuf bytes.Buffer
	var pkgLevelBuf bytes.Buffer

	ctx := &transpileContext{
		analyzer:         a,
		usedModules:      make(map[string]bool),
		packageLevelCode: &pkgLevelBuf,
	}

	// Prepend imported custom modules in topological order
	if a.GlobalEnv() != nil {
		orderedModules := getOrderedModules(program, a.GlobalEnv().ModuleASTs)
		for _, modPath := range orderedModules {
			modAST := a.GlobalEnv().ModuleASTs[modPath]
			modAnalyzer := a.GlobalEnv().ModuleAnalyzers[modPath].(*analyzer.Analyzer)
			ctx.analyzer = modAnalyzer
			ctx.CurrentModulePath = modPath
			bodyBuf.WriteString(fmt.Sprintf("\t// --- Module: %s ---\n", modPath))
			for _, stmt := range modAST.Statements {
				code, err := transpileStatement(stmt, ctx)
				if err != nil {
					return "", err
				}
				if code == "" {
					continue
				}

				if _, isTypeAlias := stmt.(*ast.TypeAliasStatement); isTypeAlias {
					ctx.packageLevelCode.WriteString(code + "\n")
					continue
				}

				bodyBuf.WriteString("\t" + code + "\n")

				if letStmt, ok := stmt.(*ast.LetStatement); ok {
					bodyBuf.WriteString(fmt.Sprintf("\t_ = %s\n", sanitizeIdentifier(ctx.CurrentModulePath)+"_"+letStmt.Name.Value))
				} else if constStmt, ok := stmt.(*ast.ConstStatement); ok {
					bodyBuf.WriteString(fmt.Sprintf("\t_ = %s\n", sanitizeIdentifier(ctx.CurrentModulePath)+"_"+constStmt.Name.Value))
				}
			}
			bodyBuf.WriteString("\n")
		}
	}
	ctx.CurrentModulePath = ""
	ctx.analyzer = a

	for _, stmt := range program.Statements {
		code, err := transpileStatement(stmt, ctx)
		if err != nil {
			return "", err
		}
		if code == "" {
			continue
		}

		if _, isTypeAlias := stmt.(*ast.TypeAliasStatement); isTypeAlias {
			ctx.packageLevelCode.WriteString(code + "\n")
			continue
		}

		bodyBuf.WriteString("\t" + code + "\n")

		// Add dummy usage for variables to prevent "declared but not used" errors
		if letStmt, ok := stmt.(*ast.LetStatement); ok {
			bodyBuf.WriteString(fmt.Sprintf("\t_ = %s\n", letStmt.Name.Value))
		} else if constStmt, ok := stmt.(*ast.ConstStatement); ok {
			bodyBuf.WriteString(fmt.Sprintf("\t_ = %s\n", constStmt.Name.Value))
		}
	}

	bodyCode := bodyBuf.String()

	var finalBuf bytes.Buffer
	finalBuf.WriteString("package main\n\n")

	if strings.Contains(bodyCode, "fmt.") || ctx.usedModules["fmt"] {
		finalBuf.WriteString("import \"fmt\"\n")
	}
	if strings.Contains(bodyCode, "math.") || ctx.usedModules["math"] || ctx.usedModules["math_rand"] {
		finalBuf.WriteString("import \"math\"\n")
	}
	if ctx.usedModules["math_rand"] {
		finalBuf.WriteString("import \"math/rand\"\n")
	}
	if ctx.usedModules["strings"] {
		finalBuf.WriteString("import \"strings\"\n")
	}
	if ctx.usedModules["utf8"] {
		finalBuf.WriteString("import \"unicode/utf8\"\n")
	}
	if ctx.usedModules["json"] {
		finalBuf.WriteString("import \"encoding/json\"\n")
	}
	if ctx.usedModules["strconv"] {
		finalBuf.WriteString("import \"strconv\"\n")
	}

	needsTime := false
	for _, stmt := range program.Statements {
		if hasDateSymbol(stmt, a) {
			needsTime = true
			break
		}
	}
	if needsTime || strings.Contains(bodyCode, "time.") || ctx.usedModules["time"] {
		finalBuf.WriteString("import \"time\"\n")
	}

	finalBuf.WriteString("\nfunc main() {\n")
	finalBuf.WriteString(bodyCode)
	finalBuf.WriteString("}\n")

	injectBuiltinDependencies(ctx, &finalBuf)

	if needsTime {
		finalBuf.WriteString(`
func parseDate(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}
`)
	}

	finalBuf.WriteString("\n")
	finalBuf.WriteString(pkgLevelBuf.String())

	return finalBuf.String(), nil
}

func hasDateSymbol(node ast.Node, a *analyzer.Analyzer) bool {
	if sym, ok := a.GetSymbol(node); ok && sym != nil && sym.Type() == environment.DATE_OBJ {
		return true
	}
	// Note: Deep traversal would check children, but for our simple transpiler, we assume it's fine.
	// For actual implementation, an ast.Visitor or a full pass would be cleaner.
	return false // Simplified for brevity
}

func transpileStatement(stmt ast.Statement, ctx *transpileContext) (string, error) {
	a := ctx.analyzer
	switch s := stmt.(type) {
	case *ast.LetStatement:
		sym, ok := a.GetSymbol(s)
		varType := ""
		if ok {
			varType = ctx.mapSymbolToGoType(sym)
		}

		val, err := transpileExpression(s.Value, ctx, varType, sym)
		if err != nil {
			return "", err
		}

		if val == "" {
			return "", nil
		}

		if varType != "" && varType != "any" {
			return fmt.Sprintf("var %s %s = %s", prefixIdentifier(ctx, s.Name.Value), varType, val), nil
		}
		return fmt.Sprintf("%s := %s", prefixIdentifier(ctx, s.Name.Value), val), nil

	case *ast.ConstStatement:
		sym, ok := a.GetSymbol(s)
		varType := ""
		if ok {
			varType = ctx.mapSymbolToGoType(sym)
		}

		val, err := transpileExpression(s.Value, ctx, varType, sym)
		if err != nil {
			return "", err
		}

		if val == "" {
			return "", nil
		}

		return fmt.Sprintf("%s := %s // const", prefixIdentifier(ctx, s.Name.Value), val), nil

	case *ast.ReturnStatement:
		if callNode, ok := s.ReturnValue.(*ast.CallExpression); ok {
			if ident, ok := callNode.Function.(*ast.Identifier); ok && ctx.currentFuncName != "" && ident.Value == ctx.currentFuncName {
				ctx.hasTailCall = true
				var buf bytes.Buffer
				for i, arg := range callNode.Arguments {
					argStr, err := transpileExpression(arg, ctx, "")
					if err != nil {
						return "", err
					}
					buf.WriteString(fmt.Sprintf("_tco%d := %s\n", i, argStr))
				}
				for i, param := range ctx.currentFunctionParams {
					buf.WriteString(fmt.Sprintf("%s = _tco%d\n", param, i))
				}
				buf.WriteString("continue")
				return buf.String(), nil
			}
		}

		val, err := transpileExpression(s.ReturnValue, ctx, "")
		if err != nil {
			return "", err
		}
		if !ctx.inFunction {
			if val != "" {
				return fmt.Sprintf("_ = %s\n\treturn", val), nil
			}
			return "return", nil
		}
		return fmt.Sprintf("return %s", val), nil
	case *ast.AssignStatement:
		val, err := transpileExpression(s.Value, ctx, "")
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s = %s", s.Name.Value, val), nil
	case *ast.ExpressionStatement:
		val, err := transpileExpression(s.Expression, ctx, "")
		if err != nil {
			return "", err
		}
		return val, nil
	case *ast.IndexAssignmentStatement:
		left, err := transpileExpression(s.Left, ctx, "")
		if err != nil {
			return "", err
		}
		index, err := transpileExpression(s.Index, ctx, "")
		if err != nil {
			return "", err
		}
		val, err := transpileExpression(s.Value, ctx, "")
		if err != nil {
			return "", err
		}
		leftSym, _ := a.GetSymbol(s.Left)
		if _, isArr := leftSym.(*symbol.ArraySymbol); isArr {
			return fmt.Sprintf("%s[int(%s)] = %s", left, index, val), nil
		}

		if indexSym, _ := a.GetSymbol(s.Index); indexSym != nil {
			if _, isStruct := indexSym.(*symbol.StructInstanceSymbol); isStruct {
				index += ".Key()"
			}
		}

		return fmt.Sprintf("%s[%s] = %s", left, index, val), nil
	case *ast.PropertyAssignmentStatement:
		val, err := transpileExpression(s.Value, ctx, "")
		if err != nil {
			return "", err
		}

		objSym, _ := a.GetSymbol(s.Object)
		if modSym, ok := objSym.(*symbol.ModuleSymbol); ok && modSym.FilePath != "" {
			return fmt.Sprintf("%s_%s = %s", sanitizeIdentifier(modSym.FilePath), s.Property.Value, val), nil
		}

		left, err := transpileExpression(s.Object, ctx, "")
		if err != nil {
			return "", err
		}
		exportedName := strings.ToUpper(s.Property.Value[:1]) + s.Property.Value[1:]
		return fmt.Sprintf("%s.%s = %s", left, exportedName, val), nil
	case *ast.BlockStatement:
		var buf bytes.Buffer
		buf.WriteString("{\n")
		for _, bstmt := range s.Statements {
			code, err := transpileStatement(bstmt, ctx)
			if err != nil {
				return "", err
			}
			buf.WriteString("\t" + code + "\n")
		}
		buf.WriteString("}")
		return buf.String(), nil
	case *ast.ImportStatement:
		return fmt.Sprintf("// import %s", s.Path), nil
	case *ast.TypeConstraintStatement:
		baseTypeGo := ctx.mapSymbolToGoType(func() symbol.Symbol { sym, _ := a.GetSymbol(s.BaseType); return sym }())
		baseTypeNoPtr := strings.TrimPrefix(baseTypeGo, "*")
		predGo, err := transpileExpression(s.Predicate, ctx, "")
		if err != nil {
			return "", err
		}
		var buf bytes.Buffer
		buf.WriteString(fmt.Sprintf("type %s %s\n", s.Name.Value, baseTypeNoPtr))
		buf.WriteString(fmt.Sprintf("var validate_%s func(%s) *%s = func(val %s) *%s {\n	pred := %s\n	if pred(val) {\n		res := (*%s)(val)\n		return res\n	}\n	return nil\n}", s.Name.Value, baseTypeGo, s.Name.Value, baseTypeGo, s.Name.Value, predGo, s.Name.Value))
		return buf.String(), nil
	case *ast.TypeAliasStatement:
		if s.StructDefinition != nil {
			var buf bytes.Buffer
			structName := prefixIdentifier(ctx, s.Name.Value)
			if len(s.TypeParameters) > 0 {
				var tps []string
				for _, tp := range s.TypeParameters {
					tps = append(tps, fmt.Sprintf("%s any", tp))
				}
				structName = fmt.Sprintf("%s[%s]", structName, strings.Join(tps, ", "))
			}
			buf.WriteString(fmt.Sprintf("type %s struct {\n", structName))
			sym, ok := a.GetSymbol(s)
			if ok {
				if structDef, isStruct := sym.(*symbol.StructDefSymbol); isStruct {
					for _, field := range s.StructDefinition.Fields {
						fieldSym := structDef.Fields[field.Name.Value]
						goType := ctx.mapSymbolToGoType(fieldSym.Type)
						if goType == "" {
							goType = "any"
						}
						// Capitalize field name to make it exported in Go
						exportedName := strings.ToUpper(field.Name.Value[:1]) + field.Name.Value[1:]
						buf.WriteString(fmt.Sprintf("\t%s %s\n", exportedName, goType))
					}
				}
			}
			buf.WriteString("}")
			return buf.String(), nil
		}
		if s.TargetType != "" || s.Signature != nil {
			sym, ok := a.GetSymbol(s)
			if ok {
				goType := ctx.mapSymbolToGoType(sym)
				if goType != "" && goType != "any" {
					return fmt.Sprintf("type %s %s", prefixIdentifier(ctx, s.Name.Value), goType), nil
				}
			}
		}
		return fmt.Sprintf("// type %s ...", prefixIdentifier(ctx, s.Name.Value)), nil

	default:
		// Fallback for unsupported statements
		return fmt.Sprintf("// unsupported statement: %T", stmt), nil
	}
}

func transpileExpression(expr ast.Expression, ctx *transpileContext, expectedType string, expectedSym ...symbol.Symbol) (string, error) {
	val, err := transpileExpressionInternal(expr, ctx, expectedType)
	if err != nil {
		return "", err
	}
	if len(expectedSym) > 0 && expectedSym[0] != nil {
		exprSym, _ := ctx.analyzer.GetSymbol(expr)
		if exprSym != nil {
			if nullSym, ok := expectedSym[0].(*symbol.NullableSymbol); ok {
				if constraint, ok := nullSym.Underlying.(*symbol.ConstraintSymbol); ok {
					if constraint.BaseType.Equals(exprSym) {
						return fmt.Sprintf("validate_%s(%s)", constraint.Name, val), nil
					}
				}
			}
		}
	}
	return val, nil
}

func transpileExpressionInternal(expr ast.Expression, ctx *transpileContext, expectedType string) (string, error) {
	if expr == nil {
		return "", nil
	}
	a := ctx.analyzer
	switch e := expr.(type) {
	case *ast.Identifier:
		importedMod := a.GetImportedModule(e)
		if importedMod != "" && builtinModules[importedMod] {
			return transpileBuiltinProperty(importedMod, e.Value, ctx)
		}
		_, filePath, ok := a.GetDefinition(e)
		if ok && filePath != "" && filePath != a.GlobalEnv().FileName {
			// Identifier was defined in another module
			return sanitizeIdentifier(filePath) + "_" + e.Value, nil
		}
		// If it's a global variable defined in the current module being transpiled (not main)
		if ctx.CurrentModulePath != "" {
			_, isGlobal := a.GlobalScope()[e.Value]
			_, filePath, ok := a.GetDefinition(e)
			if isGlobal {
				if ok && filePath == ctx.CurrentModulePath {
					return sanitizeIdentifier(ctx.CurrentModulePath) + "_" + e.Value, nil
				}
			}
		}
		return e.Value, nil
	case *ast.NumberLiteral:
		return fmt.Sprintf("%v", e.Value), nil
	case *ast.StringLiteral:
		return fmt.Sprintf("%q", e.Value), nil
	case *ast.BooleanLiteral:
		return fmt.Sprintf("%v", e.Value), nil
	case *ast.NilLiteral:
		return "nil", nil
	case *ast.DateLiteral:
		return fmt.Sprintf(`parseDate(%q)`, e.Value), nil
	case *ast.StructLiteral:
		var buf bytes.Buffer
		sym, _ := a.GetSymbol(e)
		goType := e.StructName
		if sym != nil {
			goType = strings.TrimPrefix(ctx.mapSymbolToGoType(sym), "*")
		}

		buf.WriteString(fmt.Sprintf("&%s{\n", goType))
		for name, val := range e.Fields {
			valStr, err := transpileExpression(val, ctx, "")
			if err != nil {
				return "", err
			}
			exportedName := strings.ToUpper(name[:1]) + name[1:]
			buf.WriteString(fmt.Sprintf("%s: %s,\n", exportedName, valStr))
		}
		buf.WriteString("}")
		return buf.String(), nil
	case *ast.ArrayLiteral:
		sym, _ := a.GetSymbol(e)
		goType := ctx.mapSymbolToGoType(sym)
		if goType == "" {
			goType = expectedType
		}
		if goType == "" {
			goType = "[]any"
		}
		var elements []string
		for _, el := range e.Elements {
			elStr, err := transpileExpression(el, ctx, "")
			if err != nil {
				return "", err
			}
			elements = append(elements, elStr)
		}
		return fmt.Sprintf("%s{%s}", goType, strings.Join(elements, ", ")), nil
	case *ast.MapLiteral:
		sym, _ := a.GetSymbol(e)
		goType := ctx.mapSymbolToGoType(sym)
		if goType == "" || goType == "map[any]any" {
			if expectedType != "" {
				goType = expectedType
			} else {
				goType = "map[any]any"
			}
		}
		var pairs []string
		for k, v := range e.Pairs {
			kStr, err := transpileExpression(k, ctx, "")
			if err != nil {
				return "", err
			}
			if kSym, _ := a.GetSymbol(k); kSym != nil {
				if _, isStruct := kSym.(*symbol.StructInstanceSymbol); isStruct {
					kStr += ".Key()"
				}
			}

			vStr, err := transpileExpression(v, ctx, "")
			if err != nil {
				return "", err
			}
			pairs = append(pairs, fmt.Sprintf("%s: %s", kStr, vStr))
		}
		return fmt.Sprintf("%s{%s}", goType, strings.Join(pairs, ", ")), nil
	case *ast.IndexExpression:
		left, err := transpileExpression(e.Left, ctx, "")
		if err != nil {
			return "", err
		}
		index, err := transpileExpression(e.Index, ctx, "")
		if err != nil {
			return "", err
		}
		leftSym, _ := a.GetSymbol(e.Left)
		if _, isArr := leftSym.(*symbol.ArraySymbol); isArr {
			return fmt.Sprintf("%s[int(%s)]", left, index), nil
		}

		if indexSym, _ := a.GetSymbol(e.Index); indexSym != nil {
			if _, isStruct := indexSym.(*symbol.StructInstanceSymbol); isStruct {
				index += ".Key()"
			}
		}

		return fmt.Sprintf("%s[%s]", left, index), nil
	case *ast.PrefixExpression:
		right, err := transpileExpression(e.Right, ctx, "")
		if err != nil {
			return "", err
		}
		if e.Operator == "move" {
			return right, nil // Go doesn't have move, just return the underlying identifier
		}
		if e.Operator == "!" {
			return fmt.Sprintf("(!%s)", right), nil
		}
		return fmt.Sprintf("(%s%s)", e.Operator, right), nil
	case *ast.IfExpression:
		cond, err := transpileExpression(e.Condition, ctx, "")
		if err != nil {
			return "", err
		}
		consequence, err := transpileStatement(e.Consequence, ctx)
		if err != nil {
			return "", err
		}
		res := fmt.Sprintf("if %s %s", cond, consequence)
		if e.Alternative != nil {
			alt, err := transpileStatement(e.Alternative, ctx)
			if err != nil {
				return "", err
			}
			res += fmt.Sprintf(" else %s", alt)
		}
		return res, nil
	case *ast.InfixExpression:
		left, err := transpileExpression(e.Left, ctx, "")
		if err != nil {
			return "", err
		}
		right, err := transpileExpression(e.Right, ctx, "")
		if err != nil {
			return "", err
		}
		if e.Operator == "%" {
			return fmt.Sprintf("math.Mod(%s, %s)", left, right), nil
		}
		return fmt.Sprintf("(%s %s %s)", left, e.Operator, right), nil
	case *ast.SafePipeExpression:
		leftExpr, err := transpileExpressionInternal(e.Left, ctx, "")
		if err != nil {
			return "", err
		}

		leftSym, _ := ctx.analyzer.GetSymbol(e.Left)
		leftGoType := ctx.mapSymbolToGoType(leftSym)
		if leftGoType == "" {
			leftGoType = "any"
		}

		resultSym, _ := ctx.analyzer.GetSymbol(e)
		resultGoType := ctx.mapSymbolToGoType(resultSym)
		if resultGoType == "" {
			resultGoType = "any"
		}

		needsDeref := false
		if nullableSym, ok := leftSym.(*symbol.NullableSymbol); ok {
			underlyingType := ctx.mapSymbolToGoType(nullableSym.Underlying)
			if !(strings.HasPrefix(underlyingType, "*") || strings.HasPrefix(underlyingType, "map[") ||
				strings.HasPrefix(underlyingType, "[]") || strings.HasPrefix(underlyingType, "func(") ||
				underlyingType == "any") {
				needsDeref = true
			}
		}

		valName := "_val"
		if needsDeref {
			valName = "*_val"
		}

		originalArg := e.Call.Arguments[0]
		e.Call.Arguments[0] = &ast.Identifier{Value: valName}

		callExpr, err := transpileExpressionInternal(e.Call, ctx, "")

		e.Call.Arguments[0] = originalArg

		if err != nil {
			return "", err
		}

		return fmt.Sprintf("func(_val %s) %s { if _val != nil { return %s }; return nil }(%s)", leftGoType, resultGoType, callExpr, leftExpr), nil
	case *ast.CallExpression:
		if prop, ok := e.Function.(*ast.PropertyExpression); ok {
			objSym, _ := a.GetSymbol(prop.Object)
			if modSym, isMod := objSym.(*symbol.ModuleSymbol); isMod && builtinModules[modSym.FilePath] {
				return transpileBuiltinCall(modSym.FilePath, prop.Property.Value, e.Arguments, ctx)
			}
		} else if ident, ok := e.Function.(*ast.Identifier); ok {
			importedMod := a.GetImportedModule(ident)
			if importedMod != "" && builtinModules[importedMod] {
				return transpileBuiltinCall(importedMod, ident.Value, e.Arguments, ctx)
			}
		}

		fn, err := transpileExpression(e.Function, ctx, "")
		if err != nil {
			return "", err
		}
		if fn == "print" {
			fn = "fmt.Println"
		}

		if len(e.TypeArguments) > 0 {
			var typeArgsGo []string
			for _, t := range e.TypeArguments {
				if sym, ok := ctx.analyzer.GetGlobalType(t); ok {
					typeArgsGo = append(typeArgsGo, ctx.mapSymbolToGoType(sym))
				} else {
					typeArgsGo = append(typeArgsGo, "any")
				}
			}
			fn = fmt.Sprintf("%s[%s]", fn, strings.Join(typeArgsGo, ", "))
		}

		args := []string{}
		for _, arg := range e.Arguments {
			argStr, err := transpileExpression(arg, ctx, "")
			if err != nil {
				return "", err
			}
			args = append(args, argStr)
		}
		return fmt.Sprintf("%s(%s)", fn, strings.Join(args, ", ")), nil
	case *ast.PropertyExpression:
		objSym, _ := a.GetSymbol(e.Object)
		if modSym, ok := objSym.(*symbol.ModuleSymbol); ok {
			if builtinModules[modSym.Name()] {
				if ident, ok := e.Object.(*ast.Identifier); ok {
					return transpileBuiltinProperty(ident.Value, e.Property.Value, ctx)
				}
			} else if modSym.FilePath != "" {
				return sanitizeIdentifier(modSym.FilePath) + "_" + e.Property.Value, nil
			}
		}

		obj, err := transpileExpression(e.Object, ctx, "")
		if err != nil {
			return "", err
		}
		prop := strings.ToUpper(e.Property.Value[:1]) + e.Property.Value[1:]

		if e.Safe {
			objSym, _ := a.GetSymbol(e.Object)
			objType := ctx.mapSymbolToGoType(objSym)
			if objType == "" {
				objType = "any"
			}

			propSym, _ := a.GetSymbol(e)
			propType := ctx.mapSymbolToGoType(propSym)
			if propType == "" {
				propType = "any"
			}

			// We need to know if the actual struct field is a primitive value type so we can take its address.
			fieldIsValueType := false
			if nullableObj, isNullable := objSym.(*symbol.NullableSymbol); isNullable {
				objSym = nullableObj.Underlying
			}
			var structDef *symbol.StructDefSymbol
			if inst, isInst := objSym.(*symbol.StructInstanceSymbol); isInst {
				structDef = inst.Def
			} else if def, isDef := objSym.(*symbol.StructDefSymbol); isDef {
				structDef = def
			}

			if structDef != nil {
				if fieldSym, exists := structDef.Fields[e.Property.Value]; exists {
					fieldGoType := ctx.mapSymbolToGoType(fieldSym.Type)
					if fieldGoType == "float64" || fieldGoType == "bool" || fieldGoType == "string" {
						fieldIsValueType = true
					}
				}
			}

			if !fieldIsValueType {
				return fmt.Sprintf(`func(obj %s) %s { if obj != nil { return obj.%s }; return nil }(%s)`, objType, propType, prop, obj), nil
			} else {
				return fmt.Sprintf(`func(obj %s) %s { if obj != nil { v := obj.%s; return &v }; return nil }(%s)`, objType, propType, prop, obj), nil
			}
		}

		return fmt.Sprintf("%s.%s", obj, prop), nil
	case *ast.FunctionLiteral:
		sym, _ := a.GetSymbol(e)
		fnSym, _ := sym.(*symbol.FunctionSymbol)
		var params []string
		var paramNames []string
		for i, param := range e.Parameters {
			pt := "any"
			if fnSym != nil && i < len(fnSym.ParamTypes()) {
				t := ctx.mapSymbolToGoType(fnSym.ParamTypes()[i])
				if t != "" {
					pt = t
				}
			}
			params = append(params, fmt.Sprintf("%s %s", param.Name, pt))
			paramNames = append(paramNames, param.Name)
		}
		retType := ""
		if fnSym != nil && fnSym.ReturnType() != nil && fnSym.ReturnType().Type() != environment.NULL_OBJ {
			retType = ctx.mapSymbolToGoType(fnSym.ReturnType())
		}

		fnCtx := &transpileContext{
			analyzer:              a,
			currentFuncName:       "",
			currentFunctionParams: paramNames,
			hasTailCall:           false,
			usedModules:           ctx.usedModules,
			CurrentModulePath:     ctx.CurrentModulePath,
			inFunction:            true,
		}
		if fnSym != nil {
			fnCtx.currentFuncName = fnSym.Name
		}

		body, err := transpileStatement(e.Body, fnCtx)
		if err != nil {
			return "", err
		}

		if fnCtx.hasTailCall {
			body = fmt.Sprintf("{\nfor %s\n}", body)
		}

		goFunc := fmt.Sprintf("func(%s)", strings.Join(params, ", "))

		isGeneric := fnSym != nil && len(fnSym.TypeParameters) > 0
		if isGeneric {
			var typeParamsGo []string
			for _, tp := range fnSym.TypeParameters {
				typeParamsGo = append(typeParamsGo, fmt.Sprintf("%s comparable", tp))
			}
			goFunc = fmt.Sprintf("func %s[%s](%s)", prefixIdentifier(ctx, fnSym.Name), strings.Join(typeParamsGo, ", "), strings.Join(params, ", "))
		}

		if retType != "" {
			goFunc += " " + retType
		}

		if isGeneric {
			if ctx.packageLevelCode != nil {
				ctx.packageLevelCode.WriteString(fmt.Sprintf("%s %s\n", goFunc, body))
			}
			return "", nil
		}

		return fmt.Sprintf("%s %s", goFunc, body), nil
	default:
		// Fallback for unsupported expressions
		return fmt.Sprintf("UnsupportedExpr(%T)", expr), nil
	}
}
func getOrderedModules(p *ast.Program, asts map[string]*ast.Program) []string {
	var ordered []string
	visited := make(map[string]bool)
	var visit func(prog *ast.Program)
	visit = func(prog *ast.Program) {
		for _, stmt := range prog.Statements {
			if imp, ok := stmt.(*ast.ImportStatement); ok {
				modPath := imp.Path
				if asts[modPath] != nil && !visited[modPath] {
					visited[modPath] = true
					visit(asts[modPath])
					ordered = append(ordered, modPath)
				}
			}
		}
	}
	visit(p)
	return ordered
}

func sanitizeIdentifier(path string) string {
	s := strings.ReplaceAll(path, "/", "_")
	s = strings.ReplaceAll(s, ".", "_")
	s = strings.ReplaceAll(s, "-", "_")
	return s
}

func prefixIdentifier(ctx *transpileContext, name string) string {
	if ctx.CurrentModulePath != "" {
		_, isGlobalVar := ctx.analyzer.GlobalScope()[name]
		_, isGlobalType := ctx.analyzer.GetGlobalType(name)
		if isGlobalVar || isGlobalType {
			return sanitizeIdentifier(ctx.CurrentModulePath) + "_" + name
		}
	}
	return name
}
