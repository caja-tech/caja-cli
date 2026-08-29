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
		tSym := NewGenericSymbol("T")
		arrTSym := NewArraySymbol(tSym)
		numSym := NewBasicSymbol(environment.NUMBER_OBJ)

		return map[string]Symbol{
			"len":   NewFunctionSymbol("len", []string{"arr"}, []string{"T"}, 1, []Symbol{arrTSym}, numSym),
			"push":  NewFunctionSymbol("push", []string{"arr", "item"}, []string{"T"}, 2, []Symbol{arrTSym, tSym}, arrTSym),
			"pop":   NewFunctionSymbol("pop", []string{"arr"}, []string{"T"}, 1, []Symbol{arrTSym}, arrTSym),
			"head":  NewFunctionSymbol("head", []string{"arr"}, []string{"T"}, 1, []Symbol{arrTSym}, tSym),
			"tail":  NewFunctionSymbol("tail", []string{"arr"}, []string{"T"}, 1, []Symbol{arrTSym}, arrTSym),
			"last":  NewFunctionSymbol("last", []string{"arr"}, []string{"T"}, 1, []Symbol{arrTSym}, tSym),
			"copy":  NewFunctionSymbol("copy", []string{"arr"}, []string{"T"}, 1, []Symbol{arrTSym}, arrTSym),
			"slice": NewFunctionSymbol("slice", []string{"arr", "start", "end"}, []string{"T"}, 3, []Symbol{arrTSym, numSym, numSym}, arrTSym),
			"join":  NewFunctionSymbol("join", []string{"arr", "other"}, []string{"T"}, 2, []Symbol{arrTSym, arrTSym}, arrTSym),
		}, nil, true

	case "date":
		return map[string]Symbol{
			"year": NewBuiltinSymbol(1, "year(d: Date) -> Number", "d: Date"),
			"month": NewBuiltinSymbol(1, "month(d: Date) -> Number", "d: Date"),
			"day": NewBuiltinSymbol(1, "day(d: Date) -> Number", "d: Date"),
			"weekday": NewBuiltinSymbol(1, "weekday(d: Date) -> Number", "d: Date"),
			"today": NewBuiltinSymbol(0, "today() -> Date"),
			"parse": NewBuiltinSymbol(1, "parse(dateStr: String) -> Date", "dateStr: String"),
			"addDays": NewBuiltinSymbol(2, "addDays(d: Date, days: Number) -> Date", "d: Date", "days: Number"),
			"diffDays": NewBuiltinSymbol(2, "diffDays(start: Date, end: Date) -> Number", "start: Date", "end: Date"),
			"new": NewBuiltinSymbol(3, "new(year: Number, month: Number, day: Number) -> Date", "year: Number", "month: Number", "day: Number"),
		}, nil, true

	case "string":
		return map[string]Symbol{
			"join": NewBuiltinSymbol(2, "join(elements: Array<String>, separator: String) -> String", "elements: Array<String>", "separator: String"),
			"charAt": NewBuiltinSymbol(2, "charAt(str: String, index: Number) -> String", "str: String", "index: Number"),
			"substring": NewBuiltinSymbol(3, "substring(str: String, start: Number, end: Number) -> String", "str: String", "start: Number", "end: Number"),
			"concat": NewBuiltinSymbol(2, "concat(str1: String, str2: String) -> String", "str1: String", "str2: String"),
			"split": NewBuiltinSymbol(2, "split(str: String, separator: String) -> Array<String>", "str: String", "separator: String"),
			"contains": NewBuiltinSymbol(2, "contains(str: String, search: String) -> Boolean", "str: String", "search: String"),
			"startsWith": NewBuiltinSymbol(2, "startsWith(str: String, prefix: String) -> Boolean", "str: String", "prefix: String"),
			"endsWith": NewBuiltinSymbol(2, "endsWith(str: String, suffix: String) -> Boolean", "str: String", "suffix: String"),
			"replace": NewBuiltinSymbol(3, "replace(str: String, search: String, replace: String) -> String", "str: String", "search: String", "replace: String"),
			"toUpper": NewBuiltinSymbol(1, "toUpper(str: String) -> String", "str: String"),
			"toLower": NewBuiltinSymbol(1, "toLower(str: String) -> String", "str: String"),
			"trim": NewBuiltinSymbol(1, "trim(str: String) -> String", "str: String"),
			"len": NewBuiltinSymbol(1, "len(str: String) -> Number", "str: String"),
		}, nil, true

	case "math":
		return map[string]Symbol{
			"abs": NewBuiltinSymbol(1, "abs(num: Number) -> Number", "num: Number"),
			"sqrt": NewBuiltinSymbol(1, "sqrt(num: Number) -> Number", "num: Number"),
			"pow": NewBuiltinSymbol(2, "pow(base: Number, exp: Number) -> Number", "base: Number", "exp: Number"),
			"floor": NewBuiltinSymbol(1, "floor(num: Number) -> Number", "num: Number"),
			"ceil": NewBuiltinSymbol(1, "ceil(num: Number) -> Number", "num: Number"),
			"round": NewBuiltinSymbol(1, "round(num: Number) -> Number", "num: Number"),
			"min": NewBuiltinSymbol(2, "min(a: Number, b: Number) -> Number", "a: Number", "b: Number"),
			"max": NewBuiltinSymbol(2, "max(a: Number, b: Number) -> Number", "a: Number", "b: Number"),
			"log": NewBuiltinSymbol(2, "log(num: Number, base: Number) -> Number", "num: Number", "base: Number"),
			"rand": NewBuiltinSymbol(0, "rand() -> Number"),
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
			"info": NewBuiltinSymbol(2, "info(message: String, args: Any) -> Nil", "message: String", "args: Any"),
			"warn": NewBuiltinSymbol(2, "warn(message: String, args: Any) -> Nil", "message: String", "args: Any"),
			"error": NewBuiltinSymbol(2, "error(message: String, args: Any) -> Nil", "message: String", "args: Any"),
			"export": NewBuiltinSymbol(1, "export(data: Any) -> Nil", "data: Any"),
		}, nil, true
		
	case "map":
		return map[string]Symbol{
			"containsKey": NewBuiltinSymbol(2, "containsKey(map: Map, key: String) -> Boolean", "map: Map", "key: String"),
			"delete": NewBuiltinSymbol(2, "delete(map: Map, key: String) -> Nil", "map: Map", "key: String"),
		}, map[string]Symbol{
			"KeyFunc": NewFunctionSymbol("KeyFunc", nil, nil, 0, nil, NewBasicSymbol(environment.STRING_OBJ)),
		}, true
	case "cast":
		return map[string]Symbol{
			"to": NewFunctionSymbol("to", []string{"value", "fallback"}, []string{"T", "R"}, 2, []Symbol{NewGenericSymbol("T"), NewGenericSymbol("R")}, NewGenericSymbol("R")),
		}, nil, true
	}

	return nil, nil, false
}
