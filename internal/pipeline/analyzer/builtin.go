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
	case "log.info", "log.warn", "log.error":
		return a.analyzeLogFunction(functionName, n), true
	case "log.export":
		return a.analyzeLogExportFunction(functionName, n), true
	case "page.write":
		return a.analyzePageWriteFunction(n), true
	case "browser.log", "browser.alert":
		return a.analyzeBrowserStringArgFunction(functionName, n), true
	case "browser.getElementById", "browser.createElement":
		return a.analyzeBrowserStringArgToElementFunction(functionName, n), true
	case "browser.appendChild":
		return a.analyzeBrowserAppendChildFunction(n), true
	case "browser.insertBefore":
		return a.analyzeBrowserInsertBeforeFunction(n), true
	case "browser.removeElement", "browser.focus", "browser.blur":
		return a.analyzeBrowserElementArgFunction(functionName, n), true
	case "browser.setText", "browser.setHTML", "browser.removeAttribute", "browser.toggleClass":
		return a.analyzeBrowserSetElementContentFunction(functionName, n), true
	case "browser.getValue":
		return a.analyzeBrowserGetValueFunction(n), true
	case "browser.setValue":
		return a.analyzeBrowserSetElementContentFunction(functionName, n), true
	case "browser.getChecked":
		return a.analyzeBrowserGetCheckedFunction(n), true
	case "browser.setChecked":
		return a.analyzeBrowserSetCheckedFunction(n), true
	case "browser.querySelector":
		return a.analyzeBrowserQuerySelectorFunction(n), true
	case "browser.querySelectorAll":
		return a.analyzeBrowserQuerySelectorAllFunction(n), true
	case "browser.setAttribute", "browser.setStyle":
		return a.analyzeBrowserElementStringStringFunction(functionName, n), true
	case "browser.getAttribute":
		return a.analyzeBrowserGetAttributeFunction(n), true
	case "browser.addClass", "browser.removeClass":
		return a.analyzeBrowserSetElementContentFunction(functionName, n), true
	case "browser.hasClass":
		return a.analyzeBrowserHasClassFunction(n), true
	case "browser.on":
		return a.analyzeBrowserOnFunction(n), true
	case "browser.fetch":
		return a.analyzeBrowserFetchFunction(n), true
	case "browser.fetchThen":
		return a.analyzeBrowserFetchThenFunction(n), true
	case "browser.localStorageGet":
		return a.analyzeBrowserLocalStorageGetFunction(n), true
	case "browser.localStorageSet":
		return a.analyzeBrowserLocalStorageSetFunction(n), true
	case "browser.localStorageRemove":
		return a.analyzeBrowserStringArgFunction(functionName, n), true
	case "browser.setTimeout":
		return a.analyzeBrowserSetTimeoutFunction(n), true
	case "browser.clearTimeout":
		return a.analyzeBrowserClearTimeoutFunction(n), true
	default:
		return symbol.AnySymbol(), false

	}
}

// analyzePageWriteFunction checks the arity and type for page.write(path,
// content), which writes content to disk at path (creating parent
// directories as needed) when the compiled binary runs. Produces no value
// in the compiler, so this must return NULL_OBJ — see analyzeLogFunction's
// comment for why.
func (a *Analyzer) analyzePageWriteFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 2 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 2 arguments for 'write', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	pathSymbol := a.analyze(n.Arguments[0])
	if pathSymbol.Type() != environment.STRING_OBJ && pathSymbol.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: first argument to 'write' must be String, got %s", pathSymbol.Type()))
	}

	contentSymbol := a.analyze(n.Arguments[1])
	if contentSymbol.Type() != environment.STRING_OBJ && contentSymbol.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: second argument to 'write' must be String, got %s", contentSymbol.Type()))
	}

	return symbol.NewBasicSymbol(environment.NULL_OBJ)
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
	if arg1Symbol.Type() != environment.STRING_OBJ && arg1Symbol.Type() != environment.ANY_OBJ {
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

// checkBrowserArgNotNullable reports a type error and returns true if sym is
// a Nullable value (see symbol.NullableSymbol) — every browser.* argument
// check below calls this before its own Type() check, since
// NullableSymbol.Type() deliberately forwards to its underlying type
// (harmless for the struct-nullables it was originally built for, since
// Nullable<Struct> and Struct share the same Go pointer representation), so
// a plain `sym.Type() != X` check alone would silently accept e.g. an
// Element? wherever a plain Element is required. That's unsound: it compiles
// fine (Nullable<primitive>'s Go representation is a *T pointer per
// mapSymbolToGoType) but then either misbehaves or panics at runtime
// depending on how the receiving builtin's own codegen happens to use the
// parameter (a receiver-position call like el.Set(...) auto-dereferences a
// *js.Value; a plain function argument like caja_browser_get_attribute's el
// parameter does not) — confirmed by hitting exactly this as a real bug.
// Caja has no if-narrowing, so cast.to(value, fallback) (fixed alongside
// this to correctly dereference a Nullable input — see its codegen in
// builtins.go) is the only correct way to consume a Nullable value.
func (a *Analyzer) checkBrowserArgNotNullable(sym symbol.Symbol, n *ast.CallExpression, position, functionName, expectedType string) bool {
	if _, ok := sym.(*symbol.NullableSymbol); ok {
		a.reportError(n.Token, fmt.Sprintf("type error: %s argument to '%s' must be a non-nullable %s, got %s — use cast.to(value, fallback) to unwrap it first", position, functionName, expectedType, sym.String()))
		return true
	}
	return false
}

// checkBrowserArgType reports a type error unless sym is objType (or
// ANY_OBJ, the escape hatch for a value whose type couldn't be inferred) —
// every browser.* argument check above is exactly this "not Nullable, and
// matches this one ObjectType" shape, differing only in which position/
// function name/expected type/ObjectType they pass in, so this is the one
// place that shape is spelled out rather than duplicating the
// checkBrowserArgNotNullable-then-Type()-compare pair at each call site.
// checkBrowserArgNotNullable's own error takes priority (returning early)
// so a Nullable<Element> passed where Element is wanted gets the more
// specific "use cast.to" message instead of a second, less helpful one.
func (a *Analyzer) checkBrowserArgType(sym symbol.Symbol, n *ast.CallExpression, position, functionName, typeName string, objType environment.ObjectType) {
	if a.checkBrowserArgNotNullable(sym, n, position, functionName, typeName) {
		return
	}
	if sym.Type() != objType && sym.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: %s argument to '%s' must be %s, got %s", position, functionName, typeName, sym.Type()))
	}
}

// checkBrowserHandlerTakesNoArgs reports a type error if handlerSymbol isn't
// a zero-argument function — the shared shape of browser.on's and
// browser.setTimeout's handler argument (fetchThen's handler takes 1
// argument instead, so it checks its own arity/param type directly rather
// than using this).
func (a *Analyzer) checkBrowserHandlerTakesNoArgs(handlerSymbol symbol.Symbol, n *ast.CallExpression, position, functionName string) {
	if fnSymbol, ok := handlerSymbol.(*symbol.FunctionSymbol); ok {
		if fnSymbol.Arity() != 0 {
			a.reportError(n.Token, fmt.Sprintf("type error: %s argument to '%s' must be a function taking 0 arguments, got %s", position, functionName, handlerSymbol.String()))
		}
	} else if handlerSymbol.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: %s argument to '%s' must be a function, got %s", position, functionName, handlerSymbol.Type()))
	}
}

// analyzeBrowserStringArgFunction checks the arity and type for single-String-arg
// browser functions (log/alert). These produce no value in the compiler, so this
// must return NULL_OBJ — see analyzeLogFunction's comment for why.
func (a *Analyzer) analyzeBrowserStringArgFunction(functionName string, n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 1 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 1 arguments for '%s', got %d", functionName, len(n.Arguments)))
		return symbol.AnySymbol()
	}

	argSymbol := a.analyze(n.Arguments[0])
	a.checkBrowserArgType(argSymbol, n, "first", functionName, "String", environment.STRING_OBJ)

	return symbol.NewBasicSymbol(environment.NULL_OBJ)
}

// analyzeBrowserStringArgToElementFunction checks the arity and type for any
// browser function shaped (String) -> Element — getElementById (look up an
// existing node) and createElement (make a new, detached one) share this
// exact shape; the analyzer doesn't distinguish "found" from "created", only
// that a single String goes in and an Element comes out.
func (a *Analyzer) analyzeBrowserStringArgToElementFunction(functionName string, n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 1 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 1 arguments for '%s', got %d", functionName, len(n.Arguments)))
		return symbol.AnySymbol()
	}

	argSymbol := a.analyze(n.Arguments[0])
	a.checkBrowserArgType(argSymbol, n, "first", functionName, "String", environment.STRING_OBJ)

	return symbol.NewBasicSymbol(environment.ELEMENT_OBJ)
}

// analyzeBrowserElementArgFunction checks the arity and type for any browser
// function shaped (el: Element) -> Nothing — removeElement/focus/blur all
// share this exact shape. Produces no value in the compiler, so this must
// return NULL_OBJ — see analyzeLogFunction's comment for why.
func (a *Analyzer) analyzeBrowserElementArgFunction(functionName string, n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 1 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 1 arguments for '%s', got %d", functionName, len(n.Arguments)))
		return symbol.AnySymbol()
	}

	elSymbol := a.analyze(n.Arguments[0])
	a.checkBrowserArgType(elSymbol, n, "first", functionName, "Element", environment.ELEMENT_OBJ)

	return symbol.NewBasicSymbol(environment.NULL_OBJ)
}

// analyzeBrowserAppendChildFunction checks the arity and type for
// browser.appendChild, which attaches child as the last child of parent.
// Produces no value in the compiler, so this must return NULL_OBJ — see
// analyzeLogFunction's comment for why.
func (a *Analyzer) analyzeBrowserAppendChildFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 2 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 2 arguments for 'appendChild', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	parentSymbol := a.analyze(n.Arguments[0])
	a.checkBrowserArgType(parentSymbol, n, "first", "appendChild", "Element", environment.ELEMENT_OBJ)

	childSymbol := a.analyze(n.Arguments[1])
	a.checkBrowserArgType(childSymbol, n, "second", "appendChild", "Element", environment.ELEMENT_OBJ)

	return symbol.NewBasicSymbol(environment.NULL_OBJ)
}

// analyzeBrowserInsertBeforeFunction checks the arity and type for
// browser.insertBefore, which attaches newChild as a sibling of
// referenceChild, immediately before it — the first three-Element-argument
// browser builtin (setAttribute/setStyle are Element, String, String), so
// this is a dedicated function rather than reusing
// analyzeBrowserElementStringStringFunction. Produces no value in the
// compiler, so this must return NULL_OBJ — see analyzeLogFunction's comment
// for why.
func (a *Analyzer) analyzeBrowserInsertBeforeFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 3 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 3 arguments for 'insertBefore', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	names := []string{"first", "second", "third"}
	for i, position := range names {
		argSymbol := a.analyze(n.Arguments[i])
		a.checkBrowserArgType(argSymbol, n, position, "insertBefore", "Element", environment.ELEMENT_OBJ)
	}

	return symbol.NewBasicSymbol(environment.NULL_OBJ)
}

// analyzeBrowserGetValueFunction checks the arity and type for browser.getValue,
// which reads an input/textarea/select element's underlying form value,
// returning a String.
func (a *Analyzer) analyzeBrowserGetValueFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 1 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 1 arguments for 'getValue', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	elSymbol := a.analyze(n.Arguments[0])
	a.checkBrowserArgType(elSymbol, n, "first", "getValue", "Element", environment.ELEMENT_OBJ)

	return symbol.NewBasicSymbol(environment.STRING_OBJ)
}

// analyzeBrowserGetCheckedFunction checks the arity and type for
// browser.getChecked, which reads a checkbox/radio's live checked property —
// see symbol.GetStandardModule's "getChecked" entry for why this can't go
// through getAttribute.
func (a *Analyzer) analyzeBrowserGetCheckedFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 1 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 1 arguments for 'getChecked', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	elSymbol := a.analyze(n.Arguments[0])
	a.checkBrowserArgType(elSymbol, n, "first", "getChecked", "Element", environment.ELEMENT_OBJ)

	return symbol.NewBasicSymbol(environment.BOOLEAN_OBJ)
}

// analyzeBrowserSetCheckedFunction checks the arity and type for
// browser.setChecked, which writes a checkbox/radio's live checked property.
// Produces no value in the compiler, so this must return NULL_OBJ — see
// analyzeLogFunction's comment for why.
func (a *Analyzer) analyzeBrowserSetCheckedFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 2 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 2 arguments for 'setChecked', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	elSymbol := a.analyze(n.Arguments[0])
	a.checkBrowserArgType(elSymbol, n, "first", "setChecked", "Element", environment.ELEMENT_OBJ)

	valueSymbol := a.analyze(n.Arguments[1])
	a.checkBrowserArgType(valueSymbol, n, "second", "setChecked", "Boolean", environment.BOOLEAN_OBJ)

	return symbol.NewBasicSymbol(environment.NULL_OBJ)
}

// analyzeBrowserQuerySelectorFunction checks the arity and type for
// browser.querySelector, which returns the first matching Element or Nothing
// (Nullable) if no element matches — see GetStandardModule's "querySelector"
// entry for why this is the first builtin to return a Nullable type.
func (a *Analyzer) analyzeBrowserQuerySelectorFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 1 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 1 arguments for 'querySelector', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	selectorSymbol := a.analyze(n.Arguments[0])
	a.checkBrowserArgType(selectorSymbol, n, "first", "querySelector", "String", environment.STRING_OBJ)

	return &symbol.NullableSymbol{Underlying: symbol.NewBasicSymbol(environment.ELEMENT_OBJ)}
}

// analyzeBrowserQuerySelectorAllFunction checks the arity and type for
// browser.querySelectorAll, which returns every matching Element as an
// Array<Element> — not Nullable, since no match returns an empty array,
// matching JS's own querySelectorAll (never null).
func (a *Analyzer) analyzeBrowserQuerySelectorAllFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 1 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 1 arguments for 'querySelectorAll', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	selectorSymbol := a.analyze(n.Arguments[0])
	a.checkBrowserArgType(selectorSymbol, n, "first", "querySelectorAll", "String", environment.STRING_OBJ)

	return symbol.NewArraySymbol(symbol.NewBasicSymbol(environment.ELEMENT_OBJ))
}

// analyzeBrowserElementStringStringFunction checks the arity and type for any
// browser function shaped (el: Element, String, String) -> Nothing —
// setAttribute (name, value) and setStyle (property, value) share this exact
// shape, differing only in what the two Strings mean. Produces no value in
// the compiler, so this must return NULL_OBJ — see analyzeLogFunction's
// comment for why.
func (a *Analyzer) analyzeBrowserElementStringStringFunction(functionName string, n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 3 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 3 arguments for '%s', got %d", functionName, len(n.Arguments)))
		return symbol.AnySymbol()
	}

	elSymbol := a.analyze(n.Arguments[0])
	a.checkBrowserArgType(elSymbol, n, "first", functionName, "Element", environment.ELEMENT_OBJ)

	secondSymbol := a.analyze(n.Arguments[1])
	a.checkBrowserArgType(secondSymbol, n, "second", functionName, "String", environment.STRING_OBJ)

	thirdSymbol := a.analyze(n.Arguments[2])
	a.checkBrowserArgType(thirdSymbol, n, "third", functionName, "String", environment.STRING_OBJ)

	return symbol.NewBasicSymbol(environment.NULL_OBJ)
}

// analyzeBrowserGetAttributeFunction checks the arity and type for
// browser.getAttribute, which reads a named HTML attribute off an Element —
// Nullable (String?), since the attribute can legitimately be absent
// (distinct from being present but an empty string). See querySelector's
// comment in symbol.GetStandardModule for the same reasoning applied there.
func (a *Analyzer) analyzeBrowserGetAttributeFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 2 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 2 arguments for 'getAttribute', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	elSymbol := a.analyze(n.Arguments[0])
	a.checkBrowserArgType(elSymbol, n, "first", "getAttribute", "Element", environment.ELEMENT_OBJ)

	nameSymbol := a.analyze(n.Arguments[1])
	a.checkBrowserArgType(nameSymbol, n, "second", "getAttribute", "String", environment.STRING_OBJ)

	return &symbol.NullableSymbol{Underlying: symbol.NewBasicSymbol(environment.STRING_OBJ)}
}

// analyzeBrowserHasClassFunction checks the arity and type for
// browser.hasClass, which reports whether el currently has the named class —
// the query counterpart to addClass/removeClass/toggleClass.
func (a *Analyzer) analyzeBrowserHasClassFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 2 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 2 arguments for 'hasClass', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	elSymbol := a.analyze(n.Arguments[0])
	a.checkBrowserArgType(elSymbol, n, "first", "hasClass", "Element", environment.ELEMENT_OBJ)

	nameSymbol := a.analyze(n.Arguments[1])
	a.checkBrowserArgType(nameSymbol, n, "second", "hasClass", "String", environment.STRING_OBJ)

	return symbol.NewBasicSymbol(environment.BOOLEAN_OBJ)
}

// analyzeBrowserSetElementContentFunction checks the arity and type for any
// browser function shaped (el: Element, second: String) -> Nothing —
// setText/setHTML/setValue/addClass/removeClass/removeAttribute/toggleClass
// all share this exact shape, differing only in what the second String
// means (content, form value, class/attribute name). These produce no value
// in the compiler, so this must return NULL_OBJ — see analyzeLogFunction's
// comment for why.
func (a *Analyzer) analyzeBrowserSetElementContentFunction(functionName string, n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 2 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 2 arguments for '%s', got %d", functionName, len(n.Arguments)))
		return symbol.AnySymbol()
	}

	elSymbol := a.analyze(n.Arguments[0])
	a.checkBrowserArgType(elSymbol, n, "first", functionName, "Element", environment.ELEMENT_OBJ)

	contentSymbol := a.analyze(n.Arguments[1])
	a.checkBrowserArgType(contentSymbol, n, "second", functionName, "String", environment.STRING_OBJ)

	return symbol.NewBasicSymbol(environment.NULL_OBJ)
}

// analyzeBrowserOnFunction checks the arity and type for browser.on, which
// registers a zero-argument Caja function as a DOM event handler on an
// Element for the named event ("click", "input", "submit", ... — any string
// addEventListener accepts, since it's passed straight through with no
// Caja-side enumeration). The handler's return type isn't restricted (the
// compiled Go callback discards whatever it returns), only that it takes no
// arguments. Produces no value in the compiler, so this must return
// NULL_OBJ — see analyzeLogFunction's comment for why.
func (a *Analyzer) analyzeBrowserOnFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 3 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 3 arguments for 'on', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	eventSymbol := a.analyze(n.Arguments[0])
	a.checkBrowserArgType(eventSymbol, n, "first", "on", "String", environment.STRING_OBJ)

	elSymbol := a.analyze(n.Arguments[1])
	a.checkBrowserArgType(elSymbol, n, "second", "on", "Element", environment.ELEMENT_OBJ)

	handlerSymbol := a.analyze(n.Arguments[2])
	a.checkBrowserHandlerTakesNoArgs(handlerSymbol, n, "third", "on")

	return symbol.NewBasicSymbol(environment.NULL_OBJ)
}

// analyzeBrowserFetchFunction checks the arity and type for browser.fetch,
// which performs an HTTP GET request and returns the response body as a
// String — see GetStandardModule's "fetch" entry for why the return type is
// a plain String rather than a Response handle, and why no new analyzer
// work was needed to make it compose with async/unwrap.
//
// Restricted to top-level script statements (a.functionDepth == 0, the same
// check analyzeReturnStatement already uses for a different top-level-only
// rule): calling this — even wrapped in async+unwrap — from inside any
// function permanently freezes the whole page if that function ever runs as
// (or is called from) a browser.on handler, since a goroutine that
// blocks while nested under syscall/js.handleEvent can never be resumed (see
// fetchThen's comment for the full story). This is a blanket rule rather
// than trying to prove a given function is actually reachable from a
// handler — that analysis would be fragile and could miss cases, whereas
// "not inside any function" is a simple, sound, compile-time-checkable fact.
// Genuinely safe nested cases (like a bare `async browser.fetch(url)` never
// unwrapped) are also rejected by this same blanket rule; use fetchThen
// instead for any fetch that needs to happen from inside a function.
func (a *Analyzer) analyzeBrowserFetchFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 1 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 1 arguments for 'fetch', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	if a.functionDepth > 0 {
		a.reportError(n.Token, "semantic error: 'browser.fetch' can only be called at the top level of a script, not inside a function — it can permanently freeze the page if that function ever runs as (or from) a browser.on handler; use browser.fetchThen instead")
	}

	urlSymbol := a.analyze(n.Arguments[0])
	a.checkBrowserArgType(urlSymbol, n, "first", "fetch", "String", environment.STRING_OBJ)

	return symbol.NewBasicSymbol(environment.STRING_OBJ)
}

// analyzeBrowserFetchThenFunction checks the arity and type for
// browser.fetchThen, which performs an HTTP GET request and invokes handler
// with the response body once it resolves — see GetStandardModule's
// "fetchThen" entry for why this exists alongside plain fetch (it's the
// only safe way to consume a fetch's result from inside a browser.on
// handler). Produces no value in the compiler, so this must return
// NULL_OBJ — see analyzeLogFunction's comment for why.
func (a *Analyzer) analyzeBrowserFetchThenFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 2 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 2 arguments for 'fetchThen', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	urlSymbol := a.analyze(n.Arguments[0])
	a.checkBrowserArgType(urlSymbol, n, "first", "fetchThen", "String", environment.STRING_OBJ)

	handlerSymbol := a.analyze(n.Arguments[1])
	if fnSymbol, ok := handlerSymbol.(*symbol.FunctionSymbol); ok {
		if fnSymbol.Arity() != 1 {
			a.reportError(n.Token, fmt.Sprintf("type error: second argument to 'fetchThen' must be a function taking 1 argument, got %s", handlerSymbol.String()))
		} else if paramType := fnSymbol.ParamTypes()[0]; paramType.Type() != environment.STRING_OBJ && paramType.Type() != environment.ANY_OBJ {
			a.reportError(n.Token, fmt.Sprintf("type error: second argument to 'fetchThen' must be a function taking a String, got %s", handlerSymbol.String()))
		}
	} else if handlerSymbol.Type() != environment.ANY_OBJ {
		a.reportError(n.Token, fmt.Sprintf("type error: second argument to 'fetchThen' must be a function, got %s", handlerSymbol.Type()))
	}

	return symbol.NewBasicSymbol(environment.NULL_OBJ)
}

// analyzeBrowserLocalStorageGetFunction checks the arity and type for
// browser.localStorageGet, which reads a key from localStorage — Nullable
// (String?), since the key can legitimately have never been set (distinct
// from being set to an empty string). See querySelector's comment in
// symbol.GetStandardModule for the same reasoning applied there.
func (a *Analyzer) analyzeBrowserLocalStorageGetFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 1 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 1 arguments for 'localStorageGet', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	keySymbol := a.analyze(n.Arguments[0])
	a.checkBrowserArgType(keySymbol, n, "first", "localStorageGet", "String", environment.STRING_OBJ)

	return &symbol.NullableSymbol{Underlying: symbol.NewBasicSymbol(environment.STRING_OBJ)}
}

// analyzeBrowserLocalStorageSetFunction checks the arity and type for
// browser.localStorageSet, which writes a key/value pair to localStorage.
// Produces no value in the compiler, so this must return NULL_OBJ — see
// analyzeLogFunction's comment for why.
func (a *Analyzer) analyzeBrowserLocalStorageSetFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 2 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 2 arguments for 'localStorageSet', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	keySymbol := a.analyze(n.Arguments[0])
	a.checkBrowserArgType(keySymbol, n, "first", "localStorageSet", "String", environment.STRING_OBJ)

	valueSymbol := a.analyze(n.Arguments[1])
	a.checkBrowserArgType(valueSymbol, n, "second", "localStorageSet", "String", environment.STRING_OBJ)

	return symbol.NewBasicSymbol(environment.NULL_OBJ)
}

// analyzeBrowserSetTimeoutFunction checks the arity and type for
// browser.setTimeout, which schedules a zero-argument Caja function to run
// once after delayMs milliseconds, returning the timer id as a Number — see
// symbol.GetStandardModule's "setTimeout" entry for why a Number is enough
// and no new opaque handle type is needed. Shares its handler-arity-0 check
// with analyzeBrowserOnFunction via checkBrowserHandlerTakesNoArgs.
func (a *Analyzer) analyzeBrowserSetTimeoutFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 2 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 2 arguments for 'setTimeout', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	delaySymbol := a.analyze(n.Arguments[0])
	a.checkBrowserArgType(delaySymbol, n, "first", "setTimeout", "Number", environment.NUMBER_OBJ)

	handlerSymbol := a.analyze(n.Arguments[1])
	a.checkBrowserHandlerTakesNoArgs(handlerSymbol, n, "second", "setTimeout")

	return symbol.NewBasicSymbol(environment.NUMBER_OBJ)
}

// analyzeBrowserClearTimeoutFunction checks the arity and type for
// browser.clearTimeout, which cancels a timer previously started by
// setTimeout. Kept as a dedicated single-Number-argument function rather
// than a generalized helper — no second Number-only-argument browser
// builtin exists yet to justify generalizing (same "single-use shape stays
// dedicated" convention as the rest of this file). Produces no value in the
// compiler, so this must return NULL_OBJ — see analyzeLogFunction's comment
// for why.
func (a *Analyzer) analyzeBrowserClearTimeoutFunction(n *ast.CallExpression) symbol.Symbol {
	if len(n.Arguments) != 1 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 1 arguments for 'clearTimeout', got %d", len(n.Arguments)))
		return symbol.AnySymbol()
	}

	idSymbol := a.analyze(n.Arguments[0])
	a.checkBrowserArgType(idSymbol, n, "first", "clearTimeout", "Number", environment.NUMBER_OBJ)

	return symbol.NewBasicSymbol(environment.NULL_OBJ)
}

// analyzeCastFunction checks the arity and type for the builtin cast functions.
