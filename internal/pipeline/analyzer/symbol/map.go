package symbol

import "caja-cli/internal/pipeline/environment"

// MapSymbol represents a map type in the semantic analysis.
type MapSymbol struct {
	Key   Symbol
	Value Symbol
}

// NewMapSymbol creates and returns a new MapSymbol with the given key and value symbols.
func NewMapSymbol(keySymbol, valueSymbol Symbol) *MapSymbol {
	return &MapSymbol{
		Key:   keySymbol,
		Value: valueSymbol,
	}
}

// Equals compares this MapSymbol with another Symbol to determine if they represent the same type.
func (ms *MapSymbol) Equals(other Symbol) bool {
	if other.Type() == environment.ANY_OBJ {
		return true
	}

	otherSymbol, ok := other.(*MapSymbol)
	if !ok {
		return false
	}

	if ms.Key.Type() == environment.ANY_OBJ || otherSymbol.Key.Type() == environment.ANY_OBJ {
		if ms.Value.Type() == environment.ANY_OBJ || otherSymbol.Value.Type() == environment.ANY_OBJ {
			return true
		}
		return ms.Value.Equals(otherSymbol.Value)
	}

	if ms.Value.Type() == environment.ANY_OBJ || otherSymbol.Value.Type() == environment.ANY_OBJ {
		return ms.Key.Equals(otherSymbol.Key)
	}

	return ms.Key.Equals(otherSymbol.Key) && ms.Value.Equals(otherSymbol.Value)
}

func (ms *MapSymbol) Type() environment.ObjectType {
	return environment.MAP_OBJ
}
