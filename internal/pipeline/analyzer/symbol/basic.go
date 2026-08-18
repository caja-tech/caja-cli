package symbol

import "caja-cli/internal/pipeline/environment"

// BasicSymbol represents a primitive type in the semantic analysis (e.g., NUMBER, STRING, BOOLEAN).
type BasicSymbol struct {
	symbolType environment.ObjectType
}

// NewBasicSymbol creates and returns a new BasicSymbol of the specified type.
func NewBasicSymbol(symbolType environment.ObjectType) *BasicSymbol {
	return &BasicSymbol{
		symbolType: symbolType,
	}
}

// Equals compares this BasicSymbol with another Symbol to determine if they represent the same type.
// It returns true if their types match exactly, or if either symbol is of type ANY_OBJ.
func (bs *BasicSymbol) Equals(other Symbol) bool {
	if bs.symbolType == environment.ANY_OBJ || other.Type() == environment.ANY_OBJ {
		return true
	}

	otherSymbol, ok := other.(*BasicSymbol)
	if !ok {
		return false
	}

	if bs.symbolType != otherSymbol.symbolType {
		return false
	}

	return true
}

// Type returns the underlying environment.ObjectType of this symbol.
func (bs *BasicSymbol) Type() environment.ObjectType {
	return bs.symbolType
}

// String returns the string representation of the basic type.
func (bs *BasicSymbol) String() string {
	return string(bs.Type())
}
