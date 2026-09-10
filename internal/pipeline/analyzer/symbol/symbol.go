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
			}, map[string]Symbol{
				"KeyFunc": NewFunctionSymbol(moduleName, "KeyFunc", nil, nil, 0, nil, NewBasicSymbol(environment.STRING_OBJ)),
			}, true
	case "cast":
		return map[string]Symbol{
			"to": NewFunctionSymbol(moduleName, "to", []string{"value", "fallback"}, []string{"T", "R"}, 2, []Symbol{NewGenericSymbol("T"), NewGenericSymbol("R")}, NewGenericSymbol("R")),
		}, nil, true

	case "http":
		return getHTTPStandardModule(moduleName)
	case "browser":
		return map[string]Symbol{
				"log":            NewBuiltinSymbol(moduleName, 1, "log(message: String) -> Nothing", "message: String"),
				"alert":          NewBuiltinSymbol(moduleName, 1, "alert(message: String) -> Nothing", "message: String"),
				"getElementById": NewBuiltinSymbol(moduleName, 1, "getElementById(id: String) -> Element", "id: String"),
				// createElement makes a new, detached Element (document.
				// createElement) — it isn't attached to the page until passed
				// to appendChild. Not nullable: unlike querySelector this
				// never "fails to find" anything, it always produces a fresh
				// node.
				"createElement": NewBuiltinSymbol(moduleName, 1, "createElement(tag: String) -> Element", "tag: String"),
				// appendChild attaches child as the last child of parent —
				// the way a detached createElement result actually joins the
				// page, and also the standard way to move an already-attached
				// element elsewhere in the tree (matching JS's own
				// appendChild, which reparents rather than erroring if child
				// already has a parent).
				"appendChild": NewBuiltinSymbol(moduleName, 2, "appendChild(parent: Element, child: Element) -> Nothing", "parent: Element", "child: Element"),
				// insertBefore attaches newChild as a sibling of
				// referenceChild, immediately before it — the way to insert
				// anywhere but the end (appendChild only ever adds last),
				// matching DOM's own Node.insertBefore(newChild,
				// referenceChild) called on parent.
				"insertBefore": NewBuiltinSymbol(moduleName, 3, "insertBefore(parent: Element, newChild: Element, referenceChild: Element) -> Nothing", "parent: Element", "newChild: Element", "referenceChild: Element"),
				// removeElement detaches el from wherever it currently lives
				// in the tree (Element.remove()) — a no-op if el is already
				// detached, matching remove()'s own forgiving behavior.
				"removeElement": NewBuiltinSymbol(moduleName, 1, "removeElement(el: Element) -> Nothing", "el: Element"),
				// focus/blur move keyboard focus into or out of el — expected
				// by any input/modal/dropdown component.
				"focus":   NewBuiltinSymbol(moduleName, 1, "focus(el: Element) -> Nothing", "el: Element"),
				"blur":    NewBuiltinSymbol(moduleName, 1, "blur(el: Element) -> Nothing", "el: Element"),
				"setText": NewBuiltinSymbol(moduleName, 2, "setText(el: Element, text: String) -> Nothing", "el: Element", "text: String"),
				"setHTML": NewBuiltinSymbol(moduleName, 2, "setHTML(el: Element, html: String) -> Nothing", "el: Element", "html: String"),
				// setStyle sets one inline CSS property via
				// style.setProperty, which — unlike assigning el.style.foo
				// directly — accepts standard kebab-case CSS property names
				// ("background-color", "font-size") rather than requiring
				// camelCase, matching how design-system code actually writes
				// CSS property names.
				"setStyle": NewBuiltinSymbol(moduleName, 3, "setStyle(el: Element, property: String, value: String) -> Nothing", "el: Element", "property: String", "value: String"),
				// el.value — reads/writes the underlying form value of an
				// input/textarea/select element. Not nullable: reading .value
				// off an element that doesn't have one just returns JS ""/
				// undefined (matching .value's own forgiving behavior), never
				// null, so there's nothing to model as Nullable here.
				"getValue": NewBuiltinSymbol(moduleName, 1, "getValue(el: Element) -> String", "el: Element"),
				"setValue": NewBuiltinSymbol(moduleName, 2, "setValue(el: Element, value: String) -> Nothing", "el: Element", "value: String"),
				// checked is a live DOM property, not an attribute — once a
				// user clicks a checkbox/radio, its "checked" HTML attribute
				// still only reflects the *default* state, not what's
				// actually checked now. Same attribute-vs-property split
				// that's why getValue/setValue exist separately from
				// getAttribute/setAttribute.
				"getChecked": NewBuiltinSymbol(moduleName, 1, "getChecked(el: Element) -> Boolean", "el: Element"),
				"setChecked": NewBuiltinSymbol(moduleName, 2, "setChecked(el: Element, value: Boolean) -> Nothing", "el: Element", "value: Boolean"),
				// querySelector can legitimately find nothing (no match),
				// unlike getElementById which is only ever used with a known
				// ID — so unlike the rest of this module so far, this uses
				// Caja's existing Nullable mechanism (Type?, e.g. already used
				// for struct types) rather than a non-nullable zero value.
				// querySelectorAll is NOT nullable: like JS's own
				// querySelectorAll, no match returns an empty Array<Element>,
				// never null.
				"querySelector":    NewBuiltinSymbol(moduleName, 1, "querySelector(selector: String) -> Element?", "selector: String"),
				"querySelectorAll": NewBuiltinSymbol(moduleName, 1, "querySelectorAll(selector: String) -> Array<Element>", "selector: String"),
				"setAttribute":     NewBuiltinSymbol(moduleName, 3, "setAttribute(el: Element, name: String, value: String) -> Nothing", "el: Element", "name: String", "value: String"),
				// getAttribute can legitimately find nothing (the attribute
				// isn't present) — same Nullable treatment as querySelector,
				// distinguishing "absent" from "present but empty string".
				"getAttribute": NewBuiltinSymbol(moduleName, 2, "getAttribute(el: Element, name: String) -> String?", "el: Element", "name: String"),
				// removeAttribute is setAttribute's counterpart for fully
				// clearing a boolean/data attribute (disabled, aria-expanded,
				// data-open) — setting one to "" leaves it present (and
				// therefore still "true" for boolean attributes), only
				// removeAttribute actually makes hasAttribute false again.
				"removeAttribute": NewBuiltinSymbol(moduleName, 2, "removeAttribute(el: Element, name: String) -> Nothing", "el: Element", "name: String"),
				"addClass":        NewBuiltinSymbol(moduleName, 2, "addClass(el: Element, name: String) -> Nothing", "el: Element", "name: String"),
				"removeClass":     NewBuiltinSymbol(moduleName, 2, "removeClass(el: Element, name: String) -> Nothing", "el: Element", "name: String"),
				"toggleClass":     NewBuiltinSymbol(moduleName, 2, "toggleClass(el: Element, name: String) -> Nothing", "el: Element", "name: String"),
				"hasClass":        NewBuiltinSymbol(moduleName, 2, "hasClass(el: Element, name: String) -> Boolean", "el: Element", "name: String"),
				// Centralized event registration: covers "click", "input",
				// "submit", "keydown", etc. with zero Caja-side enumeration,
				// since the event name is just passed straight through to
				// JS's addEventListener. Replaces a former per-event
				// browser.onClick builtin entirely rather than keeping both.
				//
				// Deliberately out of scope: the handler still can't access
				// the event object (event.preventDefault(), event.target,
				// key codes, ...) — browser.on("submit", ...) won't stop a
				// real page navigation yet. That needs a new opaque Event
				// type plus accessors, a distinct, larger feature from
				// "centralize the event name as a string."
				"on": NewBuiltinSymbol(moduleName, 3, "on(event: String, el: Element, handler: fn() -> Nothing) -> Nothing", "event: String", "el: Element", "handler: fn() -> Nothing"),
				// GET-only, returns the body as String — no Response type
				// here yet to carry status/headers separately. Works with
				// async/unwrap for free at the top level of a program: it's
				// an ordinary builtin whose *generated Go
				// code* blocks on a channel bridging the JS fetch Promise, so
				// `async browser.fetch(u)` runs it concurrently via the same
				// machinery every other async expression already uses.
				//
				// DO NOT call this (even via async+unwrap) from inside a
				// browser.on handler — confirmed via a minimal isolated
				// repro that doing so permanently freezes the entire page,
				// not just that call: every js.FuncOf callback (event
				// listeners, and this fetch's own then/catch) is dispatched
				// through syscall/js.handleEvent, which is not async-aware
				// the way wasm_exec.js's top-level run() is, so a goroutine
				// that blocks while still nested under a handleEvent call can
				// never be resumed. Use fetchThen instead inside a handler —
				// it never blocks, so it works from anywhere.
				"fetch": NewBuiltinSymbol(moduleName, 1, "fetch(url: String) -> String", "url: String"),
				// The safe way to fetch-and-use-a-result from inside a
				// browser.on handler (see fetch's comment above for
				// why plain fetch, even async+unwrap, cannot be): purely
				// callback-driven, so nothing ever blocks — handler runs once
				// the fetch resolves, wherever fetchThen itself was called
				// from. Panics (network failure only, matching fetch's own
				// semantics) on the same goroutine the callback runs on,
				// formatted the same clean way as any other Caja panic (see
				// caja_wrap_callback).
				"fetchThen": NewBuiltinSymbol(moduleName, 2, "fetchThen(url: String, handler: fn(body: String) -> Nothing) -> Nothing", "url: String", "handler: fn(body: String) -> Nothing"),
				// localStorageGet can legitimately find nothing (the key was
				// never set) — same Nullable treatment as querySelector/
				// getAttribute, distinguishing "never set" from "set to empty
				// string".
				"localStorageGet":    NewBuiltinSymbol(moduleName, 1, "localStorageGet(key: String) -> String?", "key: String"),
				"localStorageSet":    NewBuiltinSymbol(moduleName, 2, "localStorageSet(key: String, value: String) -> Nothing", "key: String", "value: String"),
				"localStorageRemove": NewBuiltinSymbol(moduleName, 1, "localStorageRemove(key: String) -> Nothing", "key: String"),
				// setTimeout returns the timer id as a Number — JS's own
				// return type for setTimeout, and enough to round-trip into
				// clearTimeout with no new opaque handle type needed.
				// delayMs comes first (matching JS's argument meaning) with
				// handler last, consistent with on's event-then-el-then-
				// handler ordering putting the callback last. The handler is
				// dispatched through the same caja_wrap_callback machinery
				// as on's listener, so it works from anywhere (including
				// inside a browser.on handler) without the deadlock risk
				// plain fetch has.
				"setTimeout":   NewBuiltinSymbol(moduleName, 2, "setTimeout(delayMs: Number, handler: fn() -> Nothing) -> Number", "delayMs: Number", "handler: fn() -> Nothing"),
				"clearTimeout": NewBuiltinSymbol(moduleName, 1, "clearTimeout(id: Number) -> Nothing", "id: Number"),
			}, map[string]Symbol{
				"Element": NewBasicSymbol(environment.ELEMENT_OBJ),
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
