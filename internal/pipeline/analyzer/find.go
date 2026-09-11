package analyzer

import (
	"caja-cli/internal/pipeline/analyzer/symbol"
	"caja-cli/internal/pipeline/environment"
	"caja-cli/internal/pipeline/lexer"
	"fmt"
	"strings"
)

// findVarSymbolInScope checks if a variable name has been declared in the current or
// any outer scope. It returns true if the variable is found, false otherwise.
func (a *Analyzer) findVarSymbolInScope(varName string) (ScopeEntry, bool) {
	for i := len(a.scopes) - 1; i >= 0; i-- {
		if entry, ok := a.scopes[i][varName]; ok {
			return entry, true
		}
	}

	return ScopeEntry{}, false
}

// resolveTypeRef is findTypeSymbolInTypes plus wildcard-type ambiguity
// reporting. The resolution functions themselves carry no source token, so
// this wrapper is what type-annotation sites call in order to attach the
// diagnostic to a real position.
func (a *Analyzer) resolveTypeRef(tok lexer.Token, typeExpr string) (symbol.Symbol, bool) {
	a.checkAmbiguousTypeUse(tok, typeExpr)
	return a.findTypeSymbolInTypes(typeExpr)
}

// checkAmbiguousTypeUse reports any identifier inside a type expression that
// two different wildcard imports both bound. Like the value side, this fires
// only where the name is actually used — importing both modules stays legal —
// and the qualified form (mod.Type) remains the fallback.
//
// Reports are deduped per type name because several statements resolve the
// same annotation more than once, and the analyzer tests assert exact error
// counts.
func (a *Analyzer) checkAmbiguousTypeUse(tok lexer.Token, typeExpr string) {
	if len(a.wildcardTypes) == 0 || typeExpr == "" {
		return
	}
	for _, name := range typeIdentifierFragments(typeExpr) {
		origins, ok := a.wildcardTypes[name]
		if !ok || len(origins) < 2 || a.reportedAmbiguousTypes[name] {
			continue
		}
		a.reportedAmbiguousTypes[name] = true
		a.reportError(tok, fmt.Sprintf(
			"semantic error: ambiguous type '%s': wildcard-imported from %s. Suggestion: qualify it (%s)",
			name, formatModuleList(origins), formatQualifiedSuggestions(origins, name)))
	}
}

// typeIdentifierFragments splits a type expression into the bare identifier
// names it references, so every composite form ([Shape], map[String]Shape,
// fn(Shape) -> Shape, Shape?) is covered by a single scan rather than by
// hooking each recursive branch of type resolution. Fragments containing a
// '.' are already module-qualified — the escape hatch — and are dropped.
func typeIdentifierFragments(typeExpr string) []string {
	var fragments []string
	var current strings.Builder
	qualified := false

	flush := func() {
		if current.Len() > 0 && !qualified {
			fragments = append(fragments, current.String())
		}
		current.Reset()
		qualified = false
	}

	for _, r := range typeExpr {
		switch {
		case r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'):
			current.WriteRune(r)
		case r == '.':
			qualified = true
			current.WriteRune(r)
		default:
			flush()
		}
	}
	flush()

	return fragments
}

// findTypeSymbolInTypes converts a type name string into its corresponding symbol.Symbol representation,
// checking built-in types first, and then looking up custom types in the registry.
func (a *Analyzer) findTypeSymbolInTypes(typeName string) (symbol.Symbol, bool) {
	if typeName == "" {
		return symbol.AnySymbol(), false
	}

	isNullable := false
	if strings.HasSuffix(typeName, "?") {
		isNullable = true
		typeName = typeName[:len(typeName)-1]
	}

	resultSymbol, ok := a.findTypeSymbolInTypesRaw(typeName)

	if structSym, ok := resultSymbol.(*symbol.StructDefSymbol); ok && len(structSym.TypeParameters) > 0 {
		return symbol.AnySymbol(), false
	}
	if fnSym, ok := resultSymbol.(*symbol.FunctionSymbol); ok && len(fnSym.TypeParameters) > 0 {
		return symbol.AnySymbol(), false
	}

	if isNullable {
		resultSymbol = &symbol.NullableSymbol{Underlying: resultSymbol}
	}
	return resultSymbol, ok
}

// findTypeSymbolInTypesRaw handles the actual symbol lookup without nullable wrapping.
func (a *Analyzer) findTypeSymbolInTypesRaw(typeName string) (symbol.Symbol, bool) {

	// Check for generic alias instantiation: BaseName<Arg1, Arg2>
	if !strings.HasPrefix(typeName, "fn(") && !strings.HasPrefix(typeName, "map[") {
		if ltIdx := strings.Index(typeName, "<"); ltIdx != -1 && strings.HasSuffix(typeName, ">") {
			baseName := typeName[:ltIdx]
			
			// We must split type arguments safely, respecting nested `< >` and `[ ]`
			typeArgsStr := typeName[ltIdx+1 : len(typeName)-1]
			var typeArgs []string
			current := ""
			depth := 0
			for i := 0; i < len(typeArgsStr); i++ {
				c := typeArgsStr[i]
				if c == '<' || c == '[' {
					depth++
					current += string(c)
				} else if c == '>' || c == ']' {
					depth--
					current += string(c)
				} else if c == ',' && depth == 0 {
					typeArgs = append(typeArgs, strings.TrimSpace(current))
					current = ""
				} else {
					current += string(c)
				}
			}
			if strings.TrimSpace(current) != "" {
				typeArgs = append(typeArgs, strings.TrimSpace(current))
			}

			if aliasSym, exists := a.lookupType(baseName); exists {
				var typeParams []string
				if fnSym, ok := aliasSym.(*symbol.FunctionSymbol); ok {
					typeParams = fnSym.TypeParameters
				} else if structSym, ok := aliasSym.(*symbol.StructDefSymbol); ok {
					typeParams = structSym.TypeParameters
				}

				if len(typeParams) > 0 {
					if len(typeArgs) != len(typeParams) {
						return symbol.AnySymbol(), false
					}
					
					inferredTypes := make(map[string]symbol.Symbol)
					allOk := true
					for i, argStr := range typeArgs {
						argSym, argOk := a.findTypeSymbolInTypes(argStr)
						if !argOk {
							allOk = false
						}
						inferredTypes[typeParams[i]] = argSym
					}
					
					return substituteTypes(aliasSym, inferredTypes), allOk
				}
			}
			return symbol.AnySymbol(), false
		}
	}

	if strings.HasPrefix(typeName, "fn(") {
		depth := 0
		paramsStr := ""
		endParenIdx := -1
		for i := 2; i < len(typeName); i++ {
			if typeName[i] == '(' {
				depth++
			} else if typeName[i] == ')' {
				depth--
				if depth == 0 {
					paramsStr = typeName[3:i]
					endParenIdx = i
					break
				}
			}
		}

		var params []string
		current := ""
		pDepth := 0
		for i := 0; i < len(paramsStr); i++ {
			c := paramsStr[i]
			if c == '(' || c == '[' {
				pDepth++
				current += string(c)
			} else if c == ')' || c == ']' {
				pDepth--
				current += string(c)
			} else if c == ',' && pDepth == 0 {
				params = append(params, strings.TrimSpace(current))
				current = ""
			} else {
				current += string(c)
			}
		}
		if strings.TrimSpace(current) != "" {
			params = append(params, strings.TrimSpace(current))
		}

		returnType := "Nothing"
		arrowIdx := strings.Index(typeName[endParenIdx:], "->")
		if arrowIdx != -1 {
			returnType = strings.TrimSpace(typeName[endParenIdx+arrowIdx+2:])
		}

		var paramSymbols []symbol.Symbol
		allOk := true
		for _, p := range params {
			sym, ok := a.findTypeSymbolInTypes(p)
			if !ok {
				allOk = false
			}
			paramSymbols = append(paramSymbols, sym)
		}
		retSym, ok := a.findTypeSymbolInTypes(returnType)
		if !ok {
			allOk = false
		}

		return symbol.NewFunctionSymbol("", "", nil, nil, len(params), paramSymbols, retSym), allOk
	}

	if strings.HasPrefix(typeName, "[") && strings.HasSuffix(typeName, "]") {
		innerTypeStr := typeName[1 : len(typeName)-1]
		innerSymbol, ok := a.findTypeSymbolInTypes(innerTypeStr)

		return symbol.NewArraySymbol(innerSymbol), ok
	}

	if strings.HasPrefix(typeName, "map[") {
		closingBracket := strings.Index(typeName, "]")
		if closingBracket != -1 {
			keyTypeStr := typeName[4:closingBracket]
			valTypeStr := typeName[closingBracket+1:]

			keySym, keyOk := a.findTypeSymbolInTypes(keyTypeStr)
			valSym, valOk := a.findTypeSymbolInTypes(valTypeStr)

			return symbol.NewMapSymbol(keySym, valSym), keyOk && valOk
		}
	}

	switch typeName {
	case "Number":
		return symbol.NewBasicSymbol(environment.NUMBER_OBJ), true
	case "String":
		return symbol.NewBasicSymbol(environment.STRING_OBJ), true
	case "Boolean":
		return symbol.NewBasicSymbol(environment.BOOLEAN_OBJ), true
	case "Date":
		return symbol.NewBasicSymbol(environment.DATE_OBJ), true
	}

	if strings.Contains(typeName, ".") {
		parts := strings.SplitN(typeName, ".", 2)
		modName := parts[0]
		propName := parts[1]

		entry, ok := a.findVarSymbolInScope(modName)
		if ok {
			if modSym, ok := entry.Sym.(*symbol.ModuleSymbol); ok {
				if modSym.IsPrivate(propName) {
					// Treat private types as undefined when accessed from outside
					return symbol.AnySymbol(), false
				}
				if typeSym, ok := modSym.GetType(propName); ok {
					return typeSym, true
				}
			}
		}
		return symbol.AnySymbol(), false
	}

	if sym, ok := a.lookupType(typeName); ok {
		return sym, true
	}

	return symbol.AnySymbol(), false
}
