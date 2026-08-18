package symbol

import "caja-cli/internal/pipeline/environment"

// NullSymbol represents the nil literal during semantic analysis.
type NullSymbol struct{}

// Type returns the environment.NULL_OBJ type representing this symbol.
func (n *NullSymbol) Type() environment.ObjectType { return environment.NULL_OBJ }

// Equals determines whether this symbol is compatible with another symbol.
// The null symbol is compatible with other nulls, any, and nullable symbols.
func (n *NullSymbol) Equals(other Symbol) bool {
	if other.Type() == environment.NULL_OBJ || other.Type() == environment.ANY_OBJ {
		return true
	}
	if _, ok := other.(*NullableSymbol); ok {
		return true
	}
	return false
}

// String returns the string representation of the null type.
func (n *NullSymbol) String() string {
	return string(n.Type())
}
