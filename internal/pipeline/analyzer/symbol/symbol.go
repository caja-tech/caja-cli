package symbol

import (
	"caja-cli/internal/pipeline/environment"
)

// Symbol is the interface representing a type or object in semantic analysis.
// All specific symbol types must implement this interface.
type Symbol interface {
	Equals(Symbol) bool
	Type() environment.ObjectType
	String() string
}

var anySymbol = &BasicSymbol{symbolType: environment.ANY_OBJ}

// AnySymbol returns a generic symbol representing ANY_OBJ.
// It is used as a fallback for dynamic types or when a semantic error prevents type determination.
func AnySymbol() *BasicSymbol {
	return anySymbol
}

// GetStandardModule retrieves the symbols exported by a standard builtin module by its name.
func GetStandardModule(moduleName string) (map[string]Symbol, map[string]Symbol, bool) {
	switch moduleName {
	case "array":
		return map[string]Symbol{
			"len":    NewBuiltinSymbol(1),
			"push":   NewBuiltinSymbol(2),
			"pop":    NewBuiltinSymbol(1),
			"head":   NewBuiltinSymbol(1),
			"tail":   NewBuiltinSymbol(1),
			"last":   NewBuiltinSymbol(1),
			"copy":   NewBuiltinSymbol(1),
			"slice":  NewBuiltinSymbol(3),
			"join":   NewBuiltinSymbol(2),
		}, nil, true

	case "date":
		return map[string]Symbol{
			"year":      NewBuiltinSymbol(1),
			"month":     NewBuiltinSymbol(1),
			"day":       NewBuiltinSymbol(1),
			"weekday":   NewBuiltinSymbol(1),
			"today":     NewBuiltinSymbol(0),
			"parse":     NewBuiltinSymbol(1),
			"addDays":   NewBuiltinSymbol(2),
			"diffDays":  NewBuiltinSymbol(2),
			"new":       NewBuiltinSymbol(3),
		}, nil, true

	case "string":
		return map[string]Symbol{
			"join":       NewBuiltinSymbol(2),
			"charAt":     NewBuiltinSymbol(2),
			"substring":  NewBuiltinSymbol(3),
			"concat":     NewBuiltinSymbol(2),
			"split":      NewBuiltinSymbol(2),
			"contains":   NewBuiltinSymbol(2),
			"startsWith": NewBuiltinSymbol(2),
			"endsWith":   NewBuiltinSymbol(2),
			"replace":    NewBuiltinSymbol(3),
			"toUpper":    NewBuiltinSymbol(1),
			"toLower":    NewBuiltinSymbol(1),
			"trim":       NewBuiltinSymbol(1),
			"len":        NewBuiltinSymbol(1),
		}, nil, true

	case "math":
		return map[string]Symbol{
			"abs":    NewBuiltinSymbol(1),
			"sqrt":   NewBuiltinSymbol(1),
			"pow":    NewBuiltinSymbol(2),
			"floor":  NewBuiltinSymbol(1),
			"ceil":   NewBuiltinSymbol(1),
			"round":  NewBuiltinSymbol(1),
			"min":    NewBuiltinSymbol(2),
			"max":    NewBuiltinSymbol(2),
			"log":    NewBuiltinSymbol(2),
			"rand":   NewBuiltinSymbol(0),
			"PI":     &BasicSymbol{symbolType: environment.NUMBER_OBJ},
			"E":      &BasicSymbol{symbolType: environment.NUMBER_OBJ},
			"SQRT2":  &BasicSymbol{symbolType: environment.NUMBER_OBJ},
			"LN2":    &BasicSymbol{symbolType: environment.NUMBER_OBJ},
			"LN10":   &BasicSymbol{symbolType: environment.NUMBER_OBJ},
			"LOG2E":  &BasicSymbol{symbolType: environment.NUMBER_OBJ},
			"LOG10E": &BasicSymbol{symbolType: environment.NUMBER_OBJ},
		}, nil, true

	case "log":
		return map[string]Symbol{
			"info":   NewBuiltinSymbol(2),
			"warn":   NewBuiltinSymbol(2),
			"error":  NewBuiltinSymbol(2),
			"export": NewBuiltinSymbol(1),
		}, nil, true
		
	case "map":
		return map[string]Symbol{
			"containsKey": NewBuiltinSymbol(2),
			"delete":      NewBuiltinSymbol(2),
		}, map[string]Symbol{
			"KeyFunc": NewFunctionSymbol(0, nil, NewBasicSymbol(environment.STRING_OBJ)),
		}, true
	case "cast":
		return map[string]Symbol{
			"toNumber":  NewBuiltinSymbol(2),
			"toString":  NewBuiltinSymbol(2),
			"toBoolean": NewBuiltinSymbol(2),
			"toDate":    NewBuiltinSymbol(2),
		}, nil, true
	}

	return nil, nil, false
}
