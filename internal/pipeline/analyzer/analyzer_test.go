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
			name: "Anonymous function contextual inference (LetStatement)",
			input: `
type CustomFunc fn(Number) -> Number
let f: CustomFunc = x => x * 2
`,
			expectedErrors: []string{},
		},
		{
			name: "Anonymous function contextual inference (CallExpression)",
			input: `
let applyOp = fn(op: fn(Number, Number) -> Number) -> Number {
	return op(10, 20)
}
let res = applyOp((x, y) => x + y)
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
		{
			name: "Union type declaration and variant assignment",
			input: `
type Cat struct { name String }
type Dog struct { name String }
union Animal = Cat | Dog
let animal: Animal = Cat { name: "Tom" }
`,
			expectedErrors: []string{},
		},
		{
			name: "Is expression narrows a union to a variant",
			input: `
type Cat struct { name String }
type Dog struct { name String }
union Animal = Cat | Dog
let animal: Animal = Cat { name: "Tom" }
let cat: Cat? = animal is Cat
`,
			expectedErrors: []string{},
		},
		{
			name: "Distinct names across type, define, union, let, const never collide",
			input: `
type Cat struct { name String }
type Dog struct { name String }
union Animal = Cat | Dog
let a: Animal = Cat { name: "Tom" }
const b = 10
define Adult constraints Cat with: fn(c: Cat) -> Boolean { return true }
`,
			expectedErrors: []string{},
		},
		{
			name: "Redeclaration check respects function boundaries: local variable may reuse a global name",
			input: `
let foo = 10
let f = fn() -> Number {
	let foo = 20
	return foo
}
`,
			expectedErrors: []string{},
		},
		{
			name: "Redeclaration check respects function boundaries: local generic function type params still resolve",
			input: `
const identity = fn<T>(x: T) -> T {
	return x
}
`,
			expectedErrors: []string{},
		},
		{
			name: "Redeclaration check still catches shadowing within the SAME function",
			input: `
let f = fn() -> Number {
	let x = 1
	if (x > 0) {
		let x = 2
		return x
	} else {
		return x
	}
}
`,
			expectedErrors: []string{
				"semantic error: variable 'x' is already declared",
			},
		},
		{
			name: "String interpolation accepts any expression type, not just String",
			input: `
let name = "World"
let count = 5
let greeting = "Hello, ${name}! count=${count}"
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
				"semantic error: pure functions cannot capture global/module variable 'counter'",
				"semantic error: cannot mutate outer scope variable 'counter' inside a function",
				"semantic error: pure functions cannot capture global/module variable 'counter'",
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
				"semantic error: pure functions cannot capture global/module variable 'arr'",
				"semantic error: pure functions cannot capture global/module variable 'arr'",
			},
		},
		{
			name: "Reading outer scope module variable inside a function (Purity violation)",
			input: `
let data = [1, 2, 3]

let delayed_print = fn() {
    let result = data[0]
}
`,
			expectedErrors: []string{
				"semantic error: pure functions cannot capture global/module variable 'data'",
			},
		},
		{
			name: "Reading a module variable inside a string interpolation (Purity violation)",
			input: `
let counter = 5

let f = fn() -> String {
    return "count is ${counter}"
}
`,
			expectedErrors: []string{
				"semantic error: pure functions cannot capture global/module variable 'counter'",
			},
		},
		{
			name: "Reading outer scope global constant inside a function (Allowed)",
			input: `
const PI = 3.14

let calculate = fn(r: Number) -> Number {
    return r * PI
}
`,
			expectedErrors: []string{}, // No errors expected
		},
		{
			name: "Use after move error",
			input: `
let data = [1, 2, 3]
let x = move data
let y = data[0] # Error
`,
			expectedErrors: []string{
				"semantic error: use of moved variable 'data'",
			},
		},
		{
			name: "Use after conditional move error",
			input: `
let data = [1, 2, 3]
if (true) {
	let x = move data
}
let y = data[0] # Error
`,
			expectedErrors: []string{
				"semantic error: use of moved variable 'data'",
			},
		},
		{
			name: "Cannot move constant",
			input: `
const data = [1, 2, 3]
let x = move data # Error
`,
			expectedErrors: []string{
				"semantic error: cannot move constant variable 'data'",
			},
		},
		{
			name: "Union: non-variant struct assignment rejected",
			input: `
type Cat struct { name String }
type Dog struct { name String }
type Pig struct { name String }
union Animal = Cat | Dog
let animal: Animal = Pig { name: "Porky" }
`,
			expectedErrors: []string{
				"cannot assign",
			},
		},
		{
			name: "Union: 'is' on a non-union type rejected",
			input: `
type Cat struct { name String }
let c = Cat { name: "Tom" }
let cat: Cat? = c is Cat
`,
			expectedErrors: []string{
				"'is' can only be used on a union type",
			},
		},
		{
			name: "Union: 'is' with an unlisted variant rejected",
			input: `
type Cat struct { name String }
type Dog struct { name String }
type Pig struct { name String }
union Animal = Cat | Dog
let animal: Animal = Cat { name: "Tom" }
let p: Pig? = animal is Pig
`,
			expectedErrors: []string{
				"is not a variant of union",
			},
		},
		{
			name: "Union: duplicate union name rejected",
			input: `
type Cat struct { name String }
type Dog struct { name String }
union Animal = Cat
union Animal = Dog
`,
			expectedErrors: []string{
				"is already declared",
			},
		},
		{
			name: "Union: generic struct variant rejected",
			input: `
type Box<T> struct { value T }
type Dog struct { name String }
union Weird = Box | Dog
`,
			expectedErrors: []string{
				"union variant 'Box' cannot be a generic struct type",
			},
		},
		{
			name: "Union: function-scoped declaration rejected",
			input: `
type Cat struct { name String }
type Dog struct { name String }
let f = fn() -> Number {
	union Animal = Cat | Dog
	return 0
}
`,
			expectedErrors: []string{
				"'union' can only be declared at the top level of a module",
			},
		},
		{
			name: "Type: function-scoped declaration rejected (standardized with union/define)",
			input: `
let f = fn() -> Number {
	type Local struct { x Number }
	return 0
}
`,
			expectedErrors: []string{
				"'type' can only be declared at the top level of a module",
			},
		},
		{
			name: "Define: function-scoped declaration rejected (standardized with union/type)",
			input: `
type Cat struct { age Number }
let f = fn() -> Number {
	define Adult constraints Cat with: fn(c: Cat) -> Boolean { return c.age > 18 }
	return 0
}
`,
			expectedErrors: []string{
				"'define' can only be declared at the top level of a module",
			},
		},
		{
			name: "Redeclaration: type name reused by define",
			input: `
type Cat struct { name String }
type Dog struct { name String }
define Cat constraints Dog with: fn(d: Dog) -> Boolean { return true }
`,
			expectedErrors: []string{
				"is already declared",
			},
		},
		{
			name: "Redeclaration: type name reused by union",
			input: `
type Cat struct { name String }
type Dog struct { name String }
union Cat = Dog
`,
			expectedErrors: []string{
				"is already declared",
			},
		},
		{
			name: "Redeclaration: variable shadowing an existing type name",
			input: `
type Cat struct { name String }
let Cat = 5
`,
			expectedErrors: []string{
				"is already declared",
			},
		},
		{
			name: "Redeclaration: type name reused by an existing variable",
			input: `
let Cat = 5
type Cat struct { name String }
`,
			expectedErrors: []string{
				"is already declared",
			},
		},
		{
			name: "Redeclaration: union name reused by a variable",
			input: `
type Cat struct { name String }
type Dog struct { name String }
union Animal = Cat | Dog
let Animal = 5
`,
			expectedErrors: []string{
				"is already declared",
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
			name: "If without an else branch inside a recursive function does not panic",
			input: `
let fact = fn(n: Number, acc: Number) -> Number {
	if (n == 0) {
		return acc
	}
	return fact(n - 1, acc * n)
}
`,
			expectedErrors: []string{},
		},
		{
			name: "If without an else branch, with unrelated statements, still detects unconditional recursion",
			input: `
let loop = fn(n: Number) -> Number {
	if (n == 0) {
		let unused = 1
	}
	return loop(n)
}
`,
			expectedErrors: []string{
				"semantic error: function 'loop' contains unconditional recursion and will infinitely loop",
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
			name: "Anonymous function inference failure (LetStatement wrong return)",
			input: `
type CustomFunc fn(Number) -> Number
let f: CustomFunc = x => "hello"
`,
			expectedErrors: []string{
				"type error: function declared to return Number, but body returns String",
			},
		},
		{
			name: "Anonymous function inference failure (CallExpression wrong return)",
			input: `
let applyOp = fn(op: fn(Number, Number) -> Number) -> Number {
	return op(10, 20)
}
let res = applyOp((x, y) => x == y)
`,
			expectedErrors: []string{
				"type error: function declared to return Number, but body returns Boolean",
			},
		},
		{
			name: "Anonymous function missing context error",
			input: `
let sum = (p, q) => p + q
`,
			expectedErrors: []string{
				"type error: cannot infer type for parameter 'p'. Provide an explicit type or context.",
				"type error: cannot infer type for parameter 'q'. Provide an explicit type or context.",
			},
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
		{
			name:  "map.keys() works with correct types",
			input: "import map\nlet m: map[String]Number = {}\nlet ks: [String] = map.keys(m)\nreturn ks",
		},
		{
			name:           "map.keys() rejects mismatched argument type",
			input:          "import map\nreturn map.keys(\"not a map\")",
			expectedErrors: []string{"type error: argument to 'keys' must be Map, got String"},
		},
		{
			name:           "map.keys() rejects wrong arity",
			input:          "import map\nlet m: map[String]Number = {}\nreturn map.keys(m, \"extra\")",
			expectedErrors: []string{"arity error: expected 1 argument for 'keys', got 2"},
		},
		{
			name:  "map.values() works with correct types",
			input: "import map\nlet m: map[String]Number = {}\nlet vs: [Number] = map.values(m)\nreturn vs",
		},
		{
			name:           "map.values() rejects mismatched argument type",
			input:          "import map\nreturn map.values(\"not a map\")",
			expectedErrors: []string{"type error: argument to 'values' must be Map, got String"},
		},
		{
			name:           "map.values() rejects wrong arity",
			input:          "import map\nlet m: map[String]Number = {}\nreturn map.values(m, \"extra\")",
			expectedErrors: []string{"arity error: expected 1 argument for 'values', got 2"},
		},
		{
			name:  "browser.log() works with a string argument",
			input: "import browser\nreturn browser.log(\"hello\")",
		},
		{
			name:           "browser.log() rejects a non-string argument",
			input:          "import browser\nreturn browser.log(42)",
			expectedErrors: []string{"type error: first argument to 'log' must be String, got Number"},
		},
		{
			name:           "browser.alert() rejects missing arguments",
			input:          "import browser\nreturn browser.alert()",
			expectedErrors: []string{"arity error: expected 1 arguments for 'alert', got 0"},
		},
		{
			name:  "browser.getElementById() returns an Element",
			input: "import browser\nlet el: browser.Element = browser.getElementById(\"app\")\nreturn el",
		},
		{
			name:           "browser.getElementById() rejects a non-string argument",
			input:          "import browser\nreturn browser.getElementById(42)",
			expectedErrors: []string{"type error: first argument to 'getElementById' must be String, got Number"},
		},
		{
			name:  "browser.setText() works with an Element and a String",
			input: "import browser\nlet el = browser.getElementById(\"app\")\nreturn browser.setText(el, \"hi\")",
		},
		{
			name:           "browser.setText() rejects a non-Element first argument",
			input:          "import browser\nreturn browser.setText(\"not an element\", \"hi\")",
			expectedErrors: []string{"type error: first argument to 'setText' must be Element, got String"},
		},
		{
			name:           "browser.setHTML() rejects a non-String second argument",
			input:          "import browser\nlet el = browser.getElementById(\"app\")\nreturn browser.setHTML(el, 42)",
			expectedErrors: []string{"type error: second argument to 'setHTML' must be String, got Number"},
		},
		{
			name:  "browser.getValue() works with an Element and returns String",
			input: "import browser\nlet el = browser.getElementById(\"name\")\nlet v: String = browser.getValue(el)\nreturn v",
		},
		{
			name:           "browser.getValue() rejects a non-Element argument",
			input:          "import browser\nreturn browser.getValue(\"not an element\")",
			expectedErrors: []string{"type error: first argument to 'getValue' must be Element, got String"},
		},
		{
			name:           "browser.getValue() rejects missing arguments",
			input:          "import browser\nreturn browser.getValue()",
			expectedErrors: []string{"arity error: expected 1 arguments for 'getValue', got 0"},
		},
		{
			name:  "browser.setValue() works with an Element and a String",
			input: "import browser\nlet el = browser.getElementById(\"name\")\nreturn browser.setValue(el, \"hi\")",
		},
		{
			name:           "browser.setValue() rejects a non-Element first argument",
			input:          "import browser\nreturn browser.setValue(\"not an element\", \"hi\")",
			expectedErrors: []string{"type error: first argument to 'setValue' must be Element, got String"},
		},
		{
			name:           "browser.setValue() rejects a non-String second argument",
			input:          "import browser\nlet el = browser.getElementById(\"name\")\nreturn browser.setValue(el, 42)",
			expectedErrors: []string{"type error: second argument to 'setValue' must be String, got Number"},
		},
		{
			name:  "browser.querySelector() works with a String selector and returns Nullable Element",
			input: "import browser\nlet el: browser.Element? = browser.querySelector(\".item\")\nreturn el",
		},
		{
			name:           "browser.querySelector() rejects a non-String selector",
			input:          "import browser\nreturn browser.querySelector(42)",
			expectedErrors: []string{"type error: first argument to 'querySelector' must be String, got Number"},
		},
		{
			name:           "browser.querySelector() rejects missing arguments",
			input:          "import browser\nreturn browser.querySelector()",
			expectedErrors: []string{"arity error: expected 1 arguments for 'querySelector', got 0"},
		},
		{
			name:  "browser.querySelectorAll() works with a String selector and returns Array<Element>",
			input: "import browser\nlet els: [browser.Element] = browser.querySelectorAll(\".item\")\nreturn els",
		},
		{
			name:           "browser.querySelectorAll() rejects a non-String selector",
			input:          "import browser\nreturn browser.querySelectorAll(42)",
			expectedErrors: []string{"type error: first argument to 'querySelectorAll' must be String, got Number"},
		},
		{
			name:           "browser.querySelectorAll() rejects missing arguments",
			input:          "import browser\nreturn browser.querySelectorAll()",
			expectedErrors: []string{"arity error: expected 1 arguments for 'querySelectorAll', got 0"},
		},
		{
			name:  "browser.setAttribute() works with an Element and two Strings",
			input: "import browser\nlet el = browser.getElementById(\"box\")\nreturn browser.setAttribute(el, \"data-role\", \"widget\")",
		},
		{
			name:           "browser.setAttribute() rejects a non-Element first argument",
			input:          "import browser\nreturn browser.setAttribute(\"not an element\", \"data-role\", \"widget\")",
			expectedErrors: []string{"type error: first argument to 'setAttribute' must be Element, got String"},
		},
		{
			name:           "browser.setAttribute() rejects a non-String second argument",
			input:          "import browser\nlet el = browser.getElementById(\"box\")\nreturn browser.setAttribute(el, 42, \"widget\")",
			expectedErrors: []string{"type error: second argument to 'setAttribute' must be String, got Number"},
		},
		{
			name:           "browser.setAttribute() rejects a non-String third argument",
			input:          "import browser\nlet el = browser.getElementById(\"box\")\nreturn browser.setAttribute(el, \"data-role\", 42)",
			expectedErrors: []string{"type error: third argument to 'setAttribute' must be String, got Number"},
		},
		{
			name:           "browser.setAttribute() rejects missing arguments",
			input:          "import browser\nlet el = browser.getElementById(\"box\")\nreturn browser.setAttribute(el, \"data-role\")",
			expectedErrors: []string{"arity error: expected 3 arguments for 'setAttribute', got 2"},
		},
		{
			// No explicit "String?" annotation here: Caja's parser rejects a
			// nullable annotation on a primitive type outright ("primitive
			// type 'String' cannot be nullable") — only builtins can produce
			// a Nullable<primitive> value (there's no user-facing syntax for
			// it), so this just relies on inference like every real caller
			// (e.g. the safe browser.caja sample) already does.
			name:  "browser.getAttribute() works with an Element and a String, returns Nullable String",
			input: "import browser\nlet el = browser.getElementById(\"box\")\nlet v = browser.getAttribute(el, \"data-role\")\nreturn v",
		},
		{
			name:           "browser.getAttribute() rejects a non-Element first argument",
			input:          "import browser\nreturn browser.getAttribute(\"not an element\", \"data-role\")",
			expectedErrors: []string{"type error: first argument to 'getAttribute' must be Element, got String"},
		},
		{
			name:           "browser.getAttribute() rejects a non-String second argument",
			input:          "import browser\nlet el = browser.getElementById(\"box\")\nreturn browser.getAttribute(el, 42)",
			expectedErrors: []string{"type error: second argument to 'getAttribute' must be String, got Number"},
		},
		{
			name:           "browser.getAttribute() rejects missing arguments",
			input:          "import browser\nlet el = browser.getElementById(\"box\")\nreturn browser.getAttribute(el)",
			expectedErrors: []string{"arity error: expected 2 arguments for 'getAttribute', got 1"},
		},
		{
			name:  "browser.addClass() works with an Element and a String",
			input: "import browser\nlet el = browser.getElementById(\"box\")\nreturn browser.addClass(el, \"highlight\")",
		},
		{
			name:           "browser.addClass() rejects a non-String second argument",
			input:          "import browser\nlet el = browser.getElementById(\"box\")\nreturn browser.addClass(el, 42)",
			expectedErrors: []string{"type error: second argument to 'addClass' must be String, got Number"},
		},
		{
			name:  "browser.removeClass() works with an Element and a String",
			input: "import browser\nlet el = browser.getElementById(\"box\")\nreturn browser.removeClass(el, \"highlight\")",
		},
		{
			name:           "browser.removeClass() rejects a non-Element first argument",
			input:          "import browser\nreturn browser.removeClass(\"not an element\", \"highlight\")",
			expectedErrors: []string{"type error: first argument to 'removeClass' must be Element, got String"},
		},
		{
			name:  "browser.on() works with an event String, an Element, and a niladic handler",
			input: "import browser\nlet el = browser.getElementById(\"btn\")\nreturn browser.on(\"click\", el, fn() -> Nothing { browser.log(\"clicked\") })",
		},
		{
			name:           "browser.on() rejects a non-String event",
			input:          "import browser\nlet el = browser.getElementById(\"btn\")\nreturn browser.on(42, el, fn() -> Nothing { browser.log(\"clicked\") })",
			expectedErrors: []string{"type error: first argument to 'on' must be String, got Number"},
		},
		{
			name:           "browser.on() rejects a non-Element second argument",
			input:          "import browser\nreturn browser.on(\"click\", \"not an element\", fn() -> Nothing { browser.log(\"clicked\") })",
			expectedErrors: []string{"type error: second argument to 'on' must be Element, got String"},
		},
		{
			name:           "browser.on() rejects a handler that takes arguments",
			input:          "import browser\nlet el = browser.getElementById(\"btn\")\nreturn browser.on(\"click\", el, fn(x: Number) -> Nothing { browser.log(\"clicked\") })",
			expectedErrors: []string{"type error: third argument to 'on' must be a function taking 0 arguments, got fn(x: Number) -> Nothing"},
		},
		{
			name:           "browser.on() rejects a non-function third argument",
			input:          "import browser\nlet el = browser.getElementById(\"btn\")\nreturn browser.on(\"click\", el, 42)",
			expectedErrors: []string{"type error: third argument to 'on' must be a function, got Number"},
		},
		{
			name:           "browser.on() rejects missing arguments",
			input:          "import browser\nlet el = browser.getElementById(\"btn\")\nreturn browser.on(\"click\", el)",
			expectedErrors: []string{"arity error: expected 3 arguments for 'on', got 2"},
		},
		// Systemic check: a Nullable value (Element?/String?) is rejected
		// wherever a plain, non-nullable Element/String is required —
		// NullableSymbol.Type() deliberately forwards to its underlying
		// type, so a plain .Type() check alone would silently accept it too
		// (see checkBrowserArgNotNullable in builtin.go for why that's
		// unsound). A handful of representative call sites, not all sixteen.
		{
			name:           "browser.setText() rejects a Nullable Element (querySelector's result, unnarrowed)",
			input:          "import browser\nlet el = browser.querySelector(\".item\")\nreturn browser.setText(el, \"hi\")",
			expectedErrors: []string{"type error: first argument to 'setText' must be a non-nullable Element, got Element? — use cast.to(value, fallback) to unwrap it first"},
		},
		{
			name:           "browser.getAttribute() rejects a Nullable Element even inside a nil-check (Caja has no if-narrowing)",
			input:          "import browser\nlet el = browser.querySelector(\".item\")\nif (el != nil) { return browser.getAttribute(el, \"data-role\") }\nreturn nil",
			expectedErrors: []string{"type error: first argument to 'getAttribute' must be a non-nullable Element, got Element? — use cast.to(value, fallback) to unwrap it first"},
		},
		{
			name:           "browser.on() rejects a Nullable Element for its el argument",
			input:          "import browser\nlet el = browser.querySelector(\".item\")\nreturn browser.on(\"click\", el, fn() -> Nothing { browser.log(\"x\") })",
			expectedErrors: []string{"type error: second argument to 'on' must be a non-nullable Element, got Element? — use cast.to(value, fallback) to unwrap it first"},
		},
		{
			name:           "browser.fetchThen() rejects a Nullable String url (e.g. from getAttribute)",
			input:          "import browser\nlet el = browser.getElementById(\"link\")\nlet href = browser.getAttribute(el, \"href\")\nreturn browser.fetchThen(href, fn(body: String) -> Nothing { browser.log(body) })",
			expectedErrors: []string{"type error: first argument to 'fetchThen' must be a non-nullable String, got String? — use cast.to(value, fallback) to unwrap it first"},
		},
		{
			// cast.to is the correct way to consume it — confirming the
			// systemic check doesn't also reject the *unwrapped* value.
			name:  "browser.setText() works with a querySelector result unwrapped via cast.to",
			input: "import browser\nimport cast\nlet el = browser.querySelector(\".item\")\nreturn browser.setText(cast.to(el, browser.getElementById(\"app\")), \"hi\")",
		},
		{
			name:  "browser.fetch() works with a String url and returns String",
			input: "import browser\nlet body: String = browser.fetch(\"/data\")\nreturn body",
		},
		{
			name:           "browser.fetch() rejects a non-String url",
			input:          "import browser\nreturn browser.fetch(42)",
			expectedErrors: []string{"type error: first argument to 'fetch' must be String, got Number"},
		},
		{
			name:           "browser.fetch() rejects missing arguments",
			input:          "import browser\nreturn browser.fetch()",
			expectedErrors: []string{"arity error: expected 1 arguments for 'fetch', got 0"},
		},
		{
			name:           "browser.fetch() rejects being called inside a function",
			input:          "import browser\nlet f = fn() -> String { return browser.fetch(\"/data\") }",
			expectedErrors: []string{"semantic error: 'browser.fetch' can only be called at the top level of a script, not inside a function — it can permanently freeze the page if that function ever runs as (or from) a browser.on handler; use browser.fetchThen instead"},
		},
		{
			name:           "browser.fetch() rejects being called inside a function even wrapped in async",
			input:          "import browser\nlet f = fn() -> Nothing { let t = async browser.fetch(\"/data\") }",
			expectedErrors: []string{"semantic error: 'browser.fetch' can only be called at the top level of a script, not inside a function — it can permanently freeze the page if that function ever runs as (or from) a browser.on handler; use browser.fetchThen instead"},
		},
		{
			name:  "browser.fetch() works at the top level of a script",
			input: "import browser\nlet body = browser.fetch(\"/data\")\nreturn body",
		},
		{
			name:  "browser.fetchThen() works with a String url and a String-arg handler",
			input: "import browser\nreturn browser.fetchThen(\"/data\", fn(body: String) -> Nothing { browser.log(body) })",
		},
		{
			name:           "browser.fetchThen() rejects a non-String url",
			input:          "import browser\nreturn browser.fetchThen(42, fn(body: String) -> Nothing { browser.log(body) })",
			expectedErrors: []string{"type error: first argument to 'fetchThen' must be String, got Number"},
		},
		{
			name:           "browser.fetchThen() rejects a handler taking the wrong number of arguments",
			input:          "import browser\nreturn browser.fetchThen(\"/data\", fn() -> Nothing { browser.log(\"x\") })",
			expectedErrors: []string{"type error: second argument to 'fetchThen' must be a function taking 1 argument, got fn() -> Nothing"},
		},
		{
			name:           "browser.fetchThen() rejects a handler whose argument isn't a String",
			input:          "import browser\nreturn browser.fetchThen(\"/data\", fn(n: Number) -> Nothing { browser.log(\"x\") })",
			expectedErrors: []string{"type error: second argument to 'fetchThen' must be a function taking a String, got fn(n: Number) -> Nothing"},
		},
		{
			name:           "browser.fetchThen() rejects a non-function second argument",
			input:          "import browser\nreturn browser.fetchThen(\"/data\", 42)",
			expectedErrors: []string{"type error: second argument to 'fetchThen' must be a function, got Number"},
		},
		{
			name:           "browser.fetchThen() rejects missing arguments",
			input:          "import browser\nreturn browser.fetchThen(\"/data\")",
			expectedErrors: []string{"arity error: expected 2 arguments for 'fetchThen', got 1"},
		},
		{
			name:  "browser.localStorageGet() works with a String key, returns Nullable String",
			input: "import browser\nlet v = browser.localStorageGet(\"theme\")\nreturn v",
		},
		{
			name:           "browser.localStorageGet() rejects a non-String key",
			input:          "import browser\nreturn browser.localStorageGet(42)",
			expectedErrors: []string{"type error: first argument to 'localStorageGet' must be String, got Number"},
		},
		{
			name:           "browser.localStorageGet() rejects missing arguments",
			input:          "import browser\nreturn browser.localStorageGet()",
			expectedErrors: []string{"arity error: expected 1 arguments for 'localStorageGet', got 0"},
		},
		{
			name:           "browser.localStorageGet() rejects a Nullable String key (Caja has no if-narrowing)",
			input:          "import browser\nlet el = browser.getElementById(\"box\")\nlet key = browser.getAttribute(el, \"data-key\")\nreturn browser.localStorageGet(key)",
			expectedErrors: []string{"type error: first argument to 'localStorageGet' must be a non-nullable String, got String? — use cast.to(value, fallback) to unwrap it first"},
		},
		{
			name:  "page.write() works with two String arguments",
			input: "import page\nreturn page.write(\"dist/index.html\", \"<h1>hi</h1>\")",
		},
		{
			name:           "page.write() rejects a non-String first argument",
			input:          "import page\nreturn page.write(42, \"<h1>hi</h1>\")",
			expectedErrors: []string{"type error: first argument to 'write' must be String, got Number"},
		},
		{
			name:           "page.write() rejects a non-String second argument",
			input:          "import page\nreturn page.write(\"dist/index.html\", 42)",
			expectedErrors: []string{"type error: second argument to 'write' must be String, got Number"},
		},
		{
			name:           "page.write() rejects missing arguments",
			input:          "import page\nreturn page.write(\"dist/index.html\")",
			expectedErrors: []string{"arity error: expected 2 arguments for 'write', got 1"},
		},
		{
			name:  "browser.localStorageSet() works with two String arguments",
			input: "import browser\nreturn browser.localStorageSet(\"theme\", \"dark\")",
		},
		{
			name:           "browser.localStorageSet() rejects a non-String first argument",
			input:          "import browser\nreturn browser.localStorageSet(42, \"dark\")",
			expectedErrors: []string{"type error: first argument to 'localStorageSet' must be String, got Number"},
		},
		{
			name:           "browser.localStorageSet() rejects a non-String second argument",
			input:          "import browser\nreturn browser.localStorageSet(\"theme\", 42)",
			expectedErrors: []string{"type error: second argument to 'localStorageSet' must be String, got Number"},
		},
		{
			name:           "browser.localStorageSet() rejects missing arguments",
			input:          "import browser\nreturn browser.localStorageSet(\"theme\")",
			expectedErrors: []string{"arity error: expected 2 arguments for 'localStorageSet', got 1"},
		},
		{
			name:  "browser.localStorageRemove() works with a String key",
			input: "import browser\nreturn browser.localStorageRemove(\"theme\")",
		},
		{
			name:           "browser.localStorageRemove() rejects a non-String key",
			input:          "import browser\nreturn browser.localStorageRemove(42)",
			expectedErrors: []string{"type error: first argument to 'localStorageRemove' must be String, got Number"},
		},
		{
			name:           "browser.localStorageRemove() rejects missing arguments",
			input:          "import browser\nreturn browser.localStorageRemove()",
			expectedErrors: []string{"arity error: expected 1 arguments for 'localStorageRemove', got 0"},
		},
		{
			name:  "browser.createElement() works with a String tag, returns Element",
			input: "import browser\nlet el = browser.createElement(\"li\")\nreturn el",
		},
		{
			name:           "browser.createElement() rejects a non-String tag",
			input:          "import browser\nreturn browser.createElement(42)",
			expectedErrors: []string{"type error: first argument to 'createElement' must be String, got Number"},
		},
		{
			name:           "browser.createElement() rejects missing arguments",
			input:          "import browser\nreturn browser.createElement()",
			expectedErrors: []string{"arity error: expected 1 arguments for 'createElement', got 0"},
		},
		{
			name:           "browser.createElement() rejects a Nullable String tag (Caja has no if-narrowing)",
			input:          "import browser\nlet el = browser.getElementById(\"box\")\nlet tag = browser.getAttribute(el, \"data-tag\")\nreturn browser.createElement(tag)",
			expectedErrors: []string{"type error: first argument to 'createElement' must be a non-nullable String, got String? — use cast.to(value, fallback) to unwrap it first"},
		},
		{
			name:  "browser.appendChild() works with two Elements",
			input: "import browser\nlet parent = browser.getElementById(\"list\")\nlet child = browser.createElement(\"li\")\nreturn browser.appendChild(parent, child)",
		},
		{
			name:           "browser.appendChild() rejects a non-Element first argument",
			input:          "import browser\nlet child = browser.createElement(\"li\")\nreturn browser.appendChild(\"not an element\", child)",
			expectedErrors: []string{"type error: first argument to 'appendChild' must be Element, got String"},
		},
		{
			name:           "browser.appendChild() rejects a non-Element second argument",
			input:          "import browser\nlet parent = browser.getElementById(\"list\")\nreturn browser.appendChild(parent, \"not an element\")",
			expectedErrors: []string{"type error: second argument to 'appendChild' must be Element, got String"},
		},
		{
			name:           "browser.appendChild() rejects missing arguments",
			input:          "import browser\nlet parent = browser.getElementById(\"list\")\nreturn browser.appendChild(parent)",
			expectedErrors: []string{"arity error: expected 2 arguments for 'appendChild', got 1"},
		},
		{
			name:           "browser.appendChild() rejects a Nullable Element for its second argument",
			input:          "import browser\nlet parent = browser.getElementById(\"list\")\nlet child = browser.querySelector(\".item\")\nreturn browser.appendChild(parent, child)",
			expectedErrors: []string{"type error: second argument to 'appendChild' must be a non-nullable Element, got Element? — use cast.to(value, fallback) to unwrap it first"},
		},
		{
			name:  "browser.removeElement() works with an Element",
			input: "import browser\nlet el = browser.getElementById(\"box\")\nreturn browser.removeElement(el)",
		},
		{
			name:           "browser.removeElement() rejects a non-Element argument",
			input:          "import browser\nreturn browser.removeElement(\"not an element\")",
			expectedErrors: []string{"type error: first argument to 'removeElement' must be Element, got String"},
		},
		{
			name:           "browser.removeElement() rejects missing arguments",
			input:          "import browser\nreturn browser.removeElement()",
			expectedErrors: []string{"arity error: expected 1 arguments for 'removeElement', got 0"},
		},
		{
			name:           "browser.removeElement() rejects a Nullable Element (Caja has no if-narrowing)",
			input:          "import browser\nlet el = browser.querySelector(\".item\")\nreturn browser.removeElement(el)",
			expectedErrors: []string{"type error: first argument to 'removeElement' must be a non-nullable Element, got Element? — use cast.to(value, fallback) to unwrap it first"},
		},
		{
			name:  "browser.setStyle() works with an Element and two Strings",
			input: "import browser\nlet el = browser.getElementById(\"box\")\nreturn browser.setStyle(el, \"color\", \"blue\")",
		},
		{
			name:           "browser.setStyle() rejects a non-Element first argument",
			input:          "import browser\nreturn browser.setStyle(\"not an element\", \"color\", \"blue\")",
			expectedErrors: []string{"type error: first argument to 'setStyle' must be Element, got String"},
		},
		{
			name:           "browser.setStyle() rejects a non-String second argument",
			input:          "import browser\nlet el = browser.getElementById(\"box\")\nreturn browser.setStyle(el, 42, \"blue\")",
			expectedErrors: []string{"type error: second argument to 'setStyle' must be String, got Number"},
		},
		{
			name:           "browser.setStyle() rejects a non-String third argument",
			input:          "import browser\nlet el = browser.getElementById(\"box\")\nreturn browser.setStyle(el, \"color\", 42)",
			expectedErrors: []string{"type error: third argument to 'setStyle' must be String, got Number"},
		},
		{
			name:           "browser.setStyle() rejects missing arguments",
			input:          "import browser\nlet el = browser.getElementById(\"box\")\nreturn browser.setStyle(el, \"color\")",
			expectedErrors: []string{"arity error: expected 3 arguments for 'setStyle', got 2"},
		},
		{
			name:  "browser.removeAttribute() works with an Element and a String",
			input: "import browser\nlet el = browser.getElementById(\"box\")\nreturn browser.removeAttribute(el, \"data-open\")",
		},
		{
			name:           "browser.removeAttribute() rejects a non-Element first argument",
			input:          "import browser\nreturn browser.removeAttribute(\"not an element\", \"data-open\")",
			expectedErrors: []string{"type error: first argument to 'removeAttribute' must be Element, got String"},
		},
		{
			name:           "browser.removeAttribute() rejects missing arguments",
			input:          "import browser\nlet el = browser.getElementById(\"box\")\nreturn browser.removeAttribute(el)",
			expectedErrors: []string{"arity error: expected 2 arguments for 'removeAttribute', got 1"},
		},
		{
			name:  "browser.toggleClass() works with an Element and a String",
			input: "import browser\nlet el = browser.getElementById(\"box\")\nreturn browser.toggleClass(el, \"open\")",
		},
		{
			name:           "browser.toggleClass() rejects a non-String second argument",
			input:          "import browser\nlet el = browser.getElementById(\"box\")\nreturn browser.toggleClass(el, 42)",
			expectedErrors: []string{"type error: second argument to 'toggleClass' must be String, got Number"},
		},
		{
			name:  "browser.hasClass() works with an Element and a String, returns Boolean",
			input: "import browser\nlet el = browser.getElementById(\"box\")\nlet isOpen = browser.hasClass(el, \"open\")\nreturn isOpen",
		},
		{
			name:           "browser.hasClass() rejects a non-Element first argument",
			input:          "import browser\nreturn browser.hasClass(\"not an element\", \"open\")",
			expectedErrors: []string{"type error: first argument to 'hasClass' must be Element, got String"},
		},
		{
			name:           "browser.hasClass() rejects a non-String second argument",
			input:          "import browser\nlet el = browser.getElementById(\"box\")\nreturn browser.hasClass(el, 42)",
			expectedErrors: []string{"type error: second argument to 'hasClass' must be String, got Number"},
		},
		{
			name:           "browser.hasClass() rejects missing arguments",
			input:          "import browser\nlet el = browser.getElementById(\"box\")\nreturn browser.hasClass(el)",
			expectedErrors: []string{"arity error: expected 2 arguments for 'hasClass', got 1"},
		},
		{
			name:           "browser.hasClass() rejects a Nullable Element (Caja has no if-narrowing)",
			input:          "import browser\nlet el = browser.querySelector(\".item\")\nreturn browser.hasClass(el, \"open\")",
			expectedErrors: []string{"type error: first argument to 'hasClass' must be a non-nullable Element, got Element? — use cast.to(value, fallback) to unwrap it first"},
		},
		{
			name:  "browser.focus() and browser.blur() work with an Element",
			input: "import browser\nlet el = browser.getElementById(\"name\")\nbrowser.focus(el)\nreturn browser.blur(el)",
		},
		{
			name:           "browser.focus() rejects a non-Element argument",
			input:          "import browser\nreturn browser.focus(\"not an element\")",
			expectedErrors: []string{"type error: first argument to 'focus' must be Element, got String"},
		},
		{
			name:           "browser.blur() rejects missing arguments",
			input:          "import browser\nreturn browser.blur()",
			expectedErrors: []string{"arity error: expected 1 arguments for 'blur', got 0"},
		},
		{
			name:  "browser.getChecked() works with an Element, returns Boolean",
			input: "import browser\nlet box = browser.getElementById(\"remember\")\nlet checked = browser.getChecked(box)\nreturn checked",
		},
		{
			name:           "browser.getChecked() rejects a non-Element argument",
			input:          "import browser\nreturn browser.getChecked(\"not an element\")",
			expectedErrors: []string{"type error: first argument to 'getChecked' must be Element, got String"},
		},
		{
			name:  "browser.setChecked() works with an Element and a Boolean",
			input: "import browser\nlet box = browser.getElementById(\"remember\")\nreturn browser.setChecked(box, true)",
		},
		{
			name:           "browser.setChecked() rejects a non-Boolean second argument",
			input:          "import browser\nlet box = browser.getElementById(\"remember\")\nreturn browser.setChecked(box, \"true\")",
			expectedErrors: []string{"type error: second argument to 'setChecked' must be Boolean, got String"},
		},
		{
			name:           "browser.setChecked() rejects missing arguments",
			input:          "import browser\nlet box = browser.getElementById(\"remember\")\nreturn browser.setChecked(box)",
			expectedErrors: []string{"arity error: expected 2 arguments for 'setChecked', got 1"},
		},
		{
			name:  "browser.insertBefore() works with three Elements",
			input: "import browser\nlet list = browser.getElementById(\"list\")\nlet ref = browser.getElementById(\"a-item\")\nlet newItem = browser.createElement(\"li\")\nreturn browser.insertBefore(list, newItem, ref)",
		},
		{
			name:           "browser.insertBefore() rejects a non-Element first argument",
			input:          "import browser\nlet ref = browser.getElementById(\"a-item\")\nlet newItem = browser.createElement(\"li\")\nreturn browser.insertBefore(\"not an element\", newItem, ref)",
			expectedErrors: []string{"type error: first argument to 'insertBefore' must be Element, got String"},
		},
		{
			name:           "browser.insertBefore() rejects a non-Element second argument",
			input:          "import browser\nlet list = browser.getElementById(\"list\")\nlet ref = browser.getElementById(\"a-item\")\nreturn browser.insertBefore(list, \"not an element\", ref)",
			expectedErrors: []string{"type error: second argument to 'insertBefore' must be Element, got String"},
		},
		{
			name:           "browser.insertBefore() rejects a non-Element third argument",
			input:          "import browser\nlet list = browser.getElementById(\"list\")\nlet newItem = browser.createElement(\"li\")\nreturn browser.insertBefore(list, newItem, \"not an element\")",
			expectedErrors: []string{"type error: third argument to 'insertBefore' must be Element, got String"},
		},
		{
			name:           "browser.insertBefore() rejects missing arguments",
			input:          "import browser\nlet list = browser.getElementById(\"list\")\nlet newItem = browser.createElement(\"li\")\nreturn browser.insertBefore(list, newItem)",
			expectedErrors: []string{"arity error: expected 3 arguments for 'insertBefore', got 2"},
		},
		{
			name:           "browser.insertBefore() rejects a Nullable Element for its second argument",
			input:          "import browser\nlet list = browser.getElementById(\"list\")\nlet ref = browser.getElementById(\"a-item\")\nlet newItem = browser.querySelector(\".item\")\nreturn browser.insertBefore(list, newItem, ref)",
			expectedErrors: []string{"type error: second argument to 'insertBefore' must be a non-nullable Element, got Element? — use cast.to(value, fallback) to unwrap it first"},
		},
		{
			name:  "browser.setTimeout() works with a Number and a niladic handler, returns Number",
			input: "import browser\nlet id = browser.setTimeout(2000, fn() -> Nothing { browser.log(\"fired\") })\nreturn id",
		},
		{
			name:           "browser.setTimeout() rejects a non-Number first argument",
			input:          "import browser\nreturn browser.setTimeout(\"2000\", fn() -> Nothing { browser.log(\"fired\") })",
			expectedErrors: []string{"type error: first argument to 'setTimeout' must be Number, got String"},
		},
		{
			name:           "browser.setTimeout() rejects a handler that takes arguments",
			input:          "import browser\nreturn browser.setTimeout(2000, fn(x: Number) -> Nothing { browser.log(\"fired\") })",
			expectedErrors: []string{"type error: second argument to 'setTimeout' must be a function taking 0 arguments, got fn(x: Number) -> Nothing"},
		},
		{
			name:           "browser.setTimeout() rejects a non-function second argument",
			input:          "import browser\nreturn browser.setTimeout(2000, 42)",
			expectedErrors: []string{"type error: second argument to 'setTimeout' must be a function, got Number"},
		},
		{
			name:           "browser.setTimeout() rejects missing arguments",
			input:          "import browser\nreturn browser.setTimeout(2000)",
			expectedErrors: []string{"arity error: expected 2 arguments for 'setTimeout', got 1"},
		},
		{
			name:  "browser.clearTimeout() works with a Number",
			input: "import browser\nlet id = browser.setTimeout(2000, fn() -> Nothing { browser.log(\"fired\") })\nreturn browser.clearTimeout(id)",
		},
		{
			name:           "browser.clearTimeout() rejects a non-Number argument",
			input:          "import browser\nreturn browser.clearTimeout(\"not a number\")",
			expectedErrors: []string{"type error: first argument to 'clearTimeout' must be Number, got String"},
		},
		{
			name:           "browser.clearTimeout() rejects missing arguments",
			input:          "import browser\nreturn browser.clearTimeout()",
			expectedErrors: []string{"arity error: expected 1 arguments for 'clearTimeout', got 0"},
		},
		{
			name:  "string.format() works with a literal format string and a Number value",
			input: "import string\nreturn string.format(\"%.2f\", 3.14159)",
		},
		{
			name:  "string.format() accepts any type for its second argument",
			input: "import string\ntype Point struct { x Number }\nreturn string.format(\"%v\", Point{ x: 1 })",
		},
		{
			name:           "string.format() rejects a non-String first argument",
			input:          "import string\nreturn string.format(5, \"x\")",
			expectedErrors: []string{"type error: first argument to 'format' must be String, got Number"},
		},
		{
			name:           "string.format() rejects missing arguments",
			input:          "import string\nreturn string.format(\"%d\")",
			expectedErrors: []string{"arity error: expected 2 arguments for 'format', got 1"},
		},
	}

	runTestScenarios(t, tests)
}

func TestSemanticAnalysisHTTP(t *testing.T) {
	tests := []testScenario{
		{
			name: "newRouter, route registration, use, and listen all type-check",
			input: `
import http
let router = http.newRouter()
router.get("/hello", fn(req: http.Request) -> http.Response {
	return http.ok("hi")
})
router.use(fn(next: http.Handler) -> http.Handler {
	return next
})
http.listen(router, 8080)
`,
		},
		{
			name: "router.static type-checks",
			input: `
import http
let router = http.newRouter()
router.static("/static/", "public")
`,
		},
		{
			name: "response constructors and json all type-check",
			input: `
import http
let a = http.ok("hi")
let b = http.text(201, "created")
let c = http.notFound("nope")
let d = http.badRequest("bad")
let e = http.serverError("oops")
let f = http.json(200, { "x": 1 })
`,
		},
		{
			name: "request struct fields resolve to their declared types",
			input: `
import http
import map
let handler = fn(req: http.Request) -> http.Response {
	let m = req.method
	let p = req.path
	let id = req.pathParams["id"]
	let q = req.query["q"]
	if (map.containsKey(req.headers, "Authorization")) {
		return http.ok(req.body)
	}
	return http.badRequest("missing header")
}
`,
		},
		{
			name: "middleware composition (fn(Handler) -> Handler) type-checks",
			input: `
import http
let requireAuth = fn(next: http.Handler) -> http.Handler {
	return fn(req: http.Request) -> http.Response {
		return next(req)
	}
}
let router = http.newRouter()
router.get("/admin", requireAuth(fn(req: http.Request) -> http.Response {
	return http.ok("welcome")
}))
`,
		},
		{
			name:           "router.get rejects wrong handler arity",
			input:          "import http\nlet router = http.newRouter()\nrouter.get(\"/x\", fn(a: http.Request, b: String) -> http.Response { return http.ok(\"y\") })",
			expectedErrors: []string{"type error"},
		},
		{
			name:           "router.get rejects wrong handler parameter type",
			input:          "import http\nlet router = http.newRouter()\nrouter.get(\"/x\", fn(req: String) -> http.Response { return http.ok(\"y\") })",
			expectedErrors: []string{"type error"},
		},
		{
			name:           "router.get rejects wrong argument count",
			input:          "import http\nlet router = http.newRouter()\nrouter.get(\"/x\")",
			expectedErrors: []string{"arity error"},
		},
		{
			name:           "http.listen rejects wrong argument count",
			input:          "import http\nlet router = http.newRouter()\nhttp.listen(router)",
			expectedErrors: []string{"arity error"},
		},
		{
			name: "req.ip resolves to String",
			input: `
import http
let handler = fn(req: http.Request) -> http.Response {
	let clientIp = req.ip
	return http.ok(clientIp)
}
`,
		},
		{
			name: "req.queryAll and req.headersAll resolve to Map<String, Array<String>>",
			input: `
import http
let handler = fn(req: http.Request) -> http.Response {
	let tags = req.queryAll["tag"]
	let first = tags[0]
	let accept = req.headersAll["Accept"]
	return http.ok(first)
}
`,
		},
		{
			name: "rateLimiter used via router.use type-checks",
			input: `
import http
let router = http.newRouter()
router.use(http.rateLimiter(10, 5, fn(req: http.Request) -> String {
	return req.ip
}))
`,
		},
		{
			name: "rateLimiter wrapping a single route type-checks",
			input: `
import http
let router = http.newRouter()
let limited = http.rateLimiter(10, 5, fn(req: http.Request) -> String {
	return req.ip
})
router.get("/x", limited(fn(req: http.Request) -> http.Response {
	return http.ok("y")
}))
`,
		},
		{
			name:           "rateLimiter rejects wrong argument count",
			input:          "import http\nlet mw = http.rateLimiter(10, 5)",
			expectedErrors: []string{"arity error"},
		},
		{
			name:           "rateLimiter rejects wrong keyFunc signature",
			input:          "import http\nlet mw = http.rateLimiter(10, 5, fn(req: http.Request) -> Number { return 1 })",
			expectedErrors: []string{"type error"},
		},
		{
			name: "concurrencyLimiter used via router.use type-checks",
			input: `
import http
let router = http.newRouter()
router.use(http.concurrencyLimiter(5, fn(req: http.Request) -> String {
	return req.ip
}))
`,
		},
		{
			name:           "concurrencyLimiter rejects wrong argument count",
			input:          "import http\nlet mw = http.concurrencyLimiter(5)",
			expectedErrors: []string{"arity error"},
		},
		{
			name:           "concurrencyLimiter rejects wrong keyFunc signature",
			input:          "import http\nlet mw = http.concurrencyLimiter(5, fn(req: http.Request) -> Number { return 1 })",
			expectedErrors: []string{"type error"},
		},
		{
			name: "distributedRateLimiter used via router.use type-checks",
			input: `
import http
let router = http.newRouter()
router.use(http.distributedRateLimiter(100, 60, fn(req: http.Request) -> String {
	return req.ip
}))
`,
		},
		{
			name: "distributedRateLimiter wrapping a single route type-checks",
			input: `
import http
let router = http.newRouter()
let limited = http.distributedRateLimiter(100, 60, fn(req: http.Request) -> String {
	return req.ip
})
router.get("/x", limited(fn(req: http.Request) -> http.Response {
	return http.ok("y")
}))
`,
		},
		{
			name:           "distributedRateLimiter rejects wrong argument count",
			input:          "import http\nlet mw = http.distributedRateLimiter(100, 60)",
			expectedErrors: []string{"arity error"},
		},
		{
			name:           "distributedRateLimiter rejects wrong keyFunc signature",
			input:          "import http\nlet mw = http.distributedRateLimiter(100, 60, fn(req: http.Request) -> Number { return 1 })",
			expectedErrors: []string{"type error"},
		},
		{
			name: "parseJSON result can be indexed and cast",
			input: `
import http
import cast
let handler = fn(req: http.Request) -> http.Response {
	let parsed = http.parseJSON(req.body)
	let name = cast.to(parsed["name"], "unknown")
	return http.json(200, { "name": name })
}
`,
		},
		{
			name:           "parseJSON rejects wrong argument count",
			input:          "import http\nlet parsed = http.parseJSON()",
			expectedErrors: []string{"arity error"},
		},
		{
			// http.Response/http.Request are ordinary module-exported struct
			// types (getHTTPStandardModule's second return value), so a
			// module-qualified struct literal ("http.Response{...}") must
			// resolve and type-check exactly like any other dotted struct
			// literal (find.go's dotted-name lookup through
			// ModuleSymbol.GetType) -- not just be reachable indirectly
			// through http.ok/text/json/newRouter.
			name: "http.Response and http.Request struct literals type-check directly",
			input: `
import http
let handler = fn(req: http.Request) -> http.Response {
	let synthetic = http.Request{
		method: "GET",
		path: "/x",
		pathParams: {"id": "1"},
		query: {},
		headers: {},
		body: "",
		ip: "127.0.0.1",
		queryAll: {},
		headersAll: {},
	}
	return http.Response{status: 200, headers: {"X-Custom": "yes"}, body: synthetic.method}
}
`,
		},
		{
			name:           "http.Response struct literal rejects wrong field type",
			input:          "import http\nlet r = http.Response{status: \"200\", headers: {}, body: \"x\"}",
			expectedErrors: []string{"type error"},
		},
		{
			name:           "http.Response struct literal rejects missing field",
			input:          "import http\nlet r = http.Response{status: 200, body: \"x\"}",
			expectedErrors: []string{"missing required field"},
		},
		{
			name: "newClient and all five verb methods type-check",
			input: `
import http
import cast
let client = http.newClient("https://api.example.com", {"X-Api-Key": "secret"})
let a = client.get("/x")
let b = client.post("/x", "body")
let c = client.put("/x", "body")
let d = client.delete("/x")
let e = client.patch("/x", "body")
let body = cast.to(a?.body, "fallback")
`,
		},
		{
			name: "newClient accepts an empty map literal for defaultHeaders",
			input: `
import http
let client = http.newClient("https://api.example.com", {})
let resp = client.get("/x")
`,
		},
		{
			name:           "newClient rejects wrong argument count",
			input:          "import http\nlet client = http.newClient(\"https://api.example.com\")",
			expectedErrors: []string{"arity error"},
		},
		{
			name:           "client.post rejects missing body argument",
			input:          "import http\nlet client = http.newClient(\"https://api.example.com\", {})\nclient.post(\"/x\")",
			expectedErrors: []string{"arity error"},
		},
		{
			name:           "client.get rejects wrong endpoint argument type",
			input:          "import http\nlet client = http.newClient(\"https://api.example.com\", {})\nclient.get(123)",
			expectedErrors: []string{"type error"},
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
				"semantic error: 'type' can only be declared at the top level of a module",
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
			input: "type Money Number\ntype Moment Boolean\ntype Name String\ntype Custom Number\nlet process = fn(m: Money, d: Moment, n: Name, c: Custom) -> Money { return m }\nprocess(100, true, \"John\", 42)",
		},
		{
			name:  "Type alias with array types",
			input: "type Prices [Number]\ntype Names [String]\ntype Holidays [Boolean]\ntype Collection [Number]\nlet addAll = fn(p: Prices, n: Names, h: Holidays, c: Collection) -> Prices { return p }\naddAll([1, 2], [\"a\"], [true], [1, 2])",
		},
		{
			name:           "Type alias mismatch",
			input:          "type Money Number\nlet add = fn(a: Money) -> Money { return a }\nadd(\"string\")",
			expectedErrors: []string{"type error: argument 1 expected Number, got String"},
		},
		{
			name:           "Undefined type in alias",
			input:          "type SomeType Some\nlet doSomething = fn(a: SomeType) -> Number { return 1 }",
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

func TestTypeConstraints(t *testing.T) {
	tests := []testScenario{
		{
			name: "Valid type constraint",
			input: `
type Customer struct { age Number }
define MajorCustomer constraints Customer with: fn(c: Customer) -> Boolean { return c.age > 18 }
let m: MajorCustomer? = Customer { age: 20 }
let c: Customer? = m
`,
			expectedErrors: []string{},
		},
		{
			name: "Type constraint missing base type",
			input: `
define MajorCustomer constraints Customer with: fn(c: Customer) -> Boolean { return true }
`,
			expectedErrors: []string{"semantic error: base type 'Customer' is not declared"},
		},
		{
			name: "Type constraint predicate not a function",
			input: `
type Customer struct { age Number }
define MajorCustomer constraints Customer with: 42
`,
			expectedErrors: []string{"type error: constraint predicate must be a function"},
		},
		{
			name: "Type constraint predicate wrong arg type",
			input: `
type Customer struct { age Number }
define MajorCustomer constraints Customer with: fn(c: Number) -> Boolean { return true }
`,
			expectedErrors: []string{"type error: constraint predicate must accept a single argument of type Customer"},
		},
		{
			name: "Type constraint predicate wrong return type",
			input: `
type Customer struct { age Number }
define MajorCustomer constraints Customer with: fn(c: Customer) -> Number { return 42 }
`,
			expectedErrors: []string{"type error: constraint predicate must return Boolean"},
		},
	}
	runTestScenarios(t, tests)
}

func TestAdvancedTypeConstraints(t *testing.T) {
	tests := []testScenario{
		{
			name: "Type constraint as a function parameter",
			input: `
type Customer struct { age Number }
define MajorCustomer constraints Customer with: fn(c: Customer) -> Boolean { return c.age > 18 }

let processMajor = fn(m: MajorCustomer?) -> Boolean {
	return true
}

let m: MajorCustomer? = Customer { age: 20 }
processMajor(m)
`,
			expectedErrors: []string{},
		},
		{
			name: "Type constraint inside a data-first pipeline",
			input: `
type Customer struct { age Number }
define MajorCustomer constraints Customer with: fn(c: Customer) -> Boolean { return c.age > 18 }

let processMajor = fn(m: MajorCustomer?) -> Boolean {
	return true
}

let m: MajorCustomer? = Customer { age: 20 }
m |> processMajor
`,
			expectedErrors: []string{},
		},
		{
			name: "Passing base type to non-nullable constraint parameter is prohibited by analyzer",
			input: `
type Customer struct { age Number }
define MajorCustomer constraints Customer with: fn(c: Customer) -> Boolean { return c.age > 18 }

let processMajor = fn(m: MajorCustomer) -> Boolean {
	return true
}

let c: Customer = Customer { age: 20 }
processMajor(c)
`,
			expectedErrors: []string{"type error: argument 1 expected MajorCustomer, got Customer"},
		},
		{
			name: "Casting a nullable type constraint to a non-nullable type constraint via assignment",
			input: `
type Customer struct { age Number }
define MajorCustomer constraints Customer with: fn(c: Customer) -> Boolean { return c.age > 18 }

let mNullable: MajorCustomer? = Customer { age: 20 }
let mNonNullable: MajorCustomer = mNullable
`,
			expectedErrors: []string{"type error: cannot assign MajorCustomer? to MajorCustomer"},
		},
	}
	runTestScenarios(t, tests)
}

func TestSafePipeAnalyzer(t *testing.T) {
	tests := []testScenario{
		{
			name: "Valid safe pipe into constraint",
			input: `
type Customer struct { age Number }
define MajorCustomer constraints Customer with: fn(c: Customer) -> Boolean { return c.age > 18 }

let processMajor = fn(m: MajorCustomer) -> Boolean { return true }
let m: MajorCustomer? = Customer { age: 20 }
m ?> processMajor
`,
			expectedErrors: []string{},
		},
		{
			name: "Unnecessary safe pipe",
			input: `
let m: Number = 5
let double = fn(x: Number) -> Number { return x * 2 }
m ?> double
`,
			expectedErrors: []string{"semantic error: unnecessary safe pipe on non-nullable type"},
		},
		{
			name: "Chained safe pipes (Valid)",
			input: `
type Customer struct { age Number }
define MajorCustomer constraints Customer with: fn(c: Customer) -> Boolean { return c.age > 18 }

let processMajor = fn(m: MajorCustomer) -> MajorCustomer { return m }
let printAge = fn(m: MajorCustomer) -> Number { return m.age }

let m: MajorCustomer? = Customer { age: 20 }
m ?> processMajor ?> printAge
`,
			expectedErrors: []string{},
		},
		{
			name: "Chained safe pipe followed by standard pipe (Invalid)",
			input: `
type Customer struct { age Number }
define MajorCustomer constraints Customer with: fn(c: Customer) -> Boolean { return c.age > 18 }

let processMajor = fn(m: MajorCustomer) -> MajorCustomer { return m }
let printAge = fn(m: MajorCustomer) -> Number { return m.age }

let m: MajorCustomer? = Customer { age: 20 }
m ?> processMajor |> printAge
`,
			expectedErrors: []string{"type error: argument 1 expected MajorCustomer, got MajorCustomer?"},
		},
	}
	runTestScenarios(t, tests)
}

func TestStreamPipeAnalyzer(t *testing.T) {
	tests := []testScenario{
		{
			name: "Valid stream pipe chaining",
			input: `
type Sale struct { amount Number }

let calcDiscount = fn(s: Sale, pct: Number) -> Sale { return s }
let calcProfit = fn(s: Sale) -> Number { return s.amount }

let sales = [Sale { amount: 100 }]
let result = sales |>> calcDiscount(5) |>> calcProfit
`,
			expectedErrors: []string{},
		},
		{
			name: "First stage parameter type mismatch",
			input: `
type Sale struct { amount Number }

let calcProfit = fn(s: Sale) -> Number { return s.amount }

let numbers = [1, 2, 3]
let result = numbers |>> calcProfit
`,
			expectedErrors: []string{"type error: argument 1 expected Sale, got Number"},
		},
		{
			name: "Non-array stream pipe source",
			input: `
let double = fn(x: Number) -> Number { return x * 2 }

let n: Number = 5
let result = n |>> double
`,
			expectedErrors: []string{"semantic error: stream pipe source must be an array, got Number"},
		},
		{
			name: "Safe stream pipe skip-and-drop over nullable elements",
			input: `
type Sale struct { amount Number }
define BigSale constraints Sale with: fn(s: Sale) -> Boolean { return s.amount > 150 }

let calcProfit = fn(s: BigSale) -> Number { return s.amount }

let s1: BigSale? = nil
let s2: BigSale? = Sale { amount: 200 }
let sales = [s1, s2]
let result = sales ?>> calcProfit
`,
			expectedErrors: []string{},
		},
		{
			name: "Unnecessary safe stream pipe on non-nullable element type",
			input: `
let double = fn(x: Number) -> Number { return x * 2 }

let numbers = [1, 2, 3]
let result = numbers ?>> double
`,
			expectedErrors: []string{"semantic error: unnecessary safe stream pipe on non-nullable element type"},
		},
		{
			name: "Boundary mixing: regular pipe feeding into a stream pipe",
			input: `
let isPositive = fn(x: Number) -> Boolean { return x > 0 }
let filter = fn(arr: [Number], f: fn(Number) -> Boolean) -> [Number] { return arr }
let double = fn(x: Number) -> Number { return x * 2 }

let numbers = [1, 2, 3]
let result = numbers |> filter(isPositive) |>> double
`,
			expectedErrors: []string{},
		},
	}
	runTestScenarios(t, tests)
}

func TestAsyncAwaitAnalyzer(t *testing.T) {
	tests := []testScenario{
		{
			// Deliberately no `await p` barrier before `unwrap p` here: unwrap
			// is safe to call directly on its own, without any preceding
			// await. It carries its own synchronization — see
			// transpileUnwrapExpression, which compiles it to `<-p.done`
			// before ever reading `p.val` — so there is no race condition to
			// guard against. `await` is an independent, optional barrier for
			// a different purpose (e.g. waiting on several tasks before
			// deciding to extract any of their values).
			name: "Valid async/unwrap round trip (no await barrier needed for correctness)",
			input: `
let compute = fn() -> Number { return 5 }

let p = async compute()
let r = unwrap p
`,
			expectedErrors: []string{},
		},
		{
			name: "Valid await barrier followed by individual unwraps",
			input: `
let compute = fn() -> Number { return 5 }

let p1 = async compute()
let p2 = async compute()
let p3 = async compute()
await p1 & p2 & p3
let r1 = unwrap p1
let r2 = unwrap p2
let r3 = unwrap p3
`,
			expectedErrors: []string{},
		},
		{
			// browser.fetch is an ordinary builtin (no dedicated async
			// support was added for it — see its comment in
			// symbol.GetStandardModule), so wrapping it in `async` and
			// pulling the result out via `unwrap` must work exactly like it
			// does for any user-defined function, with the unwrapped type
			// coming out as the fetch's declared String return type.
			name: "browser.fetch composes with async/unwrap like any other call",
			input: `
import browser
let t = async browser.fetch("/data")
let body: String = unwrap t
`,
			expectedErrors: []string{},
		},
		{
			name: "Valid single-operand await barrier",
			input: `
let compute = fn() -> Number { return 5 }

let p = async compute()
await p
let r = unwrap p
`,
			expectedErrors: []string{},
		},
		{
			name: "async/await/unwrap work with const statements too, not just let",
			input: `
let compute = fn() -> Number { return 5 }

const p = async compute()
await p
const r = unwrap p
`,
			expectedErrors: []string{},
		},
		{
			name: "unwrap on a non-async const value is a semantic error",
			input: `
const n = 5
const r = unwrap n
`,
			expectedErrors: []string{"semantic error: 'unwrap' requires an async value, got Number"},
		},
		{
			name: "unwrap on a non-async value is a semantic error",
			input: `
let n = 5
let r = unwrap n
`,
			expectedErrors: []string{"semantic error: 'unwrap' requires an async value, got Number"},
		},
		{
			name: "await operand that is not async is a semantic error",
			input: `
let compute = fn() -> Number { return 5 }
let p1 = async compute()
let n = 5
await p1 & n
`,
			expectedErrors: []string{"semantic error: 'await' operand is not an async value, got Number"},
		},
		{
			name: "unwrap nested inside a larger expression is a semantic error",
			input: `
let compute = fn() -> Number { return 5 }
let p = async compute()
let x = 1 + unwrap p
`,
			expectedErrors: []string{"semantic error: 'unwrap' can only be used as the value of a let/const statement or as a bare statement"},
		},
		// The following cases guard against a real bug this design once had:
		// AsyncSymbol.Type() used to forward to its underlying symbol's type
		// (mirroring NullableSymbol), which made every Type()-based check in
		// the analyzer (arithmetic, comparisons, array-literal homogeneity,
		// explicit let/const types, etc.) blind to the fact that a value was
		// still an un-unwrapped async handle rather than its eventual value.
		// That let code like `let p = async computeNum(); let x = p + 1`
		// type-check as valid Number arithmetic, when the runtime/transpiled
		// value is actually a task handle — silently bypassing the
		// synchronization unwrap/await exist to enforce. Fixed by giving
		// AsyncSymbol its own distinct Type() (environment.ASYNC_OBJ); these
		// tests make sure it can't regress.
		{
			name: "using an async value directly in arithmetic (without unwrap) is a semantic error",
			input: `
let compute = fn() -> Number { return 5 }
let p = async compute()
let x = p + 1
`,
			expectedErrors: []string{"type error: cannot add Async and Number"},
		},
		{
			name: "using an async value directly in a comparison (without unwrap) is a semantic error",
			input: `
let compute = fn() -> Number { return 5 }
let p = async compute()
let x = p > 0
`,
			expectedErrors: []string{"type error: operator '>' requires two Numbers, got Async and Number"},
		},
		{
			name: "assigning an async value to an explicitly non-async let type is a semantic error",
			input: `
let compute = fn() -> Number { return 5 }
let p: Number = async compute()
`,
			expectedErrors: []string{"type error: cannot assign async Number to Number"},
		},
		{
			name: "mixing an async value with a plain value in an array literal is a semantic error",
			input: `
let compute = fn() -> Number { return 5 }
let p = async compute()
let arr = [p, 2]
`,
			expectedErrors: []string{"type error: array elements must have the same type, expected Async, got Number"},
		},
		{
			name: "using an async value directly with a unary operator (without unwrap) is a semantic error",
			input: `
let compute = fn() -> Number { return 5 }
let p = async compute()
let x = -p
`,
			expectedErrors: []string{"type error: operator '-' requires a Number, got Async"},
		},
	}
	runTestScenarios(t, tests)
}

func TestJoinGroupAnalyzer(t *testing.T) {
	tests := []testScenario{
		{
			name: "Valid parallel join consumed by the next stage",
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
			expectedErrors: []string{},
		},
		{
			name: "Join result type mismatch against the consuming stage",
			input: `
type Loan struct { principal Number }

let fetchIndexRate = fn(loan: Loan) -> Number { return 5 }
let fetchFees = fn(loan: Loan) -> Number { return 1 }
let calculatePnl = fn(rate: String, fees: Number) -> Number { return fees }

let loans = [Loan { principal: 100 }]
let pnl = loans |>> (fetchIndexRate & fetchFees) |>> calculatePnl
`,
			expectedErrors: []string{"type error: argument 1 expected String, got Number"},
		},
		{
			name: "Dangling join group with no consuming stage",
			input: `
type Loan struct { principal Number }

let fetchIndexRate = fn(loan: Loan) -> Number { return 5 }
let fetchFees = fn(loan: Loan) -> Number { return 1 }

let loans = [Loan { principal: 100 }]
let pnl = loans |>> (fetchIndexRate & fetchFees)
`,
			expectedErrors: []string{"semantic error: a parallel join group (f & g & h) must be followed by another stage that consumes its results"},
		},
		{
			name: "Join group used outside of a stream pipe context",
			input: `
let f = fn(x: Number) -> Number { return x }
let g = fn(x: Number) -> Number { return x }
let x = (f & g)
`,
			expectedErrors: []string{"semantic error: a parallel join group (f & g & h) can only be used as a |>>/?>> stage"},
		},
		{
			name: "Join member curried arguments are type-checked",
			input: `
type Loan struct { principal Number }

let fetchIndexRate = fn(loan: Loan) -> Number { return 5 }
let fetchFees = fn(loan: Loan, schedule: String) -> Number { return 1 }
let calculatePnl = fn(rate: Number, fees: Number) -> Number { return rate + fees }

let loans = [Loan { principal: 100 }]
let pnl = loans |>> (fetchIndexRate & fetchFees(5)) |>> calculatePnl
`,
			expectedErrors: []string{"type error: argument 2 expected String, got Number"},
		},
	}
	runTestScenarios(t, tests)
}

func TestSemanticAnalysisNamedImports(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedError string
	}{
		{
			name: "Valid named import from math",
			input: `import { max } from "math"
let m = max(1, 2)
`,
			expectedError: "",
		},
		{
			name: "Named import and alias both work",
			input: `import { max } from "math" as m
let a = max(1, 2)
let b = m.min(1, 2)
`,
			expectedError: "",
		},
		{
			name: "Import conflict with local variable",
			input: `import { max } from "math"
let max = 10
`,
			expectedError: "import conflict: variable 'max' is already declared. Suggestion: create an alias for the module",
		},
		{
			name: "Import conflict with already existing local variable",
			input: `let max = 10
import { max } from "math"
`,
			expectedError: "semantic error: import statements must appear at the beginning of the file",
		},
		{
			name: "Import conflict with another named import",
			input: `import { max } from "math"
import { max } from "math"
`,
			expectedError: "import conflict: variable 'max' is already declared. Suggestion: create an alias for the module",
		},
		{
			name: "Named import missing from module",
			input: `import { notExists } from "math"
`,
			expectedError: "semantic error: module 'math' has no exported member 'notExists'",
		},
		{
			name: "Valid named type import from http",
			input: `import { Client } from "http"
let useClient = fn(c: Client) -> Client { return c }
`,
			expectedError: "",
		},
		{
			name: "Named type import missing from module",
			input: `import { NotAType } from "http"
`,
			expectedError: "semantic error: module 'http' has no exported member 'NotAType'",
		},
		{
			name: "Mixed value and type named imports from the same module",
			input: `import { newClient, Client } from "http"
let c: Client = newClient("http://example.com", {})
`,
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := environment.NewEnvironment("", "", false)
			a := New(env)
			
			// Setup a mock math module
			lexerInstance := lexer.New(tt.input)
			p := parser.New(lexerInstance)
			program := p.Parse()

			if len(p.Errors()) > 0 {
				t.Fatalf("parser errors: %v", p.Errors())
			}

			a.Run(program)
			
			if tt.expectedError != "" {
				if len(a.DiagnosticErrors()) == 0 {
					t.Fatalf("expected error '%s', got none", tt.expectedError)
				}
				if a.DiagnosticErrors()[0].Message != tt.expectedError {
					t.Errorf("expected error '%s', got '%s'", tt.expectedError, a.DiagnosticErrors()[0].Message)
				}
			} else if len(a.DiagnosticErrors()) > 0 {
				t.Fatalf("expected no errors, got: %v", a.DiagnosticErrors())
			}
		})
	}
}

// TestSemanticAnalysisWildcardImports verifies `import * from mod`: every
// exported member becomes usable bare, explicit declarations and named imports
// silently win over a wildcard in both orders, and a name offered by two
// wildcards is an error only where it is actually used — never at import time,
// with the qualified form always available as the fallback.
//
// The builtin 'array' and 'string' modules both export 'len' and 'join', which
// gives fixture-free collision cases.
func TestSemanticAnalysisWildcardImports(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedError string
	}{
		{
			name: "Wildcard members are usable bare",
			input: `import * from "array"
let n = len([1, 2])
let xs = push([1, 2], 3)
`,
		},
		{
			name: "Unquoted module specifier",
			input: `import * from array
let n = len([1, 2])
`,
		},
		{
			name: "Colliding name is fine while it is never used",
			input: `import * from "array"
import * from "string"
let xs = push([1, 2], 3)
let s = toUpper("hi")
`,
		},
		{
			name: "Colliding name errors only where it is used",
			input: `import * from "array"
import * from "string"
let n = len([1, 2])
`,
			expectedError: "semantic error: ambiguous reference to 'len': wildcard-imported from 'array' and 'string'. Suggestion: qualify it (array.len or string.len)",
		},
		{
			name: "Qualified form resolves an ambiguity",
			input: `import * from "array"
import * from "string"
let n = array.len([1, 2])
`,
		},
		{
			name: "Explicit named import before a wildcard wins silently",
			input: `import { len } from "string"
import * from "array"
let n = len("ab")
`,
		},
		{
			name: "Explicit named import after a wildcard wins silently",
			input: `import * from "array"
import { len } from "string"
let n = len("ab")
`,
		},
		{
			name: "Local declaration shadows a wildcard import silently",
			input: `import * from "array"
let push = 10
let n = push
`,
		},
		{
			name: "Local declaration inside a function shadows an ambiguous wildcard",
			input: `import * from "array"
import * from "string"
let f = fn() -> Number {
	let len = 5
	return len
}
`,
		},
		{
			name: "Wildcard combined with an alias keeps the qualified form",
			input: `import * from "array" as a
let xs = push([1, 2], 3)
let n = a.len([1, 2])
`,
		},
		{
			name: "Module alias wins over a member of the same name",
			input: `import * from "array" as len
let n = len.len([1, 2])
`,
		},
		{
			name: "Wildcard imports type-level exports",
			input: `import * from "http"
let handle = fn(req: Request) -> Response {
	return ok("hi")
}
`,
		},
		{
			name:          "Wildcard from an unknown module still reports the import failure",
			input:         `import * from "does_not_exist"`,
			expectedError: "semantic error: failed to import 'does_not_exist': module 'does_not_exist' not found in local paths or node_modules",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := environment.NewEnvironment("", "", false)
			a := New(env)

			lexerInstance := lexer.New(tt.input)
			p := parser.New(lexerInstance)
			program := p.Parse()

			if len(p.Errors()) > 0 {
				t.Fatalf("parser errors: %v", p.Errors())
			}

			a.Run(program)

			if tt.expectedError != "" {
				if len(a.DiagnosticErrors()) == 0 {
					t.Fatalf("expected error '%s', got none", tt.expectedError)
				}
				if a.DiagnosticErrors()[0].Message != tt.expectedError {
					t.Errorf("expected error '%s', got '%s'", tt.expectedError, a.DiagnosticErrors()[0].Message)
				}
			} else if len(a.DiagnosticErrors()) > 0 {
				t.Fatalf("expected no errors, got: %v", a.DiagnosticErrors())
			}
		})
	}
}

// TestSemanticAnalysisMemoModifier verifies the semantic rules enforced on a
// 'memo'-modified function literal: no generics, must return a value, no
// function-typed parameters, and 'memo' must be the direct value of a
// let/const binding. Array/map/struct/nullable-struct parameters are
// accepted, since the transpiler keys those via a content hash.
func TestSemanticAnalysisMemoModifier(t *testing.T) {
	tests := []testScenario{
		{
			name:  "Valid memo on a single Number parameter",
			input: `let powerTwo = memo fn(n: Number) -> Number { return n * n }`,
		},
		{
			name:  "Valid memo on an array parameter",
			input: `let sum = memo fn(list: [Number]) -> Number { return 0 }`,
		},
		{
			name: "Valid memo on a nullable struct parameter",
			input: `
type Node struct { val Number }
let f = memo fn(n: Node?) -> Number { return 0 }
`,
		},
		{
			name:  "Valid memo on a const binding",
			input: `const powerTwo = memo fn(n: Number) -> Number { return n * n }`,
		},
		{
			name: "Reject memo const not directly bound via let/const",
			input: `
let apply = fn(f: fn(Number) -> Number, x: Number) -> Number { return f(x) }
const result = apply(memo fn(n: Number) -> Number { return n }, 5)
`,
			expectedErrors: []string{
				"semantic error: 'memo' is currently only supported directly on a let/const binding, e.g. let name = memo fn(...) {...}",
			},
		},
		{
			name:  "Reject function-typed parameter",
			input: `let f = memo fn(cb: fn(Number) -> Number) -> Number { return cb(1) }`,
			expectedErrors: []string{
				"semantic error: memoized function parameter 'cb' has type",
			},
		},
		{
			name:  "Reject missing return value",
			input: `let f = memo fn(n: Number) { }`,
			expectedErrors: []string{
				"semantic error: memoized function must have a return type",
			},
		},
		{
			name:  "Reject generic memoized function",
			input: `let f = memo fn<T>(x: T) -> T { return x }`,
			expectedErrors: []string{
				"semantic error: 'memo' does not support generic functions",
			},
		},
		{
			name: "Reject memo not directly bound via let/const",
			input: `
let apply = fn(f: fn(Number) -> Number, x: Number) -> Number { return f(x) }
let result = apply(memo fn(n: Number) -> Number { return n }, 5)
`,
			expectedErrors: []string{
				"semantic error: 'memo' is currently only supported directly on a let/const binding, e.g. let name = memo fn(...) {...}",
			},
		},
	}
	runTestScenarios(t, tests)
}

func TestSemanticAnalysisActiveModifier(t *testing.T) {
	tests := []testScenario{
		{
			name: "Valid active let with reassignment",
			input: `
let active counter = 0
counter = counter + 1
return counter
`,
		},
		{
			name: "Active variable reads transparently as its underlying type",
			input: `
let active counter = 0
let doubled = counter * 2
return doubled
`,
		},
		{
			name: "private + active compose in either mention order",
			input: `
private let active n = "name"
n = "renamed"
return n
`,
		},
		{
			name: "active on a String works the same as Number",
			input: `
let active label = "hello"
label = "world"
return label
`,
		},
	}
	runTestScenarios(t, tests)
}

func TestSemanticAnalysisReactiveCall(t *testing.T) {
	tests := []testScenario{
		{
			name: "Valid reactive call: active result required and provided",
			input: `
import cast
let active counter = 0
let reactFn = fn(c: Number) -> String { return cast.to(c, "") }
let active result = reactFn(react counter)
return result
`,
		},
		{
			name: "Valid reactive call with multiple react params",
			input: `
import cast
let active a = 0
let active b = 1
let combine = fn(x: Number, y: Number) -> Number { return x + y }
let active total = combine(react a, react b)
return total
`,
		},
		{
			name: "Reject reactive call result not declared active",
			input: `
import cast
let active counter = 0
let reactFn = fn(c: Number) -> String { return cast.to(c, "") }
let result = reactFn(react counter)
return result
`,
			expectedErrors: []string{
				"semantic error: a variable bound to a reactive call result must be declared 'active' (e.g. 'let active result = ...')",
			},
		},
		{
			name: "Reject react on a non-active variable",
			input: `
import cast
let notActive = 0
let reactFn = fn(c: Number) -> String { return cast.to(c, "") }
let active result = reactFn(react notActive)
return result
`,
			expectedErrors: []string{
				"semantic error: 'react' requires an active variable, got 'notActive' of type Number",
			},
		},
		{
			name: "Reject react on an undeclared variable",
			input: `
let reactFn = fn(c: Number) -> Number { return c }
let active result = reactFn(react missing)
return result
`,
			expectedErrors: []string{
				"semantic error: undeclared variable 'missing'. Use 'let' to declare it.",
			},
		},
		{
			name: "Reject move + react on the same argument (move first)",
			input: `
import cast
let active counter = 0
let reactFn = fn(c: Number) -> String { return cast.to(c, "") }
let active result = reactFn(move react counter)
return result
`,
			expectedErrors: []string{
				"semantic error: 'move' and 'react' cannot be combined on the same argument — react requires re-reading the active variable on future updates, but move consumes it once",
			},
		},
		{
			name: "Reject react + move on the same argument (react first)",
			input: `
import cast
let active counter = 0
let reactFn = fn(c: Number) -> String { return cast.to(c, "") }
let active result = reactFn(react move counter)
return result
`,
			expectedErrors: []string{
				"semantic error: 'move' and 'react' cannot be combined on the same argument — react requires re-reading the active variable on future updates, but move consumes it once",
			},
		},
		{
			name: "Reactive call composes with a memoized callee",
			input: `
import cast
let reactFn = memo fn(c: Number) -> String { return cast.to(c, "") }
let active counter = 0
let active result = reactFn(react counter)
return result
`,
		},
	}
	runTestScenarios(t, tests)
}

// TestSemanticAnalysisNamedArguments verifies named-parameter call resolution:
// positional arguments fill parameters left-to-right first, named arguments
// fill the rest by name and are order-independent among themselves, and
// every mismatch (unknown name, duplicate, supplied both ways, missing
// parameter, too many positional arguments, named args on a builtin) is
// rejected with a clear error.
func TestSemanticAnalysisNamedArguments(t *testing.T) {
	tests := []testScenario{
		{
			name: "Valid call with only named arguments",
			input: `
let createUser = fn(name: String, age: Number) -> String { return name }
return createUser(name: "Ana", age: 30)
`,
		},
		{
			name: "Valid call with named arguments in a different order",
			input: `
let createUser = fn(name: String, age: Number) -> String { return name }
return createUser(age: 30, name: "Ana")
`,
		},
		{
			name: "Valid call mixing positional and named arguments",
			input: `
let createUser = fn(name: String, age: Number) -> String { return name }
return createUser("Ana", age: 30)
`,
		},
		{
			name: "Unknown named argument is rejected",
			input: `
let createUser = fn(name: String, age: Number) -> String { return name }
createUser(name: "Ana", age: 30, nickname: "Foo")
`,
			expectedErrors: []string{
				"type error: function 'createUser' has no parameter 'nickname'",
			},
		},
		{
			name: "Duplicate named argument is rejected",
			input: `
let createUser = fn(name: String, age: Number) -> String { return name }
createUser(name: "Ana", name: "Bob", age: 30)
`,
			expectedErrors: []string{
				"type error: duplicate named argument 'name'",
			},
		},
		{
			name: "Parameter supplied both positionally and by name is rejected",
			input: `
let createUser = fn(name: String, age: Number) -> String { return name }
createUser("Ana", name: "Bob", age: 30)
`,
			expectedErrors: []string{
				"type error: parameter 'name' supplied both positionally and by name",
			},
		},
		{
			name: "Missing required parameter after named resolution is rejected",
			input: `
let createUser = fn(name: String, age: Number) -> String { return name }
createUser(name: "Ana")
`,
			expectedErrors: []string{
				"arity error: missing argument for parameter 'age'",
			},
		},
		{
			name: "Too many positional arguments alongside a named argument is rejected",
			input: `
let createUser = fn(name: String, age: Number) -> String { return name }
createUser("Ana", 30, "extra", age: 40)
`,
			expectedErrors: []string{
				"arity error: too many positional arguments: expected at most 2, got 3",
				"type error: parameter 'age' supplied both positionally and by name",
			},
		},
		{
			name: "Named arguments are rejected on a builtin module function",
			input: `
import map
let m: map[String]Number = {}
map.containsKey(map: m, key: "x")
`,
			expectedErrors: []string{
				"type error: named arguments are not supported for builtin module function 'map.containsKey'",
			},
		},
		{
			name: "Named argument on a generic function still infers correctly",
			input: `
let identity = fn<T>(value: T) -> T { return value }
let n: Number = identity(value: 5)
return n
`,
		},
	}
	runTestScenarios(t, tests)
}
