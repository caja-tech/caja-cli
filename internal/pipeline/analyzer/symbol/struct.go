package symbol

import (
	"caja-cli/internal/pipeline/environment"
)

// StructFieldSymbol represents a single field definition within a struct.
type StructFieldSymbol struct {
	Type       Symbol
	IsConstant bool
}

// StructDefSymbol represents a struct type definition (the blueprint).
// It maps field names to their respective field definitions.
type StructDefSymbol struct {
	Name   string
	Fields map[string]StructFieldSymbol
}

// NewStructDefSymbol creates and returns a new StructDefSymbol.
func NewStructDefSymbol(name string, fields map[string]StructFieldSymbol) *StructDefSymbol {
	return &StructDefSymbol{
		Name:   name,
		Fields: fields,
	}
}

// Type returns the ANY_OBJ type for struct definitions during semantic checking.
func (sds *StructDefSymbol) Type() environment.ObjectType {
	// A struct definition isn't an instantiated object type in the evaluator,
	// but we can return ANY_OBJ or a special representation if needed.
	// For semantic checking, this type is just used as a reference.
	return environment.ANY_OBJ
}

// Equals returns true if the other symbol represents the same struct definition.
func (sds *StructDefSymbol) Equals(other Symbol) bool {
	if otherInstance, ok := other.(*StructInstanceSymbol); ok {
		return sds.Name == otherInstance.Def.Name
	}

	if otherDef, ok := other.(*StructDefSymbol); ok {
		return sds.Name == otherDef.Name
	}

	return false
}

// StructInstanceSymbol represents an instantiated struct object.
// It references the definition so we can validate property accesses and assignments.
type StructInstanceSymbol struct {
	Def *StructDefSymbol
}

// NewStructInstanceSymbol creates and returns a new StructInstanceSymbol.
func NewStructInstanceSymbol(def *StructDefSymbol) *StructInstanceSymbol {
	return &StructInstanceSymbol{
		Def: def,
	}
}

// Type returns the specific struct name as an ObjectType for type-checking.
func (sis *StructInstanceSymbol) Type() environment.ObjectType {
	// In the evaluator, structs might just be a generic STRUCT_OBJ, or we can just use ANY_OBJ for now,
	// since type-checking occurs in the semantic phase.
	// The underlying struct type name is really what matters.
	return environment.ObjectType(sis.Def.Name)
}

// Equals returns true if the other symbol is an instance of the same struct type.
func (sis *StructInstanceSymbol) Equals(other Symbol) bool {
	if other.Type() == environment.ANY_OBJ || sis.Type() == environment.ANY_OBJ {
		return true
	}

	otherInstance, ok := other.(*StructInstanceSymbol)
	if !ok {
		return false
	}

	return sis.Def.Name == otherInstance.Def.Name
}
