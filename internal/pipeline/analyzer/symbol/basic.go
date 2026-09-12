package symbol

import (
	"caja-cli/internal/pipeline/environment"
)

// BasicSymbol represents a primitive type in the semantic analysis (e.g., NUMBER, STRING, BOOLEAN).
type BasicSymbol struct {
	symbolType environment.ObjectType
}

// NewBasicSymbol creates and returns a new BasicSymbol of the specified type.
func NewBasicSymbol(symbolType environment.ObjectType) *BasicSymbol {
	return &BasicSymbol{
		symbolType: symbolType,
	}
}

// Equals compares this BasicSymbol with another Symbol to determine if they represent the same type.
// It returns true if their types match exactly, or if either symbol is of type ANY_OBJ.
//
// An EnumSymbol whose backing type matches this primitive is also accepted
// here — an enum value widens implicitly to its backing type (e.g. a
// CSSProperty flows anywhere a String is expected), even though the
// reverse is rejected by EnumSymbol.Equals (a bare String does not narrow
// to the enum). This one-directional asymmetry is deliberate: see
// EnumSymbol's doc comment.
func (bs *BasicSymbol) Equals(other Symbol) bool {
	if bs.symbolType == environment.ANY_OBJ || other.Type() == environment.ANY_OBJ {
		return true
	}

	if enumSymbol, ok := other.(*EnumSymbol); ok {
		return bs.symbolType == enumSymbol.BackingType
	}

	otherSymbol, ok := other.(*BasicSymbol)
	if !ok {
		return false
	}

	// Script widens to String, exactly the way an enum widens to its backing
	// type above: validated JavaScript source IS a string, so it can be
	// interpolated, measured, or written to a file like any other. The
	// reverse is NOT permitted — this check is one-directional, so a plain
	// String still fails to satisfy a Script parameter, which is the whole
	// reason the type exists (js.raw is the only way to produce one, and it
	// is what runs the validator).
	if bs.symbolType == environment.STRING_OBJ && otherSymbol.symbolType == environment.SCRIPT_OBJ {
		return true
	}

	if bs.symbolType != otherSymbol.symbolType {
		return false
	}

	return true
}

// Type returns the underlying environment.ObjectType of this symbol.
func (bs *BasicSymbol) Type() environment.ObjectType {
	return bs.symbolType
}

// String returns the string representation of the basic type.
func (bs *BasicSymbol) String() string {
	return string(bs.Type())
}
