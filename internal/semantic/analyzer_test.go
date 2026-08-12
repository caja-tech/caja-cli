package semantic

import (
	"caja-cli/internal/environment"
	"caja-cli/internal/lexer"
	"caja-cli/internal/syntax"
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
			parser := syntax.New(tknzr)
			program := parser.Parse()

			if parser.HasErrors() {
				t.Fatalf("parser errors occurred during setup: %v", parser.Errors())
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
				"type error: index assignment not supported for NUMBER",
			},
		},
		{
			name: "Array index assignment with non-number index",
			input: `
let a = [1, 2, 3]
a["hello"] = 5
`,
			expectedErrors: []string{
				"type error: array index must be NUMBER, got STRING",
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
let add = fn(a: Number, b: Number): Number { return a + b }
let result = add(10, 20)
`,
			expectedErrors: []string{},
		},
		{
			name: "Call non-function",
			input: `
let a = 10
a()
`,
			expectedErrors: []string{
				"type error: cannot call a non-function (got NUMBER)",
			},
		},
		{
			name: "Incorrect arity",
			input: `
let add = fn(a: Number, b: Number): Number { return a + b }
add(10)
`,
			expectedErrors: []string{
				"arity error: expected 2 arguments, got 1",
			},
		},
		{
			name: "Incorrect argument type",
			input: `
let check = fn(name: String): Boolean { return true }
check(10)
`,
			expectedErrors: []string{
				"type error: argument 1 expected STRING, got NUMBER",
			},
		},
		{
			name: "Incorrect return type",
			input: `
let add = fn(a: Number, b: Number): String { return a + b }
`,
			expectedErrors: []string{
				"type error: function declared to return STRING, but body returns NUMBER",
			},
		},
		{
			name: "Missing return statement",
			input: `
let bad = fn(): Number {
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
let check = fn(a: Number): Boolean {
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
let check = fn(a: Number): Boolean {
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
let factorial = fn(n: Number): Number {
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
let loop = fn(n: Number): Number {
	if (n == 0) {
		return 0
	} else {
		return loop("string instead of number")
	}
}
`,
			expectedErrors: []string{
				"type error: argument 1 expected NUMBER, got STRING",
			},
		},
		{
			name: "Unconditional recursion (infinite loop)",
			input: `
let loop = fn(): Number {
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
let alwaysLoops = fn(n: Number): Number {
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
let count = fn(n: Number): Number {
	return 1 + count(n + 1)
}
`,
			expectedErrors: []string{
				"semantic error: function 'count' contains unconditional recursion and will infinitely loop",
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
				"type error: cannot assign NUMBER to variable 'd' of type DATE",
			},
		},
		{
			name: "Function returning Date",
			input: `
let getDate = fn(): Date { return '2023-10-25' }
let d = getDate()
d = '2023-12-01'
`,
			expectedErrors: []string{},
		},
		{
			name: "Function returning wrong type instead of Date",
			input: `
let getDate = fn(): Date { return 10 }
`,
			expectedErrors: []string{
				"type error: function declared to return DATE, but body returns NUMBER",
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
				"type error: condition must be a BOOLEAN, got NUMBER",
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
				"type error: array elements must have the same type, expected NUMBER, got STRING",
			},
		},
		{
			name: "Invalid index type",
			input: `
let a = [1, 2, 3]
let b = a["zero"]
`,
			expectedErrors: []string{
				"type error: array index expected NUMBER, got STRING",
			},
		},
		{
			name: "Index operator on non-array",
			input: `
let a = 10
let b = a[0]
`,
			expectedErrors: []string{
				"type error: index operator not supported for NUMBER",
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
let sum = fn(arr: [Number]): Number { return arr[0] }
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

func TestSemanticAnalysisAnyType(t *testing.T) {
	tests := []testScenario{
		{
			name:  "Function accepting Any parameter",
			input: "let f = fn(x: Any): Any { return x }\nf(10)\nf(\"hello\")\nf(true)",
		},
		{
			name:  "Function returning Any",
			input: "let a = fn(): Any { return 10 }\nlet b = fn(): Any { return \"string\" }",
		},
		{
			name:  "Array of Any",
			input: "let getAny = fn(x: Any): Any { return x }\nlet f = fn(arr: [Any]): Any { return arr[0] }\nf([getAny(1), getAny(\"string\"), getAny(true)])",
		},
		{
			name:  "Assigning Any to concrete type is allowed statically",
			input: "let f = fn(): Any { return 10 }\nlet n = f()\nlet x = 1\nx = n",
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
			expectedErrors: []string{"type error: operator '!' requires a BOOLEAN, got NUMBER"},
		},
		{
			name:  "Valid Minus",
			input: "return -5",
		},
		{
			name:           "Invalid Minus",
			input:          "return -true",
			expectedErrors: []string{"type error: operator '-' requires a NUMBER, got BOOLEAN"},
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
			name:  "append() works with matching types",
			input: "import array\nlet arr = [1, 2]\nlet newArr = array.append(arr, 3)",
		},
		{
			name:           "append() rejects mismatched types",
			input:          "import array\nlet arr = [1, 2]\nlet newArr = array.append(arr, \"string\")",
			expectedErrors: []string{"type error: cannot append STRING to array of NUMBER"},
		},
		{
			name:  "head() works and infers type",
			input: "import array\nlet arr = [1, 2]\nlet h = array.head(arr)\nlet n = h",
		},
		{
			name:  "tail() returns array",
			input: "import array\nlet arr = [1, 2]\nlet t = array.tail(arr)\nlet res = array.append(t, 3)",
		},
		{
			name:  "last() works and infers type",
			input: "import array\nlet arr = [1, 2]\nlet l = array.last(arr)\nlet n = l",
		},
		{
			name:  "copy() returns array",
			input: "import array\nlet arr = [1, 2]\nlet c = array.copy(arr)\nlet res = array.append(c, 3)",
		},
		{
			name:  "slice() works",
			input: "import array\nlet arr = [1, 2, 3]\nlet s = array.slice(arr, 0, 2)\nlet res = array.append(s, 4)",
		},
		{
			name:           "slice() rejects non-number index",
			input:          "import array\nlet arr = [1, 2]\nlet s = array.slice(arr, \"0\", 2)",
			expectedErrors: []string{"type error: second argument to 'slice' must be NUMBER, got STRING"},
		},
		{
			name:  "join() works with matching types",
			input: "import array\nlet arrOne = [1, 2]\nlet arrTwo = [3, 4]\nlet res = array.join(arrOne, arrTwo)",
		},
		{
			name:           "join() rejects mismatched types",
			input:          "import array\nlet arrOne = [1, 2]\nlet arrTwo = [\"a\", \"b\"]\nlet res = array.join(arrOne, arrTwo)",
			expectedErrors: []string{"type error: cannot join array of NUMBER with array of STRING"},
		},
		{
			name:  "charAt() works with correct types",
			input: "import string\nreturn string.charAt(\"hello\", 1)",
		},
		{
			name:           "charAt() rejects mismatched types",
			input:          "import string\nreturn string.charAt(10, \"hello\")",
			expectedErrors: []string{"type error: first argument to 'charAt' must be STRING, got NUMBER", "type error: second argument to 'charAt' must be NUMBER, got STRING"},
		},
		{
			name:  "substring() works with correct types",
			input: "import string\nreturn string.substring(\"hello\", 1, 4)",
		},
		{
			name:           "substring() rejects mismatched types",
			input:          "import string\nreturn string.substring(10, \"start\", \"end\")",
			expectedErrors: []string{"type error: first argument to 'substring' must be STRING, got NUMBER", "type error: second argument to 'substring' must be NUMBER, got STRING", "type error: third argument to 'substring' must be NUMBER, got STRING"},
		},
		{
			name:  "concat() works with correct types",
			input: "import string\nreturn string.concat(\"hello\", \" world\")",
		},
		{
			name:           "concat() rejects mismatched types",
			input:          "import string\nreturn string.concat(10, 20)",
			expectedErrors: []string{"type error: first argument to 'concat' must be STRING, got NUMBER", "type error: second argument to 'concat' must be STRING, got NUMBER"},
		},
		{
			name:  "split() works with correct types",
			input: "import string\nreturn string.split(\"hello\", \"e\")",
		},
		{
			name:           "split() rejects mismatched types",
			input:          "import string\nreturn string.split(10, 20)",
			expectedErrors: []string{"type error: first argument to 'split' must be STRING, got NUMBER", "type error: second argument to 'split' must be STRING, got NUMBER"},
		},
		{
			name:  "contains() works with correct types",
			input: "import string\nreturn string.contains(\"hello\", \"e\")",
		},
		{
			name:           "contains() rejects mismatched types",
			input:          "import string\nreturn string.contains(10, 20)",
			expectedErrors: []string{"type error: first argument to 'contains' must be STRING, got NUMBER", "type error: second argument to 'contains' must be STRING, got NUMBER"},
		},
		{
			name:  "startsWith() works with correct types",
			input: "import string\nreturn string.startsWith(\"hello\", \"h\")",
		},
		{
			name:           "startsWith() rejects mismatched types",
			input:          "import string\nreturn string.startsWith(10, 20)",
			expectedErrors: []string{"type error: first argument to 'startsWith' must be STRING, got NUMBER", "type error: second argument to 'startsWith' must be STRING, got NUMBER"},
		},
		{
			name:  "endsWith() works with correct types",
			input: "import string\nreturn string.endsWith(\"hello\", \"o\")",
		},
		{
			name:           "endsWith() rejects mismatched types",
			input:          "import string\nreturn string.endsWith(10, 20)",
			expectedErrors: []string{"type error: first argument to 'endsWith' must be STRING, got NUMBER", "type error: second argument to 'endsWith' must be STRING, got NUMBER"},
		},
		{
			name:  "replace() works with correct types",
			input: "import string\nreturn string.replace(\"hello\", \"e\", \"a\")",
		},
		{
			name:           "replace() rejects mismatched types",
			input:          "import string\nreturn string.replace(10, 20, 30)",
			expectedErrors: []string{"type error: first argument to 'replace' must be STRING, got NUMBER", "type error: second argument to 'replace' must be STRING, got NUMBER", "type error: third argument to 'replace' must be STRING, got NUMBER"},
		},
		{
			name:  "toUpper() works with correct types",
			input: "import string\nreturn string.toUpper(\"hello\")",
		},
		{
			name:           "toUpper() rejects mismatched types",
			input:          "import string\nreturn string.toUpper(10)",
			expectedErrors: []string{"type error: first argument to 'toUpper' must be STRING, got NUMBER"},
		},
		{
			name:  "toLower() works with correct types",
			input: "import string\nreturn string.toLower(\"HELLO\")",
		},
		{
			name:           "toLower() rejects mismatched types",
			input:          "import string\nreturn string.toLower(10)",
			expectedErrors: []string{"type error: first argument to 'toLower' must be STRING, got NUMBER"},
		},
		{
			name:  "trim() works with correct types",
			input: "import string\nreturn string.trim(\"  hello  \")",
		},
		{
			name:           "trim() rejects mismatched types",
			input:          "import string\nreturn string.trim(10)",
			expectedErrors: []string{"type error: first argument to 'trim' must be STRING, got NUMBER"},
		},
		{
			name:  "string len() works with correct types",
			input: "import string\nreturn string.len(\"hello\")",
		},
		{
			name:           "strlen() rejects mismatched types",
			input:          "import string\nreturn string.len(10)",
			expectedErrors: []string{"type error: first argument to 'len' must be STRING, got NUMBER"},
		},
		{
			name:  "year() works with correct types",
			input: "import date\nlet d = '2023-10-25'\nreturn date.year(d)",
		},
		{
			name:           "year() rejects mismatched types",
			input:          "import date\nreturn date.year(\"2023\")",
			expectedErrors: []string{"type error: first argument to 'year' must be DATE, got STRING"},
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
			name:  "parseDate() works with correct types",
			input: "import date\nreturn date.parseDate(\"2023-10-25\")",
		},
		{
			name:           "parseDate() rejects mismatched types",
			input:          "import date\nreturn date.parseDate(2023)",
			expectedErrors: []string{"type error: first argument to 'parseDate' must be STRING, got NUMBER"},
		},
		{
			name:  "addDays() works with correct types",
			input: "import date\nreturn date.addDays('2023-10-25', 5)",
		},
		{
			name:           "addDays() rejects mismatched types",
			input:          "import date\nreturn date.addDays('2023-10-25', \"5\")",
			expectedErrors: []string{"type error: second argument to 'addDays' must be NUMBER, got STRING"},
		},
		{
			name:  "diffDays() works with correct types",
			input: "import date\nreturn date.diffDays('2023-10-30', '2023-10-25')",
		},
		{
			name:           "diffDays() rejects mismatched types",
			input:          "import date\nreturn date.diffDays('2023-10-30', 5)",
			expectedErrors: []string{"type error: second argument to 'diffDays' must be DATE, got NUMBER"},
		},
		{
			name:  "newDate() works with correct types",
			input: "import date\nreturn date.newDate(2023, 1, 1)",
		},
		{
			name:           "newDate() rejects mismatched types",
			input:          "import date\nreturn date.newDate(\"2023\", 1, 1)",
			expectedErrors: []string{"type error: first argument to 'newDate' must be NUMBER, got STRING"},
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
			expectedErrors: []string{"type error: first argument to 'abs' must be NUMBER, got STRING"},
		},
		{
			name:  "sqrt() works with correct types",
			input: "import math\nreturn math.sqrt(16)",
		},
		{
			name:           "sqrt() rejects mismatched types",
			input:          "import math\nreturn math.sqrt(\"hello\")",
			expectedErrors: []string{"type error: first argument to 'sqrt' must be NUMBER, got STRING"},
		},
		{
			name:  "floor() works with correct types",
			input: "import math\nreturn math.floor(4.9)",
		},
		{
			name:           "floor() rejects mismatched types",
			input:          "import math\nreturn math.floor(\"hello\")",
			expectedErrors: []string{"type error: first argument to 'floor' must be NUMBER, got STRING"},
		},
		{
			name:  "ceil() works with correct types",
			input: "import math\nreturn math.ceil(4.1)",
		},
		{
			name:           "ceil() rejects mismatched types",
			input:          "import math\nreturn math.ceil(\"hello\")",
			expectedErrors: []string{"type error: first argument to 'ceil' must be NUMBER, got STRING"},
		},
		{
			name:  "round() works with correct types",
			input: "import math\nreturn math.round(4.5)",
		},
		{
			name:           "round() rejects mismatched types",
			input:          "import math\nreturn math.round(\"hello\")",
			expectedErrors: []string{"type error: first argument to 'round' must be NUMBER, got STRING"},
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
			expectedErrors: []string{"type error: first argument to 'pow' must be NUMBER, got STRING"},
		},
		{
			name:           "pow() rejects mismatched second type",
			input:          "import math\nreturn math.pow(2, \"3\")",
			expectedErrors: []string{"type error: second argument to 'pow' must be NUMBER, got STRING"},
		},
		{
			name:  "min() works with correct types",
			input: "import math\nreturn math.min(2, 3)",
		},
		{
			name:           "min() rejects mismatched second type",
			input:          "import math\nreturn math.min(2, \"3\")",
			expectedErrors: []string{"type error: second argument to 'min' must be NUMBER, got STRING"},
		},
		{
			name:  "max() works with correct types",
			input: "import math\nreturn math.max(2, 3)",
		},
		{
			name:           "max() rejects mismatched first type",
			input:          "import math\nreturn math.max(\"2\", 3)",
			expectedErrors: []string{"type error: first argument to 'max' must be NUMBER, got STRING"},
		},
		{
			name:  "log() works with correct types",
			input: "import math\nreturn math.log(100, 10)",
		},
		{
			name:           "log() rejects mismatched second type",
			input:          "import math\nreturn math.log(100, \"10\")",
			expectedErrors: []string{"type error: second argument to 'log' must be NUMBER, got STRING"},
		},
		{
			name:  "log.info() works with correct types",
			input: "import log\nreturn log.info(\"message\", 42)",
		},
		{
			name:           "log.info() rejects mismatched first type",
			input:          "import log\nreturn log.info(42, 42)",
			expectedErrors: []string{"type error: first argument to 'info' must be STRING, got NUMBER"},
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
			expectedErrors: []string{"type error: operator 'and' requires two BOOLEANs, got NUMBER and BOOLEAN"},
		},
		{
			name:           "Logical OR with wrong right type",
			input:          "return false or \"string\"",
			expectedErrors: []string{"type error: operator 'or' requires two BOOLEANs, got BOOLEAN and STRING"},
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
private type MyFunc fn(): Number
`,
			expectedErrors: []string{},
		},
		{
			name: "Invalid private type inside block",
			input: `
if (true) {
	private type MyFunc fn(): Number
}
`,
			expectedErrors: []string{
				"semantic error: 'private' modifier is only allowed at the top-level of a module",
			},
		},
		{
			name: "Invalid private let inside function block",
			input: `
let a = fn(b: Number): Number {
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
			input: "type money Number\ntype moment Boolean\ntype name String\ntype custom Any\nlet process = fn(m: money, d: moment, n: name, c: custom): money { return m }\nprocess(100, true, \"John\", 42)",
		},
		{
			name:  "Type alias with array types",
			input: "type prices [Number]\ntype names [String]\ntype holidays [Boolean]\ntype collection [Any]\nlet addAll = fn(p: prices, n: names, h: holidays, c: collection): prices { return p }\naddAll([1, 2], [\"a\"], [true], [1, 2])",
		},
		{
			name:           "Type alias mismatch",
			input:          "type money Number\nlet add = fn(a: money): money { return a }\nadd(\"string\")",
			expectedErrors: []string{"type error: argument 1 expected NUMBER, got STRING"},
		},
		{
			name:           "Undefined type in alias",
			input:          "type someType Some\nlet doSomething = fn(a: someType): Number { return 1 }",
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
			name: "Undefined struct",
			input: `let p = Unknown { a: 1 }`,
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
			expectedErrors: []string{"type error: field 'age' expects NUMBER, got STRING"},
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
	}
	runTestScenarios(t, tests)
}
