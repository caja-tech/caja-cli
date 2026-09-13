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
			"len":   NewFunctionSymbol(moduleName, "len", []string{"arr"}, []string{"T"}, 1, []Symbol{arrTSym}, numSym),
			"push":  NewFunctionSymbol(moduleName, "push", []string{"arr", "item"}, []string{"T"}, 2, []Symbol{arrTSym, tSym}, arrTSym),
			"pop":   NewFunctionSymbol(moduleName, "pop", []string{"arr"}, []string{"T"}, 1, []Symbol{arrTSym}, arrTSym),
			"head":  NewFunctionSymbol(moduleName, "head", []string{"arr"}, []string{"T"}, 1, []Symbol{arrTSym}, tSym),
			"tail":  NewFunctionSymbol(moduleName, "tail", []string{"arr"}, []string{"T"}, 1, []Symbol{arrTSym}, arrTSym),
			"last":  NewFunctionSymbol(moduleName, "last", []string{"arr"}, []string{"T"}, 1, []Symbol{arrTSym}, tSym),
			"copy":  NewFunctionSymbol(moduleName, "copy", []string{"arr"}, []string{"T"}, 1, []Symbol{arrTSym}, arrTSym),
			"slice": NewFunctionSymbol(moduleName, "slice", []string{"arr", "start", "end"}, []string{"T"}, 3, []Symbol{arrTSym, numSym, numSym}, arrTSym),
			"join":  NewFunctionSymbol(moduleName, "join", []string{"arr", "other"}, []string{"T"}, 2, []Symbol{arrTSym, arrTSym}, arrTSym),
		}, nil, true

	case "date":
		return map[string]Symbol{
			"year":     NewBuiltinSymbol(moduleName, 1, "year(d: Date) -> Number", "d: Date"),
			"month":    NewBuiltinSymbol(moduleName, 1, "month(d: Date) -> Number", "d: Date"),
			"day":      NewBuiltinSymbol(moduleName, 1, "day(d: Date) -> Number", "d: Date"),
			"weekday":  NewBuiltinSymbol(moduleName, 1, "weekday(d: Date) -> Number", "d: Date"),
			"today":    NewBuiltinSymbol(moduleName, 0, "today() -> Date"),
			"parse":    NewBuiltinSymbol(moduleName, 1, "parse(dateStr: String) -> Date", "dateStr: String"),
			"addDays":  NewBuiltinSymbol(moduleName, 2, "addDays(d: Date, days: Number) -> Date", "d: Date", "days: Number"),
			"diffDays": NewBuiltinSymbol(moduleName, 2, "diffDays(start: Date, end: Date) -> Number", "start: Date", "end: Date"),
			"new":      NewBuiltinSymbol(moduleName, 3, "new(year: Number, month: Number, day: Number) -> Date", "year: Number", "month: Number", "day: Number"),
		}, nil, true

	case "string":
		return map[string]Symbol{
			"join":       NewBuiltinSymbol(moduleName, 2, "join(elements: Array<String>, separator: String) -> String", "elements: Array<String>", "separator: String"),
			"charAt":     NewBuiltinSymbol(moduleName, 2, "charAt(str: String, index: Number) -> String", "str: String", "index: Number"),
			"substring":  NewBuiltinSymbol(moduleName, 3, "substring(str: String, start: Number, end: Number) -> String", "str: String", "start: Number", "end: Number"),
			"concat":     NewBuiltinSymbol(moduleName, 2, "concat(str1: String, str2: String) -> String", "str1: String", "str2: String"),
			"split":      NewBuiltinSymbol(moduleName, 2, "split(str: String, separator: String) -> Array<String>", "str: String", "separator: String"),
			"contains":   NewBuiltinSymbol(moduleName, 2, "contains(str: String, search: String) -> Boolean", "str: String", "search: String"),
			"startsWith": NewBuiltinSymbol(moduleName, 2, "startsWith(str: String, prefix: String) -> Boolean", "str: String", "prefix: String"),
			"endsWith":   NewBuiltinSymbol(moduleName, 2, "endsWith(str: String, suffix: String) -> Boolean", "str: String", "suffix: String"),
			"replace":    NewBuiltinSymbol(moduleName, 3, "replace(str: String, search: String, replace: String) -> String", "str: String", "search: String", "replace: String"),
			"toUpper":    NewBuiltinSymbol(moduleName, 1, "toUpper(str: String) -> String", "str: String"),
			"toLower":    NewBuiltinSymbol(moduleName, 1, "toLower(str: String) -> String", "str: String"),
			"trim":       NewBuiltinSymbol(moduleName, 1, "trim(str: String) -> String", "str: String"),
			"len":        NewBuiltinSymbol(moduleName, 1, "len(str: String) -> Number", "str: String"),
			"format":     NewBuiltinSymbol(moduleName, 2, "format(fmt: String, value: Any) -> String", "fmt: String", "value: Any"),
		}, nil, true

	case "math":
		return map[string]Symbol{
			"abs":    NewBuiltinSymbol(moduleName, 1, "abs(num: Number) -> Number", "num: Number"),
			"sqrt":   NewBuiltinSymbol(moduleName, 1, "sqrt(num: Number) -> Number", "num: Number"),
			"pow":    NewBuiltinSymbol(moduleName, 2, "pow(base: Number, exp: Number) -> Number", "base: Number", "exp: Number"),
			"floor":  NewBuiltinSymbol(moduleName, 1, "floor(num: Number) -> Number", "num: Number"),
			"ceil":   NewBuiltinSymbol(moduleName, 1, "ceil(num: Number) -> Number", "num: Number"),
			"round":  NewBuiltinSymbol(moduleName, 1, "round(num: Number) -> Number", "num: Number"),
			"min":    NewBuiltinSymbol(moduleName, 2, "min(a: Number, b: Number) -> Number", "a: Number", "b: Number"),
			"max":    NewBuiltinSymbol(moduleName, 2, "max(a: Number, b: Number) -> Number", "a: Number", "b: Number"),
			"log":    NewBuiltinSymbol(moduleName, 2, "log(num: Number, base: Number) -> Number", "num: Number", "base: Number"),
			"rand":   NewBuiltinSymbol(moduleName, 0, "rand() -> Number"),
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
			"info":   NewBuiltinSymbol(moduleName, 2, "info(message: String, args: Any) -> Nil", "message: String", "args: Any"),
			"warn":   NewBuiltinSymbol(moduleName, 2, "warn(message: String, args: Any) -> Nil", "message: String", "args: Any"),
			"error":  NewBuiltinSymbol(moduleName, 2, "error(message: String, args: Any) -> Nil", "message: String", "args: Any"),
			"export": NewBuiltinSymbol(moduleName, 1, "export(data: Any) -> Nil", "data: Any"),
		}, nil, true

	case "map":
		return map[string]Symbol{
				"containsKey": NewBuiltinSymbol(moduleName, 2, "containsKey(map: Map, key: String) -> Boolean", "map: Map", "key: String"),
				"delete":      NewBuiltinSymbol(moduleName, 2, "delete(map: Map, key: String) -> Nil", "map: Map", "key: String"),
				"keys":        NewBuiltinSymbol(moduleName, 1, "keys(map: Map) -> Array", "map: Map"),
				"values":      NewBuiltinSymbol(moduleName, 1, "values(map: Map) -> Array", "map: Map"),
			}, map[string]Symbol{
				"KeyFunc": NewFunctionSymbol(moduleName, "KeyFunc", nil, nil, 0, nil, NewBasicSymbol(environment.STRING_OBJ)),
			}, true
	case "cast":
		return map[string]Symbol{
			"to": NewFunctionSymbol(moduleName, "to", []string{"value", "fallback"}, []string{"T", "R"}, 2, []Symbol{NewGenericSymbol("T"), NewGenericSymbol("R")}, NewGenericSymbol("R")),
		}, nil, true

	case "time":
		return map[string]Symbol{
				"now":            NewBuiltinSymbol(moduleName, 0, "now() -> Instant"),
				"sleep":          NewBuiltinSymbol(moduleName, 1, "sleep(d: Duration) -> Nothing", "d: Duration"),
				"since":          NewBuiltinSymbol(moduleName, 1, "since(t: Instant) -> Duration", "t: Instant"),
				"format":         NewBuiltinSymbol(moduleName, 2, "format(t: Instant, layout: String) -> String", "t: Instant", "layout: String"),
				"add":            NewBuiltinSymbol(moduleName, 2, "add(t: Instant, d: Duration) -> Instant", "t: Instant", "d: Duration"),
				"sub":            NewBuiltinSymbol(moduleName, 2, "sub(a: Instant, b: Instant) -> Duration", "a: Instant", "b: Instant"),
				"milliseconds":   NewBuiltinSymbol(moduleName, 1, "milliseconds(n: Number) -> Duration", "n: Number"),
				"seconds":        NewBuiltinSymbol(moduleName, 1, "seconds(n: Number) -> Duration", "n: Number"),
				"parse":          NewBuiltinSymbol(moduleName, 2, "parse(layout: String, value: String) -> Instant?", "layout: String", "value: String"),
				"parseDuration":  NewBuiltinSymbol(moduleName, 1, "parseDuration(s: String) -> Duration?", "s: String"),
				"unix":           NewBuiltinSymbol(moduleName, 2, "unix(sec: Number, nsec: Number) -> Instant", "sec: Number", "nsec: Number"),
				"unixSeconds":    NewBuiltinSymbol(moduleName, 1, "unixSeconds(t: Instant) -> Number", "t: Instant"),
				"unixMilli":      NewBuiltinSymbol(moduleName, 1, "unixMilli(t: Instant) -> Number", "t: Instant"),
				"before":         NewBuiltinSymbol(moduleName, 2, "before(a: Instant, b: Instant) -> Boolean", "a: Instant", "b: Instant"),
				"after":          NewBuiltinSymbol(moduleName, 2, "after(a: Instant, b: Instant) -> Boolean", "a: Instant", "b: Instant"),
				"equal":          NewBuiltinSymbol(moduleName, 2, "equal(a: Instant, b: Instant) -> Boolean", "a: Instant", "b: Instant"),
				"hour":           NewBuiltinSymbol(moduleName, 1, "hour(t: Instant) -> Number", "t: Instant"),
				"minute":         NewBuiltinSymbol(moduleName, 1, "minute(t: Instant) -> Number", "t: Instant"),
				"second":         NewBuiltinSymbol(moduleName, 1, "second(t: Instant) -> Number", "t: Instant"),
				"nanosecond":     NewBuiltinSymbol(moduleName, 1, "nanosecond(t: Instant) -> Number", "t: Instant"),
				"minutes":        NewBuiltinSymbol(moduleName, 1, "minutes(n: Number) -> Duration", "n: Number"),
				"hours":          NewBuiltinSymbol(moduleName, 1, "hours(n: Number) -> Duration", "n: Number"),
				"toMilliseconds": NewBuiltinSymbol(moduleName, 1, "toMilliseconds(d: Duration) -> Number", "d: Duration"),
				"toSeconds":      NewBuiltinSymbol(moduleName, 1, "toSeconds(d: Duration) -> Number", "d: Duration"),
				"toMinutes":      NewBuiltinSymbol(moduleName, 1, "toMinutes(d: Duration) -> Number", "d: Duration"),
				"toHours":        NewBuiltinSymbol(moduleName, 1, "toHours(d: Duration) -> Number", "d: Duration"),
			}, map[string]Symbol{
				"Instant":  NewBasicSymbol(environment.INSTANT_OBJ),
				"Duration": NewBasicSymbol(environment.DURATION_OBJ),
			}, true

	case "http":
		return getHTTPStandardModule(moduleName)
	case "doc":
		return map[string]Symbol{
			// write is the whole of the doc module for now: writes content
			// (a document — HTML, CSS, anything textual) to path, creating
			// parent directories as needed. Used by static-page projects to
			// produce their dist/ output when the compiled binary runs once
			// at `caja build` time. The module was renamed from `page` so the
			// name says what the call does (write a document) and leaves
			// "page" free to mean an actual page of a site.
			"write": NewBuiltinSymbol(moduleName, 2, "write(path: String, content: String) -> Nothing", "path: String", "content: String"),
		}, nil, true

	case "js":
		return map[string]Symbol{
				// raw is the only way to get a Script, and a Script is the only
				// thing @caja/ui will emit as JavaScript — so every script in a
				// page is forced through the validation below.
				//
				// Two things make this a builtin rather than an ordinary library
				// function. First, the argument must be a string LITERAL: the
				// analyzer reads it straight off the AST (see
				// analyzeJsRawFunction) and hands it to a real JavaScript parser,
				// which is only possible for source that exists at compile time.
				// Second, the validator has to run inside the caja binary — the
				// program caja emits is built as a stdlib-only module (go mod
				// init caja_build, CGO_ENABLED=0, no dependency resolution), so
				// generated code could never import a JS parser itself.
				//
				// The consequence worth stating plainly: a script assembled at
				// runtime by interpolation cannot be validated, and so cannot be
				// expressed here at all. That is the deliberate trade — an
				// unchecked script string is exactly the failure this module
				// exists to prevent.
				"raw": NewBuiltinSymbol(moduleName, 1, "raw(code: String) -> Script", "code: String"),
			}, map[string]Symbol{
				// Script is opaque and distinct from String on purpose: if it
				// were an alias, any String would satisfy a Script parameter and
				// the validation would be advisory rather than enforced. It
				// erases to a Go string at codegen (mapSymbolToGoType).
				"Script": NewBasicSymbol(environment.SCRIPT_OBJ),
			}, true

	}

	return nil, nil, false
}

// getHTTPStandardModule builds the "http" builtin module's exported symbols.
// Router is modeled as an opaque struct whose only Caja-visible members are
// function-typed fields (get/post/put/delete/patch/use) rather than
// "http.get(router, path, handler)" module-level calls: a call on a
// struct-field-of-function-type already transpiles correctly today via the
// generic PropertyExpression/CallExpression codegen path (it capitalizes
// "get" -> "Get" and emits "router.Get(...)"), so this needs no new
// dispatch logic in the compiler at all — see compiler/builtins.go's http
// case for the codegen half of this contract.
func getHTTPStandardModule(moduleName string) (map[string]Symbol, map[string]Symbol, bool) {
	numSym := NewBasicSymbol(environment.NUMBER_OBJ)
	strSym := NewBasicSymbol(environment.STRING_OBJ)
	strMapSym := NewMapSymbol(strSym, strSym)
	strArrMapSym := NewMapSymbol(strSym, NewArraySymbol(strSym))
	// A function returning nothing must resolve here as NULL_OBJ specifically
	// (not a nil ReturnType, and not the surface "Nothing" struct type Caja
	// uses for user-declared `-> Nothing` functions, which has a distinct
	// Type() of "Nothing" rather than "NULL") — matching log.info/map.delete's
	// convention. analyzeCallExpression's generic FunctionSymbol path falls
	// back to AnySymbol() when ReturnType() is nil, and Transpile only
	// suppresses printing/assigning a statement's result when its resolved
	// symbol's Type() is exactly NULL_OBJ; either mismatch would make
	// `caja run` try to print or assign a void call's result and fail to
	// compile the generated Go.
	nothingSym := NewBasicSymbol(environment.NULL_OBJ)

	requestSym := NewStructDefSymbol("Request", nil, map[string]StructFieldSymbol{
		"method":     {Type: strSym},
		"path":       {Type: strSym},
		"pathParams": {Type: strMapSym},
		"query":      {Type: strMapSym},
		"headers":    {Type: strMapSym},
		"body":       {Type: strSym},
		"ip":         {Type: strSym},
		"queryAll":   {Type: strArrMapSym},
		"headersAll": {Type: strArrMapSym},
	}, "")

	responseSym := NewStructDefSymbol("Response", nil, map[string]StructFieldSymbol{
		"status":  {Type: numSym},
		"headers": {Type: strMapSym},
		"body":    {Type: strSym},
	}, "")

	handlerSym := NewFunctionSymbol(moduleName, "Handler", []string{"req"}, nil, 1, []Symbol{requestSym}, responseSym)
	middlewareSym := NewFunctionSymbol(moduleName, "Middleware", []string{"next"}, nil, 1, []Symbol{handlerSym}, handlerSym)
	keyFuncSym := NewFunctionSymbol(moduleName, "keyFunc", []string{"req"}, nil, 1, []Symbol{requestSym}, strSym)

	routeRegister := func(name string) *FunctionSymbol {
		return NewFunctionSymbol(moduleName, name, []string{"path", "handler"}, nil, 2, []Symbol{strSym, handlerSym}, nothingSym)
	}

	routerSym := NewStructDefSymbol("Router", nil, map[string]StructFieldSymbol{
		"get":    {Type: routeRegister("get")},
		"post":   {Type: routeRegister("post")},
		"put":    {Type: routeRegister("put")},
		"delete": {Type: routeRegister("delete")},
		"patch":  {Type: routeRegister("patch")},
		"static": {Type: NewFunctionSymbol(moduleName, "static", []string{"prefix", "dir"}, nil, 2, []Symbol{strSym, strSym}, nothingSym)},
		"use":    {Type: NewFunctionSymbol(moduleName, "use", []string{"middleware"}, nil, 1, []Symbol{middlewareSym}, nothingSym)},
	}, "")

	// Client mirrors Router's own opaque-struct-with-function-fields design
	// (see the doc comment above this function) — get/post/put/delete/patch
	// are plain function-typed fields, so client.get(...) needs the exact
	// same zero-new-dispatch-code codegen path router.get(...) already gets.
	nullableResponseSym := &NullableSymbol{Underlying: responseSym}
	clientVerb := func(name string, hasBody bool) *FunctionSymbol {
		if hasBody {
			return NewFunctionSymbol(moduleName, name, []string{"endpoint", "body"}, nil, 2, []Symbol{strSym, strSym}, nullableResponseSym)
		}
		return NewFunctionSymbol(moduleName, name, []string{"endpoint"}, nil, 1, []Symbol{strSym}, nullableResponseSym)
	}

	clientSym := NewStructDefSymbol("Client", nil, map[string]StructFieldSymbol{
		"get":    {Type: clientVerb("get", false)},
		"post":   {Type: clientVerb("post", true)},
		"put":    {Type: clientVerb("put", true)},
		"delete": {Type: clientVerb("delete", false)},
		"patch":  {Type: clientVerb("patch", true)},
	}, "")

	return map[string]Symbol{
			"newRouter":              NewFunctionSymbol(moduleName, "newRouter", nil, nil, 0, nil, routerSym),
			"listen":                 NewFunctionSymbol(moduleName, "listen", []string{"router", "port"}, nil, 2, []Symbol{routerSym, numSym}, nothingSym),
			"ok":                     NewFunctionSymbol(moduleName, "ok", []string{"body"}, nil, 1, []Symbol{strSym}, responseSym),
			"text":                   NewFunctionSymbol(moduleName, "text", []string{"status", "body"}, nil, 2, []Symbol{numSym, strSym}, responseSym),
			"json":                   NewFunctionSymbol(moduleName, "json", []string{"status", "value"}, []string{"T"}, 2, []Symbol{numSym, NewGenericSymbol("T")}, responseSym),
			"parseJSON":              NewFunctionSymbol(moduleName, "parseJSON", []string{"body"}, nil, 1, []Symbol{strSym}, NewMapSymbol(strSym, AnySymbol())),
			"toJSON":                 NewFunctionSymbol(moduleName, "toJSON", []string{"value"}, []string{"T"}, 1, []Symbol{NewGenericSymbol("T")}, strSym),
			"notFound":               NewFunctionSymbol(moduleName, "notFound", []string{"body"}, nil, 1, []Symbol{strSym}, responseSym),
			"badRequest":             NewFunctionSymbol(moduleName, "badRequest", []string{"body"}, nil, 1, []Symbol{strSym}, responseSym),
			"serverError":            NewFunctionSymbol(moduleName, "serverError", []string{"body"}, nil, 1, []Symbol{strSym}, responseSym),
			"rateLimiter":            NewFunctionSymbol(moduleName, "rateLimiter", []string{"requestsPerSecond", "burst", "keyFunc"}, nil, 3, []Symbol{numSym, numSym, keyFuncSym}, middlewareSym),
			"concurrencyLimiter":     NewFunctionSymbol(moduleName, "concurrencyLimiter", []string{"maxConcurrent", "keyFunc"}, nil, 2, []Symbol{numSym, keyFuncSym}, middlewareSym),
			"distributedRateLimiter": NewFunctionSymbol(moduleName, "distributedRateLimiter", []string{"maxRequests", "windowSeconds", "keyFunc"}, nil, 3, []Symbol{numSym, numSym, keyFuncSym}, middlewareSym),
			"newClient":              NewFunctionSymbol(moduleName, "newClient", []string{"baseUrl", "defaultHeaders"}, nil, 2, []Symbol{strSym, strMapSym}, clientSym),
		}, map[string]Symbol{
			"Request":    requestSym,
			"Response":   responseSym,
			"Router":     routerSym,
			"Handler":    handlerSym,
			"Middleware": middlewareSym,
			"Client":     clientSym,
		}, true
}
