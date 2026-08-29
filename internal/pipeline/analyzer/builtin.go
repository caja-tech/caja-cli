package analyzer

import (
	"caja-cli/internal/pipeline/analyzer/symbol"
	"caja-cli/internal/pipeline/ast"
	"caja-cli/internal/pipeline/environment"
	"fmt"
)

// analyzeBuiltinCall intercepts calls to builtin functions (like len, append, head, tail)
// to provide custom, compile-time polymorphic type-checking and inference.
func (a *Analyzer) analyzeBuiltinCall(moduleName string, functionName string, n *ast.CallExpression) (symbol.Symbol, bool) {
	fullName := functionName
	if moduleName != "" {
		fullName = moduleName + "." + functionName
	}

	switch fullName {
	case "string.charAt":
		return a.analyzeStringCharAtFunction(n), true
	case "string.substring":
		return a.analyzeStringSubstringFunction(n), true
	case "string.concat":
		return a.analyzeStringConcatFunction(n), true
	case "string.split":
		return a.analyzeStringSplitFunction(n), true
	case "string.contains", "string.startsWith", "string.endsWith":
		return a.analyzeStringMatchFunction(functionName, n), true
	case "string.replace":
		return a.analyzeStringReplaceFunction(n), true
	case "string.toUpper", "string.toLower", "string.trim":
		return a.analyzeStringTransformFunction(functionName, n), true
	case "string.len":
		return a.analyzeStringLenFunction(n), true
	case "string.join":
		return a.analyzeStringJoinFunction(n), true
	case "date.year", "date.month", "date.day", "date.weekday":
		return a.analyzeDateComponentFunction(functionName, n), true
	case "date.today":
		return a.analyzeDateTodayFunction(n), true
	case "date.parse":
		return a.analyzeDateParseFunction(n), true
	case "date.addDays":
		return a.analyzeDateAddDaysFunction(n), true
	case "date.diffDays":
		return a.analyzeDateDiffDaysFunction(n), true
	case "date.new":
		return a.analyzeDateNewFunction(n), true
	case "math.abs", "math.sqrt", "math.floor", "math.ceil", "math.round":
		return a.analyzeMathOneArgFunction(functionName, n), true
	case "math.rand":
		return a.analyzeMathZeroArgFunction(functionName, n), true
	case "math.pow", "math.min", "math.max", "math.log":
		return a.analyzeMathTwoArgFunction(functionName, n), true
	case "log.info", "log.warn", "log.error":
		return a.analyzeLogFunction(functionName, n), true
	case "log.export":
		return a.analyzeLogExportFunction(functionName, n), true
	case "map.containsKey":
		return a.analyzeMapContainsKeyFunction(n), true
	case "map.delete":
		return a.analyzeMapDeleteFunction(n), true
	default:
		return symbol.AnySymbol(), false

	}
}

// analyzeStringCharAtFunction checks the arity and type for the builtin string 'charAt' function, returning a STRING.
func (a *Analyzer) analyzeStringCharAtFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 2 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 2 arguments for 'charAt', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	strSymbol := a.analyze(n.Arguments[0])
	idxSymbol := a.analyze(n.Arguments[1])

	if strSymbol.Type() != environment.STRING_OBJ && strSymbol.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: first argument to 'charAt' must be String, got %s", strSymbol.Type()))
	}
	if idxSymbol.Type() != environment.NUMBER_OBJ && idxSymbol.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: second argument to 'charAt' must be Number, got %s", idxSymbol.Type()))
	}

	return symbol.NewBasicSymbol(environment.STRING_OBJ)
}

// analyzeStringSubstringFunction checks the arity and type for the builtin string 'substring' function, returning a STRING.
func (a *Analyzer) analyzeStringSubstringFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 3 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 3 arguments for 'substring', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	strSymbol := a.analyze(n.Arguments[0])
	startSymbol := a.analyze(n.Arguments[1])
	endSymbol := a.analyze(n.Arguments[2])

	if strSymbol.Type() != environment.STRING_OBJ && strSymbol.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: first argument to 'substring' must be String, got %s", strSymbol.Type()))
	}
	if startSymbol.Type() != environment.NUMBER_OBJ && startSymbol.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: second argument to 'substring' must be Number, got %s", startSymbol.Type()))
	}
	if endSymbol.Type() != environment.NUMBER_OBJ && endSymbol.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: third argument to 'substring' must be Number, got %s", endSymbol.Type()))
	}

	return symbol.NewBasicSymbol(environment.STRING_OBJ)
}

// analyzeStringConcatFunction checks the arity and type for the builtin string 'concat' function, returning a STRING.
func (a *Analyzer) analyzeStringConcatFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 2 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 2 arguments for 'concat', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	str1Symbol := a.analyze(n.Arguments[0])
	str2Symbol := a.analyze(n.Arguments[1])

	if str1Symbol.Type() != environment.STRING_OBJ && str1Symbol.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: first argument to 'concat' must be String, got %s", str1Symbol.Type()))
	}
	if str2Symbol.Type() != environment.STRING_OBJ && str2Symbol.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: second argument to 'concat' must be String, got %s", str2Symbol.Type()))
	}

	return symbol.NewBasicSymbol(environment.STRING_OBJ)
}

// analyzeStringSplitFunction checks the arity and type for the builtin string 'split' function, returning an ARRAY of STRINGs.
func (a *Analyzer) analyzeStringSplitFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 2 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 2 arguments for 'split', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	strSymbol := a.analyze(n.Arguments[0])
	delimSymbol := a.analyze(n.Arguments[1])

	if strSymbol.Type() != environment.STRING_OBJ && strSymbol.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: first argument to 'split' must be String, got %s", strSymbol.Type()))
	}
	if delimSymbol.Type() != environment.STRING_OBJ && delimSymbol.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: second argument to 'split' must be String, got %s", delimSymbol.Type()))
	}

	return symbol.NewArraySymbol(symbol.NewBasicSymbol(environment.STRING_OBJ))
}

// analyzeStringMatchFunction checks the arity and type for string matching functions (e.g. contains), returning a BOOLEAN.
func (a *Analyzer) analyzeStringMatchFunction(functionName string, n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 2 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 2 arguments for '%s', got %d", functionName, len(n.Arguments)))
		return symbol.AnySymbol()
	}

	strSymbol := a.analyze(n.Arguments[0])
	subSymbol := a.analyze(n.Arguments[1])

	if strSymbol.Type() != environment.STRING_OBJ && strSymbol.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: first argument to '%s' must be String, got %s", functionName, strSymbol.Type()))
	}
	if subSymbol.Type() != environment.STRING_OBJ && subSymbol.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: second argument to '%s' must be String, got %s", functionName, subSymbol.Type()))
	}

	return symbol.NewBasicSymbol(environment.BOOLEAN_OBJ)
}

// analyzeStringReplaceFunction checks the arity and type for the builtin string 'replace' function, returning a STRING.
func (a *Analyzer) analyzeStringReplaceFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 3 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 3 arguments for 'replace', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	strSymbol := a.analyze(n.Arguments[0])
	oldSymbol := a.analyze(n.Arguments[1])
	newSymbol := a.analyze(n.Arguments[2])

	if strSymbol.Type() != environment.STRING_OBJ && strSymbol.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: first argument to 'replace' must be String, got %s", strSymbol.Type()))
	}
	if oldSymbol.Type() != environment.STRING_OBJ && oldSymbol.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: second argument to 'replace' must be String, got %s", oldSymbol.Type()))
	}
	if newSymbol.Type() != environment.STRING_OBJ && newSymbol.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: third argument to 'replace' must be String, got %s", newSymbol.Type()))
	}

	return symbol.NewBasicSymbol(environment.STRING_OBJ)
}

// analyzeStringTransformFunction checks the arity and type for string transformations (e.g. toUpper), returning a STRING.
func (a *Analyzer) analyzeStringTransformFunction(functionName string, n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 1 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 1 arguments for '%s', got %d", functionName, len(n.Arguments)))
		return symbol.AnySymbol()
	}

	strSymbol := a.analyze(n.Arguments[0])

	if strSymbol.Type() != environment.STRING_OBJ && strSymbol.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: first argument to '%s' must be String, got %s", functionName, strSymbol.Type()))
	}

	return symbol.NewBasicSymbol(environment.STRING_OBJ)
}

// analyzeStringLenFunction checks the arity and type for the builtin string 'len' function, returning a NUMBER.
func (a *Analyzer) analyzeStringLenFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 1 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 1 arguments for 'len', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	strSymbol := a.analyze(n.Arguments[0])

	if strSymbol.Type() != environment.STRING_OBJ && strSymbol.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: first argument to 'len' must be String, got %s", strSymbol.Type()))
	}

	return symbol.NewBasicSymbol(environment.NUMBER_OBJ)
}

// analyzeStringJoinFunction checks the arity and type for the builtin string 'join' function, returning a STRING.
func (a *Analyzer) analyzeStringJoinFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 2 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 2 arguments for 'join', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	arrSymbol, ok := a.analyze(n.Arguments[0]).(*symbol.ArraySymbol)
	if !ok {
		a.reportError(n.Token, fmt.Sprintf("type error: cannot parse array symbol, got %s", n.Arguments[0]))
		return symbol.AnySymbol()
	}
	delimSymbol := a.analyze(n.Arguments[1])

	if arrSymbol.Type() != environment.ARRAY_OBJ && arrSymbol.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: first argument to 'join' must be an ARRAY, got %s", arrSymbol.Type()))
	} else if arrSymbol.Type() == environment.ARRAY_OBJ && arrSymbol.ElementSymbol() != nil && arrSymbol.ElementSymbol().Type() != environment.STRING_OBJ && arrSymbol.ElementSymbol().Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: array elements for 'join' must be String, got %s", arrSymbol.ElementSymbol().Type()))
	}

	if delimSymbol.Type() != environment.STRING_OBJ && delimSymbol.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: second argument to 'join' must be String, got %s", delimSymbol.Type()))
	}

	return symbol.NewBasicSymbol(environment.STRING_OBJ)
}

// analyzeDateComponentFunction checks the arity and type for date component accessors (e.g. year, month), returning a NUMBER.
func (a *Analyzer) analyzeDateComponentFunction(functionName string, n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 1 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 1 arguments for '%s', got %d", functionName, len(n.Arguments)))
		return symbol.AnySymbol()
	}

	dateSymbol := a.analyze(n.Arguments[0])

	if dateSymbol.Type() != environment.DATE_OBJ && dateSymbol.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: first argument to '%s' must be Date, got %s", functionName, dateSymbol.Type()))
	}

	return symbol.NewBasicSymbol(environment.NUMBER_OBJ)
}

// analyzeDateTodayFunction checks the arity and type for the builtin date 'today' function, returning a DATE.
func (a *Analyzer) analyzeDateTodayFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 0 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 0 arguments for 'today', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	return symbol.NewBasicSymbol(environment.DATE_OBJ)
}

// analyzeDateParseFunction checks the arity and type for the builtin date 'parse' function, returning a DATE.
func (a *Analyzer) analyzeDateParseFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 1 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 1 arguments for 'parse', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	strSymbol := a.analyze(n.Arguments[0])

	if strSymbol.Type() != environment.STRING_OBJ && strSymbol.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: first argument to 'parse' must be String, got %s", strSymbol.Type()))
	}

	return symbol.NewBasicSymbol(environment.DATE_OBJ)
}

// analyzeDateAddDaysFunction checks the arity and type for the builtin date 'addDays' function, returning a DATE.
func (a *Analyzer) analyzeDateAddDaysFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 2 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 2 arguments for 'addDays', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	dateSymbol := a.analyze(n.Arguments[0])
	numSymbol := a.analyze(n.Arguments[1])

	if dateSymbol.Type() != environment.DATE_OBJ && dateSymbol.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: first argument to 'addDays' must be Date, got %s", dateSymbol.Type()))
	}
	if numSymbol.Type() != environment.NUMBER_OBJ && numSymbol.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: second argument to 'addDays' must be Number, got %s", numSymbol.Type()))
	}

	return symbol.NewBasicSymbol(environment.DATE_OBJ)
}

// analyzeDateDiffDaysFunction checks the arity and type for the builtin date 'diffDays' function, returning a NUMBER.
func (a *Analyzer) analyzeDateDiffDaysFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 2 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 2 arguments for 'diffDays', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	date1Symbol := a.analyze(n.Arguments[0])
	date2Symbol := a.analyze(n.Arguments[1])

	if date1Symbol.Type() != environment.DATE_OBJ && date1Symbol.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: first argument to 'diffDays' must be Date, got %s", date1Symbol.Type()))
	}
	if date2Symbol.Type() != environment.DATE_OBJ && date2Symbol.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: second argument to 'diffDays' must be Date, got %s", date2Symbol.Type()))
	}

	return symbol.NewBasicSymbol(environment.NUMBER_OBJ)
}

// analyzeDateNewFunction checks the arity and type for the builtin date 'new' function, returning a DATE.
func (a *Analyzer) analyzeDateNewFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 3 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 3 arguments for 'new', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	yearSymbol := a.analyze(n.Arguments[0])
	monthSymbol := a.analyze(n.Arguments[1])
	daySymbol := a.analyze(n.Arguments[2])

	if yearSymbol.Type() != environment.NUMBER_OBJ && yearSymbol.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: first argument to 'new' must be Number, got %s", yearSymbol.Type()))
	}
	if monthSymbol.Type() != environment.NUMBER_OBJ && monthSymbol.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: second argument to 'new' must be Number, got %s", monthSymbol.Type()))
	}
	if daySymbol.Type() != environment.NUMBER_OBJ && daySymbol.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: third argument to 'new' must be Number, got %s", daySymbol.Type()))
	}

	return symbol.NewBasicSymbol(environment.DATE_OBJ)
}

// analyzeMathZeroArgFunction checks the arity and type for 0-argument math functions, returning a NUMBER.
func (a *Analyzer) analyzeMathZeroArgFunction(functionName string, n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 0 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 0 arguments for '%s', got %d", functionName, len(n.Arguments)))
		return symbol.AnySymbol()
	}

	return symbol.NewBasicSymbol(environment.NUMBER_OBJ)
}

// analyzeMathOneArgFunction checks the arity and type for 1-argument math functions, returning a NUMBER.
func (a *Analyzer) analyzeMathOneArgFunction(functionName string, n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 1 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 1 arguments for '%s', got %d", functionName, len(n.Arguments)))
		return symbol.AnySymbol()
	}

	argSymbol := a.analyze(n.Arguments[0])
	if argSymbol.Type() != environment.NUMBER_OBJ && argSymbol.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: first argument to '%s' must be Number, got %s", functionName, argSymbol.Type()))
	}

	return symbol.NewBasicSymbol(environment.NUMBER_OBJ)
}

// analyzeMathTwoArgFunction checks the arity and type for 2-argument math functions, returning a NUMBER.
func (a *Analyzer) analyzeMathTwoArgFunction(functionName string, n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 2 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 2 arguments for '%s', got %d", functionName, len(n.Arguments)))
		return symbol.AnySymbol()
	}

	arg1Symbol := a.analyze(n.Arguments[0])
	if arg1Symbol.Type() != environment.NUMBER_OBJ && arg1Symbol.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: first argument to '%s' must be Number, got %s", functionName, arg1Symbol.Type()))
	}
	arg2Symbol := a.analyze(n.Arguments[1])
	if arg2Symbol.Type() != environment.NUMBER_OBJ && arg2Symbol.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: second argument to '%s' must be Number, got %s", functionName, arg2Symbol.Type()))
	}

	return symbol.NewBasicSymbol(environment.NUMBER_OBJ)
}

// analyzeLogFunction checks the arity and type for 2-argument log functions, returning a STRING.
func (a *Analyzer) analyzeLogFunction(functionName string, n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 2 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 2 arguments for '%s', got %d", functionName, len(n.Arguments)))
		return symbol.AnySymbol()
	}

	arg1Symbol := a.analyze(n.Arguments[0])
	if arg1Symbol.Type() != environment.STRING_OBJ && arg1Symbol.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: first argument to '%s' must be String, got %s", functionName, arg1Symbol.Type()))
	}
	// The second argument can be anything, so we just analyze it without type checking
	_ = a.analyze(n.Arguments[1])

	return symbol.NewBasicSymbol(environment.STRING_OBJ)
}

// analyzeLogExportFunction checks the arity for the log.export function, returning ANY.
func (a *Analyzer) analyzeLogExportFunction(functionName string, n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 1 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 1 argument for '%s', got %d", functionName, len(n.Arguments)))
	} else {
		_ = a.analyze(n.Arguments[0])
	}
	return symbol.AnySymbol()
}

// analyzeMapContainsKeyFunction checks the arity and type for map.containsKey, returning a BOOLEAN.
func (a *Analyzer) analyzeMapContainsKeyFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 2 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 2 arguments for 'containsKey', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	mapSymbol := a.analyze(n.Arguments[0])
	keySymbol := a.analyze(n.Arguments[1])

	if mapSymbol.Type() != environment.MAP_OBJ && mapSymbol.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: first argument to 'containsKey' must be Map, got %s", mapSymbol.Type()))
	}

	if mapSym, ok := mapSymbol.(*symbol.MapSymbol); ok {
		if !mapSym.Key.Equals(keySymbol) && keySymbol.Type() != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: map index must be %s, got %s", mapSym.Key.Type(), keySymbol.Type()))
		}
	}

	return symbol.NewBasicSymbol(environment.BOOLEAN_OBJ)
}

// analyzeMapDeleteFunction checks the arity and type for map.delete, returning a MAP.
func (a *Analyzer) analyzeMapDeleteFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 2 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 2 arguments for 'delete', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	mapSymbol := a.analyze(n.Arguments[0])
	keySymbol := a.analyze(n.Arguments[1])

	if mapSymbol.Type() != environment.MAP_OBJ && mapSymbol.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: first argument to 'delete' must be Map, got %s", mapSymbol.Type()))
	}

	if mapSym, ok := mapSymbol.(*symbol.MapSymbol); ok {
		if !mapSym.Key.Equals(keySymbol) && keySymbol.Type() != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: map index must be %s, got %s", mapSym.Key.Type(), keySymbol.Type()))
		}
	}

	return mapSymbol
}

// analyzeCastFunction checks the arity and type for the builtin cast functions.
