package compiler

import (
	"caja-cli/internal/script"
	"strings"
	"testing"
)

func TestTranspile(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string // substrings expected in the output Go code
	}{
		{
			name: "Type Constraints",
			input: `
				type Customer struct { age Number }
				define MajorCustomer constraints Customer with: fn(c: Customer) -> Boolean { return c.age > 18 }
				let m: MajorCustomer? = Customer { age: 20 }
			`,
			expected: []string{
				"type MajorCustomer Customer",
				"var validate_MajorCustomer func(*Customer) *MajorCustomer = func(val *Customer) *MajorCustomer {\n\tpred := func(c *Customer) bool {\n\treturn (c.Age > 18.0)\n}\n\tif pred(val) {\n\t\tres := (*MajorCustomer)(val)\n\t\treturn res\n\t}\n\treturn nil\n}",
				"var m *MajorCustomer = validate_MajorCustomer((&Customer{\nAge: 20.0,\n}))",
			},
		},
		{
			name: "Union Types",
			input: `
				type Cat struct { name String }
				type Dog struct { name String }
				union Animal = Cat | Dog
				let animal: Animal = Cat { name: "Tom" }
				let cat: Cat? = animal is Cat
			`,
			expected: []string{
				"type Animal interface{ isAnimal() }",
				"func (*Cat) isAnimal() {}",
				"func (*Dog) isAnimal() {}",
				// (&Cat{...}) is self-parenthesized (see the struct-literal
				// codegen comment) to avoid Go parsing &Type{...}.Field as
				// &(Type{...}.Field). The `is` narrowing result is wrapped in
				// cajaShare: it aliases animal's own underlying struct (an
				// IsExpression isn't in isOwned's fresh-temporary list), so
				// mutating the narrowed value must not corrupt a second,
				// independent narrowing of the same union value.
				"var animal Animal = (&Cat{\nName: \"Tom\",\n})",
				"var cat *Cat = cajaShare(func() *Cat { if v, ok := (animal).(*Cat); ok { return v }; return nil }())",
			},
		},
		{
			name: "Safe Pipeline",
			input: `
				type Customer struct { age Number }
				let process = fn(c: Customer) -> Customer { return c }
				let x: Customer? = nil
				let y = x ?> process
			`,
			expected: []string{
				"var y *Customer = func(_val *Customer) *Customer { if _val != nil { return process(_val) }; return nil }(x)",
			},
		},
		{
			name: "Named Imports and Aliases",
			input: `
				import { PI, max } from "math"
				import "math" as m
				let max_val = max(10, 20)
				let min_val = m.min(10, 20)
				let pi_val = PI
			`,
			expected: []string{
				"var max_val float64 = math.Max(10.0, 20.0)",
				"var min_val float64 = math.Min(10.0, 20.0)",
				"var pi_val float64 = math.Pi",
			},
		},
		{
			name: "Anonymous functions",
			input: `
				let double = (p: Number) => p * 2
				let sum: fn(Number, Number) -> Number = (p, q) => p + q
				
				let applyOp = fn(op: fn(Number, Number) -> Number) -> Number {
				   return op(10, 20)
				}
				let res = applyOp((x, y) => x + y)
			`,
			expected: []string{
				"var double func(float64) float64 = func(p float64) float64 {\n\treturn (p * 2.0)\n}",
				"var sum func(float64, float64) float64 = func(p float64, q float64) float64 {\n\treturn (p + q)\n}",
				"var res float64 = applyOp(func(x float64, y float64) float64 {\n\treturn (x + y)\n})",
			},
		},
		{
			name: "Variable declarations",
			input: `
				let x = 10
				const y = "hello"
			`,
			expected: []string{
				"var x float64 = 10",
				"y := \"hello\" // const",
				"_ = x",
				"_ = y",
			},
		},
		{
			name: "Arithmetic operations",
			input: `
				let result = (10 + 5) * 2
			`,
			expected: []string{
				"var result float64 = ((10.0 + 5.0) * 2.0)",
				"_ = result",
			},
		},
		{
			name: "Power operator",
			input: `
				let result = 2 ^ 10
			`,
			expected: []string{
				"var result float64 = math.Pow(2.0, 10.0)",
				"import \"math\"",
			},
		},
		{
			name: "Variable assignment",
			input: `
				let x = 10
				x = 20
			`,
			expected: []string{
				"var x float64 = 10",
				"x = 20",
			},
		},
		{
			name: "Array literal",
			input: `
				import "array" as array
				import "math" as math
				import "string" as string
				import "date" as date
				let arr = [1, 2, 3]
			`,
			expected: []string{
				"var arr *cajaArray[float64] = (&cajaArray[float64]{Data: []float64{1.0, 2.0, 3.0}})",
			},
		},
		{
			name: "Array indexing",
			input: `
				import "array" as array
				import "math" as math
				import "string" as string
				import "date" as date
				let arr = [10, 20]
				let x = arr[1]
			`,
			expected: []string{
				"var x float64 = arr.Data[int(1.0)]",
			},
		},
		{
			name: "Array index assignment",
			input: `
				import "array" as array
				import "math" as math
				import "string" as string
				import "date" as date
				let arr = [1, 2, 3]
				arr[0] = 5
			`,
			expected: []string{
				"arr.Data[int(0.0)] = 5.0",
			},
		},
		{
			name: "Matrix literal",
			input: `
				let matrix = [[1, 2], [3, 4]]
			`,
			expected: []string{
				"var matrix *cajaArray[*cajaArray[float64]] = (&cajaArray[*cajaArray[float64]]{Data: []*cajaArray[float64]{(&cajaArray[float64]{Data: []float64{1.0, 2.0}}), (&cajaArray[float64]{Data: []float64{3.0, 4.0}})}})",
			},
		},
		{
			name: "Matrix indexing",
			input: `
				let matrix = [[1, 2], [3, 4]]
				let x = matrix[1][0]
			`,
			expected: []string{
				"var x float64 = matrix.Data[int(1.0)].Data[int(0.0)]",
			},
		},
		{
			name: "Map literal",
			input: `
				let m = {"a": 1}
			`,
			expected: []string{
				"var m *cajaMap[string, float64] = (&cajaMap[string, float64]{Data: map[string]float64{\"a\": 1.0}})",
			},
		},
		{
			name: "Map assignment and indexing",
			input: `
				let m: map[String]Number = {}
				m["a"] = 1
				let x = m["a"]
			`,
			expected: []string{
				"var m *cajaMap[string, float64] = (&cajaMap[string, float64]{Data: map[string]float64{}})",
				"m.Data[\"a\"] = 1",
				"var x float64 = m.Data[\"a\"]",
			},
		},
		{
			name: "Type Aliases",
			input: `
				type Money Number
				type Matrix [[Number]]
				type StringMap map[String]String
				type Predicate fn(Number) -> Boolean
				type Callback fn(String, Number)
			`,
			expected: []string{
				"type Money float64",
				"type Matrix *cajaArray[*cajaArray[float64]]",
				"type StringMap *cajaMap[string, string]",
				"type Predicate func(float64) bool",
				"type Callback func(string, float64)",
			},
		},
		{
			name: "Structs",
			input: `
				type Node struct {
					value Number
					left Node?
					right Node?
				}
				type Dog struct {
					bark fn() -> String
				}
				let root = Node{value: 10, left: nil, right: nil}
				root.value = 20
				let myDog = Dog{bark: fn() -> String { return "woof" }}
				let b = myDog.bark()
			`,
			expected: []string{
				"type Node struct {",
				"Value float64",
				"Left *Node",
				"Right *Node",
				"}",
				"type Dog struct {",
				"Bark func() string",
				"}",
				"var root *Node = (&Node{",
				"Value: 10.0,",
				"Left: nil,",
				"Right: nil,",
				"}",
				"root.Value = 20",
				"var myDog *Dog = (&Dog{",
				"Bark: func() string {",
				"return \"woof\"",
				"}",
				"var b string = myDog.Bark()",
			},
		},
		{
			name: "Functions and Closures",
			input: `
				let makeMultiplier = fn(factor: Number) -> fn(Number) -> Number {
					return fn(x: Number) -> Number {
						return x * factor
					}
				}
				
				let apply = fn(f: fn(Number) -> Number, val: Number) -> Number {
					return f(val)
				}
				
				let double = makeMultiplier(2)
				let result = apply(double, 10)
			`,
			expected: []string{
				"var makeMultiplier func(float64) func(float64) float64 = func(factor float64) func(float64) float64 {",
				"return func(x float64) float64 {",
				"return (x * factor)",
				"}",
				"}",
				"var apply func(func(float64) float64, float64) float64 = func(f func(float64) float64, val float64) float64 {",
				"return f(val)",
				"}",
				"var double func(float64) float64 = makeMultiplier(2.0)",
				"var result float64 = apply(double, 10.0)",
			},
		},
		{
			name: "Safe Navigation",
			input: `
				type Node struct {
					value Number
					left Node?
				}
				let tree: Node? = nil
				let val = tree?.left?.value
			`,
			expected: []string{
				"type Node struct {",
				"Value float64",
				"Left *Node",
				"}",
				"var tree *Node = nil",
				"var val *float64 = func(obj *Node) *float64 { if obj != nil { v := obj.Value; return &v }; return nil }(func(obj *Node) *Node { if obj != nil { return obj.Left }; return nil }(tree))",
			},
		},
		{
			name: "Tail Call Optimization",
			input: `
				let fact = fn(n: Number, acc: Number) -> Number {
					if (n == 0) {
						return acc
					}
					return fact(n-1, acc*n)
				}
				let val = fact(10, 1)
			`,
			expected: []string{
				"var fact func(float64, float64) float64 = func(n float64, acc float64) float64 {",
				"for {",
				"if (n == 0.0) {",
				"return acc",
				"}",
				"_tco0 := (n - 1.0)",
				"_tco1 := (acc * n)",
				"n = _tco0",
				"acc = _tco1",
				"continue",
				"}",
				"}",
				"var val float64 = fact(10.0, 1.0)",
			},
		},
		{
			name: "Non-Tail Self-Recursion Uses Two-Step Declaration",
			input: `
				let factorial = fn(n: Number) -> Number {
					if (n == 0) {
						return 1
					}
					return n * factorial(n - 1)
				}
				let val = factorial(5)
			`,
			expected: []string{
				"var factorial func(float64) float64\n",
				"factorial = func(n float64) float64 {",
				"return (n * factorial((n - 1.0)))",
				"var val float64 = factorial(5.0)",
			},
		},
		{
			// A "-> Nothing" function returning a call to a DIFFERENT
			// "-> Nothing" function (not itself — that's the tail-call
			// rewrite case above) must not wrap the call in "_ = ...": the
			// callee has zero Go return values, so "_ = innerFn()" is
			// itself invalid Go ("(no value) used as value") — this is
			// exactly the shape a public wrapper delegating to a private
			// recursive helper takes (e.g. @caja/std's
			// forEachIndexed/_forEachIndexed pair).
			name: "Return Of A Different Nothing-Returning Call Skips Discard Assignment",
			input: `
				let inner = fn(n: Number) -> Nothing {
					return
				}
				let outer = fn(n: Number) -> Nothing {
					return inner(n)
				}
				outer(5)
			`,
			expected: []string{
				"\tinner(n)\n\treturn",
			},
		},
		{
			name: "String Interpolation",
			input: `
				let name = "World"
				let count = 5
				let greeting = "Hello, ${name}! count=${count}"
			`,
			expected: []string{
				// name is already String-typed, so it's emitted bare; count
				// is Number-typed, so it's wrapped in caja_format_value —
				// the same runtime helper log.info's args formatting already
				// uses — to auto-stringify it (Kotlin-style automatic
				// toString()), rather than requiring the user to cast it.
				`var greeting string = ("Hello, " + name + "! count=" + caja_format_value(count))`,
			},
		},
		{
			name: "String Format With Literal Format String",
			input: `
				import string
				let price = 3.14159
				let count = 5
				let precise = string.format("%.2f", price)
				let padded = string.format("%5d", count)
				let label = string.format("Hello, %s!", "Caja")
			`,
			expected: []string{
				// %.2f is float-family: the value passes through as-is (a
				// plain float64 already works for that verb).
				`var precise string = fmt.Sprintf("%.2f", price)`,
				// %5d is integer-family, and Caja's Number always compiles
				// to Go float64 — which fmt.Sprintf rejects for %d-family
				// verbs (emitting "%!d(float64=...)") unless wrapped in
				// int(...) first, which is only possible to do here because
				// the format string is a compile-time literal.
				`var padded string = fmt.Sprintf("%5d", int(count))`,
				// %s is a generic verb: no int(...) wrapping, even though
				// the value here is a String, not a Number.
				`var label string = fmt.Sprintf("Hello, %s!", "Caja")`,
			},
		},
		{
			name: "String Format With Runtime-Computed Format String",
			input: `
				import string
				let fmtStr = string.concat("%", "d")
				let count = 5
				let result = string.format(fmtStr, count)
			`,
			expected: []string{
				// The format string isn't a literal, so its verb can't be
				// inspected at compile time — the value passes through
				// un-wrapped, accepting Go's own non-panicking "%!d(...)"
				// mismatch behavior as the documented limitation for this
				// case (see transpileStringFormatCall's doc comment).
				`var result string = fmt.Sprintf(fmtStr, count)`,
			},
		},
		{
			name: "Pipeline Operator",
			input: `
				let isEven = fn(x: Number) -> Boolean { return x % 2 == 0 }
				let filter = fn(arr: [Number], f: fn(Number) -> Boolean) -> [Number] { return arr }
				let mapArr = fn(arr: [Number], f: fn(Number) -> Number) -> [Number] { return arr }
				
				let result = [1, 2, 3] |> filter(isEven) |> mapArr(fn(x: Number) -> Number { return x * 2 })
			`,
			expected: []string{
				"var isEven func(float64) bool = func(x float64) bool {",
				"return (math.Mod(x, 2.0) == 0.0)",
				"}",
				"var filter func(*cajaArray[float64], func(float64) bool) *cajaArray[float64] = func(arr *cajaArray[float64], f func(float64) bool) *cajaArray[float64] {",
				"return arr",
				"}",
				"var mapArr func(*cajaArray[float64], func(float64) float64) *cajaArray[float64] = func(arr *cajaArray[float64], f func(float64) float64) *cajaArray[float64] {",
				"return arr",
				"}",
				"var result *cajaArray[float64] = mapArr(filter((&cajaArray[float64]{Data: []float64{1.0, 2.0, 3.0}}), isEven), func(x float64) float64 {",
				"return (x * 2.0)",
				"})",
			},
		},
		{
			name: "Stream Pipeline",
			input: `
				type Sale struct { amount Number }

				let calcDiscount = fn(s: Sale, pct: Number) -> Sale {
					return Sale { amount: s.amount - (s.amount * pct / 100) }
				}
				let calcProfit = fn(s: Sale) -> Number { return s.amount * 0.3 }

				let sales = [Sale { amount: 100 }]
				let result = sales |>> calcDiscount(5) |>> calcProfit
			`,
			expected: []string{
				"var result *cajaArray[float64] = func() *cajaArray[float64] {",
				"_stream0_done := make(chan struct{})",
				"defer close(_stream0_done)",
				"_stream0_ch0 := make(chan *Sale)",
				"go func() {",
				"defer close(_stream0_ch0)",
				"for _, v := range sales.Data {",
				"select {",
				"case _stream0_ch0 <- v:",
				"case <-_stream0_done:",
				"_stream0_ch1 := make(chan *Sale)",
				"out := calcDiscount(v, 5.0)",
				"case _stream0_ch1 <- out:",
				"_stream0_ch2 := make(chan float64)",
				"out := calcProfit(v)",
				"case _stream0_ch2 <- out:",
				"_stream0_out := &cajaArray[float64]{}",
				"for v := range _stream0_ch2 {",
				"_stream0_out.Data = append(_stream0_out.Data, v)",
				"return _stream0_out",
			},
		},
		{
			name: "Safe Stream Pipeline drops nil items before calling the stage",
			input: `
				type Sale struct { amount Number }
				define BigSale constraints Sale with: fn(s: Sale) -> Boolean { return s.amount > 150 }

				let calcProfit = fn(s: BigSale) -> Number { return s.amount * 0.3 }

				let s1: BigSale? = nil
				let sales = [s1]
				let result = sales ?>> calcProfit
			`,
			expected: []string{
				"_stream0_ch0 := make(chan *BigSale)",
				"for v := range _stream0_ch0 {",
				"if v == nil {",
				"continue",
				"}",
				"out := calcProfit(v)",
			},
		},
		{
			name: "Stream Pipeline Parallel Join",
			input: `
				type Loan struct { principal Number }
				type Calendar struct { isHoliday Boolean }

				let resolveCalendar = fn(loan: Loan) -> Calendar { return Calendar { isHoliday: false } }
				let fetchIndexRate = fn(loan: Loan) -> Number { return 5 }
				let fetchFees = fn(loan: Loan) -> Number { return 1 }
				let calculatePnl = fn(cal: Calendar, rate: Number, fees: Number) -> Number { return rate + fees }

				let loans = [Loan { principal: 100 }]
				let pnl = loans |>> (resolveCalendar & fetchIndexRate & fetchFees) |>> calculatePnl
			`,
			expected: []string{
				"import \"sync\"",
				"var pnl *cajaArray[float64] = func() *cajaArray[float64] {",
				"_stream0_ch0 := make(chan *Loan)",
				// exactly one fused stage/channel — the join group does not
				// get its own separate channel boundary
				"_stream0_ch1 := make(chan float64)",
				"var _stream0_stage0_join0 *Calendar",
				"var _stream0_stage0_join1 float64",
				"var _stream0_stage0_join2 float64",
				"var _stream0_stage0_wg sync.WaitGroup",
				"_stream0_stage0_wg.Add(3)",
				"go func() {",
				"defer _stream0_stage0_wg.Done()",
				"_stream0_stage0_join0 = resolveCalendar(v)",
				"_stream0_stage0_join1 = fetchIndexRate(v)",
				"_stream0_stage0_join2 = fetchFees(v)",
				"_stream0_stage0_wg.Wait()",
				"out := calculatePnl(_stream0_stage0_join0, _stream0_stage0_join1, _stream0_stage0_join2)",
			},
		},
		{
			name: "Builtins",
			input: `
				import "array" as array
				import "math" as math
				import "string" as string
				import "date" as date
				let arr = [1, 2, 3]
				let arr2 = array.push(arr, 4)
				let p = array.pop(arr2)
				let l = array.len(p)
				let m = math.abs(-5)
				let s = string.toUpper("caja")
				let d = date.today()
				let pi = math.PI
			`,
			expected: []string{
				"var arr *cajaArray[float64] = (&cajaArray[float64]{Data: []float64{1.0, 2.0, 3.0}})",
				"var arr2 *cajaArray[float64] = caja_array_push(arr, 4.0)",
				"var p *cajaArray[float64] = caja_array_pop(arr2)",
				"var l float64 = float64(len(p.Data))",
				"var m float64 = math.Abs((-5.0))",
				"var s string = strings.ToUpper(\"caja\")",
				"var d time.Time = caja_date_today()",
				"var pi float64 = math.Pi",
			},
		},
		{
			name: "Browser builtins",
			input: `
				import browser
				let el = browser.getElementById("app")
				browser.setText(el, "hi")
				browser.setHTML(el, "<b>hi</b>")
				browser.log("hello")
				browser.alert("hi")
				browser.setValue(el, "typed value")
				let v = browser.getValue(el)
			`,
			expected: []string{
				"var el js.Value = js.Global().Get(\"document\").Call(\"getElementById\", \"app\")",
				"el.Set(\"textContent\", \"hi\")",
				"el.Set(\"innerHTML\", \"<b>hi</b>\")",
				"js.Global().Get(\"console\").Call(\"log\", \"hello\")",
				"js.Global().Call(\"alert\", \"hi\")",
				"el.Set(\"value\", \"typed value\")",
				`var v string = el.Get("value").String()`,
				"import \"syscall/js\"",
				// Every browser-module program blocks forever at the end of
				// main, not just ones that register an event listener — see
				// the comment above this emission in transpiler.go for why
				// this isn't gated on "on" specifically, and why it's a
				// time.Sleep loop rather than a bare select{} (the latter is
				// fragile once anything else, like browser.fetch, also
				// blocks on an ordinary channel waiting for a JS callback).
				"for {",
				"time.Sleep(time.Second)",
			},
		},
		{
			name: "Browser on registers a listener for the given event and blocks main forever",
			input: `
				import browser
				let el = browser.getElementById("btn")
				let handleClick = fn() -> Nothing {
					browser.log("clicked")
				}
				browser.on("click", el, handleClick)
			`,
			expected: []string{
				"var handleClick func() = func() {",
				`el.Call("addEventListener", "click", js.FuncOf(func(this js.Value, args []js.Value) any {`,
				"handleClick()",
				"return nil",
				// A registered js.FuncOf listener only keeps working while the
				// wasm instance's Go runtime is still scheduling — main must
				// never return, or the event can't reach the Go closure
				// anymore (see the comment above this emission in
				// transpiler.go for why this is a time.Sleep loop, not a bare
				// select{}).
				"for {",
				"time.Sleep(time.Second)",
			},
		},
		{
			// The event name is just passed straight through to
			// addEventListener with no Caja-side enumeration, so a
			// non-"click" event needs no special-casing anywhere.
			name: "Browser on works with an arbitrary event name",
			input: `
				import browser
				let el = browser.getElementById("name")
				let handleInput = fn() -> Nothing {
					browser.log("input changed")
				}
				browser.on("input", el, handleInput)
			`,
			expected: []string{
				`el.Call("addEventListener", "input", js.FuncOf(func(this js.Value, args []js.Value) any {`,
			},
		},
		{
			name: "Browser fetch call is a plain blocking Go call",
			input: `
				import browser
				let body = browser.fetch("/data")
				browser.log(body)
			`,
			expected: []string{
				`var body string = caja_browser_fetch("/data")`,
				`js.Global().Get("console").Call("log", body)`,
			},
		},
		{
			// async/unwrap need no fetch-specific codegen: async's existing
			// goroutine-wrapping IIFE (transpileAsyncExpression) applies to
			// caja_browser_fetch's call expression exactly like it would to
			// any user function call.
			name: "Browser fetch composes with async/unwrap using existing codegen",
			input: `
				import browser
				let t = async browser.fetch("/data")
				let body = unwrap t
			`,
			expected: []string{
				"func() *asyncTask {",
				`t.val = caja_browser_fetch("/data")`,
				"<-(t).done",
				"(t).val.(string)",
			},
		},
		{
			// fetchThen is the safe way to consume a fetch from inside a
			// browser.on handler: purely callback-driven codegen, no
			// channel/blocking construct at the call site at all.
			name: "Browser fetchThen is purely callback-driven",
			input: `
				import browser
				let handleBody = fn(body: String) -> Nothing {
					browser.log(body)
				}
				browser.fetchThen("/data", handleBody)
			`,
			expected: []string{
				"var handleBody func(string) = func(body string) {",
				`caja_browser_fetch_then("/data", func(body string) { handleBody(body) })`,
			},
		},
		{
			// on's handler dispatch must go through caja_wrap_callback so
			// a panic inside the handler gets clean caja_panic_location()
			// formatting instead of crashing with a raw Go stack trace on a
			// goroutine main()'s own recover never sees.
			name: "Browser on wraps the handler call in caja_wrap_callback",
			input: `
				import browser
				let el = browser.getElementById("btn")
				let handleClick = fn() -> Nothing {
					browser.log("clicked")
				}
				browser.on("click", el, handleClick)
			`,
			expected: []string{
				`js.FuncOf(func(this js.Value, args []js.Value) any {`,
				"caja_wrap_callback(func() { handleClick() })",
			},
		},
		{
			// querySelector's Element? maps to *js.Value (mapSymbolToGoType's
			// NullableSymbol case), and querySelectorAll's Array<Element> maps
			// to *cajaArray[js.Value] (the same array representation
			// string.split etc. already use) — neither invents a new
			// representation.
			name: "Browser querySelector is nullable, querySelectorAll returns Array<Element>",
			input: `
				import browser
				let el = browser.querySelector(".item")
				let els = browser.querySelectorAll(".item")
			`,
			expected: []string{
				`var el *js.Value = caja_browser_query_selector(".item")`,
				`var els *cajaArray[js.Value] = caja_browser_query_selector_all(".item")`,
			},
		},
		{
			name: "Browser setAttribute, addClass, and removeClass are plain calls",
			input: `
				import browser
				let el = browser.getElementById("box")
				browser.setAttribute(el, "data-role", "widget")
				browser.addClass(el, "highlight")
				browser.removeClass(el, "a")
			`,
			expected: []string{
				`el.Call("setAttribute", "data-role", "widget")`,
				`el.Get("classList").Call("add", "highlight")`,
				`el.Get("classList").Call("remove", "a")`,
			},
		},
		{
			// getAttribute's String? maps to *string, the same nullable
			// representation querySelector's Element? uses — see
			// caja_browser_get_attribute's own injected comment.
			name: "Browser getAttribute is nullable",
			input: `
				import browser
				let el = browser.getElementById("box")
				let role = browser.getAttribute(el, "data-role")
			`,
			expected: []string{
				`var role *string = caja_browser_get_attribute(el, "data-role")`,
			},
		},
		{
			// cast.to must dereference a Nullable input (a *T pointer, e.g.
			// String? from browser.getAttribute) rather than forward it raw
			// — passing that pointer straight into the identity/fmt.Sprintf
			// paths below would either type-mismatch or, worse, compile fine
			// as `any` and panic at runtime inside syscall/js.ValueOf (a real
			// bug this pins down: confirmed via headless Chrome that a
			// pre-fix build of this exact pattern panicked with "ValueOf:
			// invalid value"). A nil input must fall through to the fallback
			// exactly like every other "couldn't produce a value" case here
			// (an unparseable string, etc.) already does.
			name: "cast.to dereferences a Nullable input with a nil-safe fallback",
			input: `
				import browser
				import cast
				let el = browser.getElementById("box")
				let role = browser.getAttribute(el, "data-role")
				let display = cast.to(role, "not set")
			`,
			expected: []string{
				"func() string {",
				"p := role",
				"if p == nil {",
				`return "not set"`,
				"v := *p",
				"return v",
			},
		},
		{
			name: "Browser createElement, appendChild, removeElement, and setStyle are plain calls",
			input: `
				import browser
				let list = browser.getElementById("list")
				let item = browser.createElement("li")
				browser.appendChild(list, item)
				browser.setStyle(item, "color", "blue")
				browser.removeElement(item)
			`,
			expected: []string{
				`js.Global().Get("document").Call("createElement", "li")`,
				`list.Call("appendChild", item)`,
				`item.Get("style").Call("setProperty", "color", "blue")`,
				`item.Call("remove")`,
			},
		},
		{
			name: "Browser removeAttribute, toggleClass, and hasClass are plain calls",
			input: `
				import browser
				let el = browser.getElementById("box")
				browser.removeAttribute(el, "data-open")
				browser.toggleClass(el, "open")
				let isOpen = browser.hasClass(el, "open")
			`,
			expected: []string{
				`el.Call("removeAttribute", "data-open")`,
				`el.Get("classList").Call("toggle", "open")`,
				`var isOpen bool = el.Get("classList").Call("contains", "open").Bool()`,
			},
		},
		{
			name: "Browser focus and blur are plain calls",
			input: `
				import browser
				let el = browser.getElementById("name")
				browser.focus(el)
				browser.blur(el)
			`,
			expected: []string{
				`el.Call("focus")`,
				`el.Call("blur")`,
			},
		},
		{
			name: "Browser getChecked and setChecked read/write the live checked property",
			input: `
				import browser
				let box = browser.getElementById("remember")
				let checked = browser.getChecked(box)
				browser.setChecked(box, true)
			`,
			expected: []string{
				`var checked bool = box.Get("checked").Bool()`,
				`box.Set("checked", true)`,
			},
		},
		{
			name: "Browser insertBefore is a plain call",
			input: `
				import browser
				let list = browser.getElementById("list")
				let ref = browser.getElementById("a-item")
				let newItem = browser.createElement("li")
				browser.insertBefore(list, newItem, ref)
			`,
			expected: []string{
				`list.Call("insertBefore", newItem, ref)`,
			},
		},
		{
			// setTimeout's handler is wrapped in caja_wrap_callback, the same
			// machinery on's listener uses, and JS's own (callback, delay)
			// argument order is swapped relative to Caja's (delayMs, handler).
			name: "Browser setTimeout wraps the handler and returns a Number, clearTimeout is a plain call",
			input: `
				import browser
				let id = browser.setTimeout(2000, fn() -> Nothing { browser.log("fired") })
				browser.clearTimeout(id)
			`,
			expected: []string{
				`js.Global().Call("setTimeout", js.FuncOf(func(this js.Value, args []js.Value) any {`,
				`caja_wrap_callback(func() {`,
				`}), 2000.0).Float()`,
				`js.Global().Call("clearTimeout", id)`,
			},
		},
		{
			// localStorageGet's String? maps to *string, the same nullable
			// representation getAttribute's String? uses.
			name: "Browser localStorageGet is nullable, localStorageSet and localStorageRemove are plain calls",
			input: `
				import browser
				let theme = browser.localStorageGet("theme")
				browser.localStorageSet("theme", "dark")
				browser.localStorageRemove("theme")
			`,
			expected: []string{
				`var theme *string = caja_browser_local_storage_get("theme")`,
				`js.Global().Get("localStorage").Call("setItem", "theme", "dark")`,
				`js.Global().Get("localStorage").Call("removeItem", "theme")`,
			},
		},
		{
			name: "active declares a cajaActive cell and assignment uses Set",
			input: `
let active counter = 0
counter = counter + 1
`,
			expected: []string{
				`var counter *cajaActive[float64] = newCajaActive(0.0)`,
				`counter.Set((counter.Get() + 1.0))`,
			},
		},
		{
			name: "a plain (non-active) let assignment is untouched by the active machinery",
			input: `
let active counter = 0
let plain = 1
plain = 2
`,
			expected: []string{
				`var plain float64 = 1.0`,
				`plain = 2.0`,
			},
		},
		{
			name: "reactive call registers a dependent and computes the initial value",
			input: `
import cast
let active counter = 0
let reactFn = fn(c: Number) -> String { return cast.to(c, "") }
let active result = reactFn(react counter)
`,
			expected: []string{
				`var result *cajaActive[string] = func() *cajaActive[string] {`,
				`cell := newCajaActive(reactFn(counter.Get()))`,
				`cell.recompute = func() { cell.cajaSetFromRecompute(reactFn(counter.Get())) }`,
				`counter.addDependent(cell)`,
				`return cell`,
				`}()`,
			},
		},
		{
			name: "reactive call with multiple react params registers a dependent on each",
			input: `
let active a = 0
let active b = 1
let combine = fn(x: Number, y: Number) -> Number { return x + y }
let active total = combine(react a, react b)
`,
			expected: []string{
				`cell := newCajaActive(combine(a.Get(), b.Get()))`,
				`cell.recompute = func() { cell.cajaSetFromRecompute(combine(a.Get(), b.Get())) }`,
				`a.addDependent(cell)`,
				`b.addDependent(cell)`,
			},
		},
		{
			name: "Move semantics in array push",
			input: `
import "array"
let a = [1, 2, 3]
let b = array.push(move a, 4)
`,
			expected: []string{
				"caja_array_push_owned(a, 4.0)", // Should use zero-copy in-place append
			},
		},
		{
			name: "Pipeline temporary values are implicitly moved",
			input: `
import "array"
let a = [1, 2, 3]
let b = a |> array.push(4) |> array.push(5)
`,
			expected: []string{
				"caja_array_push_owned(caja_array_push(a, 4.0), 5.0)",
			},
		},
		{
			name: "Move in pipeline",
			input: `
import "array"
let a = [1, 2, 3]
let b = move a |> array.push(4)
`,
			expected: []string{
				"caja_array_push_owned(a, 4.0)",
			},
		},
		{
			name: "Memoized Function",
			input: `
				let powerTwo = memo fn(n: Number) -> Number {
					return n * n
				}
				let val = powerTwo(4)
			`,
			expected: []string{
				"var _memo_powerTwo sync.Map",
				"var powerTwo func(n float64) float64",
				"var powerTwo_impl func(n float64) float64",
				"key := n",
				"if v, ok := _memo_powerTwo.Load(key); ok {",
				"return v.(float64)",
				"result := powerTwo_impl(n)",
				"_memo_powerTwo.Store(key, result)",
				"powerTwo_impl = func(n float64) float64 {",
				"import \"sync\"",
			},
		},
		{
			name: "Memoized Function via const",
			input: `
				const powerTwo = memo fn(n: Number) -> Number {
					return n * n
				}
				const val = powerTwo(4)
			`,
			expected: []string{
				"var _memo_powerTwo sync.Map",
				"var powerTwo func(n float64) float64",
				"var powerTwo_impl func(n float64) float64",
				"key := n",
				"if v, ok := _memo_powerTwo.Load(key); ok {",
				"return v.(float64)",
				"result := powerTwo_impl(n)",
				"_memo_powerTwo.Store(key, result)",
				"powerTwo_impl = func(n float64) float64 {",
			},
		},
		{
			name: "Memoized Function With Array Parameter",
			input: `
				import "array"
				let sum = memo fn(list: [Number]) -> Number {
					if (array.len(list) == 0) {
						return 0
					}
					return array.head(list) + sum(array.tail(list))
				}
				let val = sum([1, 2, 3])
			`,
			expected: []string{
				"key := caja_memo_hash(list)",
				"func caja_memo_hash(v any) uint64 {",
				"import \"hash/fnv\"",
				"import \"encoding/json\"",
			},
		},
		{
			name: "Async/Unwrap basic",
			input: `
let compute = fn(n: Number) -> Number { return n * 2 }
let p = async compute(1)
let r = unwrap p
`,
			expected: []string{
				"type asyncTask struct",
				"go func() {",
				"close(t.done)",
				"<-(p).done",
				".val.(float64)",
			},
		},
		{
			name: "Await join barrier",
			input: `
let compute = fn(n: Number) -> Number { return n * 2 }
let p1 = async compute(1)
let p2 = async compute(2)
await p1 & p2
`,
			expected: []string{
				"<-(p1).done",
				"<-(p2).done",
			},
		},
		{
			name: "Async/Unwrap work with const statements too, not just let",
			input: `
let compute = fn(n: Number) -> Number { return n * 2 }
const p = async compute(1)
const r = unwrap p
`,
			expected: []string{
				"asyncTask{done: make(chan struct{})}",
				".val.(float64)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the input script to get the AST
			program, _, a, err := script.ParseWithDir(tt.input, "", "test.caja")
			if err != nil {
				t.Fatalf("Failed to parse script: %v", err)
			}

			// Transpile the AST
			goCode, err := Transpile(program, a, TranspileOptions{})
			if err != nil {
				t.Fatalf("Transpile failed: %v", err)
			}

			// These assertions check codegen shape, not the //line directives
			// Transpile now interleaves per-statement (see TestLineDirectives)
			// — strip them so directives inserted mid-block (e.g. inside a
			// nested function literal's body) don't break a substring match.
			goCodeNoDirectives := stripLineDirectives(goCode)

			// Verify the expected Go code is present
			for _, exp := range tt.expected {
				if !strings.Contains(goCodeNoDirectives, exp) {
					t.Errorf("Expected output to contain:\n%s\n\nGot:\n%s", exp, goCodeNoDirectives)
				}
			}
		})
	}
}

// TestMaybeShareValueElidesDeadRebind pins the codegen shape for the
// dead-name elision in maybeShareValue: rebinding a parameter to a local
// that mutates-and-returns it (the pattern forced by parameter immutability)
// must not wrap the rebind in cajaShare when the parameter's name is never
// read again — that rebind is the last live reference, not a new alias.
func TestMaybeShareValueElidesDeadRebind(t *testing.T) {
	input := `
		type Portfolio struct { value Number }
		let addInterest = fn(p: Portfolio) -> Portfolio {
			let local = p
			local.value = local.value + 10
			return local
		}
	`
	program, _, a, err := script.ParseWithDir(input, "", "test.caja")
	if err != nil {
		t.Fatalf("Failed to parse script: %v", err)
	}
	goCode, err := Transpile(program, a, TranspileOptions{})
	if err != nil {
		t.Fatalf("Transpile failed: %v", err)
	}
	goCode = stripLineDirectives(goCode)

	if strings.Contains(goCode, "local *Portfolio = cajaShare(p)") {
		t.Errorf("expected the dead rebind `let local = p` to skip cajaShare, got:\n%s", goCode)
	}
	if !strings.Contains(goCode, "local *Portfolio = p") {
		t.Errorf("expected `let local = p` to transpile to a plain assignment, got:\n%s", goCode)
	}
}

// TestIsExpressionOwnedWhenSourceIsOwned pins isOwned's *ast.IsExpression
// case: a union `is` narrowing extracts the same underlying pointer its
// source already holds, so it's only safe to skip cajaShare when that source
// itself is owned (here, `move a` — the analyzer has already guaranteed no
// other reference to a's value exists). Narrowing a NON-owned source (b,
// used with no move) must still get cajaShare, since the narrowed pointer
// could otherwise be aliased by whatever else already references b.
func TestIsExpressionOwnedWhenSourceIsOwned(t *testing.T) {
	input := `
		type Cat struct { name String }
		union Animal = Cat
		let a: Animal = Cat{name: "Tom"}
		let cat: Cat? = move a is Cat
		let b: Animal = Cat{name: "Rex"}
		let cat2: Cat? = b is Cat
	`
	program, _, a, err := script.ParseWithDir(input, "", "test.caja")
	if err != nil {
		t.Fatalf("Failed to parse script: %v", err)
	}
	goCode, err := Transpile(program, a, TranspileOptions{})
	if err != nil {
		t.Fatalf("Transpile failed: %v", err)
	}
	goCode = stripLineDirectives(goCode)

	if !strings.Contains(goCode, "var cat *Cat = func() *Cat { if v, ok := (a).(*Cat); ok { return v }; return nil }()") {
		t.Errorf("expected `move a is Cat` to skip cajaShare (a is provably the sole owner), got:\n%s", goCode)
	}
	if !strings.Contains(goCode, "var cat2 *Cat = cajaShare(func() *Cat { if v, ok := (b).(*Cat); ok { return v }; return nil }())") {
		t.Errorf("expected `b is Cat` (no move) to still wrap in cajaShare, got:\n%s", goCode)
	}
}

// stripLineDirectives removes every `//line file:N` directive Transpile
// interleaves into the generated source (see lineDirective), so tests that
// assert on codegen shape don't need to account for them appearing mid-block.
func stripLineDirectives(code string) string {
	lines := strings.Split(code, "\n")
	kept := lines[:0]
	for _, line := range lines {
		if strings.HasPrefix(line, "//line ") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// TestTranspileEmitsLineDirectives verifies each top-level statement gets a
// `//line <file>:<N>` directive at its own source line, at column 0 (required
// for the Go compiler to honor it — see lineDirective) — the mechanism that
// lets a runtime panic or compile error in the generated binary be reported
// against the original .caja file/line instead of a deleted temp Go file.
func TestTranspileEmitsLineDirectives(t *testing.T) {
	input := "let arr = [1, 2, 3]\nlet x = 1\narr[10]\n"
	program, _, a, err := script.ParseWithDir(input, "", "panic_test.caja")
	if err != nil {
		t.Fatalf("failed to parse script: %v", err)
	}

	goCode, err := Transpile(program, a, TranspileOptions{})
	if err != nil {
		t.Fatalf("Transpile failed: %v", err)
	}

	for _, want := range []string{"//line panic_test.caja:1\n", "//line panic_test.caja:2\n"} {
		if !strings.Contains(goCode, want) {
			t.Errorf("expected generated code to contain %q, got:\n%s", want, goCode)
		}
	}
	for _, line := range strings.Split(goCode, "\n") {
		if strings.Contains(line, "//line ") && line != strings.TrimLeft(line, " \t") {
			t.Errorf("found a //line directive with leading whitespace (Go compiler silently ignores these): %q", line)
		}
	}
}

// TestPinRangeToLineCoversStreamPipeAndAsync verifies that
// transpileStreamPipeExpression and transpileAsyncExpression/
// transpileUnwrapExpression — which build multi-line Go (goroutines,
// channels, IIFEs) directly and bypass transpileStatement's per-statement
// directive injection entirely — still get .caja source coverage via
// pinRangeToLine: the SAME directive, for the expression's own line, must
// appear more than once (once per internal line break in the generated
// block), not just once at the top the way a normal statement gets it.
func TestPinRangeToLineCoversStreamPipeAndAsync(t *testing.T) {
	input := `
		let calcDiscount = fn(s: Number, pct: Number) -> Number {
			return s - (s * pct / 100)
		}
		let sales = [100, 200, 300]
		let result = sales |>> calcDiscount(5)

		let fetchRate = fn(x: Number) -> Number {
			return x * 0.05
		}
		let pending = async fetchRate(10)
		let rate = unwrap pending
	`
	program, _, a, err := script.ParseWithDir(input, "", "pipe_test.caja")
	if err != nil {
		t.Fatalf("failed to parse script: %v", err)
	}

	goCode, err := Transpile(program, a, TranspileOptions{})
	if err != nil {
		t.Fatalf("Transpile failed: %v", err)
	}

	// `sales |>> calcDiscount(5)` is on line 6.
	pipeDirective := "//line pipe_test.caja:6\n"
	if n := strings.Count(goCode, pipeDirective); n < 2 {
		t.Errorf("expected %q to be repeated across the stream-pipe block (pinRangeToLine), found %d occurrence(s) in:\n%s", pipeDirective, n, goCode)
	}

	// `async fetchRate(10)` is on line 11.
	asyncDirective := "//line pipe_test.caja:11\n"
	if n := strings.Count(goCode, asyncDirective); n < 2 {
		t.Errorf("expected %q to be repeated across the async block (pinRangeToLine), found %d occurrence(s) in:\n%s", asyncDirective, n, goCode)
	}

	// `unwrap pending` is on line 12.
	unwrapDirective := "//line pipe_test.caja:12\n"
	if n := strings.Count(goCode, unwrapDirective); n < 2 {
		t.Errorf("expected %q to be repeated across the unwrap block (pinRangeToLine), found %d occurrence(s) in:\n%s", unwrapDirective, n, goCode)
	}
}
