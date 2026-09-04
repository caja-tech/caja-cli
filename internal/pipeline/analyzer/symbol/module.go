package symbol

import (
	"caja-cli/internal/pipeline/environment"

	"caja-cli/internal/pipeline/lexer"
)

// ModuleSymbol represents a module in the semantic analysis,
// containing its name and the scope of exported symbols.
type ModuleSymbol struct {
	symbolType  environment.ObjectType
	name        string
	scope       map[string]Symbol
	types       map[string]Symbol
	privates    map[string]bool
	constants   map[string]bool
	Definitions map[string]lexer.Token
	FilePath    string
}

// NewModuleSymbol creates and returns a new ModuleSymbol with the specified name and exported scope.
func NewModuleSymbol(name string, scope map[string]Symbol, types map[string]Symbol, privates map[string]bool, constants map[string]bool, definitions map[string]lexer.Token, filePath string) *ModuleSymbol {
	if types == nil {
		types = make(map[string]Symbol)
	}
	if privates == nil {
		privates = make(map[string]bool)
	}
	if constants == nil {
		constants = make(map[string]bool)
	}
	if definitions == nil {
		definitions = make(map[string]lexer.Token)
	}
	return &ModuleSymbol{symbolType: environment.MODULE_OBJ, name: name, scope: scope, types: types, privates: privates, constants: constants, Definitions: definitions, FilePath: filePath}
}

// Equals compares this ModuleSymbol with another Symbol to determine if they represent the same module.
// It returns true if both are ModuleSymbols, and they share the same name.
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

// IsPrivate returns true if the given symbol name is marked as private within this module.
func (ms *ModuleSymbol) IsPrivate(symbolName string) bool {
	return ms.privates[symbolName]
}

// IsConstant returns true if the given symbol name is marked as a constant within this module.
func (ms *ModuleSymbol) IsConstant(symbolName string) bool {
	return ms.constants[symbolName]
}

// GetType retrieves an exported type from the module's types map.
func (ms *ModuleSymbol) GetType(typeName string) (Symbol, bool) {
	symbol, ok := ms.types[typeName]
	return symbol, ok
}

// String returns the string representation of the module type.
func (ms *ModuleSymbol) String() string {
	return string(ms.Type())
}

func (ms *ModuleSymbol) GetSymbols() map[string]Symbol {
	return ms.scope
}

func (ms *ModuleSymbol) GetTypes() map[string]Symbol {
	return ms.types
}
