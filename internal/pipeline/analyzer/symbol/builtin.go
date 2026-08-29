package symbol

import "caja-cli/internal/pipeline/environment"

// BuiltinSymbol represents a built-in function type in the semantic analysis,
// tracking the expected number of arguments (arity).
type BuiltinSymbol struct {
	symbolType environment.ObjectType
	arity      int
	Label      string
	Params     []string
}

// NewBuiltinSymbol creates and returns a new BuiltinSymbol with the specified arity.
func NewBuiltinSymbol(arity int, label string, params ...string) *BuiltinSymbol {
	return &BuiltinSymbol{
		symbolType: environment.BUILTIN_OBJ,
		arity:      arity,
		Label:      label,
		Params:     params,
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

func (bs *BuiltinSymbol) String() string {
	if bs.Label != "" {
		return bs.Label
	}
	return string(bs.Type())
}
