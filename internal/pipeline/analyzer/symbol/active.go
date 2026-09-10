package symbol

import (
	"caja-cli/internal/pipeline/environment"
	"fmt"
)

// ActiveSymbol wraps an underlying Symbol, indicating that the value is
// declared `active` — mutable and observable, so a `react`-marked argument
// at a call site can subscribe to its changes (see analyzePrefixExpression's
// "react" case). Unlike AsyncSymbol (which deliberately keeps Type()
// distinct so an un-awaited value can never silently pass as its underlying
// type), Type() here forwards to Underlying, like NullableSymbol does:
// there is no "might be missing" case to force a caller to confront, and
// the whole point is that an active Number keeps behaving like a plain
// Number in ordinary expressions (arithmetic, being passed to another
// function, ...). This is safe specifically because the compiler
// separately guarantees every read gets unwrapped (see
// transpileExpressionInternal's *ast.Identifier case) — the gap that made
// forwarding dangerous for Nullable (silent accept + no compiler-side
// unwrap) doesn't exist here.
type ActiveSymbol struct {
	Underlying Symbol
}

// Type returns the environment.ObjectType of the underlying symbol.
func (as *ActiveSymbol) Type() environment.ObjectType {
	return as.Underlying.Type()
}

// Equals checks type compatibility against the underlying type, or against
// another active value of the same underlying type.
func (as *ActiveSymbol) Equals(other Symbol) bool {
	if other.Type() == environment.ANY_OBJ {
		return true
	}
	if otherActive, ok := other.(*ActiveSymbol); ok {
		return as.Underlying.Equals(otherActive.Underlying)
	}
	return as.Underlying.Equals(other)
}

// String returns the string representation of the active type.
func (as *ActiveSymbol) String() string {
	return fmt.Sprintf("active %s", as.Underlying.String())
}
