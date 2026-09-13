package analyzer

import (
	"caja-cli/internal/pipeline/analyzer/symbol"
	"caja-cli/internal/pipeline/ast"
	"caja-cli/internal/pipeline/environment"
	"fmt"
)

// acceptsStringArg reports whether a symbol may be passed where a builtin
// declares a String parameter.
//
// Three things qualify. String itself, obviously. Any, because an
// unresolved type must not produce a cascade of errors from one unknown.
// And Script — validated JavaScript source, which genuinely IS text: it
// can be measured with string.len, searched with string.contains, or
// written straight to a .js file with doc.write, and refusing it here would
// be an arbitrary hole in a type that behaves like a String everywhere
// else (BasicSymbol.Equals already widens it for ordinary Caja functions
// and struct fields).
//
// The widening is one-directional, and stays that way: a plain String is
// still rejected wherever a Script is required, which is what forces every
// script through js.raw and therefore through the validator.
//
// Stated once here rather than inline at each call site — there are ~20 of
// them, and a new string-like type should not have to find them all.
func acceptsStringArg(s symbol.Symbol) bool {
	switch s.Type() {
	case environment.STRING_OBJ, environment.ANY_OBJ, environment.SCRIPT_OBJ:
		return true
	}
	return false
}

// analyzeBuiltinCall intercepts calls to builtin functions (like len, append, head, tail)
// to provide custom, compile-time polymorphic type-checking and inference.
func (a *Analyzer) analyzeBuiltinCall(moduleName string, functionName string, n *ast.CallExpression) (symbol.Symbol, bool) {
	fullName := functionName
	if moduleName != "" {
		fullName = moduleName + "." + functionName
	}

	switch fullName {
	case "js.raw":
		return a.analyzeJsRawFunction(n), true
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
	case "string.format":
		return a.analyzeStringFormatFunction(n), true
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
	case "time.now":
		return a.analyzeTimeNowFunction(n), true
	case "time.sleep":
		return a.analyzeTimeSleepFunction(n), true
	case "time.since":
		return a.analyzeTimeSinceFunction(n), true
	case "time.format":
		return a.analyzeTimeFormatFunction(n), true
	case "time.add":
		return a.analyzeTimeAddFunction(n), true
	case "time.sub":
		return a.analyzeTimeSubFunction(n), true
	case "time.milliseconds", "time.seconds", "time.minutes", "time.hours":
		return a.analyzeTimeDurationConstructorFunction(functionName, n), true
	case "time.parse":
		return a.analyzeTimeParseFunction(n), true
	case "time.parseDuration":
		return a.analyzeTimeParseDurationFunction(n), true
	case "time.unix":
		return a.analyzeTimeUnixFunction(n), true
	case "time.unixSeconds", "time.unixMilli", "time.hour", "time.minute", "time.second", "time.nanosecond":
		return a.analyzeTimeInstantToNumberFunction(functionName, n), true
	case "time.before", "time.after", "time.equal":
		return a.analyzeTimeInstantComparisonFunction(functionName, n), true
	case "time.toMilliseconds", "time.toSeconds", "time.toMinutes", "time.toHours":
		return a.analyzeTimeDurationToNumberFunction(functionName, n), true
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
	case "doc.write":
		return a.analyzeDocWriteFunction(n), true
	case "map.keys":
		return a.analyzeMapKeysFunction(n), true
	case "map.values":
		return a.analyzeMapValuesFunction(n), true
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

	if !acceptsStringArg(strSymbol) {
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

	if !acceptsStringArg(strSymbol) {
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

// analyzeDocWriteFunction checks the arity and type for doc.write(path,
// content), which writes content to disk at path (creating parent
// directories as needed) when the compiled binary runs. Produces no value
// in the compiler, so this must return NULL_OBJ — see analyzeLogFunction's
// comment for why.
func (a *Analyzer) analyzeDocWriteFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 2 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 2 arguments for 'write', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	pathSymbol := a.analyze(n.Arguments[0])
	if !acceptsStringArg(pathSymbol) {
		a.reportError(n.Token, fmt.Sprintf("type error: first argument to 'write' must be String, got %s", pathSymbol.Type()))
	}

	contentSymbol := a.analyze(n.Arguments[1])
	if !acceptsStringArg(contentSymbol) {
		a.reportError(n.Token, fmt.Sprintf("type error: second argument to 'write' must be String, got %s", contentSymbol.Type()))
	}

	return symbol.NewBasicSymbol(environment.NULL_OBJ)
}

// analyzeStringConcatFunction checks the arity and type for the builtin string 'concat' function, returning a STRING.
// analyzeStringFormatFunction checks arity for the builtin string 'format'
// function — printf-style formatting via Go's own fmt.Sprintf, mirroring
// log.info's own "first arg String, second arg Any" shape (the value being
// formatted can be any type, same as the arg log.info accepts). No
// compile-time validation of the format string's verbs against the value's
// type happens here — the compiler special-cases a literal format string to
// pick the right Go conversion (see transpileStringFormatCall); a
// runtime-computed format string can't be verb-checked at all, and is
// documented as a known limitation (Go's own fmt.Sprintf never panics on a
// verb/type mismatch, it just emits inline error text like "%!d(string=x)").
func (a *Analyzer) analyzeStringFormatFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 2 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 2 arguments for 'format', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	fmtSymbol := a.analyze(n.Arguments[0])
	if !acceptsStringArg(fmtSymbol) {
		a.reportError(n.Token, fmt.Sprintf("type error: first argument to 'format' must be String, got %s", fmtSymbol.Type()))
	}
	// The value being formatted can be any type, so it's analyzed (for
	// purity/move-semantics tracking) without a type restriction — the
	// same "second argument can be anything" shape as log.info.
	_ = a.analyze(n.Arguments[1])

	return symbol.NewBasicSymbol(environment.STRING_OBJ)
}

func (a *Analyzer) analyzeStringConcatFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 2 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 2 arguments for 'concat', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	str1Symbol := a.analyze(n.Arguments[0])
	str2Symbol := a.analyze(n.Arguments[1])

	if !acceptsStringArg(str1Symbol) {
		a.reportError(n.Token, fmt.Sprintf("type error: first argument to 'concat' must be String, got %s", str1Symbol.Type()))
	}
	if !acceptsStringArg(str2Symbol) {
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

	if !acceptsStringArg(strSymbol) {
		a.reportError(n.Token, fmt.Sprintf("type error: first argument to 'split' must be String, got %s", strSymbol.Type()))
	}
	if !acceptsStringArg(delimSymbol) {
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

	if !acceptsStringArg(strSymbol) {
		a.reportError(n.Token, fmt.Sprintf("type error: first argument to '%s' must be String, got %s", functionName, strSymbol.Type()))
	}
	if !acceptsStringArg(subSymbol) {
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

	if !acceptsStringArg(strSymbol) {
		a.reportError(n.Token, fmt.Sprintf("type error: first argument to 'replace' must be String, got %s", strSymbol.Type()))
	}
	if !acceptsStringArg(oldSymbol) {
		a.reportError(n.Token, fmt.Sprintf("type error: second argument to 'replace' must be String, got %s", oldSymbol.Type()))
	}
	if !acceptsStringArg(newSymbol) {
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

	if !acceptsStringArg(strSymbol) {
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

	if !acceptsStringArg(strSymbol) {
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
	} else if arrSymbol.Type() == environment.ARRAY_OBJ && arrSymbol.ElementSymbol() != nil && !acceptsStringArg(arrSymbol.ElementSymbol()) {
		a.reportError(n.Token, fmt.Sprintf("type error: array elements for 'join' must be String, got %s", arrSymbol.ElementSymbol().Type()))
	}

	if !acceptsStringArg(delimSymbol) {
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

	if !acceptsStringArg(strSymbol) {
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

// checkTimeArgNotNullable reports a type error and returns true if sym is a
// Nullable value — mirrors checkBrowserArgNotNullable exactly (see its doc
// comment for why NullableSymbol.Type() forwarding makes a plain Type()
// check unsound). Needed because time.parse (Instant?) is the first
// Nullable-producing function in this module; every time.* argument that
// expects a plain Instant/Duration must reject an unnarrowed Nullable here.
func (a *Analyzer) checkTimeArgNotNullable(sym symbol.Symbol, n *ast.CallExpression, position, functionName, expectedType string) bool {
	if _, ok := sym.(*symbol.NullableSymbol); ok {
		a.reportError(n.Token, fmt.Sprintf("type error: %s argument to '%s' must be a non-nullable %s, got %s — use cast.to(value, fallback) to unwrap it first", position, functionName, expectedType, sym.String()))
		return true
	}
	return false
}

// checkTimeArgType reports a type error unless sym is objType (or ANY_OBJ),
// after first rejecting a Nullable sym via checkTimeArgNotNullable — mirrors
// checkBrowserArgType.
func (a *Analyzer) checkTimeArgType(sym symbol.Symbol, n *ast.CallExpression, position, functionName, typeName string, objType environment.ObjectType) {
	if a.checkTimeArgNotNullable(sym, n, position, functionName, typeName) {
		return
	}
	if sym.Type() != objType && sym.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: %s argument to '%s' must be %s, got %s", position, functionName, typeName, sym.Type()))
	}
}

// analyzeTimeNowFunction checks the arity for the builtin time 'now' function, returning an INSTANT.
func (a *Analyzer) analyzeTimeNowFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 0 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 0 arguments for 'now', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	return symbol.NewBasicSymbol(environment.INSTANT_OBJ)
}

// analyzeTimeSleepFunction checks the arity and type for the builtin time 'sleep' function, returning NULL
// (Nothing's runtime representation for builtins — see the other -> Nothing builtins, e.g. analyzeLogFunction).
func (a *Analyzer) analyzeTimeSleepFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 1 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 1 arguments for 'sleep', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	durationSymbol := a.analyze(n.Arguments[0])
	a.checkTimeArgType(durationSymbol, n, "first", "sleep", "Duration", environment.DURATION_OBJ)

	return symbol.NewBasicSymbol(environment.NULL_OBJ)
}

// analyzeTimeSinceFunction checks the arity and type for the builtin time 'since' function, returning a DURATION.
func (a *Analyzer) analyzeTimeSinceFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 1 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 1 arguments for 'since', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	instantSymbol := a.analyze(n.Arguments[0])
	a.checkTimeArgType(instantSymbol, n, "first", "since", "Instant", environment.INSTANT_OBJ)

	return symbol.NewBasicSymbol(environment.DURATION_OBJ)
}

// analyzeTimeFormatFunction checks the arity and type for the builtin time 'format' function, returning a STRING.
func (a *Analyzer) analyzeTimeFormatFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 2 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 2 arguments for 'format', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	instantSymbol := a.analyze(n.Arguments[0])
	layoutSymbol := a.analyze(n.Arguments[1])
	a.checkTimeArgType(instantSymbol, n, "first", "format", "Instant", environment.INSTANT_OBJ)
	a.checkTimeArgType(layoutSymbol, n, "second", "format", "String", environment.STRING_OBJ)

	return symbol.NewBasicSymbol(environment.STRING_OBJ)
}

// analyzeTimeAddFunction checks the arity and type for the builtin time 'add' function, returning an INSTANT.
func (a *Analyzer) analyzeTimeAddFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 2 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 2 arguments for 'add', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	instantSymbol := a.analyze(n.Arguments[0])
	durationSymbol := a.analyze(n.Arguments[1])
	a.checkTimeArgType(instantSymbol, n, "first", "add", "Instant", environment.INSTANT_OBJ)
	a.checkTimeArgType(durationSymbol, n, "second", "add", "Duration", environment.DURATION_OBJ)

	return symbol.NewBasicSymbol(environment.INSTANT_OBJ)
}

// analyzeTimeSubFunction checks the arity and type for the builtin time 'sub' function, returning a DURATION.
func (a *Analyzer) analyzeTimeSubFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 2 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 2 arguments for 'sub', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	instant1Symbol := a.analyze(n.Arguments[0])
	instant2Symbol := a.analyze(n.Arguments[1])
	a.checkTimeArgType(instant1Symbol, n, "first", "sub", "Instant", environment.INSTANT_OBJ)
	a.checkTimeArgType(instant2Symbol, n, "second", "sub", "Instant", environment.INSTANT_OBJ)

	return symbol.NewBasicSymbol(environment.DURATION_OBJ)
}

// analyzeTimeDurationConstructorFunction checks the arity and type for the builtin time
// 'milliseconds'/'seconds'/'minutes'/'hours' functions, returning a DURATION.
func (a *Analyzer) analyzeTimeDurationConstructorFunction(functionName string, n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 1 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 1 arguments for '%s', got %d", functionName, len(n.Arguments)))
		return symbol.AnySymbol()
	}

	numSymbol := a.analyze(n.Arguments[0])
	a.checkTimeArgType(numSymbol, n, "first", functionName, "Number", environment.NUMBER_OBJ)

	return symbol.NewBasicSymbol(environment.DURATION_OBJ)
}

// analyzeTimeParseFunction checks the arity and type for the builtin time
// 'parse' function, returning an Instant? (Nullable) since an invalid
// layout/value pairing can fail to parse — same Nullable treatment as
// browser.querySelector (see analyzeBrowserQuerySelectorFunction).
func (a *Analyzer) analyzeTimeParseFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 2 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 2 arguments for 'parse', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	layoutSymbol := a.analyze(n.Arguments[0])
	valueSymbol := a.analyze(n.Arguments[1])
	a.checkTimeArgType(layoutSymbol, n, "first", "parse", "String", environment.STRING_OBJ)
	a.checkTimeArgType(valueSymbol, n, "second", "parse", "String", environment.STRING_OBJ)

	return &symbol.NullableSymbol{Underlying: symbol.NewBasicSymbol(environment.INSTANT_OBJ)}
}

// analyzeTimeParseDurationFunction checks the arity and type for the builtin
// time 'parseDuration' function, returning a Duration? (Nullable) since an
// unparseable duration string (e.g. missing unit, malformed number) can fail
// — same Nullable treatment as analyzeTimeParseFunction above.
func (a *Analyzer) analyzeTimeParseDurationFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 1 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 1 arguments for 'parseDuration', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	strSymbol := a.analyze(n.Arguments[0])
	a.checkTimeArgType(strSymbol, n, "first", "parseDuration", "String", environment.STRING_OBJ)

	return &symbol.NullableSymbol{Underlying: symbol.NewBasicSymbol(environment.DURATION_OBJ)}
}

// analyzeTimeUnixFunction checks the arity and type for the builtin time
// 'unix' function, returning an INSTANT.
func (a *Analyzer) analyzeTimeUnixFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 2 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 2 arguments for 'unix', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	secSymbol := a.analyze(n.Arguments[0])
	nsecSymbol := a.analyze(n.Arguments[1])
	a.checkTimeArgType(secSymbol, n, "first", "unix", "Number", environment.NUMBER_OBJ)
	a.checkTimeArgType(nsecSymbol, n, "second", "unix", "Number", environment.NUMBER_OBJ)

	return symbol.NewBasicSymbol(environment.INSTANT_OBJ)
}

// analyzeTimeInstantToNumberFunction checks the arity and type for the
// builtin time 'unixSeconds'/'unixMilli'/'hour'/'minute'/'second'/
// 'nanosecond' functions, all of which take a single Instant and return a
// NUMBER — one helper for all six, mirroring analyzeDateComponentFunction.
func (a *Analyzer) analyzeTimeInstantToNumberFunction(functionName string, n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 1 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 1 arguments for '%s', got %d", functionName, len(n.Arguments)))
		return symbol.AnySymbol()
	}

	instantSymbol := a.analyze(n.Arguments[0])
	a.checkTimeArgType(instantSymbol, n, "first", functionName, "Instant", environment.INSTANT_OBJ)

	return symbol.NewBasicSymbol(environment.NUMBER_OBJ)
}

// analyzeTimeInstantComparisonFunction checks the arity and type for the
// builtin time 'before'/'after'/'equal' functions, all of which take two
// Instants and return a BOOLEAN.
func (a *Analyzer) analyzeTimeInstantComparisonFunction(functionName string, n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 2 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 2 arguments for '%s', got %d", functionName, len(n.Arguments)))
		return symbol.AnySymbol()
	}

	aSymbol := a.analyze(n.Arguments[0])
	bSymbol := a.analyze(n.Arguments[1])
	a.checkTimeArgType(aSymbol, n, "first", functionName, "Instant", environment.INSTANT_OBJ)
	a.checkTimeArgType(bSymbol, n, "second", functionName, "Instant", environment.INSTANT_OBJ)

	return symbol.NewBasicSymbol(environment.BOOLEAN_OBJ)
}

// analyzeTimeDurationToNumberFunction checks the arity and type for the
// builtin time 'toMilliseconds'/'toSeconds'/'toMinutes'/'toHours' functions,
// all of which take a single Duration and return a NUMBER.
func (a *Analyzer) analyzeTimeDurationToNumberFunction(functionName string, n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 1 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 1 arguments for '%s', got %d", functionName, len(n.Arguments)))
		return symbol.AnySymbol()
	}

	durationSymbol := a.analyze(n.Arguments[0])
	a.checkTimeArgType(durationSymbol, n, "first", functionName, "Duration", environment.DURATION_OBJ)

	return symbol.NewBasicSymbol(environment.NUMBER_OBJ)
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

// analyzeLogFunction checks the arity and type for 2-argument log functions
// (info/warn/error). These produce no value in the compiler (the only
// backend that still runs — see GetStandardModule's "-> Nil" label for
// these), so this must return NULL_OBJ: it previously returned STRING_OBJ,
// which made `caja run`'s print-last-statement feature wrongly try to treat
// a trailing `log.info(...)` as a value, generating Go that assigned a
// void call's result — a compile error.
func (a *Analyzer) analyzeLogFunction(functionName string, n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 2 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 2 arguments for '%s', got %d", functionName, len(n.Arguments)))
		return symbol.AnySymbol()
	}

	arg1Symbol := a.analyze(n.Arguments[0])
	if !acceptsStringArg(arg1Symbol) {
		a.reportError(n.Token, fmt.Sprintf("type error: first argument to '%s' must be String, got %s", functionName, arg1Symbol.Type()))
	}
	// The second argument can be anything, so we just analyze it without type checking
	_ = a.analyze(n.Arguments[1])

	return symbol.NewBasicSymbol(environment.NULL_OBJ)
}

// analyzeLogExportFunction checks the arity for the log.export function.
// Produces no value in the compiler (see analyzeLogFunction's comment —
// same "-> Nil" label, same reasoning); previously returned AnySymbol(),
// which isn't NULL_OBJ either and hit the identical print-result bug.
func (a *Analyzer) analyzeLogExportFunction(functionName string, n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 1 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 1 argument for '%s', got %d", functionName, len(n.Arguments)))
	} else {
		_ = a.analyze(n.Arguments[0])
	}
	return symbol.NewBasicSymbol(environment.NULL_OBJ)
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

// analyzeMapKeysFunction checks the arity and type for map.keys, returning an Array of the map's key type.
func (a *Analyzer) analyzeMapKeysFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 1 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 1 argument for 'keys', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	mapSymbol := a.analyze(n.Arguments[0])

	if mapSymbol.Type() != environment.MAP_OBJ && mapSymbol.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: argument to 'keys' must be Map, got %s", mapSymbol.Type()))
	}

	if mapSym, ok := mapSymbol.(*symbol.MapSymbol); ok {
		return symbol.NewArraySymbol(mapSym.Key)
	}
	return symbol.NewArraySymbol(symbol.AnySymbol())
}

// analyzeMapValuesFunction checks the arity and type for map.values, returning an Array of the map's value type.
func (a *Analyzer) analyzeMapValuesFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 1 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 1 argument for 'values', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	mapSymbol := a.analyze(n.Arguments[0])

	if mapSymbol.Type() != environment.MAP_OBJ && mapSymbol.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: argument to 'values' must be Map, got %s", mapSymbol.Type()))
	}

	if mapSym, ok := mapSymbol.(*symbol.MapSymbol); ok {
		return symbol.NewArraySymbol(mapSym.Value)
	}
	return symbol.NewArraySymbol(symbol.AnySymbol())
}

// analyzeCastFunction checks the arity and type for the builtin cast functions.
