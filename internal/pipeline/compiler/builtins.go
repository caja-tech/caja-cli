package compiler

import (
	"bytes"
	"caja-cli/internal/pipeline/analyzer/symbol"
	"caja-cli/internal/pipeline/ast"
	"fmt"
	"strings"
)

var builtinModules = map[string]bool{
	"array":   true,
	"date":    true,
	"string":  true,
	"math":    true,
	"log":     true,
	"map":     true,
	"cast":    true,
	"browser": true,
}

// UsesBrowserModule reports whether transpiled Go source came from a Caja
// program that imports the browser module. The browser module compiles to
// syscall/js calls, which only build under GOOS=js/GOARCH=wasm — callers use
// this to auto-select that target instead of surfacing a raw Go build error.
func UsesBrowserModule(goSource string) bool {
	return strings.Contains(goSource, "\"syscall/js\"")
}

// isOwned reports whether n is a freshly-produced, definitely-unaliased
// temporary — a call result, a safe-pipe or stream-pipe result (both always
// build a brand new cajaArray, even if individual elements inside aren't
// themselves fresh), a struct/array/map literal, or an explicit `move` —
// safe to hand off without a defensive copy (array.push/array.pop's owned
// fast path) or without marking a value shared (maybeShareValue). This is
// also what gives `move` real meaning: `let b = move a` skips
// maybeShareValue's cajaShare wrap entirely, so b aliases a's pointer
// unmarked — a real, if unenforced, ownership handoff.
func isOwned(n ast.Expression) bool {
	if prefix, ok := n.(*ast.PrefixExpression); ok && prefix.Operator == "move" {
		return true
	}
	switch e := n.(type) {
	case *ast.CallExpression, *ast.SafePipeExpression, *ast.StreamPipeExpression, *ast.StructLiteral, *ast.ArrayLiteral, *ast.MapLiteral:
		return true
	case *ast.IsExpression:
		// A union `is` narrowing extracts the same underlying pointer its
		// source already holds — it's only unaliased if the source itself
		// was (e.g. `move a is Cat`, or narrowing a freshly-returned union
		// value straight off a call), not merely because narrowing is
		// itself a distinct expression shape.
		return isOwned(e.Left)
	}
	return false
}

func transpileBuiltinCall(module string, fn string, args []ast.Expression, ctx *transpileContext) (string, error) {
	ctx.usedModules[module] = true

	var argStrs []string
	for _, arg := range args {
		s, err := transpileExpression(arg, ctx, "")
		if err != nil {
			return "", err
		}
		argStrs = append(argStrs, s)
	}

	switch module {
	case "math":
		if fn == "log" {
			return fmt.Sprintf("(math.Log(%s) / math.Log(%s))", argStrs[0], argStrs[1]), nil
		}
		if fn == "rand" {
			ctx.usedModules["math_rand"] = true
			return "rand.Float64()", nil
		}
		return fmt.Sprintf("math.%s(%s)", strings.Title(fn), strings.Join(argStrs, ", ")), nil

	case "string":
		ctx.usedModules["strings"] = true
		switch fn {
		case "len":
			ctx.usedModules["utf8"] = true
			return fmt.Sprintf("float64(utf8.RuneCountInString(%s))", argStrs[0]), nil
		case "charAt":
			return fmt.Sprintf("string([]rune(%s)[int(%s)])", argStrs[0], argStrs[1]), nil
		case "substring":
			return fmt.Sprintf("string([]rune(%s)[int(%s):int(%s)])", argStrs[0], argStrs[1], argStrs[2]), nil
		case "concat":
			return fmt.Sprintf("(%s + %s)", argStrs[0], argStrs[1]), nil
		case "split":
			ctx.usedModules["cow_array"] = true
			return fmt.Sprintf("&cajaArray[string]{Data: strings.Split(%s, %s)}", argStrs[0], argStrs[1]), nil
		case "contains":
			return fmt.Sprintf("strings.Contains(%s, %s)", argStrs[0], argStrs[1]), nil
		case "startsWith":
			return fmt.Sprintf("strings.HasPrefix(%s, %s)", argStrs[0], argStrs[1]), nil
		case "endsWith":
			return fmt.Sprintf("strings.HasSuffix(%s, %s)", argStrs[0], argStrs[1]), nil
		case "replace":
			return fmt.Sprintf("strings.ReplaceAll(%s, %s, %s)", argStrs[0], argStrs[1], argStrs[2]), nil
		case "toUpper":
			return fmt.Sprintf("strings.ToUpper(%s)", argStrs[0]), nil
		case "toLower":
			return fmt.Sprintf("strings.ToLower(%s)", argStrs[0]), nil
		case "trim":
			return fmt.Sprintf("strings.TrimSpace(%s)", argStrs[0]), nil
		case "join":
			return fmt.Sprintf("strings.Join(%s.Data, %s)", argStrs[0], argStrs[1]), nil
		}

	case "array":
		ctx.usedModules["cow_array"] = true
		switch fn {
		case "len":
			return fmt.Sprintf("float64(len(%s.Data))", argStrs[0]), nil
		case "head":
			return fmt.Sprintf("%s.Data[0]", argStrs[0]), nil
		case "last":
			return fmt.Sprintf("%s.Data[len(%s.Data)-1]", argStrs[0], argStrs[0]), nil
		case "push":
			if isOwned(args[0]) {
				return fmt.Sprintf("caja_array_push_owned(%s, %s)", argStrs[0], argStrs[1]), nil
			}
			return fmt.Sprintf("caja_array_push(%s, %s)", argStrs[0], argStrs[1]), nil
		case "pop":
			if isOwned(args[0]) {
				return fmt.Sprintf("caja_array_pop_owned(%s)", argStrs[0]), nil
			}
			return fmt.Sprintf("caja_array_pop(%s)", argStrs[0]), nil
		default:
			return fmt.Sprintf("caja_array_%s(%s)", fn, strings.Join(argStrs, ", ")), nil
		}

	case "date":
		ctx.usedModules["time"] = true
		switch fn {
		case "today":
			return "caja_date_today()", nil
		case "new":
			return fmt.Sprintf("time.Date(int(%s), time.Month(%s), int(%s), 0, 0, 0, 0, time.UTC)", argStrs[0], argStrs[1], argStrs[2]), nil
		case "addDays":
			return fmt.Sprintf("%s.AddDate(0, 0, int(%s))", argStrs[0], argStrs[1]), nil
		case "diffDays":
			return fmt.Sprintf("float64(%s.Sub(%s).Hours() / 24)", argStrs[1], argStrs[0]), nil
		case "parse":
			return fmt.Sprintf("parseDate(%s)", argStrs[0]), nil
		case "year":
			return fmt.Sprintf("float64(%s.Year())", argStrs[0]), nil
		case "month":
			return fmt.Sprintf("float64(%s.Month())", argStrs[0]), nil
		case "day":
			return fmt.Sprintf("float64(%s.Day())", argStrs[0]), nil
		case "weekday":
			return fmt.Sprintf("float64(%s.Weekday())", argStrs[0]), nil
		}

	case "map":
		ctx.usedModules["cow_map"] = true
		switch fn {
		case "containsKey":
			return fmt.Sprintf("caja_map_containsKey(%s, %s)", argStrs[0], argStrs[1]), nil
		case "delete":
			return fmt.Sprintf("caja_map_delete(%s, %s)", argStrs[0], argStrs[1]), nil
		}

	case "log":
		switch fn {
		case "export":
			ctx.usedModules["log_export"] = true
			ctx.usedModules["json"] = true
			ctx.usedModules["csv"] = true
			enableValueFormatting(ctx)
			return fmt.Sprintf("caja_log_export(%s)", argStrs[0]), nil
		case "info", "warn", "error":
			ctx.usedModules["log_"+fn] = true
			enableValueFormatting(ctx)
			return fmt.Sprintf("caja_log_%s(%s, %s)", fn, argStrs[0], argStrs[1]), nil
		}

	case "cast":
		if fn == "to" {
			ctx.usedModules["fmt"] = true

			fallbackSym, _ := ctx.analyzer.GetSymbol(args[1])
			fallbackType := ""
			switch args[1].(type) {
			case *ast.StringLiteral:
				fallbackType = "String"
			case *ast.NumberLiteral:
				fallbackType = "Number"
			case *ast.BooleanLiteral:
				fallbackType = "Boolean"
			default:
				if fallbackSym != nil {
					fallbackType = string(fallbackSym.Type())
				}
			}

			// inputExpr is the Go expression cast.to's logic below treats as
			// "the plain input value" — normally just the argument itself,
			// but a Nullable input (e.g. String? from browser.getAttribute)
			// is actually a *T pointer in Go (see mapSymbolToGoType's
			// NullableSymbol case), which none of that logic understands: it
			// would either get forwarded raw into fmt.Sprintf("%v", ...)
			// (printing a pointer address, not the value) or returned
			// directly as cast.to's own result (a type mismatch/runtime
			// panic at the call site, since callers expect the plain,
			// non-pointer type). So for a Nullable input, inputExpr becomes
			// a local "v" holding the dereferenced value, and the entire
			// result computed below gets wrapped in a nil-check that falls
			// straight through to the fallback when the pointer is nil —
			// exactly like every other "couldn't produce a value" case here
			// (an unparseable string, etc.) already does.
			inputSym, _ := ctx.analyzer.GetSymbol(args[0])
			inputType := ""
			inputExpr := argStrs[0]
			var nullableUnderlyingGoType string
			if inputSym != nil {
				if nullableSym, ok := inputSym.(*symbol.NullableSymbol); ok {
					nullableUnderlyingGoType = ctx.mapSymbolToGoType(nullableSym.Underlying)
					inputType = string(nullableSym.Underlying.Type())
					inputExpr = "v"
				} else {
					inputType = string(inputSym.Type())
				}
			}

			// Generate the value string representation based on input type
			valStr := fmt.Sprintf("fmt.Sprintf(\"%%v\", %s)", inputExpr)
			if inputType == "Date" {
				valStr = fmt.Sprintf("%s.Format(\"2006-01-02\")", inputExpr)
			} else if inputType == "String" {
				valStr = inputExpr
			}

			var result string
			switch fallbackType {
			case "String":
				result = valStr
			case "Number":
				if inputType == "Number" {
					result = inputExpr
				} else {
					ctx.usedModules["strconv"] = true
					result = fmt.Sprintf("func() float64 { v, err := strconv.ParseFloat(%s, 64); if err != nil { return %s }; return v }()", valStr, argStrs[1])
				}
			case "Boolean":
				if inputType == "Boolean" {
					result = inputExpr
				} else {
					ctx.usedModules["strconv"] = true
					result = fmt.Sprintf("func() bool { v, err := strconv.ParseBool(%s); if err != nil { return %s }; return v }()", valStr, argStrs[1])
				}
			case "Date":
				if inputType == "Date" {
					result = inputExpr
				} else {
					ctx.usedModules["time"] = true
					result = fmt.Sprintf("func() time.Time { v, err := time.Parse(\"2006-01-02\", %s); if err != nil { return %s }; return v }()", valStr, argStrs[1])
				}
			default:
				result = inputExpr
			}

			if nullableUnderlyingGoType != "" {
				outputGoType := "any"
				if fallbackSym != nil {
					if t := ctx.mapSymbolToGoType(fallbackSym); t != "" {
						outputGoType = t
					}
				}
				return fmt.Sprintf("func() %s { p := %s; if p == nil { return %s }; v := *p; return %s }()", outputGoType, argStrs[0], argStrs[1], result), nil
			}
			return result, nil
		}

	case "browser":
		ctx.usedModules["syscall/js"] = true
		switch fn {
		case "log":
			return fmt.Sprintf("js.Global().Get(\"console\").Call(\"log\", %s)", argStrs[0]), nil
		case "alert":
			return fmt.Sprintf("js.Global().Call(\"alert\", %s)", argStrs[0]), nil
		case "getElementById":
			return fmt.Sprintf("js.Global().Get(\"document\").Call(\"getElementById\", %s)", argStrs[0]), nil
		case "createElement":
			return fmt.Sprintf("js.Global().Get(\"document\").Call(\"createElement\", %s)", argStrs[0]), nil
		case "appendChild":
			return fmt.Sprintf("%s.Call(\"appendChild\", %s)", argStrs[0], argStrs[1]), nil
		case "insertBefore":
			return fmt.Sprintf("%s.Call(\"insertBefore\", %s, %s)", argStrs[0], argStrs[1], argStrs[2]), nil
		case "removeElement":
			return fmt.Sprintf("%s.Call(\"remove\")", argStrs[0]), nil
		case "focus":
			return fmt.Sprintf("%s.Call(\"focus\")", argStrs[0]), nil
		case "blur":
			return fmt.Sprintf("%s.Call(\"blur\")", argStrs[0]), nil
		case "setStyle":
			return fmt.Sprintf("%s.Get(\"style\").Call(\"setProperty\", %s, %s)", argStrs[0], argStrs[1], argStrs[2]), nil
		case "setText":
			return fmt.Sprintf("%s.Set(\"textContent\", %s)", argStrs[0], argStrs[1]), nil
		case "setHTML":
			return fmt.Sprintf("%s.Set(\"innerHTML\", %s)", argStrs[0], argStrs[1]), nil
		case "getValue":
			return fmt.Sprintf("%s.Get(\"value\").String()", argStrs[0]), nil
		case "setValue":
			return fmt.Sprintf("%s.Set(\"value\", %s)", argStrs[0], argStrs[1]), nil
		case "getChecked":
			return fmt.Sprintf("%s.Get(\"checked\").Bool()", argStrs[0]), nil
		case "setChecked":
			return fmt.Sprintf("%s.Set(\"checked\", %s)", argStrs[0], argStrs[1]), nil
		case "querySelector":
			// caja_browser_query_selector returns *js.Value (nil for "no
			// match"), matching Element?'s Go type — see mapSymbolToGoType's
			// NullableSymbol case.
			ctx.usedModules["browser_query_selector"] = true
			return fmt.Sprintf("caja_browser_query_selector(%s)", argStrs[0]), nil
		case "querySelectorAll":
			// caja_browser_query_selector_all returns *cajaArray[js.Value],
			// matching Array<Element>'s Go type.
			ctx.usedModules["browser_query_selector_all"] = true
			ctx.usedModules["cow_array"] = true
			return fmt.Sprintf("caja_browser_query_selector_all(%s)", argStrs[0]), nil
		case "setAttribute":
			return fmt.Sprintf("%s.Call(\"setAttribute\", %s, %s)", argStrs[0], argStrs[1], argStrs[2]), nil
		case "getAttribute":
			// caja_browser_get_attribute returns *string (nil for "attribute
			// absent"), matching String?'s Go type — see mapSymbolToGoType's
			// NullableSymbol case, same representation querySelector uses.
			ctx.usedModules["browser_get_attribute"] = true
			return fmt.Sprintf("caja_browser_get_attribute(%s, %s)", argStrs[0], argStrs[1]), nil
		case "removeAttribute":
			return fmt.Sprintf("%s.Call(\"removeAttribute\", %s)", argStrs[0], argStrs[1]), nil
		case "addClass":
			return fmt.Sprintf("%s.Get(\"classList\").Call(\"add\", %s)", argStrs[0], argStrs[1]), nil
		case "removeClass":
			return fmt.Sprintf("%s.Get(\"classList\").Call(\"remove\", %s)", argStrs[0], argStrs[1]), nil
		case "toggleClass":
			return fmt.Sprintf("%s.Get(\"classList\").Call(\"toggle\", %s)", argStrs[0], argStrs[1]), nil
		case "hasClass":
			return fmt.Sprintf("%s.Get(\"classList\").Call(\"contains\", %s).Bool()", argStrs[0], argStrs[1]), nil
		case "on":
			// A listener registered via js.FuncOf only keeps firing as long
			// as the wasm instance's Go runtime is still scheduling, which
			// requires main() to never return — see the time.Sleep keep-alive
			// loop emitted at the end of main() below, gated on
			// ctx.usedModules["syscall/js"] (i.e. on any browser-module usage
			// at all, not specifically on "on" — see that comment for why,
			// including why it's a sleep loop and not a bare select {}). The
			// event name is passed straight through to addEventListener, so
			// this one case covers "click"/"input"/"submit"/etc. with no
			// Caja-side enumeration.
			//
			// The handler call is wrapped in caja_wrap_callback because this
			// goroutine is dispatched via syscall/js.handleEvent, invisible to
			// main()'s own deferred recover — an unrecovered panic in the
			// handler would otherwise crash with a raw Go stack trace instead
			// of our normal caja_panic_location()-formatted message.
			ctx.usedModules["browser_wrap_callback"] = true
			return fmt.Sprintf("%s.Call(\"addEventListener\", %s, js.FuncOf(func(this js.Value, args []js.Value) any {\ncaja_wrap_callback(func() { %s() })\nreturn nil\n}))", argStrs[1], argStrs[0], argStrs[2]), nil
		case "fetch":
			// caja_browser_fetch (injectBuiltinDependencies) is an ordinary
			// blocking Go function from the caller's point of view — it
			// bridges the JS fetch Promise onto a channel internally, so a
			// plain call here already works synchronously; wrapping the call
			// in `async` (unmodified Caja syntax, see AsyncExpression) is
			// what makes it run concurrently, for free. DO NOT call this from
			// inside a browser.on handler — see GetStandardModule's
			// "fetch" comment for why that permanently freezes the page; use
			// fetchThen there instead.
			ctx.usedModules["browser_fetch"] = true
			return fmt.Sprintf("caja_browser_fetch(%s)", argStrs[0]), nil
		case "fetchThen":
			// caja_browser_fetch_then is purely callback-driven (no blocking
			// anywhere), which is what makes it safe to call from inside a
			// browser.on handler where plain fetch is not — see
			// GetStandardModule's "fetchThen" comment.
			ctx.usedModules["browser_fetch_then"] = true
			ctx.usedModules["browser_wrap_callback"] = true
			return fmt.Sprintf("caja_browser_fetch_then(%s, func(body string) { %s(body) })", argStrs[0], argStrs[1]), nil
		case "localStorageGet":
			// caja_browser_local_storage_get returns *string (nil for "key
			// never set"), matching String?'s Go type — same representation
			// getAttribute uses.
			ctx.usedModules["browser_local_storage_get"] = true
			return fmt.Sprintf("caja_browser_local_storage_get(%s)", argStrs[0]), nil
		case "localStorageSet":
			return fmt.Sprintf("js.Global().Get(\"localStorage\").Call(\"setItem\", %s, %s)", argStrs[0], argStrs[1]), nil
		case "localStorageRemove":
			return fmt.Sprintf("js.Global().Get(\"localStorage\").Call(\"removeItem\", %s)", argStrs[0]), nil
		case "setTimeout":
			// The handler is wrapped in caja_wrap_callback for the same
			// reason on's listener is — dispatched via syscall/js.handleEvent,
			// invisible to main()'s own deferred recover. JS's own argument
			// order is (callback, delay), so argStrs[1] (handler) and
			// argStrs[0] (delayMs) are swapped going into Call. The returned
			// timer id comes back as a JS number, read out via .Float() to
			// match Number's Go type (float64).
			ctx.usedModules["browser_wrap_callback"] = true
			return fmt.Sprintf("js.Global().Call(\"setTimeout\", js.FuncOf(func(this js.Value, args []js.Value) any {\ncaja_wrap_callback(func() { %s() })\nreturn nil\n}), %s).Float()", argStrs[1], argStrs[0]), nil
		case "clearTimeout":
			return fmt.Sprintf("js.Global().Call(\"clearTimeout\", %s)", argStrs[0]), nil
		}
	}

	return "", fmt.Errorf("unsupported builtin %s.%s", module, fn)
}

func injectBuiltinDependencies(ctx *transpileContext, buf *bytes.Buffer) {
	// caja_panic_location is always injected: main()'s recover handler always
	// calls it (see Transpile). Go preserves the full panicking goroutine's
	// stack across a deferred recover, so runtime.Callers here still sees the
	// frame where the panic actually originated, not just main()'s own frame.
	// The `//line` directives emitted per-statement rewrite each frame's
	// reported File/Line to the original .caja source, so the first frame
	// found here ending in ".caja" is the statement that panicked.
	buf.WriteString(`
func caja_panic_location() string {
	pcs := make([]uintptr, 64)
	n := runtime.Callers(0, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if strings.HasSuffix(frame.File, ".caja") {
			return fmt.Sprintf("%s:%d", frame.File, frame.Line)
		}
		if !more {
			break
		}
	}
	return ""
}

// cajaContainer lets caja_format_value/caja_memo_hash/caja_flush_export
// treat a copy-on-write array/map wrapper the same as the bare Go slice/map
// it holds, by unwrapping to the raw data before their reflect.Slice/
// reflect.Map handling runs, rather than needing their own reflect.Struct
// special case for the wrapper type itself. Always injected (like
// caja_panic_location above) since those three functions reference it
// unconditionally, regardless of whether this particular program uses any
// arrays/maps.
type cajaContainer interface {
	cajaRawData() any
}
`)

	if ctx.usedModules["async_panic_guard"] {
		buf.WriteString(`
// A panic inside a spawned goroutine (stream-pipe stage, join call, async
// task) can't be caught by main()'s own recover -- Go only propagates
// panic/recover within the same goroutine, so an unrecovered goroutine panic
// crashes the whole process directly. Each such goroutine instead recovers
// locally and records the failure here (first panic wins; the program is
// aborting either way, so later ones are dropped) rather than letting the
// process crash mid-dump or silently finishing with a truncated/wrong
// result -- closing a channel for cleanup after a recovered panic looks
// identical to "no more items" to a downstream consumer, so the synchronous
// code waiting on that result (a pipe's collector loop, unwrap, await, or
// main() itself as a last-resort check for a fire-and-forget task) must
// explicitly check caja_check_async_panic before trusting what it received.
var cajaAsyncPanicMu sync.Mutex
var cajaAsyncPanicSet bool
var cajaAsyncPanicMsg string

func caja_report_async_panic(r any) {
	cajaAsyncPanicMu.Lock()
	defer cajaAsyncPanicMu.Unlock()
	if cajaAsyncPanicSet {
		return
	}
	cajaAsyncPanicSet = true
	if loc := caja_panic_location(); loc != "" {
		cajaAsyncPanicMsg = fmt.Sprintf("%v\n    at %s", r, loc)
	} else {
		cajaAsyncPanicMsg = fmt.Sprintf("%v", r)
	}
}

func caja_check_async_panic() {
	cajaAsyncPanicMu.Lock()
	set, msg := cajaAsyncPanicSet, cajaAsyncPanicMsg
	cajaAsyncPanicMu.Unlock()
	if set {
		fmt.Fprintln(os.Stderr, "error:", msg)
		os.Exit(1)
	}
}
`)
	}

	if ctx.usedModules["cow_shared"] || ctx.usedModules["cow_array"] || ctx.usedModules["cow_map"] {
		buf.WriteString(`
// cajaSharable/cajaShare back copy-on-write for structs, arrays, and maps:
// every generated struct type (see the struct-literal codegen) and both
// generic container wrappers below have cajaSetShared/cajaClone methods, so
// a single generic helper works uniformly across all of them. Go generics
// preserve the concrete type through instantiation, so cajaShare(p) on a
// *Point returns a *Point, not this interface.
type cajaSharable interface {
	cajaSetShared()
}

func cajaShare[T cajaSharable](v T) T {
	v.cajaSetShared()
	return v
}
`)
	}

	if ctx.usedModules["active_cell"] {
		buf.WriteString(`
// cajaReactiveNode is the non-generic identity every cajaActive[T] also
// implements, used purely for topological batch-update bookkeeping in
// cajaPropagate — separate from its typed Get/Set API. Mirrors
// cajaContainer's existing pattern for the same reason: a graph-walking
// algorithm needs one concrete type to hold heterogeneous *cajaActive[T]
// instances (e.g. *cajaActive[float64] alongside *cajaActive[string]) in a
// single slice.
type cajaReactiveNode interface {
	cajaDependents() []cajaReactiveNode
	cajaRecompute()
}

// cajaActive is the runtime representation of an 'active' variable — a
// mutable, observable cell.
type cajaActive[T any] struct {
	value      T
	dependents []cajaReactiveNode
	recompute  func() // nil for a source active variable, never a reactive-call result
}

func newCajaActive[T any](initial T) *cajaActive[T] {
	return &cajaActive[T]{value: initial}
}

func (c *cajaActive[T]) Get() T { return c.value }

// Set is the entry point for a genuine source write (an ordinary Caja
// assignment to an active variable) — it starts one complete, synchronous
// propagation batch. Never called from inside a recompute closure itself
// (see cajaSetFromRecompute) — a recompute already runs as part of an
// in-progress batch, and calling Set from within one would re-enter
// cajaPropagate and reintroduce the exact "diamond" glitch this design
// exists to fix (a shared downstream consumer of two changed dependencies
// recomputing twice, transiently observing one dependency updated and the
// other still stale).
func (c *cajaActive[T]) Set(v T) {
	c.value = v
	cajaPropagate(c)
}

// cajaSetFromRecompute updates value without starting a new propagation
// batch. Used only by a reactive call's own recompute closure (see
// transpileReactiveCallExpression), which runs as one step of an
// already-in-progress batch.
func (c *cajaActive[T]) cajaSetFromRecompute(v T) { c.value = v }

func (c *cajaActive[T]) cajaDependents() []cajaReactiveNode { return c.dependents }

func (c *cajaActive[T]) cajaRecompute() {
	if c.recompute != nil {
		c.recompute()
	}
}

func (c *cajaActive[T]) addDependent(n cajaReactiveNode) {
	c.dependents = append(c.dependents, n)
}

// cajaPropagate runs one synchronous topological batch update over every
// node transitively reachable from root, so a downstream consumer of
// multiple changed dependencies (a "diamond": b = f(react a), c = g(react
// a), d = h(react b, react c)) recomputes exactly once, only after every
// one of its own dependencies within this batch has already settled —
// fixing the glitch where cascading each edge immediately let d observe a
// torn state (b updated, c still stale) and recompute twice. Standard DFS
// post-order topological sort over the "dependents" edges: cycles are
// structurally impossible here (Caja requires declaring a variable before
// it can be `+"`react`"+`-ed, so the dependency graph is always a DAG by
// construction), so no cycle detection is needed. Still fully synchronous
// and single-threaded, same call stack as the triggering assignment —
// goroutine-based propagation was rejected instead, verified via go
// test -race to produce real data races on this exact unsynchronized
// state (Caja's function purity restricts capturing outer Caja variables,
// not concurrent access to this runtime-introduced graph state).
func cajaPropagate(root cajaReactiveNode) {
	visited := map[cajaReactiveNode]bool{root: true}
	var order []cajaReactiveNode
	var visit func(cajaReactiveNode)
	visit = func(n cajaReactiveNode) {
		for _, dep := range n.cajaDependents() {
			if !visited[dep] {
				visited[dep] = true
				visit(dep)
			}
		}
		order = append(order, n)
	}
	visit(root)
	// order is root's dependents in reverse-topological (post) order, with
	// root itself appended last; root's own value is already set by Set,
	// so walk the rest backwards for correct forward-topological order.
	for i := len(order) - 2; i >= 0; i-- {
		order[i].cajaRecompute()
	}
}
`)
	}

	if ctx.usedModules["cow_array"] {
		buf.WriteString(`
type cajaArray[T any] struct {
	Data       []T
	cajaShared bool
}

// cajaSetShared cascades into every element: cloning this array later only
// gives it a fresh backing slice (see cajaClone) -- the elements themselves
// are unchanged pointers, still reachable through whichever other binding
// this array was aliased from, so an element-typed struct/array/map must be
// marked shared too, or mutating through it later would bypass copy-on-write
// entirely. any(elem) lets a generic T be probed for cajaSharable at
// runtime; primitive element types (float64, string, bool) simply don't
// implement it and are skipped.
func (a *cajaArray[T]) cajaSetShared() {
	if a == nil {
		return
	}
	a.cajaShared = true
	for _, elem := range a.Data {
		if s, ok := any(elem).(cajaSharable); ok {
			s.cajaSetShared()
		}
	}
}

func (a *cajaArray[T]) cajaClone() *cajaArray[T] {
	if a == nil {
		return nil
	}
	newData := make([]T, len(a.Data))
	copy(newData, a.Data)
	return &cajaArray[T]{Data: newData}
}

func (a *cajaArray[T]) cajaRawData() any {
	return a.Data
}

func caja_array_push[T any](arr *cajaArray[T], item T) *cajaArray[T] {
	newData := append([]T{}, arr.Data...)
	return &cajaArray[T]{Data: append(newData, item)}
}
func caja_array_push_owned[T any](arr *cajaArray[T], item T) *cajaArray[T] {
	arr.Data = append(arr.Data, item)
	return arr
}
func caja_array_pop[T any](arr *cajaArray[T]) *cajaArray[T] {
	if len(arr.Data) == 0 {
		return arr
	}
	newData := append([]T{}, arr.Data...)
	return &cajaArray[T]{Data: newData[:len(newData)-1]}
}
func caja_array_pop_owned[T any](arr *cajaArray[T]) *cajaArray[T] {
	if len(arr.Data) == 0 {
		return arr
	}
	arr.Data = arr.Data[:len(arr.Data)-1]
	return arr
}
func caja_array_tail[T any](arr *cajaArray[T]) *cajaArray[T] {
	if len(arr.Data) <= 1 {
		return &cajaArray[T]{}
	}
	newData := append([]T{}, arr.Data...)
	return &cajaArray[T]{Data: newData[1:]}
}
func caja_array_copy[T any](arr *cajaArray[T]) *cajaArray[T] {
	newData := make([]T, len(arr.Data))
	copy(newData, arr.Data)
	return &cajaArray[T]{Data: newData}
}
func caja_array_slice[T any](arr *cajaArray[T], start float64, end float64) *cajaArray[T] {
	if int(start) < 0 || int(end) > len(arr.Data) || int(start) > int(end) {
		return &cajaArray[T]{}
	}
	newData := append([]T{}, arr.Data...)
	return &cajaArray[T]{Data: newData[int(start):int(end)]}
}
func caja_array_join[T any](arr *cajaArray[T], other *cajaArray[T]) *cajaArray[T] {
	newData := append([]T{}, arr.Data...)
	return &cajaArray[T]{Data: append(newData, other.Data...)}
}
`)
	}

	if ctx.usedModules["cow_map"] {
		buf.WriteString(`
type cajaMap[K comparable, V any] struct {
	Data       map[K]V
	cajaShared bool
}

// cajaSetShared cascades into every value, for the same reason
// cajaArray.cajaSetShared does: cloning this map later only gives it a fresh
// backing map (see cajaClone) -- the values themselves are unchanged
// pointers, still reachable through whichever other binding this map was
// aliased from.
func (m *cajaMap[K, V]) cajaSetShared() {
	if m == nil {
		return
	}
	m.cajaShared = true
	for _, v := range m.Data {
		if s, ok := any(v).(cajaSharable); ok {
			s.cajaSetShared()
		}
	}
}

// cajaClone's shallow copy also duplicates any struct-instance key's Key()
// closure byte-for-byte (function pointer + captured environment), so a
// cloned map's keys hash identically to the original's unless a Key()
// closure captured something by reference that later diverges independently
// of the clone -- not a pattern caja's own value semantics otherwise produce.
func (m *cajaMap[K, V]) cajaClone() *cajaMap[K, V] {
	if m == nil {
		return nil
	}
	newData := make(map[K]V, len(m.Data))
	for k, v := range m.Data {
		newData[k] = v
	}
	return &cajaMap[K, V]{Data: newData}
}

func (m *cajaMap[K, V]) cajaRawData() any {
	return m.Data
}

func caja_map_containsKey[K comparable, V any](m *cajaMap[K, V], key K) bool {
	_, ok := m.Data[key]
	return ok
}
func caja_map_delete[K comparable, V any](m *cajaMap[K, V], key K) *cajaMap[K, V] {
	newData := make(map[K]V, len(m.Data))
	for k, v := range m.Data {
		newData[k] = v
	}
	delete(newData, key)
	return &cajaMap[K, V]{Data: newData}
}
`)
	}

	if ctx.usedModules["time"] {
		buf.WriteString(`
func caja_date_today() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}
`)
	}

	if ctx.usedModules["log_export"] {
		buf.WriteString(`
var cajaExportedValues []any

func caja_log_export(v any) {
	b, _ := json.Marshal(v)
	fmt.Println(string(b))
	cajaExportedValues = append(cajaExportedValues, v)
}

// caja_flush_export writes every log.export'd value to CAJA_EXPORT_PATH as
// CSV, one row per value (an array value's elements become that row's
// columns; anything else becomes a single-column row). Set only by ` + "`caja run`" + `
// via --export; absent (e.g. a standalone ` + "`caja build`" + ` binary), this is a
// no-op so caja_log_export's stdout JSON-line output is unaffected.
func caja_flush_export() {
	path := os.Getenv("CAJA_EXPORT_PATH")
	if path == "" || len(cajaExportedValues) == 0 {
		return
	}
	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to create export file:", err)
		return
	}
	defer f.Close()

	w := csv.NewWriter(f)
	for _, v := range cajaExportedValues {
		if c, ok := v.(cajaContainer); ok {
			v = c.cajaRawData()
		}
		var row []string
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
			for i := 0; i < rv.Len(); i++ {
				row = append(row, caja_format_value(rv.Index(i).Interface()))
			}
		} else {
			row = []string{caja_format_value(v)}
		}
		if err := w.Write(row); err != nil {
			fmt.Fprintln(os.Stderr, "failed to write export file:", err)
			return
		}
	}
	w.Flush()
}
`)
	}
	if ctx.usedModules["print_result"] {
		buf.WriteString(`
func caja_print_result(v any) {
	fmt.Println(caja_format_value(v))
}
`)
	}
	if ctx.usedModules["format_value"] {
		buf.WriteString(`
// caja_format_value renders a compiled runtime value the same way the
// tree-walking interpreter's environment.Object.Inspect() implementations
// do, so ` + "`caja run`" + `'s auto-printed result and CSV export match the
// interpreter's output formatting.
func caja_format_value(v any) string {
	if c, ok := v.(cajaContainer); ok {
		return caja_format_value(c.cajaRawData())
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Invalid:
		return "nil"
	case reflect.Float64, reflect.Float32:
		return fmt.Sprintf("%g", rv.Float())
	case reflect.String:
		return rv.String()
	case reflect.Bool:
		return fmt.Sprintf("%t", rv.Bool())
	case reflect.Slice, reflect.Array:
		parts := make([]string, rv.Len())
		for i := range parts {
			parts[i] = caja_format_value(rv.Index(i).Interface())
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case reflect.Map:
		keys := rv.MapKeys()
		sort.Slice(keys, func(i, j int) bool {
			return fmt.Sprintf("%v", keys[i].Interface()) < fmt.Sprintf("%v", keys[j].Interface())
		})
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = fmt.Sprintf("%s: %s", caja_format_value(k.Interface()), caja_format_value(rv.MapIndex(k).Interface()))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case reflect.Ptr:
		if rv.IsNil() {
			return "nil"
		}
		return caja_format_value(rv.Elem().Interface())
	case reflect.Struct:
		if t, ok := v.(time.Time); ok {
			return t.Format("2006-01-02")
		}
		rt := rv.Type()
		var parts []string
		for i := 0; i < rv.NumField(); i++ {
			// Skip unexported fields (e.g. cajaShared, copy-on-write's
			// hidden bookkeeping bit): reflect.Value.Interface() panics on
			// an unexported field, and even if it didn't, this is internal
			// state that should never appear in user-visible output.
			if !rt.Field(i).IsExported() {
				continue
			}
			parts = append(parts, fmt.Sprintf("%s: %s", rt.Field(i).Name, caja_format_value(rv.Field(i).Interface())))
		}
		return rt.Name() + " { " + strings.Join(parts, ", ") + " }"
	default:
		return fmt.Sprintf("%v", v)
	}
}
`)
	}
	if ctx.usedModules["log_info"] {
		buf.WriteString(`
func caja_log_info(msg string, args any) {
	fmt.Printf("[INFO] %s %s\n", msg, caja_format_value(args))
}
`)
	}
	if ctx.usedModules["log_warn"] {
		buf.WriteString(`
func caja_log_warn(msg string, args any) {
	fmt.Printf("[WARN] %s %s\n", msg, caja_format_value(args))
}
`)
	}
	if ctx.usedModules["log_error"] {
		buf.WriteString(`
func caja_log_error(msg string, args any) {
	fmt.Printf("[ERROR] %s %s\n", msg, caja_format_value(args))
}
`)
	}

	if ctx.usedModules["fnv"] {
		buf.WriteString(`
// caja_memo_hash builds a fixed-size cache key for a memoized function
// parameter that isn't natively comparable (arrays, maps, structs), by
// streaming a JSON encoding of its contents through a 64-bit FNV-1a hash
// instead of materializing a full string. JSON encoding is used instead of
// fmt formatting because fmt only dereferences a pointer value at the top
// level (e.g. a lone *Node prints as &{...}) — a pointer nested inside a
// slice or map (e.g. []*Node, which is how Caja compiles an array of struct
// instances) prints as its raw address instead, which would silently hash
// by identity rather than content. encoding/json dereferences pointers
// recursively at any depth, so two equal-content struct slices always hash
// the same regardless of which underlying pointers they hold.
// Known limitation: a 64-bit hash is not guaranteed collision-free, so two
// distinct values could in principle hash to the same key; the odds are
// negligible for realistic workloads and this is accepted deliberately
// rather than paying for a verify-on-hit check on every cache lookup.
func caja_memo_hash(v any) uint64 {
	if c, ok := v.(cajaContainer); ok {
		v = c.cajaRawData()
	}
	h := fnv.New64a()
	json.NewEncoder(h).Encode(v)
	return h.Sum64()
}
`)
	}

	if ctx.usedModules["browser_wrap_callback"] {
		buf.WriteString(`
// caja_wrap_callback runs fn with the same panic recovery and clean error
// formatting main()'s own top-level recover uses. A goroutine dispatched to
// service a JS callback (an addEventListener listener, or one of the fetch
// Promise callbacks in caja_browser_fetch_then below) is invisible to
// main()'s defer — an unrecovered panic there would otherwise crash with a
// raw Go stack trace instead of a caja_panic_location()-formatted message.
func caja_wrap_callback(fn func()) {
	defer func() {
		if r := recover(); r != nil {
			if loc := caja_panic_location(); loc != "" {
				fmt.Fprintf(os.Stderr, "error: %v\n    at %s\n", r, loc)
			} else {
				fmt.Fprintln(os.Stderr, "error:", r)
			}
			os.Exit(1)
		}
	}()
	fn()
}
`)
	}

	if ctx.usedModules["browser_fetch"] {
		buf.WriteString(`
// caja_browser_fetch bridges JS's Promise-based fetch onto a plain blocking
// Go call: two chained Promises (fetch's own, then the Response's .text())
// both funnel into the same buffered channel, so exactly one value is ever
// sent regardless of which stage settles first.
//
// Blocking on a channel like this is safe ONLY when called from a goroutine
// that isn't itself nested inside a syscall/js.handleEvent dispatch (i.e.
// Caja's top level, or an "async <expr>"-spawned goroutine that started
// there) — confirmed via a minimal isolated repro that calling this (even
// wrapped in async+unwrap) from inside a browser.on handler permanently
// freezes the whole page: handleEvent, unlike wasm_exec.js's top-level
// run(), is not async-aware, so a goroutine blocking while nested under it
// can never be resumed. See browser.fetchThen (below) for the callback-driven
// alternative that's safe from inside a handler, since it never blocks.
func caja_browser_fetch(url string) string {
	type fetchResult struct {
		text string
		err  string
	}
	resultCh := make(chan fetchResult, 1)

	var thenResponse, thenText, catchErr js.Func
	catchErr = js.FuncOf(func(this js.Value, args []js.Value) any {
		msg := args[0].String()
		if args[0].Type() == js.TypeObject {
			if m := args[0].Get("message"); m.Type() == js.TypeString {
				msg = m.String()
			}
		}
		resultCh <- fetchResult{err: msg}
		return nil
	})
	defer catchErr.Release()

	thenText = js.FuncOf(func(this js.Value, args []js.Value) any {
		resultCh <- fetchResult{text: args[0].String()}
		return nil
	})
	defer thenText.Release()

	thenResponse = js.FuncOf(func(this js.Value, args []js.Value) any {
		args[0].Call("text").Call("then", thenText).Call("catch", catchErr)
		return nil
	})
	defer thenResponse.Release()

	js.Global().Call("fetch", url).Call("then", thenResponse).Call("catch", catchErr)

	result := <-resultCh
	if result.err != "" {
		panic(fmt.Sprintf("fetch %q failed: %s", url, result.err))
	}
	return result.text
}
`)
	}

	if ctx.usedModules["browser_fetch_then"] {
		buf.WriteString(`
// caja_browser_fetch_then performs an HTTP GET the same way caja_browser_fetch
// does, but purely through callbacks — nothing here ever blocks a goroutine,
// which is what makes it safe to call from inside a browser.on handler
// (unlike caja_browser_fetch, even wrapped in async+unwrap — see its comment
// above). onSuccess runs through caja_wrap_callback so a panic inside the
// Caja handler gets our normal clean error formatting; a network failure
// panics the same way caja_browser_fetch does, also through
// caja_wrap_callback since this runs on a handleEvent-dispatched goroutine
// main()'s own recover never sees. Each js.Func is released as soon as it's
// known which one fired (exactly one of thenText/catchErr ever does, since a
// Promise settles once) rather than leaked, since fetchThen is meant to be
// called repeatedly (e.g. once per click).
func caja_browser_fetch_then(url string, onSuccess func(string)) {
	var thenResponse, thenText, catchErr js.Func
	catchErr = js.FuncOf(func(this js.Value, args []js.Value) any {
		msg := args[0].String()
		if args[0].Type() == js.TypeObject {
			if m := args[0].Get("message"); m.Type() == js.TypeString {
				msg = m.String()
			}
		}
		thenResponse.Release()
		thenText.Release()
		catchErr.Release()
		caja_wrap_callback(func() { panic(fmt.Sprintf("fetch %q failed: %s", url, msg)) })
		return nil
	})
	thenText = js.FuncOf(func(this js.Value, args []js.Value) any {
		thenResponse.Release()
		thenText.Release()
		catchErr.Release()
		caja_wrap_callback(func() { onSuccess(args[0].String()) })
		return nil
	})
	thenResponse = js.FuncOf(func(this js.Value, args []js.Value) any {
		args[0].Call("text").Call("then", thenText).Call("catch", catchErr)
		return nil
	})

	js.Global().Call("fetch", url).Call("then", thenResponse).Call("catch", catchErr)
}
`)
	}

	if ctx.usedModules["browser_query_selector"] {
		buf.WriteString(`
// caja_browser_query_selector wraps document.querySelector, returning nil
// for "no match" (Element?'s Go type is *js.Value — see mapSymbolToGoType's
// NullableSymbol case) rather than a zero/undefined js.Value, so a Caja
// caller can null-check it directly instead of hitting a confusing panic
// from calling a method on an undefined JS value.
func caja_browser_query_selector(selector string) *js.Value {
	el := js.Global().Get("document").Call("querySelector", selector)
	if el.IsNull() {
		return nil
	}
	return &el
}
`)
	}

	if ctx.usedModules["browser_query_selector_all"] {
		buf.WriteString(`
// caja_browser_query_selector_all wraps document.querySelectorAll, copying
// the JS NodeList into a *cajaArray[js.Value] (Array<Element>'s Go type) —
// unlike caja_browser_query_selector this is never nil: no match yields an
// empty NodeList/array, matching JS's own querySelectorAll.
func caja_browser_query_selector_all(selector string) *cajaArray[js.Value] {
	nodeList := js.Global().Get("document").Call("querySelectorAll", selector)
	n := nodeList.Get("length").Int()
	els := make([]js.Value, n)
	for i := 0; i < n; i++ {
		els[i] = nodeList.Index(i)
	}
	return &cajaArray[js.Value]{Data: els}
}
`)
	}

	if ctx.usedModules["browser_get_attribute"] {
		buf.WriteString(`
// caja_browser_get_attribute wraps Element.getAttribute, returning nil for
// "attribute absent" (String?'s Go type is *string — see mapSymbolToGoType's
// NullableSymbol case) rather than an empty string, distinguishing that from
// an attribute that's present but genuinely empty (e.g. alt="").
func caja_browser_get_attribute(el js.Value, name string) *string {
	if !el.Call("hasAttribute", name).Bool() {
		return nil
	}
	v := el.Call("getAttribute", name).String()
	return &v
}
`)
	}

	if ctx.usedModules["browser_local_storage_get"] {
		buf.WriteString(`
// caja_browser_local_storage_get wraps localStorage.getItem, returning nil
// for "key never set" (String?'s Go type is *string — see
// mapSymbolToGoType's NullableSymbol case) rather than an empty string,
// distinguishing that from a key that's set to a genuinely empty string.
func caja_browser_local_storage_get(key string) *string {
	v := js.Global().Get("localStorage").Call("getItem", key)
	if v.IsNull() {
		return nil
	}
	s := v.String()
	return &s
}
`)
	}
}

func transpileBuiltinProperty(module string, prop string, ctx *transpileContext) (string, error) {
	ctx.usedModules[module] = true

	if module == "math" {
		switch prop {
		case "PI":
			return "math.Pi", nil
		case "E":
			return "math.E", nil
		case "SQRT2":
			return "math.Sqrt2", nil
		case "LN2":
			return "math.Ln2", nil
		case "LN10":
			return "math.Ln10", nil
		case "LOG2E":
			return "math.Log2E", nil
		case "LOG10E":
			return "math.Log10E", nil
		}
	}

	// Fallback to capitalizing first letter
	return fmt.Sprintf("%s.%s", module, strings.Title(prop)), nil
}
