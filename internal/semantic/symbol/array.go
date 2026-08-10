package symbol

import "caja-cli/internal/environment"

// ArraySymbol represents an array type in the semantic analysis,
// keeping track of the type of elements it contains.
type ArraySymbol struct {
	symbolType    environment.ObjectType
	elementSymbol Symbol
}

// NewArraySymbol creates and returns a new ArraySymbol with the given element symbol.
func NewArraySymbol(elementSymbol Symbol) *ArraySymbol {
	return &ArraySymbol{
		symbolType:    environment.ARRAY_OBJ,
		elementSymbol: elementSymbol,
	}
}

// Equals compares this ArraySymbol with another Symbol to determine if they represent the same type.
// It returns true if they are both ArraySymbols and their inner element symbols match, or if either is an ANY_OBJ.
func (as *ArraySymbol) Equals(other Symbol) bool {
	otherSymbol, ok := other.(*ArraySymbol)
	if !ok {
		return false
	}

	if as.symbolType == environment.ANY_OBJ || otherSymbol.symbolType == environment.ANY_OBJ {
		return true
	}

	if as.symbolType != otherSymbol.symbolType {
		return false
	}

	if as.elementSymbol == nil && otherSymbol.elementSymbol == nil {
		return true
	}

	if as.elementSymbol == nil || otherSymbol.elementSymbol == nil {
		return false
	}

	return as.elementSymbol.Equals(otherSymbol.elementSymbol)
}

// Type returns the underlying environment.ObjectType of this symbol, which is always ARRAY_OBJ.
func (as *ArraySymbol) Type() environment.ObjectType {
	return as.symbolType
}

// ElementSymbol returns the Symbol representing the type of elements contained in the array.
func (as *ArraySymbol) ElementSymbol() Symbol {
	return as.elementSymbol
}
