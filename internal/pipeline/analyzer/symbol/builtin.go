package symbol

import "caja-cli/internal/pipeline/environment"

// BuiltinSymbol represents a built-in function type in the semantic analysis,
// tracking the expected number of arguments (arity).
type BuiltinSymbol struct {
	symbolType environment.ObjectType
	arity      int
}

// NewBuiltinSymbol creates and returns a new BuiltinSymbol with the specified arity.
func NewBuiltinSymbol(arity int) *BuiltinSymbol {
	return &BuiltinSymbol{
		symbolType: environment.BUILTIN_OBJ,
		arity:      arity,
	}
}

// Equals compares this BuiltinSymbol with another Symbol to determine if they represent the same type.
// It returns true if both are BuiltinSymbols with matching arities, or if the other symbol is ANY_OBJ.
func (bs *BuiltinSymbol) Equals(other Symbol) bool {
	otherSymbol, ok := other.(*BuiltinSymbol)
	if !ok {
		return false
	}

	if otherSymbol.symbolType == environment.ANY_OBJ {
		return true
	}

	if bs.symbolType != otherSymbol.symbolType {
		return false
	}

	if bs.arity != otherSymbol.arity {
		return false
	}

	return true
}

// Type returns the underlying environment.ObjectType of this symbol, which is always BUILTIN_OBJ.
func (bs *BuiltinSymbol) Type() environment.ObjectType {
	return bs.symbolType
}

// String returns the string representation of the builtin type.
func (bs *BuiltinSymbol) String() string {
	return string(bs.Type())
}
