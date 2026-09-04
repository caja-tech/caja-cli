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
				"var validate_MajorCustomer func(*Customer) *MajorCustomer = func(val *Customer) *MajorCustomer {\n\tpred := func(c *Customer) bool {\n\treturn (c.Age > 18)\n}\n\tif pred(val) {\n\t\tres := (*MajorCustomer)(val)\n\t\treturn res\n\t}\n\treturn nil\n}",
				"var m *MajorCustomer = validate_MajorCustomer(&Customer{\nAge: 20,\n})",
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
				"var max_val float64 = math.Max(10, 20)",
				"var min_val float64 = math.Min(10, 20)",
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
				"var double func(float64) float64 = func(p float64) float64 {\n\treturn (p * 2)\n}",
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
				"var result float64 = ((10 + 5) * 2)",
				"_ = result",
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
				"var arr []float64 = []float64{1, 2, 3}",
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
				"var x float64 = arr[int(1)]",
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
				"arr[int(0)] = 5",
			},
		},
		{
			name: "Matrix literal",
			input: `
				let matrix = [[1, 2], [3, 4]]
			`,
			expected: []string{
				"var matrix [][]float64 = [][]float64{[]float64{1, 2}, []float64{3, 4}}",
			},
		},
		{
			name: "Matrix indexing",
			input: `
				let matrix = [[1, 2], [3, 4]]
				let x = matrix[1][0]
			`,
			expected: []string{
				"var x float64 = matrix[int(1)][int(0)]",
			},
		},
		{
			name: "Map literal",
			input: `
				let m = {"a": 1}
			`,
			expected: []string{
				"var m map[string]float64 = map[string]float64{\"a\": 1}",
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
				"var m map[string]float64 = map[string]float64{}",
				"m[\"a\"] = 1",
				"var x float64 = m[\"a\"]",
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
				"type Matrix [][]float64",
				"type StringMap map[string]string",
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
				"var root *Node = &Node{",
				"Value: 10,",
				"Left: nil,",
				"Right: nil,",
				"}",
				"root.Value = 20",
				"var myDog *Dog = &Dog{",
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
				"var double func(float64) float64 = makeMultiplier(2)",
				"var result float64 = apply(double, 10)",
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
				"if (n == 0) {",
				"return acc",
				"}",
				"_tco0 := (n - 1)",
				"_tco1 := (acc * n)",
				"n = _tco0",
				"acc = _tco1",
				"continue",
				"}",
				"}",
				"var val float64 = fact(10, 1)",
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
				"return (math.Mod(x, 2) == 0)",
				"}",
				"var filter func([]float64, func(float64) bool) []float64 = func(arr []float64, f func(float64) bool) []float64 {",
				"return arr",
				"}",
				"var mapArr func([]float64, func(float64) float64) []float64 = func(arr []float64, f func(float64) float64) []float64 {",
				"return arr",
				"}",
				"var result []float64 = mapArr(filter([]float64{1, 2, 3}, isEven), func(x float64) float64 {",
				"return (x * 2)",
				"})",
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
				"var arr []float64 = []float64{1, 2, 3}",
				"var arr2 []float64 = caja_array_push(arr, 4)",
				"var p []float64 = caja_array_pop(arr2)",
				"var l float64 = float64(len(p))",
				"var m float64 = math.Abs((-5))",
				"var s string = strings.ToUpper(\"caja\")",
				"var d time.Time = caja_date_today()",
				"var pi float64 = math.Pi",
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
				"append(a, 4)", // Should use zero-copy in-place append
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
				"append(caja_array_push(a, 4), 5)",
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
				"append(a, 4)",
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
			goCode, err := Transpile(program, a)
			if err != nil {
				t.Fatalf("Transpile failed: %v", err)
			}

			// Verify the expected Go code is present
			for _, exp := range tt.expected {
				if !strings.Contains(goCode, exp) {
					t.Errorf("Expected output to contain:\n%s\n\nGot:\n%s", exp, goCode)
				}
			}
		})
	}
}
