package semantic

import (
	"caja-cli/internal/environment"
	"caja-cli/internal/semantic/symbol"
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

	if strings.HasPrefix(typeName, "[") && strings.HasSuffix(typeName, "]") {
		innerTypeStr := typeName[1 : len(typeName)-1]
		innerSymbol, ok := a.findTypeSymbolInTypes(innerTypeStr)

		return symbol.NewArraySymbol(innerSymbol), ok
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

	if sym, ok := a.types[typeName]; ok {
		return sym, true
	}

	return symbol.AnySymbol(), false
}
