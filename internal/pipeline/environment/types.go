package environment

// ObjectType tags a semantic value's kind. Originally the runtime tag for the
// tree-walking interpreter's Object values, it now lives on purely as the
// type-identity vocabulary the analyzer and compiler packages share (see
// symbol.Symbol.Type()) — every concrete Object implementation the
// interpreter used has been removed along with the interpreter itself.
type ObjectType string

const (
	ANY_OBJ          ObjectType = "Any"
	NUMBER_OBJ       ObjectType = "Number"
	STRING_OBJ       ObjectType = "String"
	BOOLEAN_OBJ      ObjectType = "Boolean"
	DATE_OBJ         ObjectType = "Date"
	INSTANT_OBJ      ObjectType = "Instant"
	DURATION_OBJ     ObjectType = "Duration"
	ELEMENT_OBJ      ObjectType = "Element"
	FUNCTION_OBJ     ObjectType = "Function"
	BUILTIN_OBJ      ObjectType = "Builtin"
	ARRAY_OBJ        ObjectType = "Array"
	MAP_OBJ          ObjectType = "Map"
	RETURN_VALUE_OBJ ObjectType = "RETURN_VALUE"
	MODULE_OBJ       ObjectType = "MODULE"
	NULL_OBJ         ObjectType = "NULL"
	ASYNC_OBJ        ObjectType = "Async"
)

// IsReferenceType reports whether a value of the given type is passed/shared
// by reference rather than by value.
func IsReferenceType(t ObjectType) bool {
	switch t {
	case NUMBER_OBJ, STRING_OBJ, BOOLEAN_OBJ, DATE_OBJ, INSTANT_OBJ, DURATION_OBJ:
		return false
	}
	return true
}
