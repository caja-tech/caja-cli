package symbol

import "caja-cli/internal/environment"

// ModuleSymbol represents a module in the semantic analysis,
// containing its name and the scope of exported symbols.
type ModuleSymbol struct {
	symbolType environment.ObjectType
	name       string
	scope      map[string]Symbol
}

// NewModuleSymbol creates and returns a new ModuleSymbol with the specified name and exported scope.
func NewModuleSymbol(name string, scope map[string]Symbol) *ModuleSymbol {
	return &ModuleSymbol{symbolType: environment.MODULE_OBJ, name: name, scope: scope}
}

// Equals compares this ModuleSymbol with another Symbol to determine if they represent the same module.
// It returns true if both are ModuleSymbols and they share the same name.
func (ms *ModuleSymbol) Equals(other Symbol) bool {
	otherSymbol, ok := other.(*ModuleSymbol)
	if !ok {
		return false
	}

	return otherSymbol.Type() == ms.symbolType && otherSymbol.name == ms.name
}

// Type returns the underlying environment.ObjectType of this symbol, which is always MODULE_OBJ.
func (ms *ModuleSymbol) Type() environment.ObjectType {
	return ms.symbolType
}

// GetSymbol retrieves an exported symbol from the module's scope by its name.
// It returns the symbol and a boolean indicating whether it was found.
func (ms *ModuleSymbol) GetSymbol(symbolName string) (Symbol, bool) {
	symbol, ok := ms.scope[symbolName]
	return symbol, ok
}
