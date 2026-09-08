package symbol

import (
	"caja-cli/internal/pipeline/environment"
)

// UnionSymbol represents a closed union type: a compile-time-only label for
// "one of these struct types" (e.g. `union Animal = Cat | Dog | Pig`). It has
// no runtime representation of its own — a union-typed value is simply
// whichever variant struct it actually is.
type UnionSymbol struct {
	Name     string
	Variants map[string]*StructDefSymbol // keyed by struct name, e.g. "Cat" -> StructDefSymbol
	FilePath string
}

// NewUnionSymbol creates and returns a new UnionSymbol.
func NewUnionSymbol(name string, variants map[string]*StructDefSymbol, filePath string) *UnionSymbol {
	return &UnionSymbol{
		Name:     name,
		Variants: variants,
		FilePath: filePath,
	}
}

// Type returns the union's own name as its ObjectType, consistent with how
// StructDefSymbol/ConstraintSymbol represent their type identity.
func (u *UnionSymbol) Type() environment.ObjectType {
	return environment.ObjectType(u.Name)
}

// Equals returns true if other is ANY_OBJ, another UnionSymbol with the same
// name, or a struct definition/instance that is one of this union's listed
// variants - this is what lets `let animal: Animal = Cat{...}` type-check.
func (u *UnionSymbol) Equals(other Symbol) bool {
	if other.Type() == environment.ANY_OBJ {
		return true
	}

	if otherUnion, ok := other.(*UnionSymbol); ok {
		return u.Name == otherUnion.Name
	}

	if otherInstance, ok := other.(*StructInstanceSymbol); ok {
		_, isVariant := u.Variants[otherInstance.Def.Name]
		return isVariant
	}

	if otherDef, ok := other.(*StructDefSymbol); ok {
		_, isVariant := u.Variants[otherDef.Name]
		return isVariant
	}

	return false
}

// String returns the union's own name.
func (u *UnionSymbol) String() string {
	return u.Name
}
