package analyzer

import (
	"caja-cli/internal/pipeline/analyzer/symbol"
	"caja-cli/internal/pipeline/environment"
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
	if isNullable {
		resultSymbol = &symbol.NullableSymbol{Underlying: resultSymbol}
	}
	return resultSymbol, ok
}

// findTypeSymbolInTypesRaw handles the actual symbol lookup without nullable wrapping.
func (a *Analyzer) findTypeSymbolInTypesRaw(typeName string) (symbol.Symbol, bool) {

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
	case "Any":
		return symbol.AnySymbol(), true
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

	if sym, ok := a.types[typeName]; ok {
		return sym, true
	}

	return symbol.AnySymbol(), false
}
