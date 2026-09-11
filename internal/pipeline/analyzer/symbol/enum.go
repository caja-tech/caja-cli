package symbol

import (
	"caja-cli/internal/pipeline/environment"
)

// EnumSymbol represents a closed enum type whose backing primitive type is
// inferred from its members' literal values (e.g.
// `enum CSSProperty { Padding = "padding", Margin = "margin" }` infers a
// String backing type). Members maps each member name to its literal value
// (e.g. "Padding" -> "padding"), used both for member-lookup during
// analysis and directly by the compiler to emit the member's value at
// codegen time — an enum has no distinct Go runtime type of its own, it
// erases to its backing type (see transpiler.go's mapSymbolToGoType, which
// deliberately has no EnumSymbol case and so falls through to the generic
// Type()-based switch below).
//
// The asymmetric subtyping rule that makes this useful — an enum value
// widens implicitly to its backing type, but a bare backing-type value does
// NOT narrow to the enum — is split across two places: Type() here forwards
// to BackingType so BasicSymbol.Equals's generic Type()-comparison path
// would normally treat an enum value as interchangeable with its backing
// type, so BasicSymbol.Equals has an explicit EnumSymbol case reversing that
// for the enum-widens-to-backing-type direction; EnumSymbol.Equals below
// handles the reverse (backing-type-narrows-to-enum, which is rejected)
// simply by requiring `other` to be this exact same enum.
type EnumSymbol struct {
	Name        string
	Members     map[string]string // member name -> backing literal value
	BackingType environment.ObjectType
	FilePath    string
}

// NewEnumSymbol creates and returns a new EnumSymbol.
func NewEnumSymbol(name string, members map[string]string, backingType environment.ObjectType, filePath string) *EnumSymbol {
	return &EnumSymbol{
		Name:        name,
		Members:     members,
		BackingType: backingType,
		FilePath:    filePath,
	}
}

// Type forwards to the enum's backing primitive type. This is what lets an
// enum value flow anywhere its backing type is expected almost "for free"
// (map values, string.concat arguments, ...) via the generic Type()-based
// switches used throughout the analyzer and compiler — see the doc comment
// above for the one place (BasicSymbol.Equals) this forwarding needs an
// explicit counterpart to actually take effect, since Equals (not Type())
// is what parameter/assignment type-checking actually calls.
func (es *EnumSymbol) Type() environment.ObjectType {
	return es.BackingType
}

// Equals reports whether other is a value of this exact same enum type.
// Nominal, not structural: neither a different enum nor a bare value of the
// backing type (even a literal matching one of this enum's own members) is
// accepted here — that asymmetry (backing-type values do NOT narrow to the
// enum) is the entire reason this type exists. The reverse direction (an
// enum value widening to satisfy a plain backing-type parameter) is handled
// on BasicSymbol.Equals's side, not here.
func (es *EnumSymbol) Equals(other Symbol) bool {
	if es.BackingType == environment.ANY_OBJ || other.Type() == environment.ANY_OBJ {
		return true
	}

	otherEnum, ok := other.(*EnumSymbol)
	if !ok {
		return false
	}

	return es.Name == otherEnum.Name
}

// String returns the enum's own name.
func (es *EnumSymbol) String() string {
	return es.Name
}
