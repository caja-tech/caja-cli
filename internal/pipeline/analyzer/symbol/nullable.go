package symbol

import (
	"caja-cli/internal/pipeline/environment"
	"fmt"
)
// NullableSymbol wraps an underlying Symbol, indicating that it can accept a null value.
type NullableSymbol struct {
	Underlying Symbol
}

// Type returns the environment.ObjectType of the underlying symbol.
func (ns *NullableSymbol) Type() environment.ObjectType {
	// A nullable symbol is typically still resolved to its underlying runtime type (e.g. STRUCT_OBJ),
	// but for semantic checks we can return the underlying type.
	return ns.Underlying.Type()
}

// Equals checks type compatibility. It returns true if the other symbol is NULL_OBJ,
// ANY_OBJ, or structurally matches the underlying type.
func (ns *NullableSymbol) Equals(other Symbol) bool {
	if other.Type() == environment.NULL_OBJ || other.Type() == environment.ANY_OBJ {
		return true
	}

	// If the other is also nullable, compare their underlying types.
	if otherNullable, ok := other.(*NullableSymbol); ok {
		return ns.Underlying.Equals(otherNullable.Underlying)
	}

	if constraint, ok := ns.Underlying.(*ConstraintSymbol); ok {
		if constraint.BaseType.Equals(other) {
			return true
		}
	}

	// If the other is NOT nullable (but we are), it's still acceptable to assign a non-null
	// value to a nullable variable.
	return ns.Underlying.Equals(other)
}

// String returns the string representation of the nullable type.
func (ns *NullableSymbol) String() string {
	return fmt.Sprintf("%s?", ns.Underlying.String())
}
