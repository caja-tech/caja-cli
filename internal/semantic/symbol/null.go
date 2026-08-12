package symbol

import "caja-cli/internal/environment"

// NullSymbol represents the nil literal during semantic analysis.
type NullSymbol struct{}

func (n *NullSymbol) Type() environment.ObjectType { return environment.NULL_OBJ }
func (n *NullSymbol) Equals(other Symbol) bool {
	if other.Type() == environment.NULL_OBJ || other.Type() == environment.ANY_OBJ {
		return true
	}
	if _, ok := other.(*NullableSymbol); ok {
		return true
	}
	return false
}
