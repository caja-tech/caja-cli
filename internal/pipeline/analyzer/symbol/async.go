package symbol

import (
	"caja-cli/internal/pipeline/environment"
	"fmt"
)

// AsyncSymbol wraps an underlying Symbol, indicating that the value is
// produced by `async <expr>` and must be unwrapped with `await` before it
// can be used as its underlying type.
type AsyncSymbol struct {
	Underlying Symbol
}

// Type returns environment.ASYNC_OBJ, a distinct type from the underlying
// symbol's — deliberately NOT mirroring NullableSymbol.Type() (which does
// forward to its underlying type, since a nullable value is otherwise
// runtime-compatible with its non-null counterpart). An async handle is not
// interchangeable with its eventual value at all until unwrapped: many
// analyzer checks compare types via raw `sym.Type() == environment.X_OBJ`
// rather than `sym.Equals(...)` (e.g. analyzeInfixExpression's arithmetic
// operators), and if AsyncSymbol.Type() forwarded to the underlying type,
// those checks would incorrectly accept an un-unwrapped async value
// wherever its underlying type is expected (e.g. `let p = async computeNum();
// let x = p + 1` would type-check as valid Number arithmetic, when at
// runtime/in the transpiled Go it is actually adding to a raw task handle —
// exactly the un-synchronized-access mistake `unwrap`/`await` exist to
// prevent). Keeping Type() distinct closes that gap everywhere uniformly,
// without needing to special-case AsyncSymbol in every analyze* function
// that performs a Type()-based check.
func (as *AsyncSymbol) Type() environment.ObjectType {
	return environment.ASYNC_OBJ
}

// Equals checks type compatibility against another async value of the same
// underlying type, or against ANY_OBJ.
func (as *AsyncSymbol) Equals(other Symbol) bool {
	if other.Type() == environment.ANY_OBJ {
		return true
	}
	if otherAsync, ok := other.(*AsyncSymbol); ok {
		return as.Underlying.Equals(otherAsync.Underlying)
	}
	return false
}

// String returns the string representation of the async type.
func (as *AsyncSymbol) String() string {
	return fmt.Sprintf("async %s", as.Underlying.String())
}
