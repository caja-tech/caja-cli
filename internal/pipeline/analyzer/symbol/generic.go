package symbol

import "caja-cli/internal/pipeline/environment"

// GenericSymbol represents a placeholder type for generic type parameters (e.g. T, R).
type GenericSymbol struct {
	Name string
}

// NewGenericSymbol creates a new generic type symbol.
func NewGenericSymbol(name string) *GenericSymbol {
	return &GenericSymbol{Name: name}
}

// Type returns ANY_OBJ because a generic type can represent any object at runtime.
func (gs *GenericSymbol) Type() environment.ObjectType {
	return environment.ANY_OBJ
}

// String returns the generic parameter's name.
func (gs *GenericSymbol) String() string {
	return gs.Name
}

// Equals compares this generic symbol with another. During regular type checking
// (without inference context), a generic symbol only equals itself or ANY_OBJ.
func (gs *GenericSymbol) Equals(other Symbol) bool {
	if other.Type() == environment.ANY_OBJ {
		return true
	}

	otherGen, ok := other.(*GenericSymbol)
	if !ok {
		return false
	}
	return gs.Name == otherGen.Name
}
