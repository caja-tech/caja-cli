package symbol

import (
	"caja-cli/internal/pipeline/environment"
	"testing"
)

// TestBasicSymbolEqualsScriptWidening pins the ONE-DIRECTIONAL widening of
// Script to String at the unit level, where the asymmetry is a single `if`
// that would be trivially easy to "tidy" into a symmetric comparison.
//
// Direction matters and is easy to misread: the receiver is the DECLARED
// type (the parameter / field / annotation being satisfied) and the
// argument is the ACTUAL type flowing into it. So String.Equals(Script) is
// "a Script was passed where a String was declared" — allowed, because
// validated JavaScript source genuinely is text. Script.Equals(String) is
// "a plain String was passed where a Script was declared" — rejected,
// which is the entire reason the type exists: js.raw is the only producer
// of a Script, and js.raw is what runs the validator. Make that symmetric
// and the compile-time JavaScript check silently becomes advisory.
func TestBasicSymbolEqualsScriptWidening(t *testing.T) {
	tests := []struct {
		name     string
		declared environment.ObjectType
		actual   environment.ObjectType
		want     bool
	}{
		{"Script satisfies a declared String", environment.STRING_OBJ, environment.SCRIPT_OBJ, true},
		{"String does NOT satisfy a declared Script", environment.SCRIPT_OBJ, environment.STRING_OBJ, false},
		{"Script satisfies a declared Script", environment.SCRIPT_OBJ, environment.SCRIPT_OBJ, true},
		{"String satisfies a declared String", environment.STRING_OBJ, environment.STRING_OBJ, true},

		// The widening is String-specific, not "Script is compatible with
		// everything": a Script must not leak into a Number/Boolean slot
		// just because it erases to a Go string at codegen.
		{"Script does NOT satisfy a declared Number", environment.NUMBER_OBJ, environment.SCRIPT_OBJ, false},
		{"Script does NOT satisfy a declared Boolean", environment.BOOLEAN_OBJ, environment.SCRIPT_OBJ, false},
		{"Number does NOT satisfy a declared Script", environment.SCRIPT_OBJ, environment.NUMBER_OBJ, false},

		// Any stays permissive in both directions, exactly as before — an
		// unresolved type must not cascade errors out of one unknown.
		{"Any satisfies a declared Script", environment.SCRIPT_OBJ, environment.ANY_OBJ, true},
		{"Script satisfies a declared Any", environment.ANY_OBJ, environment.SCRIPT_OBJ, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			declared := NewBasicSymbol(tt.declared)
			actual := NewBasicSymbol(tt.actual)
			if got := declared.Equals(actual); got != tt.want {
				t.Errorf("%s.Equals(%s) = %v, want %v", tt.declared, tt.actual, got, tt.want)
			}
		})
	}
}

// TestBasicSymbolScriptIsNotAReferenceType guards the other half of the
// "a Script is just a string" story. Script erases to a plain Go string at
// codegen (mapSymbolToGoType), so it must be copied by value like every
// other primitive; classifying it as a reference type would put it on the
// copy-on-write path built for genuinely shared values.
func TestBasicSymbolScriptIsNotAReferenceType(t *testing.T) {
	if environment.IsReferenceType(environment.SCRIPT_OBJ) {
		t.Errorf("expected %s to be a value type, matching String", environment.SCRIPT_OBJ)
	}
	// Sanity check against a type that genuinely is one, so this cannot
	// pass vacuously by IsReferenceType returning false for everything.
	if !environment.IsReferenceType(environment.ARRAY_OBJ) {
		t.Errorf("expected %s to still be a reference type", environment.ARRAY_OBJ)
	}
}
