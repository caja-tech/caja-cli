package symbol

import "caja-cli/internal/pipeline/environment"

// NullableSymbol wraps an underlying Symbol, indicating that it can accept a null value.
type NullableSymbol struct {
	Underlying Symbol
}

func (ns *NullableSymbol) Type() environment.ObjectType {
	// A nullable symbol is typically still resolved to its underlying runtime type (e.g. STRUCT_OBJ),
	// but for semantic checks we can return the underlying type.
	return ns.Underlying.Type()
}

func (ns *NullableSymbol) Equals(other Symbol) bool {
	if other.Type() == environment.NULL_OBJ || other.Type() == environment.ANY_OBJ {
		return true
	}

	// If the other is also nullable, compare their underlying types.
	if otherNullable, ok := other.(*NullableSymbol); ok {
		return ns.Underlying.Equals(otherNullable.Underlying)
	}

	// If the other is NOT nullable (but we are), it's still acceptable to assign a non-null
	// value to a nullable variable.
	return ns.Underlying.Equals(other)
}
