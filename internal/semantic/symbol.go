package semantic

import "caja-cli/internal/environment"

// Symbol represents a semantic entity (like a variable or function) tracked during
// analysis, containing its type information and function signature details if applicable.
type Symbol struct {
	Type        environment.ObjectType
	Arity       int
	ParamTypes  []Symbol
	ReturnType  *Symbol
	ElementType *Symbol
}

var anySymbol = Symbol{Type: environment.ANY_OBJ, Arity: 0}

// Equals checks if two symbols represent the same semantic type.
// It handles ANY_OBJ as a wild card, and strictly compares function signatures.
func (s Symbol) Equals(other Symbol) bool {
	if s.Type == environment.ANY_OBJ || other.Type == environment.ANY_OBJ {
		return true
	}
	if s.Type != other.Type {
		return false
	}

	if s.Type == environment.FUNCTION_OBJ {
		if s.Arity != other.Arity {
			return false
		}
		for i, pt := range s.ParamTypes {
			if !pt.Equals(other.ParamTypes[i]) {
				return false
			}
		}

		if s.ReturnType != nil && other.ReturnType != nil {
			if !s.ReturnType.Equals(*other.ReturnType) {
				return false
			}
		} else if s.ReturnType != other.ReturnType {
			return false
		}
	} else if s.Type == environment.ARRAY_OBJ {
		if s.ElementType == nil && other.ElementType == nil {
			return true
		}
		if s.ElementType == nil || other.ElementType == nil {
			return false
		}
		return s.ElementType.Equals(*other.ElementType)
	}

	return true
}
