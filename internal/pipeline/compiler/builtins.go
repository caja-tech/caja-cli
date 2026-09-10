package compiler

import (
	"bytes"
	"caja-cli/internal/pipeline/ast"
	"fmt"
	"strings"
)

var builtinModules = map[string]bool{
	"array":  true,
	"date":   true,
	"string": true,
	"math":   true,
	"log":    true,
	"map":    true,
	"cast":   true,
	"http":   true,
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
			
			fallbackType := ""
			switch args[1].(type) {
			case *ast.StringLiteral:
				fallbackType = "String"
			case *ast.NumberLiteral:
				fallbackType = "Number"
			case *ast.BooleanLiteral:
				fallbackType = "Boolean"
			default:
				if fallbackSym, _ := ctx.analyzer.GetSymbol(args[1]); fallbackSym != nil {
					fallbackType = string(fallbackSym.Type())
				}
			}
			
			inputType := ""
			if inputSym, _ := ctx.analyzer.GetSymbol(args[0]); inputSym != nil {
				inputType = string(inputSym.Type())
			}

			// Generate the value string representation based on input type
			valStr := fmt.Sprintf("fmt.Sprintf(\"%%v\", %s)", argStrs[0])
			if inputType == "Date" {
				valStr = fmt.Sprintf("%s.Format(\"2006-01-02\")", argStrs[0])
			} else if inputType == "String" {
				valStr = argStrs[0]
			}
			
			switch fallbackType {
			case "String":
				return valStr, nil
			case "Number":
				if inputType == "Number" { return argStrs[0], nil }
				ctx.usedModules["strconv"] = true
				return fmt.Sprintf("func() float64 { v, err := strconv.ParseFloat(%s, 64); if err != nil { return %s }; return v }()", valStr, argStrs[1]), nil
			case "Boolean":
				if inputType == "Boolean" { return argStrs[0], nil }
				ctx.usedModules["strconv"] = true
				return fmt.Sprintf("func() bool { v, err := strconv.ParseBool(%s); if err != nil { return %s }; return v }()", valStr, argStrs[1]), nil
			case "Date":
				if inputType == "Date" { return argStrs[0], nil }
				ctx.usedModules["time"] = true
				return fmt.Sprintf("func() time.Time { v, err := time.Parse(\"2006-01-02\", %s); if err != nil { return %s }; return v }()", valStr, argStrs[1]), nil
			}
			
			return fmt.Sprintf("%s", argStrs[0]), nil
		}

	case "http":
		ctx.usedModules["cow_map"] = true
		// caja_http_adapt (always injected once http is used, not just for
		// parseJSON callers) builds Request.QueryAll/HeadersAll as
		// *cajaArray[string] values, so cajaArray[T]'s type definition must
		// always be available too, regardless of which specific http.*
		// functions this program happens to call.
		ctx.usedModules["cow_array"] = true
		switch fn {
		case "newRouter":
			return "caja_http_new_router()", nil
		case "listen":
			return fmt.Sprintf("caja_http_listen(%s, %s)", argStrs[0], argStrs[1]), nil
		case "ok":
			return fmt.Sprintf("caja_http_ok(%s)", argStrs[0]), nil
		case "text":
			return fmt.Sprintf("caja_http_text(%s, %s)", argStrs[0], argStrs[1]), nil
		case "notFound":
			return fmt.Sprintf("caja_http_text(404, %s)", argStrs[0]), nil
		case "badRequest":
			return fmt.Sprintf("caja_http_text(400, %s)", argStrs[0]), nil
		case "serverError":
			return fmt.Sprintf("caja_http_text(500, %s)", argStrs[0]), nil
		case "json":
			ctx.usedModules["json"] = true
			ctx.usedModules["reflect"] = true
			return fmt.Sprintf("caja_http_json(%s, %s)", argStrs[0], argStrs[1]), nil
		case "parseJSON":
			// http_parse_json gates its own Go glue separately from the rest of
			// the json-serialization glue (caja_http_json/caja_http_to_jsonable,
			// gated on "json" alone) because it additionally needs cajaArray[T]
			// (for nested JSON arrays) — a program that only ever calls
			// http.json(...) to serialize responses, never parseJSON, must not
			// get cajaArray-dependent code injected with no "cow_array"-gated
			// cajaArray[T] type definition to back it.
			ctx.usedModules["json"] = true
			ctx.usedModules["cow_array"] = true
			ctx.usedModules["http_parse_json"] = true
			return fmt.Sprintf("caja_http_parse_json(%s)", argStrs[0]), nil
		case "rateLimiter":
			// "sync" isn't picked up by the bodyCode string-scan the "sync"
			// import gate otherwise relies on (transpiler.go) — cajaRateLimiter's
			// sync.Mutex lives in injectBuiltinDependencies' output, appended
			// to finalBuf after that gate already ran. http_rate_limiter gates
			// injecting the limiter's own Go glue (see injectBuiltinDependencies)
			// separately from the rest of the http module, so a program that
			// uses http but never calls rateLimiter doesn't get a sync.Mutex
			// struct field with no "sync" import to back it.
			ctx.usedModules["sync"] = true
			ctx.usedModules["http_rate_limiter"] = true
			return fmt.Sprintf("caja_http_rate_limit(%s, %s, %s)", argStrs[0], argStrs[1], argStrs[2]), nil
		case "concurrencyLimiter":
			// Separate gate from http_rate_limiter (see that case's comment
			// for why) -- a program using one limiter shouldn't get the
			// other's unrelated Go glue injected too.
			ctx.usedModules["sync"] = true
			ctx.usedModules["http_concurrency_limiter"] = true
			return fmt.Sprintf("caja_http_concurrency_limit(%s, %s)", argStrs[0], argStrs[1]), nil
		case "distributedRateLimiter":
			// Separate gate from http_rate_limiter/http_concurrency_limiter
			// (see those cases' comments) -- this limiter's Go glue (RESP
			// client, connection pool, Lua script) is unrelated to either
			// in-memory limiter's and must not be injected into a program
			// that never calls distributedRateLimiter. No "sync" flag is
			// needed here: cajaRedisPool is backed by a buffered channel,
			// not a sync.Mutex. "strconv" reuses the existing generic
			// import gate; "bufio" needs its own new gated import (see
			// transpiler.go).
			ctx.usedModules["bufio"] = true
			ctx.usedModules["strconv"] = true
			ctx.usedModules["http_distributed_rate_limiter"] = true
			return fmt.Sprintf("caja_http_distributed_rate_limit(%s, %s, %s)", argStrs[0], argStrs[1], argStrs[2]), nil
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

	if ctx.usedModules["dynamic_index"] {
		buf.WriteString(`
// caja_dynamic_index indexes into a value reached through an Any-typed
// intermediate step -- e.g. a nested field pulled out of a
// Map<_, Any>-shaped container such as an http.parseJSON result, where the
// analyzer can't statically prove the value is a map or array once it's
// widened to Any. key is either a string (map lookup) or a float64 (array
// index, matching how every Caja number is represented).
func caja_dynamic_index(v any, key any) any {
	switch container := v.(type) {
	case *cajaMap[string, any]:
		k, ok := key.(string)
		if !ok {
			panic(fmt.Sprintf("cannot index a map with a %T key", key))
		}
		return container.Data[k]
	case *cajaArray[any]:
		idx, ok := key.(float64)
		if !ok {
			panic(fmt.Sprintf("cannot index an array with a %T key", key))
		}
		return container.Data[int(idx)]
	default:
		panic(fmt.Sprintf("cannot index into value of type %T", v))
	}
}
`)
	}

	if ctx.usedModules["http"] {
		buf.WriteString(`
// Request/Response/Router carry the same cajaShared/cajaSetShared/cajaClone
// trio the compiler generates for every user-defined Caja struct (see
// transpiler.go's struct-literal codegen): isSharableSymbol treats any
// StructDefSymbol as copy-on-write-tracked, so maybeShareValue will wrap a
// Request/Response/Router value in cajaShare(...) wherever it's aliased into
// a second binding (a call argument, a let, a return) unless it's a freshly
// constructed value -- these methods are what makes that generic wrapping
// compile, they aren't optional glue.
type Request struct {
	Method     string
	Path       string
	PathParams *cajaMap[string, string]
	Query      *cajaMap[string, string]
	Headers    *cajaMap[string, string]
	Body       string
	Ip         string
	QueryAll   *cajaMap[string, *cajaArray[string]]
	HeadersAll *cajaMap[string, *cajaArray[string]]
	cajaShared bool
}

func (s *Request) cajaSetShared() {
	if s == nil {
		return
	}
	s.cajaShared = true
	s.PathParams.cajaSetShared()
	s.Query.cajaSetShared()
	s.Headers.cajaSetShared()
	s.QueryAll.cajaSetShared()
	s.HeadersAll.cajaSetShared()
}
func (s *Request) cajaClone() *Request {
	if s == nil {
		return nil
	}
	clone := *s
	clone.cajaShared = false
	return &clone
}

type Response struct {
	Status     float64
	Headers    *cajaMap[string, string]
	Body       string
	cajaShared bool
}

func (s *Response) cajaSetShared() {
	if s == nil {
		return
	}
	s.cajaShared = true
	s.Headers.cajaSetShared()
}
func (s *Response) cajaClone() *Response {
	if s == nil {
		return nil
	}
	clone := *s
	clone.cajaShared = false
	return &clone
}

type cajaHttpRoute struct {
	Method     string
	Pattern    string
	ParamNames []string
	Handler    func(*Request) *Response
}

type cajaHttpStatic struct {
	Prefix string
	Dir    string
}

type Router struct {
	cajaRoutes      []cajaHttpRoute
	cajaMiddlewares []func(func(*Request) *Response) func(*Request) *Response
	cajaStatics     []cajaHttpStatic
	Get             func(string, func(*Request) *Response)
	Post            func(string, func(*Request) *Response)
	Put             func(string, func(*Request) *Response)
	Delete          func(string, func(*Request) *Response)
	Patch           func(string, func(*Request) *Response)
	Static          func(string, string)
	Use             func(func(func(*Request) *Response) func(*Request) *Response)
	cajaShared      bool
}

func (s *Router) cajaSetShared() {
	if s == nil {
		return
	}
	s.cajaShared = true
}
func (s *Router) cajaClone() *Router {
	if s == nil {
		return nil
	}
	clone := *s
	clone.cajaShared = false
	return &clone
}

func caja_http_empty_map() *cajaMap[string, string] {
	return &cajaMap[string, string]{Data: map[string]string{}}
}

// caja_http_ensure_headers lazily initializes resp.Headers, shared by every
// header-writing helper below (the rate-limit and concurrency-limit header
// setters) that must tolerate a handler-built Response with a nil Headers
// map -- defined here, alongside caja_http_empty_map, since it's always
// injected whenever any http feature is used, so every independently-gated
// limiter block below can rely on it without pulling in the other's glue.
func caja_http_ensure_headers(resp *Response) {
	if resp.Headers == nil {
		resp.Headers = caja_http_empty_map()
	}
}

// caja_http_convert_pattern rewrites Caja's Express-style ":name" path
// params into Go 1.22 ServeMux's "{name}" wildcard syntax, and returns the
// extracted param names (order preserved) so the per-request adapter knows
// which req.PathValue(name) calls to make. Done at route-registration time
// rather than compile time, since a route pattern can be an arbitrary string
// expression, not just a literal -- one code path has to handle both.
//
// A ":name*" segment (trailing "*" marker) is a greedy wildcard, capturing
// the rest of the path including any slashes -- converted to Go's "{name...}"
// syntax instead of "{name}". Go requires a "..." wildcard to be the
// pattern's last segment; a misplaced one panics at mux.Handle (surfaced at
// caja_http_listen time, the same way a duplicate route registration already
// panics there -- a startup-time configuration error, not a per-request one).
func caja_http_convert_pattern(pattern string) (string, []string) {
	segments := strings.Split(pattern, "/")
	var names []string
	for i, seg := range segments {
		if strings.HasPrefix(seg, ":") && len(seg) > 1 {
			if strings.HasSuffix(seg, "*") && len(seg) > 2 {
				name := seg[1 : len(seg)-1]
				names = append(names, name)
				segments[i] = "{" + name + "...}"
				continue
			}
			name := seg[1:]
			names = append(names, name)
			segments[i] = "{" + name + "}"
		}
	}
	return strings.Join(segments, "/"), names
}

func caja_http_add_route(r *Router, method string, pattern string, h func(*Request) *Response) {
	convertedPattern, paramNames := caja_http_convert_pattern(pattern)
	r.cajaRoutes = append(r.cajaRoutes, cajaHttpRoute{Method: method, Pattern: convertedPattern, ParamNames: paramNames, Handler: h})
}

// caja_http_add_static registers a static file mount. The prefix is
// normalized to always end in "/" before being handed to mux.Handle/
// http.StripPrefix, since Go 1.22 ServeMux's trailing-slash subtree-match
// syntax means "/static" (no slash) and "/static/" behave differently --
// this makes router.static("/static", dir) and router.static("/static/", dir)
// equivalent regardless of which the Caja caller wrote.
func caja_http_add_static(r *Router, prefix string, dir string) {
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	r.cajaStatics = append(r.cajaStatics, cajaHttpStatic{Prefix: prefix, Dir: dir})
}

// cajaNoListingFS wraps a http.FileSystem so opening a directory that has no
// index.html reports it as not-found rather than letting http.FileServer
// fall through to its built-in auto-generated directory listing -- serving
// a static mount shouldn't accidentally expose the served tree's file names
// and structure to anyone who requests a bare directory path.
type cajaNoListingFS struct {
	http.FileSystem
}

func (fs cajaNoListingFS) Open(name string) (http.File, error) {
	f, err := fs.FileSystem.Open(name)
	if err != nil {
		return nil, err
	}
	stat, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if stat.IsDir() {
		indexPath := strings.TrimSuffix(name, "/") + "/index.html"
		idx, err := fs.FileSystem.Open(indexPath)
		if err != nil {
			f.Close()
			return nil, os.ErrNotExist
		}
		idx.Close()
	}
	return f, nil
}

// caja_http_path_has_dotfile reports whether any segment of a URL path
// starts with "." -- used to 404 requests reaching for a dotfile (.env,
// .git/..., .DS_Store) that happens to live inside a served directory,
// before the request ever reaches the file system.
func caja_http_path_has_dotfile(urlPath string) bool {
	for _, seg := range strings.Split(urlPath, "/") {
		if seg != "" && strings.HasPrefix(seg, ".") {
			return true
		}
	}
	return false
}

// caja_http_static_handler builds the handler a static mount registers on
// the mux: dotfile requests are rejected before touching the file system,
// then StripPrefix + FileServer(cajaNoListingFS(...)) serves everything
// else, with directory-listing suppressed by cajaNoListingFS above.
func caja_http_static_handler(prefix string, dir string) http.Handler {
	fileServer := http.FileServer(cajaNoListingFS{http.Dir(dir)})
	stripped := http.StripPrefix(strings.TrimSuffix(prefix, "/"), fileServer)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if caja_http_path_has_dotfile(r.URL.Path) {
			http.NotFound(w, r)
			return
		}
		stripped.ServeHTTP(w, r)
	})
}

func caja_http_new_router() *Router {
	r := &Router{}
	r.Get = func(path string, h func(*Request) *Response) { caja_http_add_route(r, "GET", path, h) }
	r.Post = func(path string, h func(*Request) *Response) { caja_http_add_route(r, "POST", path, h) }
	r.Put = func(path string, h func(*Request) *Response) { caja_http_add_route(r, "PUT", path, h) }
	r.Delete = func(path string, h func(*Request) *Response) { caja_http_add_route(r, "DELETE", path, h) }
	r.Patch = func(path string, h func(*Request) *Response) { caja_http_add_route(r, "PATCH", path, h) }
	r.Static = func(prefix string, dir string) { caja_http_add_static(r, prefix, dir) }
	r.Use = func(mw func(func(*Request) *Response) func(*Request) *Response) {
		r.cajaMiddlewares = append(r.cajaMiddlewares, mw)
	}
	return r
}

// caja_http_adapt wraps a resolved (already middleware-composed) handler as
// a stdlib http.HandlerFunc, with its own recover: net/http runs each
// request on its own goroutine, so a panic here would otherwise bypass
// main()'s top-level recover/caja_panic_location reporting entirely and just
// drop the connection -- this local recover reproduces the same clean error
// reporting and turns it into a 500 instead.
func caja_http_adapt(paramNames []string, h func(*Request) *Response) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				if loc := caja_panic_location(); loc != "" {
					fmt.Fprintf(os.Stderr, "error: %v\n    at %s\n", rec, loc)
				} else {
					fmt.Fprintln(os.Stderr, "error:", rec)
				}
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintln(w, "internal server error")
			}
		}()

		pathParams := map[string]string{}
		for _, name := range paramNames {
			pathParams[name] = req.PathValue(name)
		}
		query := map[string]string{}
		for k, v := range req.URL.Query() {
			if len(v) > 0 {
				query[k] = v[0]
			}
		}
		headers := map[string]string{}
		for k, v := range req.Header {
			if len(v) > 0 {
				headers[k] = v[0]
			}
		}
		// req.URL.Query() and req.Header are already Go's map[string][]string
		// -- QueryAll/HeadersAll expose every value for a repeated key,
		// alongside the first-value-wins Query/Headers above for the common
		// case where callers don't care about repeats.
		queryAll := map[string]*cajaArray[string]{}
		for k, v := range req.URL.Query() {
			queryAll[k] = &cajaArray[string]{Data: v}
		}
		headersAll := map[string]*cajaArray[string]{}
		for k, v := range req.Header {
			headersAll[k] = &cajaArray[string]{Data: v}
		}
		bodyBytes, _ := io.ReadAll(req.Body)

		ip := req.RemoteAddr
		if host, _, splitErr := net.SplitHostPort(req.RemoteAddr); splitErr == nil {
			ip = host
		}
		// CAJA_TRUST_PROXY opts into reading the client IP from a
		// proxy-supplied header instead of the raw TCP peer address --
		// unset by default, since X-Forwarded-For/X-Real-IP are ordinary
		// request headers ANY direct client can set to whatever they like.
		// Trusting them unconditionally would let a client trivially spoof
		// req.ip and bypass an IP-keyed rate limiter entirely. This must
		// only be set by a deployer who knows every request actually
		// arrives through a reverse proxy they control (their own nginx,
		// a platform gateway) that overwrites/sets these headers itself --
		// never appropriate for a server exposed directly to the internet.
		if os.Getenv("CAJA_TRUST_PROXY") != "" {
			if fwd := req.Header.Get("X-Forwarded-For"); fwd != "" {
				if first := strings.TrimSpace(strings.Split(fwd, ",")[0]); first != "" {
					ip = first
				}
			} else if real := strings.TrimSpace(req.Header.Get("X-Real-IP")); real != "" {
				ip = real
			}
		}

		resp := h(&Request{
			Method:     req.Method,
			Path:       req.URL.Path,
			PathParams: &cajaMap[string, string]{Data: pathParams},
			Query:      &cajaMap[string, string]{Data: query},
			Headers:    &cajaMap[string, string]{Data: headers},
			Body:       string(bodyBytes),
			Ip:         ip,
			QueryAll:   &cajaMap[string, *cajaArray[string]]{Data: queryAll},
			HeadersAll: &cajaMap[string, *cajaArray[string]]{Data: headersAll},
		})
		if resp == nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if resp.Headers != nil {
			for k, v := range resp.Headers.Data {
				w.Header().Set(k, v)
			}
		}
		status := int(resp.Status)
		if status == 0 {
			status = 200
		}
		w.WriteHeader(status)
		fmt.Fprint(w, resp.Body)
	}
}

// caja_http_listen builds the *http.ServeMux at listen time (not at each
// router.get/post call), so router.use(...) calls made anywhere before this
// point apply to every route regardless of registration order. Middlewares
// wrap innermost-to-outermost in reverse registration order, so the first
// router.use(...) call ends up outermost / runs first.
//
// SIGINT/SIGTERM are caught here rather than left to Go's default
// terminate-immediately behavior, so an in-flight request gets a chance to
// finish instead of being cut off mid-response when a deploy or orchestrator
// stops the process -- a real backend needs this the same way any other
// production HTTP server does. caja_http_shutdown_timeout bounds how long
// that drain is allowed to take before the process exits anyway.
const caja_http_shutdown_timeout = 10 * time.Second

// caja_http_read_header_timeout/caja_http_read_timeout/caja_http_write_timeout/
// caja_http_idle_timeout guard against a slow or stalled client tying up a
// connection (and the goroutine serving it) indefinitely -- an *http.Server
// with none of these set is the textbook slowloris vulnerability: a client
// that opens a connection and trickles bytes (or none at all) forever never
// times out on its own. ReadHeaderTimeout matters most for that specific
// attack (Go added it, separate from ReadTimeout, exactly to bound how long
// reading just the request line/headers can take). Fixed for now, not yet
// exposed as a Caja-level configuration option.
const (
	caja_http_read_header_timeout = 5 * time.Second
	caja_http_read_timeout        = 15 * time.Second
	caja_http_write_timeout       = 15 * time.Second
	caja_http_idle_timeout        = 60 * time.Second
)

func caja_http_listen(r *Router, port float64) {
	mux := http.NewServeMux()
	for _, route := range r.cajaRoutes {
		finalHandler := route.Handler
		for i := len(r.cajaMiddlewares) - 1; i >= 0; i-- {
			finalHandler = r.cajaMiddlewares[i](finalHandler)
		}
		mux.Handle(route.Method+" "+route.Pattern, caja_http_adapt(route.ParamNames, finalHandler))
	}

	// Static mounts are registered directly as stdlib http.Handlers, bypassing
	// caja_http_adapt entirely -- a static mount's handler is never adapted
	// into Caja's Request/Response shape, so router.use(...) middleware does
	// NOT wrap static file responses (a structural limitation of this design,
	// not a bug). caja_http_static_handler gives directory index.html
	// defaulting, Range/If-Modified-Since support, and ".."-path-traversal
	// protection for free via http.FileServer/http.Dir, plus suppressing
	// auto-generated directory listings and hiding dotfiles (both on top of
	// stdlib defaults, see caja_http_static_handler's own doc comment), with
	// no filesystem access ever exposed to Caja source itself. A static
	// prefix colliding with an existing route pattern panics at mux.Handle,
	// same as a duplicate route registration already does above.
	for _, static := range r.cajaStatics {
		mux.Handle(static.Prefix, caja_http_static_handler(static.Prefix, static.Dir))
	}

	// CAJA_HTTP_PORT (set by caja listen's --port flag) overrides whatever
	// port literal the .caja source passed here, so the deployment/run
	// environment controls the actual bind port rather than the source code
	// -- the source's own argument still matters for any other way the
	// compiled program gets run (a plain caja build binary, caja run without
	// listen), where no such env var is set.
	if envPort := os.Getenv("CAJA_HTTP_PORT"); envPort != "" {
		var p int
		if _, err := fmt.Sscanf(envPort, "%d", &p); err == nil {
			port = float64(p)
		}
	}

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", int(port)),
		Handler:           mux,
		ReadHeaderTimeout: caja_http_read_header_timeout,
		ReadTimeout:       caja_http_read_timeout,
		WriteTimeout:      caja_http_write_timeout,
		IdleTimeout:       caja_http_idle_timeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	case <-ctx.Done():
		stop()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), caja_http_shutdown_timeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			panic(err)
		}
	}
}

func caja_http_ok(body string) *Response {
	return caja_http_text(200, body)
}
func caja_http_text(status float64, body string) *Response {
	return &Response{Status: status, Headers: caja_http_empty_map(), Body: body}
}
`)
		if ctx.usedModules["http_rate_limiter"] || ctx.usedModules["http_distributed_rate_limiter"] {
			buf.WriteString(`
// cajaRateLimitResult carries enough of a rate-limit check's outcome to fill
// in the standard X-RateLimit-* response headers, shared by both
// http.rateLimiter (in-memory token bucket) and http.distributedRateLimiter
// (Redis-backed fixed window) -- conceptually the same "rate limit" concept
// and response contract either way, just enforced differently server-side.
// This block is gated on either limiter's flag rather than folded into one
// specific limiter's own gated block, so a program using only one of the two
// still gets this shared type/helper without pulling in the other limiter's
// unrelated Go glue.
type cajaRateLimitResult struct {
	Allowed      bool
	Limit        int
	Remaining    int
	ResetSeconds int // seconds until the limiter fully resets
	RetryAfter   int // seconds until at least one more request is allowed; only meaningful when !Allowed
}

// caja_http_apply_rate_limit_headers sets the standard X-RateLimit-* headers
// on every response a rate-limited route produces (so a well-behaved client
// can see its remaining quota before ever hitting the limit, not just after),
// plus Retry-After when the request was rejected -- the one header of the
// four with defined meaning on a 429 specifically (RFC 7231).
func caja_http_apply_rate_limit_headers(resp *Response, result cajaRateLimitResult) {
	caja_http_ensure_headers(resp)
	resp.Headers.Data["X-RateLimit-Limit"] = fmt.Sprintf("%d", result.Limit)
	resp.Headers.Data["X-RateLimit-Remaining"] = fmt.Sprintf("%d", result.Remaining)
	resp.Headers.Data["X-RateLimit-Reset"] = fmt.Sprintf("%d", result.ResetSeconds)
	if !result.Allowed {
		resp.Headers.Data["Retry-After"] = fmt.Sprintf("%d", result.RetryAfter)
	}
}
`)
		}
		if ctx.usedModules["http_rate_limiter"] {
			buf.WriteString(`
// cajaRateLimiter implements a per-key token-bucket rate limiter entirely in
// hand-rolled stdlib Go: Caja's purity rule forbids a closure from mutating
// anything captured from an enclosing scope, so per-key request counters
// can't be represented as Caja-level state at all -- this state has to live
// in Go, owned by the closure caja_http_rate_limit returns once, and shared
// across every request through that closure's capture of rl.
type cajaRateLimiterEntry struct {
	tokens   float64
	lastSeen time.Time
}

type cajaRateLimiter struct {
	mu      sync.Mutex
	rate    float64
	burst   float64
	buckets map[string]*cajaRateLimiterEntry
}

func caja_http_new_rate_limiter(rate float64, burst float64) *cajaRateLimiter {
	rl := &cajaRateLimiter{rate: rate, burst: burst, buckets: map[string]*cajaRateLimiterEntry{}}
	go rl.evictLoop()
	return rl
}

// evictLoop bounds memory for unbounded key spaces (e.g. per-IP limiting on
// a public server that sees many distinct client IPs) by periodically
// dropping buckets that have gone idle -- without this, a rate limiter keyed
// by req.ip would leak one map entry per distinct IP forever.
func (rl *cajaRateLimiter) evictLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().Add(-10 * time.Minute)
		rl.mu.Lock()
		for key, entry := range rl.buckets {
			if entry.lastSeen.Before(cutoff) {
				delete(rl.buckets, key)
			}
		}
		rl.mu.Unlock()
	}
}

// cajaRateLimitResult (shared with distributedRateLimiter -- see its own
// gated block above) carries enough of the bucket's state after an allow()
// check to fill in the standard rate-limit response headers, so a caller
// never has to reach back into the (mutex-guarded) bucket itself.
func (rl *cajaRateLimiter) allow(key string) cajaRateLimitResult {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	entry, ok := rl.buckets[key]
	if !ok {
		entry = &cajaRateLimiterEntry{tokens: rl.burst, lastSeen: now}
		rl.buckets[key] = entry
	}

	elapsed := now.Sub(entry.lastSeen).Seconds()
	entry.tokens += elapsed * rl.rate
	if entry.tokens > rl.burst {
		entry.tokens = rl.burst
	}
	entry.lastSeen = now

	allowed := entry.tokens >= 1
	if allowed {
		entry.tokens -= 1
	}

	remaining := int(entry.tokens)
	if remaining < 0 {
		remaining = 0
	}

	// ceilSeconds(tokensNeeded) rounds a token deficit up to a whole number
	// of seconds at the bucket's refill rate -- always at least 1s when a
	// deficit exists, so a client is never told "retry immediately" while
	// still under the limit.
	ceilSeconds := func(tokensNeeded float64) int {
		if tokensNeeded <= 0 {
			return 0
		}
		seconds := tokensNeeded / rl.rate
		whole := int(seconds)
		if seconds > float64(whole) {
			whole++
		}
		if whole < 1 {
			whole = 1
		}
		return whole
	}

	result := cajaRateLimitResult{
		Allowed:      allowed,
		Limit:        int(rl.burst),
		Remaining:    remaining,
		ResetSeconds: ceilSeconds(rl.burst - entry.tokens),
	}
	if !allowed {
		result.RetryAfter = ceilSeconds(1 - entry.tokens)
	}
	return result
}

func caja_http_rate_limit(rps float64, burst float64, keyFunc func(*Request) string) func(func(*Request) *Response) func(*Request) *Response {
	rl := caja_http_new_rate_limiter(rps, burst)
	return func(next func(*Request) *Response) func(*Request) *Response {
		return func(req *Request) *Response {
			result := rl.allow(keyFunc(req))
			if !result.Allowed {
				resp := caja_http_text(429, "too many requests")
				caja_http_apply_rate_limit_headers(resp, result)
				return resp
			}
			resp := next(req)
			if resp == nil {
				return resp
			}
			caja_http_apply_rate_limit_headers(resp, result)
			return resp
		}
	}
}
`)
		}
		if ctx.usedModules["http_concurrency_limiter"] {
			buf.WriteString(`
// cajaConcurrencyLimiter caps how many requests for a given key may be
// in flight at once, as opposed to cajaRateLimiter's per-second throughput
// cap -- useful for protecting an expensive operation (an upload, a slow
// downstream call) where the cost is how many run simultaneously, not how
// often they're requested. Unlike the rate limiter, an idle key's entry is
// deleted immediately once its count returns to zero (see release below),
// so no periodic eviction loop is needed here.
type cajaConcurrencyLimiter struct {
	mu      sync.Mutex
	max     int
	current map[string]int
}

func caja_http_new_concurrency_limiter(max float64) *cajaConcurrencyLimiter {
	return &cajaConcurrencyLimiter{max: int(max), current: map[string]int{}}
}

// tryAcquire reports whether key is under its concurrency limit and, if so,
// reserves a slot for it. The returned count is the key's in-flight total
// after this call either way (equal to the limit when acquisition fails,
// since that's the only way it can fail) -- callers use it to fill in
// X-Concurrency-* response headers without a second lock round-trip.
func (cl *cajaConcurrencyLimiter) tryAcquire(key string) (acquired bool, count int) {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	if cl.current[key] >= cl.max {
		return false, cl.current[key]
	}
	cl.current[key]++
	return true, cl.current[key]
}

func (cl *cajaConcurrencyLimiter) release(key string) {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	cl.current[key]--
	if cl.current[key] <= 0 {
		delete(cl.current, key)
	}
}

// caja_http_apply_concurrency_headers mirrors
// caja_http_apply_rate_limit_headers's role for the concurrency limiter --
// no Retry-After equivalent, since a freed slot isn't governed by a fixed
// refill rate the way a token bucket is; there's nothing accurate to tell
// a client about "when" here, so this deliberately doesn't invent one.
func caja_http_apply_concurrency_headers(resp *Response, max int, count int) {
	caja_http_ensure_headers(resp)
	remaining := max - count
	if remaining < 0 {
		remaining = 0
	}
	resp.Headers.Data["X-Concurrency-Limit"] = fmt.Sprintf("%d", max)
	resp.Headers.Data["X-Concurrency-Remaining"] = fmt.Sprintf("%d", remaining)
}

func caja_http_concurrency_limit(max float64, keyFunc func(*Request) string) func(func(*Request) *Response) func(*Request) *Response {
	cl := caja_http_new_concurrency_limiter(max)
	return func(next func(*Request) *Response) func(*Request) *Response {
		return func(req *Request) *Response {
			key := keyFunc(req)
			acquired, count := cl.tryAcquire(key)
			if !acquired {
				resp := caja_http_text(429, "too many concurrent requests")
				caja_http_apply_concurrency_headers(resp, int(max), count)
				return resp
			}
			// Released via defer, not after a normal return, so a panicking
			// handler still frees its slot -- caja_http_adapt's per-request
			// recover runs after this deferred release (defers execute in
			// LIFO order as the panic unwinds this stack frame), so a slot
			// can never leak permanently just because a handler panicked.
			defer cl.release(key)
			resp := next(req)
			if resp == nil {
				return resp
			}
			caja_http_apply_concurrency_headers(resp, int(max), count)
			return resp
		}
	}
}
`)
		}
		if ctx.usedModules["http_distributed_rate_limiter"] {
			buf.WriteString(`
// caja_redis_bulk frames one RESP bulk string: a byte-length prefix followed
// by exactly that many bytes. len(s) on a Go string is byte length, matching
// RESP's contract exactly (correct even for a multi-byte-UTF-8 key), and
// because the framing is length-prefixed rather than delimiter-terminated,
// arbitrary bytes embedded in s (including \r or \n) need no escaping and
// can never be mistaken for protocol structure.
func caja_redis_bulk(s string) string {
	return fmt.Sprintf("$%d\r\n%s\r\n", len(s), s)
}

// caja_redis_build_eval builds the one RESP command this client ever sends:
// EVAL script 1 key windowSecondsStr -- a RESP array of 5 bulk strings.
func caja_redis_build_eval(script string, key string, windowSecondsStr string) string {
	return "*5\r\n" +
		caja_redis_bulk("EVAL") +
		caja_redis_bulk(script) +
		caja_redis_bulk("1") +
		caja_redis_bulk(key) +
		caja_redis_bulk(windowSecondsStr)
}

// caja_redis_read_line reads one RESP protocol line, trimming its trailing
// \r\n.
func caja_redis_read_line(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func caja_redis_read_integer_reply(r *bufio.Reader) (int, error) {
	line, err := caja_redis_read_line(r)
	if err != nil {
		return 0, err
	}
	if line == "" || line[0] != ':' {
		return 0, fmt.Errorf("redis: expected integer reply, got %q", line)
	}
	return strconv.Atoi(line[1:])
}

// caja_redis_read_eval_reply parses exactly the two reply shapes this client
// ever needs to understand: a 2-element Array of Integer replies (the
// success case -- {current, ttl} from the Lua script below), or an Error
// reply. Anything else (malformed line, wrong array arity, wrong element
// type) is a protocol error -- every failure path here funnels into the
// same fail-open action one level up in allow(), so none of these error
// cases need distinguishing from one another.
func caja_redis_read_eval_reply(r *bufio.Reader) (count int, ttl int, err error) {
	line, err := caja_redis_read_line(r)
	if err != nil {
		return 0, 0, err
	}
	if line == "" {
		return 0, 0, fmt.Errorf("redis: empty reply line")
	}
	switch line[0] {
	case '-':
		return 0, 0, fmt.Errorf("redis: error reply: %s", line[1:])
	case '*':
		n, err := strconv.Atoi(line[1:])
		if err != nil || n != 2 {
			return 0, 0, fmt.Errorf("redis: unexpected array length %q", line[1:])
		}
		if count, err = caja_redis_read_integer_reply(r); err != nil {
			return 0, 0, err
		}
		if ttl, err = caja_redis_read_integer_reply(r); err != nil {
			return 0, 0, err
		}
		return count, ttl, nil
	default:
		return 0, 0, fmt.Errorf("redis: unexpected reply type %q", line[0])
	}
}

const (
	cajaRedisPoolSize    = 8
	cajaRedisDialTimeout = 200 * time.Millisecond
	cajaRedisIOTimeout   = 200 * time.Millisecond
)

// cajaRedisPool is a small fixed-size pool of raw TCP connections to a
// single Redis-compatible server, backing cajaDistributedRateLimiter. A
// single shared connection would serialize every rate-limit check in the
// whole process through one round-trip at a time; a full generic pooling
// library is more than this narrow a use needs. 8 is a v1 sizing guess, not
// a measurement: enough that a moderately concurrent server rarely blocks
// waiting for a free connection, small enough not to open an excessive
// number of sockets to Redis per replica.
type cajaRedisPool struct {
	addr  string
	conns chan net.Conn
}

func caja_redis_new_pool(addr string) *cajaRedisPool {
	return &cajaRedisPool{addr: addr, conns: make(chan net.Conn, cajaRedisPoolSize)}
}

// get returns a pooled idle connection if one exists, otherwise dials a
// fresh one with a short timeout. If addr is "" (CAJA_REDIS_ADDR unset),
// net.DialTimeout fails immediately here -- the same path an unreachable
// address takes, so "not configured" and "unreachable" need no special
// casing anywhere above this.
func (p *cajaRedisPool) get() (net.Conn, error) {
	select {
	case c := <-p.conns:
		return c, nil
	default:
		return net.DialTimeout("tcp", p.addr, cajaRedisDialTimeout)
	}
}

// put returns a healthy, protocol-clean connection to the pool, or closes
// it if the pool is already full.
func (p *cajaRedisPool) put(c net.Conn) {
	select {
	case p.conns <- c:
	default:
		c.Close()
	}
}

// discard closes a connection that errored mid-use -- it may be left in an
// unknown protocol state (a half-written command, an unread/partial reply)
// and must never be returned to the pool for reuse.
func (p *cajaRedisPool) discard(c net.Conn) {
	c.Close()
}

// cajaDistributedRateLimitScript atomically increments the request counter
// for a key, sets its expiry only on the first increment of a window (so a
// key that already has a TTL running never gets it reset), and reads the
// remaining TTL back -- one round trip covers the whole check instead of
// racing a separate INCR against a separate EXPIRE.
//
// Deferred v2 optimization: plain EVAL re-sends and re-parses this script
// server-side on every call; SCRIPT LOAD + EVALSHA would cache it and send
// just a hash, but needs a NOSCRIPT-error fallback (e.g. after a Redis
// restart clears its script cache) to stay correct -- not worth that
// complexity yet for a script this small.
const cajaDistributedRateLimitScript = "local current = redis.call(\"INCR\", KEYS[1])\n" +
	"if current == 1 then\n" +
	"    redis.call(\"EXPIRE\", KEYS[1], ARGV[1])\n" +
	"end\n" +
	"local ttl = redis.call(\"TTL\", KEYS[1])\n" +
	"return {current, ttl}"

// cajaDistributedRateLimiter enforces a fixed-window request-count limit
// shared across every replica of the same Caja server behind a load
// balancer, via a single Redis-compatible store -- unlike cajaRateLimiter's
// per-process in-memory token bucket, whose state doesn't survive being one
// of several replicas. Trades the token bucket's smooth throughput shaping
// for a fixed window counter (INCR+EXPIRE), which admits up to 2x the
// configured limit right at a window boundary -- an accepted tradeoff for a
// single-round-trip check.
type cajaDistributedRateLimiter struct {
	pool          *cajaRedisPool
	maxRequests   int
	windowSeconds int
}

func caja_http_new_distributed_rate_limiter(maxRequests float64, windowSeconds float64) *cajaDistributedRateLimiter {
	// CAJA_REDIS_ADDR is where the shared Redis-compatible store is
	// reached -- a deployment/infra concern (which Redis instance this
	// replica talks to), not something committed Caja source should
	// hardcode, matching CAJA_HTTP_PORT/CAJA_TRUST_PROXY's precedent.
	// Read once here, at construction time, rather than per-request or
	// at listen time: the address backing this limiter's entire state is
	// fixed for the whole process's lifetime, so there's no "value can
	// change later" reason to defer the lookup.
	addr := os.Getenv("CAJA_REDIS_ADDR")
	return &cajaDistributedRateLimiter{
		pool:          caja_redis_new_pool(addr),
		maxRequests:   int(maxRequests),
		windowSeconds: int(windowSeconds),
	}
}

// allow performs the single EVAL round-trip. The bool return is false
// exactly when Redis could not be consulted at all (dial failure, timeout,
// malformed reply) -- every such failure collapses to the same fail-open
// action one level up in caja_http_distributed_rate_limit, so no Go error
// is surfaced here.
func (rl *cajaDistributedRateLimiter) allow(key string) (cajaRateLimitResult, bool) {
	conn, err := rl.pool.get()
	if err != nil {
		return cajaRateLimitResult{}, false
	}
	if err := conn.SetDeadline(time.Now().Add(cajaRedisIOTimeout)); err != nil {
		rl.pool.discard(conn)
		return cajaRateLimitResult{}, false
	}

	// Namespaced under "caja:ratelimit:" so this doesn't collide with
	// unrelated keys in a shared Redis instance, and additionally
	// disambiguated by this limiter's own (maxRequests, windowSeconds)
	// config so two differently-configured distributedRateLimiter calls
	// keyed the same way (e.g. both by req.ip) don't share one counter.
	// Two limiters with identical config and keyFunc intentionally (or
	// accidentally) sharing state is an accepted v1 edge case.
	redisKey := fmt.Sprintf("caja:ratelimit:%d:%d:%s", rl.maxRequests, rl.windowSeconds, key)
	windowSecondsStr := fmt.Sprintf("%d", rl.windowSeconds)
	cmd := caja_redis_build_eval(cajaDistributedRateLimitScript, redisKey, windowSecondsStr)

	if _, err := conn.Write([]byte(cmd)); err != nil {
		rl.pool.discard(conn)
		return cajaRateLimitResult{}, false
	}

	count, ttl, err := caja_redis_read_eval_reply(bufio.NewReader(conn))
	if err != nil {
		rl.pool.discard(conn)
		return cajaRateLimitResult{}, false
	}
	rl.pool.put(conn)

	if ttl < 0 { // no TTL somehow set; treat as a full fresh window
		ttl = rl.windowSeconds
	}
	remaining := rl.maxRequests - count
	if remaining < 0 {
		remaining = 0
	}
	result := cajaRateLimitResult{
		Allowed:      count <= rl.maxRequests,
		Limit:        rl.maxRequests,
		Remaining:    remaining,
		ResetSeconds: ttl,
	}
	if !result.Allowed {
		result.RetryAfter = ttl
	}
	return result, true
}

func caja_http_distributed_rate_limit(maxRequests float64, windowSeconds float64, keyFunc func(*Request) string) func(func(*Request) *Response) func(*Request) *Response {
	rl := caja_http_new_distributed_rate_limiter(maxRequests, windowSeconds)
	return func(next func(*Request) *Response) func(*Request) *Response {
		return func(req *Request) *Response {
			result, ok := rl.allow(keyFunc(req))
			if !ok {
				// Fail open: Redis was unreachable/timed out/replied
				// unexpectedly. Let the request through with no
				// X-RateLimit-* headers -- there's no real limiter data
				// to report, and fabricating e.g. "Remaining: 0" would
				// misrepresent a check that never actually happened.
				return next(req)
			}
			if !result.Allowed {
				resp := caja_http_text(429, "too many requests")
				caja_http_apply_rate_limit_headers(resp, result)
				return resp
			}
			resp := next(req)
			if resp == nil {
				return resp
			}
			caja_http_apply_rate_limit_headers(resp, result)
			return resp
		}
	}
}
`)
		}
		if ctx.usedModules["json"] {
			buf.WriteString(`
func caja_http_to_jsonable(v any) any {
	if c, ok := v.(cajaContainer); ok {
		v = c.cajaRawData()
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		out := make([]any, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			out[i] = caja_http_to_jsonable(rv.Index(i).Interface())
		}
		return out
	case reflect.Map:
		out := make(map[string]any, rv.Len())
		for _, k := range rv.MapKeys() {
			out[fmt.Sprintf("%v", k.Interface())] = caja_http_to_jsonable(rv.MapIndex(k).Interface())
		}
		return out
	case reflect.Ptr:
		if rv.IsNil() {
			return nil
		}
		return caja_http_to_jsonable(rv.Elem().Interface())
	case reflect.Struct:
		out := make(map[string]any)
		rt := rv.Type()
		for i := 0; i < rt.NumField(); i++ {
			if !rt.Field(i).IsExported() {
				continue
			}
			// Lowercase the leading letter so a struct-built response
			// ({Name: "x"} -> Go field Name) serializes with the same key
			// casing a map-literal-built response would ({"name": "x"}) --
			// otherwise the same logical response looks different depending
			// on which Caja construct happened to build it.
			name := rt.Field(i).Name
			jsonKey := strings.ToLower(name[:1]) + name[1:]
			out[jsonKey] = caja_http_to_jsonable(rv.Field(i).Interface())
		}
		return out
	default:
		return v
	}
}

func caja_http_json(status float64, value any) *Response {
	b, err := json.Marshal(caja_http_to_jsonable(value))
	if err != nil {
		panic(err)
	}
	headers := caja_http_empty_map()
	headers.Data["Content-Type"] = "application/json"
	return &Response{Status: status, Headers: headers, Body: string(b)}
}
`)
		}
		if ctx.usedModules["http_parse_json"] {
			buf.WriteString(`
// caja_http_from_jsonable is the mirror image of caja_http_to_jsonable: after
// encoding/json unmarshals a request body into plain Go maps/slices, any
// nested map[string]any/[]any needs wrapping in *cajaMap/*cajaArray too, or
// Caja code indexing into a nested value (parsed["user"]["name"]) would hit
// a raw Go map with no .Data field / cajaMap methods to satisfy the codegen
// every other map/array value in the language already relies on.
func caja_http_from_jsonable(v any) any {
	switch val := v.(type) {
	case map[string]any:
		wrapped := make(map[string]any, len(val))
		for k, vv := range val {
			wrapped[k] = caja_http_from_jsonable(vv)
		}
		return &cajaMap[string, any]{Data: wrapped}
	case []any:
		wrapped := make([]any, len(val))
		for i, vv := range val {
			wrapped[i] = caja_http_from_jsonable(vv)
		}
		return &cajaArray[any]{Data: wrapped}
	default:
		return v
	}
}

// caja_http_parse_json only accepts a top-level JSON object -- the common
// shape for a REST API request body. A bare JSON array/string/number/etc as
// the whole body fails json.Unmarshal's type check against map[string]any
// and panics, same as any other malformed body; the per-request recover in
// caja_http_adapt turns that into a 500 rather than crashing the server.
func caja_http_parse_json(body string) *cajaMap[string, any] {
	var raw map[string]any
	if err := json.Unmarshal([]byte(body), &raw); err != nil {
		panic(err)
	}
	return caja_http_from_jsonable(raw).(*cajaMap[string, any])
}
`)
		}
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
}

func transpileBuiltinProperty(module string, prop string, ctx *transpileContext) (string, error) {
	ctx.usedModules[module] = true
	
	if module == "math" {
		switch prop {
		case "PI": return "math.Pi", nil
		case "E": return "math.E", nil
		case "SQRT2": return "math.Sqrt2", nil
		case "LN2": return "math.Ln2", nil
		case "LN10": return "math.Ln10", nil
		case "LOG2E": return "math.Log2E", nil
		case "LOG10E": return "math.Log10E", nil
		}
	}
	
	// Fallback to capitalizing first letter
	return fmt.Sprintf("%s.%s", module, strings.Title(prop)), nil
}
