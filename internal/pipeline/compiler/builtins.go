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
	switch n.(type) {
	case *ast.CallExpression, *ast.SafePipeExpression, *ast.StreamPipeExpression, *ast.StructLiteral, *ast.ArrayLiteral, *ast.MapLiteral:
		return true
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
