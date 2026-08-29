package analyzer

import (
	"caja-cli/internal/pipeline/environment"
	"caja-cli/internal/pipeline/lexer"
	"caja-cli/internal/pipeline/parser"
	"strings"
	"testing"
)

type testScenario struct {
	name           string
	input          string
	expectedErrors []string
}

func runTestScenarios(t *testing.T, tests []testScenario) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tknzr := lexer.New(tt.input)
			p := parser.New(tknzr)
			program := p.Parse()

			if p.HasErrors() {
				t.Fatalf("parser errors occurred during setup: %v", p.Errors())
			}

			env := environment.NewEnvironment("", "", false)
			analyzer := New(env)
			analyzer.analyze(program)

			errors := analyzer.Errors()

			if len(errors) != len(tt.expectedErrors) {
				t.Fatalf("expected %d errors, got %d. Errors: %v", len(tt.expectedErrors), len(errors), errors)
			}

			for i, err := range errors {
				if !strings.Contains(err, tt.expectedErrors[i]) {
					t.Errorf("expected error to contain %q, got %q", tt.expectedErrors[i], err)
				}
			}
		})
	}
}

func TestSemanticAnalysisSuccess(t *testing.T) {
	tests := []testScenario{
		{
			name: "Valid declarations and assignments",
			input: `
let a = 10
a = 20
`,
			expectedErrors: []string{},
		},
		{
			name: "Valid expressions",
			input: `
let a = 10
let b = a * 2
return b
`,
			expectedErrors: []string{},
		},
		{
			name: "Valid inner scope access (lexical scoping)",
			input: `
let a = 10
if (a > 5) {
	a = 20
}
`,
			expectedErrors: []string{},
		},
		{
			name: "Explicit type at variable declaration with custom type and structs",
			input: `
type MyType fn(Number) -> Number
let a: MyType = nil

type Person struct {
	name String
	age Number
}
let b: Person = nil
`,
			expectedErrors: []string{},
		},
		{
			name: "Type alias of a function signature",
			input: `
type CustomFunc fn(Number) -> Number
let f: CustomFunc = fn(x: Number) -> Number { return x }
`,
			expectedErrors: []string{},
		},
		{
			name: "map.KeyFunc usage",
			input: `
import map
type CustomStruct struct {
	key map.KeyFunc
	value Number
}
let d: map[CustomStruct]Number = {}
`,
			expectedErrors: []string{},
		},
		{
			name: "map with custom function value",
			input: `
import string
type CustomFunc fn(String) -> String

let dict: map[String]CustomFunc = {
	"a": fn(x: String) -> String { return string.concat("A", x) },
	"b": fn(x: String) -> String { return string.concat("B", x) }
}

return string.concat(dict["a"]("x"), dict["b"]("y"))
`,
			expectedErrors: []string{},
		},
		{
			name: "Empty struct definition and instantiation",
			input: `
type EmptyStruct struct {}
let a: EmptyStruct = EmptyStruct {}
`,
			expectedErrors: []string{},
		},
		{
			name: "Function returning Nothing with empty return",
			input: `
let f = fn() -> Nothing {
	return
}
`,
			expectedErrors: []string{},
		},
		{
			name: "Function returning Nothing with implicit return",
			input: `
let f = fn() -> Nothing {
	let a = 1
}
`,
			expectedErrors: []string{},
		},
		{
			name:           "Top level return Nothing struct",
			input:          `return Nothing {}`,
			expectedErrors: []string{},
		},
		{
			name: "Custom empty struct requires explicit return",
			input: `
type MyEmpty struct {}
let f = fn() -> MyEmpty {
}
`,
			expectedErrors: []string{
				"[Line 3, Column 9] semantic error: function is missing a guaranteed return statement. All code paths must return a value.",
				"[Line 3, Column 9] type error: function declared to return MyEmpty, but body returns Any",
			},
		},
		{
			name: "Custom empty struct with explicit return Nothing",
			input: `
type MyEmpty struct {}
let f = fn() -> MyEmpty {
	return Nothing {}
}
`,
			expectedErrors: []string{},
		},
	}
	runTestScenarios(t, tests)
}

func TestSemanticAnalysisErrors(t *testing.T) {
	tests := []testScenario{
		{
			name: "Assignment before declaration",
			input: `
x = 10
`,
			expectedErrors: []string{
				"semantic error: undeclared variable 'x'. Use 'let' to declare it.",
			},
		},
		{
			name: "Usage before declaration in expression",
			input: `
let a = x + 5
`,
			expectedErrors: []string{
				"semantic error: undeclared variable 'x'",
			},
		},
		{
			name: "Out of scope usage (scope leak prevention)",
			input: `
if (1 > 0) {
	let x = 10
}
x = 20
`,
			expectedErrors: []string{
				"semantic error: undeclared variable 'x'. Use 'let' to declare it.",
			},
		},
		{
			name: "Redeclaration in the same scope",
			input: `
let a = 10
let a = 20
`,
			expectedErrors: []string{
				"semantic error: variable 'a' is already declared",
			},
		},
		{
			name: "Redeclaration in an inner scope (shadowing prevention)",
			input: `
let a = 10
if (1 > 0) {
	let a = 20
}
`,
			expectedErrors: []string{
				"semantic error: variable 'a' is already declared",
			},
		},
		{
			name: "Array index assignment on non-array",
			input: `
let a = 10
a[0] = 5
`,
			expectedErrors: []string{
				"type error: index assignment not supported for Number",
			},
		},
		{
			name: "Array index assignment with non-number index",
			input: `
let a = [1, 2, 3]
a["hello"] = 5
`,
			expectedErrors: []string{
				"type error: array index must be Number, got String",
			},
		},
	{
			name: "Undefined custom type in variable declaration",
			input: `
let c: CustomType = nil
`,
			expectedErrors: []string{
				"semantic error: variable 'c' type is not declared: 'CustomType'",
			},
		},
		{
			name: "Mismatched assignment to explicit Number variable",
			input: `
let x: Number = "hello"
`,
			expectedErrors: []string{
				"type error: cannot assign String to Number",
			},
		},
		{
			name: "Mismatched assignment to explicit String variable",
			input: `
let s: String = 42
`,
			expectedErrors: []string{
				"type error: cannot assign Number to String",
			},
		},
		{
			name: "Mismatched assignment to explicit Array variable",
			input: `
let a: [Number] = ["hello"]
`,
			expectedErrors: []string{
				"type error: cannot assign [String] to [Number]",
			},
		},
		{
			name: "Mismatched assignment to explicit Const variable",
			input: `
const b: Boolean = 1
`,
			expectedErrors: []string{
				"type error: cannot assign Number to Boolean",
			},
		},
		{
			name: "Using Any as a variable type throws an error",
			input: `
let x: Any = 10
`,
			expectedErrors: []string{
				"semantic error: variable 'x' type is not declared: 'Any'",
			},
		},
		{
			name: "Using Any as a function return type throws an error",
			input: `
let f = fn() -> Any { return 10 }
`,
			expectedErrors: []string{
				"semantic error: function return type is not declared: 'Any'",
				"type error: cannot resolve type name for Any",
			},
		},
		{
			name: "Mutating property of const struct throws error",
			input: `
type MyStruct struct { val Number }
const obj = MyStruct{ val: 1 }
obj.val = 2
`,
			expectedErrors: []string{
				"semantic error: cannot mutate property/index of constant variable 'obj'",
			},
		},
		{
			name: "Mutating index of const array throws error",
			input: `
const arr = [1, 2]
arr[0] = 10
`,
			expectedErrors: []string{
				"semantic error: cannot mutate property/index of constant variable 'arr'",
			},
		},
		{
			name: "Mutating parameter struct property throws error",
			input: `
type MyStruct struct { value Number }
let myStruct: MyStruct = MyStruct{ value: 0 }
let f = fn(s: MyStruct) -> MyStruct {
    s.value = 10
    return s
}
f(myStruct)
return myStruct.value
`,
			expectedErrors: []string{
				"semantic error: cannot mutate property/index of constant variable 's'",
			},
		},
		{
			name: "Mutating outer scope variable inside a function",
			input: `
let counter = 0

let add_to_counter = fn() -> Number {
    counter = counter + 1 
    return counter
}
`,
			expectedErrors: []string{
				"semantic error: cannot mutate outer scope variable 'counter' inside a function",
			},
		},
		{
			name: "Mutating outer scope array inside a function",
			input: `
let arr = [1, 2, 3]
let mutate_arr = fn() -> [Number] {
    arr[0] = 10
    return arr
}
`,
			expectedErrors: []string{
				"semantic error: cannot mutate outer scope variable 'arr' inside a function",
			},
		},
	}
	runTestScenarios(t, tests)
}

func TestSemanticAnalysisFunctions(t *testing.T) {
	tests := []testScenario{
		{
			name: "Valid function declaration and call",
			input: `
let add = fn(a: Number, b: Number) -> Number { return a + b }
let result = add(10, 20)
`,
			expectedErrors: []string{},
		},
		{
			name: "Valid higher-order function receiving and returning an inline function",
			input: `
let apply = fn(cb: fn(Number) -> String) -> fn(Number) -> String { return cb }
let myCb = fn(a: Number) -> String { return "test" }
apply(myCb)
`,
			expectedErrors: []string{},
		},
		{
			name: "Invalid argument for higher-order function (wrong return type)",
			input: `
let apply = fn(cb: fn(Number) -> String) -> String { return cb(10) }
let myCb = fn(a: Number) -> Number { return a }
apply(myCb)
`,
			expectedErrors: []string{
				"type error: argument 1 expected fn(Number) -> String, got myCb(a: Number) -> Number",
			},
		},
		{
			name: "Invalid argument for higher-order function (wrong parameter type)",
			input: `
let apply = fn(cb: fn(Number) -> String) -> String { return cb(10) }
let myCb = fn(a: String) -> String { return a }
apply(myCb)
`,
			expectedErrors: []string{
				"type error: argument 1 expected fn(Number) -> String, got myCb(a: String) -> String",
			},
		},
		{
			name: "Call non-function",
			input: `
let a = 10
a()
`,
			expectedErrors: []string{
				"type error: cannot call a non-function (got Number)",
			},
		},
		{
			name: "Incorrect arity",
			input: `
let add = fn(a: Number, b: Number) -> Number { return a + b }
add(10)
`,
			expectedErrors: []string{
				"arity error: expected 2 arguments, got 1",
			},
		},
		{
			name: "Incorrect argument type",
			input: `
let check = fn(name: String) -> Boolean { return true }
check(10)
`,
			expectedErrors: []string{
				"type error: argument 1 expected String, got Number",
			},
		},
		{
			name: "Incorrect return type",
			input: `
let add = fn(a: Number, b: Number) -> String { return a + b }
`,
			expectedErrors: []string{
				"type error: function declared to return String, but body returns Number",
			},
		},
		{
			name: "Missing return statement",
			input: `
let bad = fn() -> Number {
	let a = 10
}
`,
			expectedErrors: []string{
				"semantic error: function is missing a guaranteed return statement. All code paths must return a value.",
			},
		},
		{
			name: "Missing return in alternative path",
			input: `
let check = fn(a: Number) -> Boolean {
	if (a > 0) {
		return true
	}
}
`,
			expectedErrors: []string{
				"semantic error: function is missing a guaranteed return statement. All code paths must return a value.",
			},
		},
		{
			name: "Valid return in all paths",
			input: `
let check = fn(a: Number) -> Boolean {
	if (a > 0) {
		return true
	} else {
		return false
	}
}
`,
			expectedErrors: []string{},
		},
		{
			name: "Valid recursive function",
			input: `
let factorial = fn(n: Number) -> Number {
	if (n == 0) {
		return 1
	} else {
		return n * factorial(n - 1)
	}
}
`,
			expectedErrors: []string{},
		},
		{
			name: "Invalid recursive function call (wrong argument type)",
			input: `
let loop = fn(n: Number) -> Number {
	if (n == 0) {
		return 0
	} else {
		return loop("string instead of number")
	}
}
`,
			expectedErrors: []string{
				"type error: argument 1 expected Number, got String",
			},
		},
		{
			name: "Unconditional recursion (infinite loop)",
			input: `
let loop = fn() -> Number {
	return loop()
}
`,
			expectedErrors: []string{
				"semantic error: function 'loop' contains unconditional recursion and will infinitely loop",
			},
		},
		{
			name: "Unconditional recursion in all if-else branches",
			input: `
let alwaysLoops = fn(n: Number) -> Number {
	if (n > 0) {
		return alwaysLoops(n - 1)
	} else {
		return alwaysLoops(n + 1)
	}
}
`,
			expectedErrors: []string{
				"semantic error: function 'alwaysLoops' contains unconditional recursion and will infinitely loop",
			},
		},
		{
			name: "Unconditional recursion in infix expression",
			input: `
let count = fn(n: Number) -> Number {
	return 1 + count(n + 1)
}
`,
			expectedErrors: []string{
				"semantic error: function 'count' contains unconditional recursion and will infinitely loop",
			},
		},
		{
			name: "Generic map passed as argument (happy path)",
			input: `
				let f = fn<T, P>(x: T, m: map[T]P) -> P { return m[x] }
				let m: map[String]Number = { "1": 10 }
				let a = f::<String, Number>("1", m)
			`,
			expectedErrors: []string{},
		},
		{
			name: "Generic map passed as argument (implicit binding happy path)",
			input: `
				let f = fn<T, P>(x: T, m: map[T]P) -> P { return m[x] }
				let m: map[String]Number = { "1": 10 }
				let a = f("1", m)
			`,
			expectedErrors: []string{},
		},
		{
			name: "Generic map passed as argument (invalid generic instantiation mismatch)",
			input: `
				let f = fn<T, P>(x: T, m: map[T]P) -> P { return m[x] }
				let m: map[String]Number = { "1": 10 }
				let a = f::<Boolean, Number>(true, m)
			`,
			expectedErrors: []string{
				"[Line 4, Column 14] type error: argument 2 expected map[Boolean]Number, got map[String]Number",
			},
		},
	}
	runTestScenarios(t, tests)
}

func TestSemanticAnalysisDates(t *testing.T) {
	tests := []testScenario{
		{
			name: "Valid date declaration and reassignment",
			input: `
let d = '2023-10-25'
d = '2023-12-01'
`,
			expectedErrors: []string{},
		},
		{
			name: "Invalid assignment to date variable",
			input: `
let d = '2023-10-25'
d = 10
`,
			expectedErrors: []string{
				"type error: cannot assign Number to variable 'd' of type Date",
			},
		},
		{
			name: "Function returning Date",
			input: `
let getDate = fn() -> Date { return '2023-10-25' }
let d = getDate()
d = '2023-12-01'
`,
			expectedErrors: []string{},
		},
		{
			name: "Function returning wrong type instead of Date",
			input: `
let getDate = fn() -> Date { return 10 }
`,
			expectedErrors: []string{
				"type error: function declared to return Date, but body returns Number",
			},
		},
	}
	runTestScenarios(t, tests)
}

func TestSemanticAnalysisArrays(t *testing.T) {
	tests := []testScenario{
		{
			name: "if condition requires boolean",
			input: `
if (1) {
	let a = 10
}
`,
			expectedErrors: []string{
				"type error: condition must be a Boolean, got Number",
			},
		},
		{
			name: "Valid array declaration and access",
			input: `
let a = [1, 2, 3]
let b = a[0]
`,
			expectedErrors: []string{},
		},
		{
			name: "Heterogeneous array (type mismatch)",
			input: `
let a = [1, "two", 3]
`,
			expectedErrors: []string{
				"type error: array elements must have the same type, expected Number, got String",
			},
		},
		{
			name: "Invalid index type",
			input: `
let a = [1, 2, 3]
let b = a["zero"]
`,
			expectedErrors: []string{
				"type error: array index expected Number, got String",
			},
		},
		{
			name: "Index operator on non-array",
			input: `
let a = 10
let b = a[0]
`,
			expectedErrors: []string{
				"type error: index operator not supported for Number",
			},
		},
		{
			name: "Valid nested array",
			input: `
let a = [[1, 2], [3, 4]]
let b = a[0][1]
`,
			expectedErrors: []string{},
		},
		{
			name: "Array in function signature",
			input: `
let sum = fn(arr: [Number]) -> Number { return arr[0] }
let res = sum([1, 2, 3])
`,
			expectedErrors: []string{},
		},
		{
			name: "Valid array index assignment",
			input: `
let a = [1, 2, 3]
a[0] = 5
`,
			expectedErrors: []string{},
		},
		{
			name: "Nested array index assignment",
			input: `
let a = [[1, 2], [3, 4]]
a[0][1] = 5
`,
			expectedErrors: []string{},
		},
		{
			name: "Dynamic variable array index assignment",
			input: `
let a = [1, 2, 3]
let index = 1
a[index] = 5
`,
			expectedErrors: []string{},
		},
		{
			name: "Dynamic expression array index assignment",
			input: `
let a = [1, 2, 3]
a[1 + 1] = 5
`,
			expectedErrors: []string{},
		},
	}
	runTestScenarios(t, tests)
}

func TestSemanticAnalysisGenericsReplaceAny(t *testing.T) {
	tests := []testScenario{
		{
			name:  "Generic function replacing Any parameter",
			input: "let f = fn<T>(x: T) -> T { return x }\nf(10)\nf(\"hello\")\nf(true)",
		},
		{
			name:  "Generic array",
			input: "let f = fn<T>(arr: [T]) -> T { return arr[0] }\nf([1, 2, 3])",
		},
	}

	runTestScenarios(t, tests)
}

func TestSemanticAnalysisPrefix(t *testing.T) {
	tests := []testScenario{
		{
			name:  "Valid Bang",
			input: "return !true",
		},
		{
			name:           "Invalid Bang",
			input:          "return !5",
			expectedErrors: []string{"type error: operator '!' requires a Boolean, got Number"},
		},
		{
			name:  "Valid Minus",
			input: "return -5",
		},
		{
			name:           "Invalid Minus",
			input:          "return -true",
			expectedErrors: []string{"type error: operator '-' requires a Number, got Boolean"},
		},
	}
	runTestScenarios(t, tests)
}

func TestSemanticAnalysisBuiltins(t *testing.T) {
	tests := []testScenario{
		{
			name:  "len() works on array of Numbers",
			input: "import array\nlet arr = [1, 2, 3]\narray.len(arr)",
		},
		{
			name:  "len() works on array of Strings",
			input: "import array\nlet arr = [\"a\", \"b\"]\narray.len(arr)",
		},
		{
			name:  "push() works with matching types",
			input: "import array\nlet arr = [1, 2]\nlet newArr = array.push(arr, 3)",
		},
		{
			name:           "push() rejects mismatched types",
			input:          "import array\nlet arr = [1, 2]\nlet newArr = array.push(arr, \"string\")",
			expectedErrors: []string{"type inference error: conflicting types for T: Number and String", "type error: argument 2 expected Number, got String"},
		},
		{
			name:  "pop() returns array",
			input: "import array\nlet arr = [1, 2]\nlet t = array.pop(arr)\nlet res = array.push(t, 3)",
		},
		{
			name:  "head() works and infers type",
			input: "import array\nlet arr = [1, 2]\nlet h = array.head(arr)\nlet n = h",
		},
		{
			name:  "tail() returns array",
			input: "import array\nlet arr = [1, 2]\nlet t = array.tail(arr)\nlet res = array.push(t, 3)",
		},
		{
			name:  "last() works and infers type",
			input: "import array\nlet arr = [1, 2]\nlet l = array.last(arr)\nlet n = l",
		},
		{
			name:  "copy() returns array",
			input: "import array\nlet arr = [1, 2]\nlet c = array.copy(arr)\nlet res = array.push(c, 3)",
		},
		{
			name:  "slice() works",
			input: "import array\nlet arr = [1, 2, 3]\nlet s = array.slice(arr, 0, 2)\nlet res = array.push(s, 4)",
		},
		{
			name:           "slice() rejects non-number index",
			input:          "import array\nlet arr = [1, 2]\nlet s = array.slice(arr, \"0\", 2)",
			expectedErrors: []string{"type error: argument 2 expected Number, got String"},
		},
		{
			name:  "join() works with matching types",
			input: "import array\nlet arrOne = [1, 2]\nlet arrTwo = [3, 4]\nlet res = array.join(arrOne, arrTwo)",
		},
		{
			name:           "join() rejects mismatched types",
			input:          "import array\nlet arrOne = [1, 2]\nlet arrTwo = [\"a\", \"b\"]\nlet res = array.join(arrOne, arrTwo)",
			expectedErrors: []string{"type inference error: conflicting types for T: Number and String", "type error: argument 2 expected [Number], got [String]"},
		},
		{
			name:  "array generic function assignment to variable",
			input: "import array\nlet pushFunc = array.push\nlet arr = [1, 2]\npushFunc(arr, 3)",
		},
		{
			name:           "array generic function assignment to variable enforces types",
			input:          "import array\nlet pushFunc = array.push\nlet arr = [1, 2]\npushFunc(arr, \"string\")",
			expectedErrors: []string{"type inference error: conflicting types for T: Number and String", "type error: argument 2 expected Number, got String"},
		},
		{
			name:  "explicit generic turbofish instantiation of array function",
			input: "import array\nlet arr = [1, 2]\narray.push::<Number>(arr, 3)",
		},
		{
			name:           "explicit generic turbofish instantiation enforces explicit type",
			input:          "import array\nlet arr = [\"a\", \"b\"]\narray.push::<Number>(arr, \"c\")",
			expectedErrors: []string{"type error: argument 1 expected [Number], got [String]", "type error: argument 2 expected Number, got String"},
		},
		{
			name:  "array generic functions on array of generic structs",
			input: "import array\ntype Container<T> struct { val T }\nlet arr = [Container::<Number> { val: 1 }]\nlet pushFunc = array.push\nlet res = pushFunc(arr, Container::<Number> { val: 2 })\nlet item = array.head(res)",
		},
		{
			name:  "charAt() works with correct types",
			input: "import string\nreturn string.charAt(\"hello\", 1)",
		},
		{
			name:           "charAt() rejects mismatched types",
			input:          "import string\nreturn string.charAt(10, \"hello\")",
			expectedErrors: []string{"type error: first argument to 'charAt' must be String, got Number", "type error: second argument to 'charAt' must be Number, got String"},
		},
		{
			name:  "substring() works with correct types",
			input: "import string\nreturn string.substring(\"hello\", 1, 4)",
		},
		{
			name:           "substring() rejects mismatched types",
			input:          "import string\nreturn string.substring(10, \"start\", \"end\")",
			expectedErrors: []string{"type error: first argument to 'substring' must be String, got Number", "type error: second argument to 'substring' must be Number, got String", "type error: third argument to 'substring' must be Number, got String"},
		},
		{
			name:  "concat() works with correct types",
			input: "import string\nreturn string.concat(\"hello\", \" world\")",
		},
		{
			name:           "concat() rejects mismatched types",
			input:          "import string\nreturn string.concat(10, 20)",
			expectedErrors: []string{"type error: first argument to 'concat' must be String, got Number", "type error: second argument to 'concat' must be String, got Number"},
		},
		{
			name:  "join() works with correct types",
			input: "import string\nreturn string.join([\"hello\", \"world\"], \",\")",
		},
		{
			name:           "join() rejects mismatched types",
			input:          "import string\nreturn string.join([10, 20], 20)",
			expectedErrors: []string{"type error: array elements for 'join' must be String, got Number", "type error: second argument to 'join' must be String, got Number"},
		},
		{
			name:           "join() rejects invalid array element types",
			input:          "import string\nreturn string.join([10, 20], \",\")",
			expectedErrors: []string{"type error: array elements for 'join' must be String, got Number"},
		},
		{
			name:  "split() works with correct types",
			input: "import string\nreturn string.split(\"hello\", \"e\")",
		},
		{
			name:           "split() rejects mismatched types",
			input:          "import string\nreturn string.split(10, 20)",
			expectedErrors: []string{"type error: first argument to 'split' must be String, got Number", "type error: second argument to 'split' must be String, got Number"},
		},
		{
			name:  "contains() works with correct types",
			input: "import string\nreturn string.contains(\"hello\", \"e\")",
		},
		{
			name:           "contains() rejects mismatched types",
			input:          "import string\nreturn string.contains(10, 20)",
			expectedErrors: []string{"type error: first argument to 'contains' must be String, got Number", "type error: second argument to 'contains' must be String, got Number"},
		},
		{
			name:  "startsWith() works with correct types",
			input: "import string\nreturn string.startsWith(\"hello\", \"h\")",
		},
		{
			name:           "startsWith() rejects mismatched types",
			input:          "import string\nreturn string.startsWith(10, 20)",
			expectedErrors: []string{"type error: first argument to 'startsWith' must be String, got Number", "type error: second argument to 'startsWith' must be String, got Number"},
		},
		{
			name:  "endsWith() works with correct types",
			input: "import string\nreturn string.endsWith(\"hello\", \"o\")",
		},
		{
			name:           "endsWith() rejects mismatched types",
			input:          "import string\nreturn string.endsWith(10, 20)",
			expectedErrors: []string{"type error: first argument to 'endsWith' must be String, got Number", "type error: second argument to 'endsWith' must be String, got Number"},
		},
		{
			name:  "replace() works with correct types",
			input: "import string\nreturn string.replace(\"hello\", \"e\", \"a\")",
		},
		{
			name:           "replace() rejects mismatched types",
			input:          "import string\nreturn string.replace(10, 20, 30)",
			expectedErrors: []string{"type error: first argument to 'replace' must be String, got Number", "type error: second argument to 'replace' must be String, got Number", "type error: third argument to 'replace' must be String, got Number"},
		},
		{
			name:  "toUpper() works with correct types",
			input: "import string\nreturn string.toUpper(\"hello\")",
		},
		{
			name:           "toUpper() rejects mismatched types",
			input:          "import string\nreturn string.toUpper(10)",
			expectedErrors: []string{"type error: first argument to 'toUpper' must be String, got Number"},
		},
		{
			name:  "toLower() works with correct types",
			input: "import string\nreturn string.toLower(\"HELLO\")",
		},
		{
			name:           "toLower() rejects mismatched types",
			input:          "import string\nreturn string.toLower(10)",
			expectedErrors: []string{"type error: first argument to 'toLower' must be String, got Number"},
		},
		{
			name:  "trim() works with correct types",
			input: "import string\nreturn string.trim(\"  hello  \")",
		},
		{
			name:           "trim() rejects mismatched types",
			input:          "import string\nreturn string.trim(10)",
			expectedErrors: []string{"type error: first argument to 'trim' must be String, got Number"},
		},
		{
			name:  "string len() works with correct types",
			input: "import string\nreturn string.len(\"hello\")",
		},
		{
			name:           "strlen() rejects mismatched types",
			input:          "import string\nreturn string.len(10)",
			expectedErrors: []string{"type error: first argument to 'len' must be String, got Number"},
		},
		{
			name:  "year() works with correct types",
			input: "import date\nlet d = '2023-10-25'\nreturn date.year(d)",
		},
		{
			name:           "year() rejects mismatched types",
			input:          "import date\nreturn date.year(\"2023\")",
			expectedErrors: []string{"type error: first argument to 'year' must be Date, got String"},
		},
		{
			name:  "month() works with correct types",
			input: "import date\nreturn date.month('2023-10-25')",
		},
		{
			name:  "day() works with correct types",
			input: "import date\nreturn date.day('2023-10-25')",
		},
		{
			name:  "weekday() works with correct types",
			input: "import date\nreturn date.weekday('2023-10-25')",
		},
		{
			name:           "weekday() rejects missing arguments",
			input:          "import date\nreturn date.weekday()",
			expectedErrors: []string{"arity error: expected 1 arguments for 'weekday', got 0"},
		},
		{
			name:  "today() works with correct types",
			input: "import date\nreturn date.today()",
		},
		{
			name:           "today() rejects unexpected arguments",
			input:          "import date\nreturn date.today(1)",
			expectedErrors: []string{"arity error: expected 0 arguments for 'today', got 1"},
		},
		{
			name:  "parse() works with correct types",
			input: "import date\nreturn date.parse(\"2023-10-25\")",
		},
		{
			name:           "parse() rejects mismatched types",
			input:          "import date\nreturn date.parse(2023)",
			expectedErrors: []string{"type error: first argument to 'parse' must be String, got Number"},
		},
		{
			name:  "addDays() works with correct types",
			input: "import date\nreturn date.addDays('2023-10-25', 5)",
		},
		{
			name:           "addDays() rejects mismatched types",
			input:          "import date\nreturn date.addDays('2023-10-25', \"5\")",
			expectedErrors: []string{"type error: second argument to 'addDays' must be Number, got String"},
		},
		{
			name:  "diffDays() works with correct types",
			input: "import date\nreturn date.diffDays('2023-10-30', '2023-10-25')",
		},
		{
			name:           "diffDays() rejects mismatched types",
			input:          "import date\nreturn date.diffDays('2023-10-30', 5)",
			expectedErrors: []string{"type error: second argument to 'diffDays' must be Date, got Number"},
		},
		{
			name:  "new() works with correct types",
			input: "import date\nreturn date.new(2023, 1, 1)",
		},
		{
			name:           "new() rejects mismatched types",
			input:          "import date\nreturn date.new(\"2023\", 1, 1)",
			expectedErrors: []string{"type error: first argument to 'new' must be Number, got String"},
		},
		{
			name:  "abs() works with correct types",
			input: "import math\nreturn math.abs(-10.5)",
		},
		{
			name:           "abs() rejects missing arguments",
			input:          "import math\nreturn math.abs()",
			expectedErrors: []string{"arity error: expected 1 arguments for 'abs', got 0"},
		},
		{
			name:           "abs() rejects mismatched types",
			input:          "import math\nreturn math.abs(\"hello\")",
			expectedErrors: []string{"type error: first argument to 'abs' must be Number, got String"},
		},
		{
			name:  "sqrt() works with correct types",
			input: "import math\nreturn math.sqrt(16)",
		},
		{
			name:           "sqrt() rejects mismatched types",
			input:          "import math\nreturn math.sqrt(\"hello\")",
			expectedErrors: []string{"type error: first argument to 'sqrt' must be Number, got String"},
		},
		{
			name:  "floor() works with correct types",
			input: "import math\nreturn math.floor(4.9)",
		},
		{
			name:           "floor() rejects mismatched types",
			input:          "import math\nreturn math.floor(\"hello\")",
			expectedErrors: []string{"type error: first argument to 'floor' must be Number, got String"},
		},
		{
			name:  "ceil() works with correct types",
			input: "import math\nreturn math.ceil(4.1)",
		},
		{
			name:           "ceil() rejects mismatched types",
			input:          "import math\nreturn math.ceil(\"hello\")",
			expectedErrors: []string{"type error: first argument to 'ceil' must be Number, got String"},
		},
		{
			name:  "round() works with correct types",
			input: "import math\nreturn math.round(4.5)",
		},
		{
			name:           "round() rejects mismatched types",
			input:          "import math\nreturn math.round(\"hello\")",
			expectedErrors: []string{"type error: first argument to 'round' must be Number, got String"},
		},
		{
			name:  "pow() works with correct types",
			input: "import math\nreturn math.pow(2, 3)",
		},
		{
			name:           "pow() rejects missing arguments",
			input:          "import math\nreturn math.pow(2)",
			expectedErrors: []string{"arity error: expected 2 arguments for 'pow', got 1"},
		},
		{
			name:           "pow() rejects mismatched first type",
			input:          "import math\nreturn math.pow(\"2\", 3)",
			expectedErrors: []string{"type error: first argument to 'pow' must be Number, got String"},
		},
		{
			name:           "pow() rejects mismatched second type",
			input:          "import math\nreturn math.pow(2, \"3\")",
			expectedErrors: []string{"type error: second argument to 'pow' must be Number, got String"},
		},
		{
			name:  "min() works with correct types",
			input: "import math\nreturn math.min(2, 3)",
		},
		{
			name:           "min() rejects mismatched second type",
			input:          "import math\nreturn math.min(2, \"3\")",
			expectedErrors: []string{"type error: second argument to 'min' must be Number, got String"},
		},
		{
			name:  "max() works with correct types",
			input: "import math\nreturn math.max(2, 3)",
		},
		{
			name:           "max() rejects mismatched first type",
			input:          "import math\nreturn math.max(\"2\", 3)",
			expectedErrors: []string{"type error: first argument to 'max' must be Number, got String"},
		},
		{
			name:  "log() works with correct types",
			input: "import math\nreturn math.log(100, 10)",
		},
		{
			name:           "log() rejects mismatched second type",
			input:          "import math\nreturn math.log(100, \"10\")",
			expectedErrors: []string{"type error: second argument to 'log' must be Number, got String"},
		},
		{
			name:  "rand() works with 0 arguments",
			input: "import math\nreturn math.rand()",
		},
		{
			name:           "rand() rejects number argument",
			input:          "import math\nreturn math.rand(42)",
			expectedErrors: []string{"arity error: expected 0 arguments for 'rand', got 1"},
		},
		{
			name:           "rand() rejects string argument",
			input:          "import math\nreturn math.rand(\"hello\")",
			expectedErrors: []string{"arity error: expected 0 arguments for 'rand', got 1"},
		},
		{
			name:           "rand() rejects boolean argument",
			input:          "import math\nreturn math.rand(true)",
			expectedErrors: []string{"arity error: expected 0 arguments for 'rand', got 1"},
		},
		{
			name:           "rand() rejects date argument",
			input:          "import math\nreturn math.rand(2026-08-12)",
			expectedErrors: []string{"arity error: expected 0 arguments for 'rand', got 1"},
		},
		{
			name:  "log.info() works with correct types",
			input: "import log\nreturn log.info(\"message\", 42)",
		},
		{
			name:           "log.info() rejects mismatched first type",
			input:          "import log\nreturn log.info(42, 42)",
			expectedErrors: []string{"type error: first argument to 'info' must be String, got Number"},
		},
		{
			name:           "log.info() rejects missing arguments",
			input:          "import log\nreturn log.info(\"message\")",
			expectedErrors: []string{"arity error: expected 2 arguments for 'info', got 1"},
		},
		{
			name:  "log.warn() works with any second type",
			input: "import log\nreturn log.warn(\"message\", [1, 2, 3])",
		},
		{
			name:  "log.error() works with any second type",
			input: "import log\nreturn log.error(\"message\", true)",
		},
		{
			name:  "log.export() accepts any valid argument",
			input: "import log\nreturn log.export([1, 2, 3])",
		},
		{
			name:           "log.export() rejects missing arguments",
			input:          "import log\nreturn log.export()",
			expectedErrors: []string{"arity error: expected 1 argument for 'export', got 0"},
		},
		{
			name:           "log.export() rejects too many arguments",
			input:          "import log\nreturn log.export(1, 2)",
			expectedErrors: []string{"arity error: expected 1 argument for 'export', got 2"},
		},
		{
			name:  "map.delete() works with correct types",
			input: "import map\nlet m: map[String]Number = {}\nreturn map.delete(m, \"key\")",
		},
		{
			name:           "map.delete() rejects mismatched key type",
			input:          "import map\nlet m: map[String]Number = {}\nreturn map.delete(m, 42)",
			expectedErrors: []string{"type error: map index must be String, got Number"},
		},
		{
			name:           "map.delete() rejects mismatched first argument type",
			input:          "import map\nreturn map.delete(\"not a map\", \"key\")",
			expectedErrors: []string{"type error: first argument to 'delete' must be Map, got String"},
		},
		{
			name:           "map.delete() rejects missing arguments",
			input:          "import map\nlet m: map[String]Number = {}\nreturn map.delete(m)",
			expectedErrors: []string{"arity error: expected 2 arguments for 'delete', got 1"},
		},
	}

	runTestScenarios(t, tests)
}

func TestSemanticLogicalOperators(t *testing.T) {
	tests := []testScenario{
		{
			name:           "Logical AND with correct types",
			input:          "return true and false",
			expectedErrors: []string{},
		},
		{
			name:           "Logical OR with correct types",
			input:          "return true or false",
			expectedErrors: []string{},
		},
		{
			name:           "Logical XOR with correct types",
			input:          "return true xor false",
			expectedErrors: []string{},
		},
		{
			name:           "Logical AND with wrong left type",
			input:          "return 1 and true",
			expectedErrors: []string{"type error: operator 'and' requires two Booleans, got Number and Boolean"},
		},
		{
			name:           "Logical OR with wrong right type",
			input:          "return false or \"string\"",
			expectedErrors: []string{"type error: operator 'or' requires two Booleans, got Boolean and String"},
		},
	}
	runTestScenarios(t, tests)
}

func TestSemanticAnalysisConstModifier(t *testing.T) {
	tests := []testScenario{
		{
			name:  "Valid const declaration",
			input: "const a = 10",
		},
		{
			name:  "Invalid const reassignment",
			input: "const a = 10\na = 20",
			expectedErrors: []string{
				"[Line 2, Column 3] semantic error: cannot assign to constant variable 'a'",
			},
		},
		{
			name:  "Valid let reassignment",
			input: "let a = 10\na = 20",
		},
	}
	runTestScenarios(t, tests)
}

func TestSemanticAnalysisPrivateModifier(t *testing.T) {
	tests := []testScenario{
		{
			name: "Valid private let at top level",
			input: `
private let a = 10
`,
			expectedErrors: []string{},
		},
		{
			name: "Invalid private let inside block",
			input: `
if (true) {
	private let a = 10
}
`,
			expectedErrors: []string{
				"semantic error: 'private' modifier is only allowed at the top-level of a module",
			},
		},
		{
			name: "Valid private type at top level",
			input: `
private type MyFunc fn() -> Number
`,
			expectedErrors: []string{},
		},
		{
			name: "Invalid private type inside block",
			input: `
if (true) {
	private type MyFunc fn() -> Number
}
`,
			expectedErrors: []string{
				"semantic error: 'private' modifier is only allowed at the top-level of a module",
			},
		},
		{
			name: "Invalid private let inside function block",
			input: `
let a = fn(b: Number) -> Number {
	private let c = 10
	return b + c
}
`,
			expectedErrors: []string{
				"semantic error: 'private' modifier is only allowed at the top-level of a module",
			},
		},
	}
	runTestScenarios(t, tests)
}

func TestSemanticAnalysisTypeAliasUsage(t *testing.T) {
	tests := []testScenario{
		{
			name:  "Type alias with primitive types",
			input: "type money Number\ntype moment Boolean\ntype name String\ntype custom Number\nlet process = fn(m: money, d: moment, n: name, c: custom) -> money { return m }\nprocess(100, true, \"John\", 42)",
		},
		{
			name:  "Type alias with array types",
			input: "type prices [Number]\ntype names [String]\ntype holidays [Boolean]\ntype collection [Number]\nlet addAll = fn(p: prices, n: names, h: holidays, c: collection) -> prices { return p }\naddAll([1, 2], [\"a\"], [true], [1, 2])",
		},
		{
			name:           "Type alias mismatch",
			input:          "type money Number\nlet add = fn(a: money) -> money { return a }\nadd(\"string\")",
			expectedErrors: []string{"type error: argument 1 expected Number, got String"},
		},
		{
			name:           "Undefined type in alias",
			input:          "type someType Some\nlet doSomething = fn(a: someType) -> Number { return 1 }",
			expectedErrors: []string{"type error: cannot resolve type name for Some"},
		},
	}
	runTestScenarios(t, tests)
}
func TestSemanticAnalysisStructs(t *testing.T) {
	tests := []testScenario{
		{
			name: "Struct with nested struct field",
			input: `
				type User struct {
					name String
				}
				type Profile struct {
					user User
					age Number
				}
				let p = Profile {
					user: User { name: "Bob" },
					age: 30
				}
				let n = p.user.name
			`,
			expectedErrors: []string{},
		},
		{
			name: "Generic map in generic struct instantiation success",
			input: `
				type CustomStruct<T, P> struct { m map[T]P }
				let c = CustomStruct::<String, Number> { m: {} }
				let mapInstance = c.m
				mapInstance["a"] = 100
			`,
			expectedErrors: []string{},
		},
		{
			name: "Generic map in generic struct invalid assignment (key mismatch)",
			input: `
				type CustomStruct<T, P> struct { m map[T]P }
				let c = CustomStruct::<String, Number> { m: {} }
				let mapInstance = c.m
				mapInstance[10] = 100
			`,
			expectedErrors: []string{
				"[Line 5, Column 21] type error: map index must be String, got Number",
			},
		},
		{
			name: "Generic map in generic struct invalid assignment (value mismatch)",
			input: `
				type CustomStruct<T, P> struct { m map[T]P }
				let c = CustomStruct::<String, Number> { m: {} }
				let mapInstance = c.m
				mapInstance["a"] = "hello"
			`,
			expectedErrors: []string{
				"[Line 5, Column 22] type error: cannot assign String to map with value type Number",
			},
		},
		{
			name: "Type alias generic struct instantiation success",
			input: `
				type CustomStruct<T> struct { f fn(T) -> T }
				let c = CustomStruct::<String> { f: fn(x: String) -> String { return x } }
			`,
			expectedErrors: []string{},
		},
		{
			name: "Type alias generic struct instantiation type error",
			input: `
				type CustomStruct<T> struct { f fn(T) -> T }
				let c = CustomStruct::<String> { f: fn(x: Number) -> Number { return x } }
			`,
			expectedErrors: []string{
				"[Line 3, Column 36] type error: field 'f' expects fn(String) -> String, got fn(x: Number) -> Number",
			},
		},
		{
			name: "Type alias generic struct instantiation missing turbofish arguments",
			input: `
				type CustomStruct<T> struct { f fn(T) -> T }
				let c = CustomStruct { f: fn(x: String) -> String { return x } }
			`,
			expectedErrors: []string{
				"[Line 3, Column 26] type error: missing type arguments for generic struct 'CustomStruct'",
				"[Line 3, Column 26] type error: field 'f' expects fn(T) -> T, got fn(x: String) -> String",
			},
		},
		{
			name: "Type alias generic struct instantiation incorrect turbofish arguments count",
			input: `
				type CustomStruct<T> struct { f fn(T) -> T }
				let c = CustomStruct::<String, Number> { f: fn(x: String) -> String { return x } }
			`,
			expectedErrors: []string{
				"[Line 3, Column 44] type error: expected 1 type arguments for struct 'CustomStruct', got 2",
				"[Line 3, Column 44] type error: field 'f' expects fn(T) -> T, got fn(x: String) -> String",
			},
		},
		{
			name: "Type alias generic struct instantiation with undefined type argument",
			input: `
				type CustomStruct<T> struct { f fn(T) -> T }
				let c = CustomStruct::<Unknown> { f: fn(x: String) -> String { return x } }
			`,
			expectedErrors: []string{
				"[Line 3, Column 37] type error: cannot resolve type name for Unknown",
			},
		},
		{
			name: "Multiple type parameters generic struct instantiation success",
			input: `
				type MapLike<K, V> struct {
					key fn() -> K
					val fn() -> V
				}
				let m = MapLike::<String, Number> {
					key: fn() -> String { return "test" },
					val: fn() -> Number { return 42 }
				}
			`,
			expectedErrors: []string{},
		},
		{
			name: "Generic struct instantiated inside a generic function",
			input: `
				type CustomStruct<T> struct {
					p T
				}

				let f = fn<T>(x: T) -> T {
					let cs = CustomStruct::<T> { p: x }
					return cs.p
				}

				let result = f::<String>("hello")
			`,
			expectedErrors: []string{},
		},
		{
			name: "Valid struct with inline function property",
			input: `
				type Action struct {
					run fn(Number) -> String
				}
				let a = Action {
					run: fn(n: Number) -> String { return "valid" }
				}
			`,
			expectedErrors: []string{},
		},
		{
			name: "Invalid struct with inline function property",
			input: `
				type Action struct {
					run fn(Number) -> String
				}
				let a = Action {
					run: fn(n: Number) -> Number { return 10 }
				}
			`,
			expectedErrors: []string{
				"type error: field 'run' expects fn(Number) -> String, got fn(n: Number) -> Number",
			},
		},
		{
			name: "Recursive struct definition",
			input: `
				type Node struct {
					value Number
					next Node
				}
				let n = Node {
					value: 1,
					next: Node {
						value: 2,
						next: Node {
							value: 3
						}
					}
				}
			`,
			expectedErrors: []string{"semantic error: missing required field 'next' in struct literal"},
		},
		{
			name: "Recursive struct definition with nil",
			input: `
				type Node struct {
					value Number
					next Node?
				}
				let n = Node {
					value: 1,
					next: Node {
						value: 2,
						next: nil
					}
				}
			`,
		},
		{
			name:           "Undefined struct",
			input:          `let p = Unknown { a: 1 }`,
			expectedErrors: []string{"semantic error: undefined struct 'Unknown'"},
		},
		{
			name: "Missing required struct field",
			input: `
				type User struct {
					name String
					age Number
				}
				let u = User { name: "Bob" }
			`,
			expectedErrors: []string{"semantic error: missing required field 'age' in struct literal"},
		},
		{
			name: "Type mismatch in struct field",
			input: `
				type User struct {
					age Number
				}
				let u = User { age: "30" }
			`,
			expectedErrors: []string{"type error: field 'age' expects Number, got String"},
		},
		{
			name: "Access undefined struct property",
			input: `
				type User struct {
					name String
				}
				let u = User { name: "Bob" }
				let x = u.age
			`,
			expectedErrors: []string{"semantic error: property 'age' not found on struct 'User'"},
		},
		{
			name: "Update const struct property",
			input: `
				type User struct {
					const name String
				}
				let u = User { name: "Bob" }
				u.name = "Alice"
			`,
			expectedErrors: []string{"semantic error: cannot assign to constant property 'name' on struct 'User'"},
		},
		{
			name: "Update non-existent struct property",
			input: `
				type User struct {
					name String
				}
				let u = User { name: "Bob" }
				u.age = 30
			`,
			expectedErrors: []string{"semantic error: property 'age' not found on struct 'User'"},
		},
	}
	runTestScenarios(t, tests)
}

func TestSemanticNullableNavigation(t *testing.T) {
	tests := []testScenario{
		{
			name: "Valid consecutive safe navigation",
			input: `
				type B struct { c Number }
				type A struct { b B? }
				let a = A { b: nil }
				let x = a.b?.c
			`,
		},
		{
			name: "Invalid missing safe navigation on nullable struct property",
			input: `
				type B struct { c Number }
				type A struct { b B? }
				let a = A { b: nil }
				let x = a.b.c
			`,
			expectedErrors: []string{"semantic error: property access on nullable type requires safe navigation operator '?.'"},
		},
		{
			name: "Invalid unnecessary safe navigation on non-nullable struct property",
			input: `
				type B struct { c Number }
				type A struct { b B }
				let a = A { b: B { c: 1 } }
				let x = a?.b.c
			`,
			expectedErrors: []string{"semantic error: unnecessary safe navigation on non-nullable type"},
		},
		{
			name: "Valid safe assignment",
			input: `
				type B struct { c Number }
				type A struct { b B? }
				let a = A { b: nil }
				a.b?.c = 2
			`,
		},
		{
			name: "Invalid safe assignment missing safe navigation",
			input: `
				type B struct { c Number }
				type A struct { b B? }
				let a = A { b: nil }
				a.b.c = 2
			`,
			expectedErrors: []string{"semantic error: property assignment on nullable type requires safe navigation operator '?.'"},
		},
		{
			name: "Invalid safe assignment unnecessary safe navigation",
			input: `
				type B struct { c Number }
				type A struct { b B }
				let a = A { b: B { c: 1 } }
				a?.b.c = 2
			`,
			expectedErrors: []string{"semantic error: unnecessary safe navigation on non-nullable type"},
		},
		{
			name:           "cast.to::<String, Number>() rejects invalid fallback",
			input:          "import cast\nreturn cast.to::<String, Number>(\"123\", \"err\")",
			expectedErrors: []string{"type error: argument 2 expected Number, got String"},
		},
		{
			name:  "cast.to() works with any first argument without turbofish due to implicit generic binding of fallback",
			input: "import cast\nreturn cast.to(123, \"fallback\")",
		},
		{
			name:           "cast.to::<String, Boolean>() rejects invalid fallback",
			input:          "import cast\nreturn cast.to::<String, Boolean>(\"true\", 1)",
			expectedErrors: []string{"type error: argument 2 expected Boolean, got Number"},
		},
		{
			name: "assign invalid type to map.KeyFunc property",
			input: `
				import map
				type CustomStruct struct { key map.KeyFunc }
				let s = CustomStruct{key: 10}
			`,
			expectedErrors: []string{"type error: field 'key' expects KeyFunc() -> String, got Number"},
		},
		{
			name: "assign wrong function signature to map.KeyFunc property",
			input: `
				import map
				type CustomStruct struct { key map.KeyFunc }
				let s = CustomStruct{key: fn() -> Number { return 10 }}
			`,
			expectedErrors: []string{"type error: field 'key' expects KeyFunc() -> String, got fn() -> Number"},
		},
		{
			name: "struct with duplicate fields of same type",
			input: `
				type CustomStruct struct {
					name String
					name String
				}
			`,
			expectedErrors: []string{"semantic error: duplicate field 'name' in struct 'CustomStruct'"},
		},
		{
			name: "struct with duplicate fields of different types",
			input: `
				type CustomStruct struct {
					name String
					name Number
				}
			`,
			expectedErrors: []string{"semantic error: duplicate field 'name' in struct 'CustomStruct'"},
		},
	}
	runTestScenarios(t, tests)
}
