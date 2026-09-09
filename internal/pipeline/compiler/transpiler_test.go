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
			`,
			expected: []string{
				"var el js.Value = js.Global().Get(\"document\").Call(\"getElementById\", \"app\")",
				"el.Set(\"textContent\", \"hi\")",
				"el.Set(\"innerHTML\", \"<b>hi</b>\")",
				"js.Global().Get(\"console\").Call(\"log\", \"hello\")",
				"js.Global().Call(\"alert\", \"hi\")",
				"import \"syscall/js\"",
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
