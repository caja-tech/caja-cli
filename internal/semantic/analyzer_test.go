package semantic

import (
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

			if len(parser.Errors()) != 0 {
				t.Fatalf("parser errors occurred during setup: %v", parser.Errors())
			}

			analyzer := New()
			analyzer.Analyze(program)

			errors := analyzer.Errors()

			if len(errors) != len(tt.expectedErrors) {
				t.Fatalf("expected %d errors, got %d.", len(tt.expectedErrors), len(errors))
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
			input: "let arr = [1, 2, 3]\nlen(arr)",
		},
		{
			name:  "len() works on array of Strings",
			input: "let arr = [\"a\", \"b\"]\nlen(arr)",
		},
		{
			name:  "append() works with matching types",
			input: "let arr = [1, 2]\nlet newArr = append(arr, 3)",
		},
		{
			name:           "append() rejects mismatched types",
			input:          "let arr = [1, 2]\nlet newArr = append(arr, \"string\")",
			expectedErrors: []string{"type error: cannot append STRING to array of NUMBER"},
		},
		{
			name:  "head() works and infers type",
			input: "let arr = [1, 2]\nlet h = head(arr)\nlet n = h",
		},
		{
			name:  "tail() returns array",
			input: "let arr = [1, 2]\nlet t = tail(arr)\nlet res = append(t, 3)",
		},
		{
			name:  "last() works and infers type",
			input: "let arr = [1, 2]\nlet l = last(arr)\nlet n = l",
		},
		{
			name:  "copy() returns array",
			input: "let arr = [1, 2]\nlet c = copy(arr)\nlet res = append(c, 3)",
		},
		{
			name:  "slice() works",
			input: "let arr = [1, 2, 3]\nlet s = slice(arr, 0, 2)\nlet res = append(s, 4)",
		},
		{
			name:           "slice() rejects non-number index",
			input:          "let arr = [1, 2]\nlet s = slice(arr, \"0\", 2)",
			expectedErrors: []string{"type error: second argument to 'slice' must be NUMBER, got STRING"},
		},
		{
			name:  "join() works with matching types",
			input: "let arrOne = [1, 2]\nlet arrTwo = [3, 4]\nlet res = join(arrOne, arrTwo)",
		},
		{
			name:           "join() rejects mismatched types",
			input:          "let arrOne = [1, 2]\nlet arrTwo = [\"a\", \"b\"]\nlet res = join(arrOne, arrTwo)",
			expectedErrors: []string{"type error: cannot join array of NUMBER with array of STRING"},
		},
		{
			name:  "charAt() works with correct types",
			input: "return charAt(\"hello\", 1)",
		},
		{
			name:           "charAt() rejects mismatched types",
			input:          "return charAt(10, \"hello\")",
			expectedErrors: []string{"type error: first argument to 'charAt' must be STRING, got NUMBER", "type error: second argument to 'charAt' must be NUMBER, got STRING"},
		},
		{
			name:  "substring() works with correct types",
			input: "return substring(\"hello\", 1, 4)",
		},
		{
			name:           "substring() rejects mismatched types",
			input:          "return substring(10, \"start\", \"end\")",
			expectedErrors: []string{"type error: first argument to 'substring' must be STRING, got NUMBER", "type error: second argument to 'substring' must be NUMBER, got STRING", "type error: third argument to 'substring' must be NUMBER, got STRING"},
		},
		{
			name:  "concat() works with correct types",
			input: "return concat(\"hello\", \" world\")",
		},
		{
			name:           "concat() rejects mismatched types",
			input:          "return concat(10, 20)",
			expectedErrors: []string{"type error: first argument to 'concat' must be STRING, got NUMBER", "type error: second argument to 'concat' must be STRING, got NUMBER"},
		},
		{
			name:  "split() works with correct types",
			input: "return split(\"hello\", \"e\")",
		},
		{
			name:           "split() rejects mismatched types",
			input:          "return split(10, 20)",
			expectedErrors: []string{"type error: first argument to 'split' must be STRING, got NUMBER", "type error: second argument to 'split' must be STRING, got NUMBER"},
		},
		{
			name:  "contains() works with correct types",
			input: "return contains(\"hello\", \"e\")",
		},
		{
			name:           "contains() rejects mismatched types",
			input:          "return contains(10, 20)",
			expectedErrors: []string{"type error: first argument to 'contains' must be STRING, got NUMBER", "type error: second argument to 'contains' must be STRING, got NUMBER"},
		},
		{
			name:  "startsWith() works with correct types",
			input: "return startsWith(\"hello\", \"h\")",
		},
		{
			name:           "startsWith() rejects mismatched types",
			input:          "return startsWith(10, 20)",
			expectedErrors: []string{"type error: first argument to 'startsWith' must be STRING, got NUMBER", "type error: second argument to 'startsWith' must be STRING, got NUMBER"},
		},
		{
			name:  "endsWith() works with correct types",
			input: "return endsWith(\"hello\", \"o\")",
		},
		{
			name:           "endsWith() rejects mismatched types",
			input:          "return endsWith(10, 20)",
			expectedErrors: []string{"type error: first argument to 'endsWith' must be STRING, got NUMBER", "type error: second argument to 'endsWith' must be STRING, got NUMBER"},
		},
		{
			name:  "replace() works with correct types",
			input: "return replace(\"hello\", \"e\", \"a\")",
		},
		{
			name:           "replace() rejects mismatched types",
			input:          "return replace(10, 20, 30)",
			expectedErrors: []string{"type error: first argument to 'replace' must be STRING, got NUMBER", "type error: second argument to 'replace' must be STRING, got NUMBER", "type error: third argument to 'replace' must be STRING, got NUMBER"},
		},
		{
			name:  "toUpper() works with correct types",
			input: "return toUpper(\"hello\")",
		},
		{
			name:           "toUpper() rejects mismatched types",
			input:          "return toUpper(10)",
			expectedErrors: []string{"type error: first argument to 'toUpper' must be STRING, got NUMBER"},
		},
		{
			name:  "toLower() works with correct types",
			input: "return toLower(\"HELLO\")",
		},
		{
			name:           "toLower() rejects mismatched types",
			input:          "return toLower(10)",
			expectedErrors: []string{"type error: first argument to 'toLower' must be STRING, got NUMBER"},
		},
		{
			name:  "trim() works with correct types",
			input: "return trim(\"  hello  \")",
		},
		{
			name:           "trim() rejects mismatched types",
			input:          "return trim(10)",
			expectedErrors: []string{"type error: first argument to 'trim' must be STRING, got NUMBER"},
		},
		{
			name:  "strlen() works with correct types",
			input: "return strlen(\"hello\")",
		},
		{
			name:           "strlen() rejects mismatched types",
			input:          "return strlen(10)",
			expectedErrors: []string{"type error: first argument to 'strlen' must be STRING, got NUMBER"},
		},
		{
			name:  "year() works with correct types",
			input: "let d = '2023-10-25'\nreturn year(d)",
		},
		{
			name:           "year() rejects mismatched types",
			input:          "return year(\"2023\")",
			expectedErrors: []string{"type error: first argument to 'year' must be DATE, got STRING"},
		},
		{
			name:  "month() works with correct types",
			input: "return month('2023-10-25')",
		},
		{
			name:  "day() works with correct types",
			input: "return day('2023-10-25')",
		},
		{
			name:  "weekday() works with correct types",
			input: "return weekday('2023-10-25')",
		},
		{
			name:           "weekday() rejects missing arguments",
			input:          "return weekday()",
			expectedErrors: []string{"arity error: expected 1 arguments for 'weekday', got 0"},
		},
		{
			name:  "today() works with correct types",
			input: "return today()",
		},
		{
			name:           "today() rejects unexpected arguments",
			input:          "return today(1)",
			expectedErrors: []string{"arity error: expected 0 arguments for 'today', got 1"},
		},
		{
			name:  "parseDate() works with correct types",
			input: "return parseDate(\"2023-10-25\")",
		},
		{
			name:           "parseDate() rejects mismatched types",
			input:          "return parseDate(2023)",
			expectedErrors: []string{"type error: first argument to 'parseDate' must be STRING, got NUMBER"},
		},
		{
			name:  "addDays() works with correct types",
			input: "return addDays('2023-10-25', 5)",
		},
		{
			name:           "addDays() rejects mismatched types",
			input:          "return addDays('2023-10-25', \"5\")",
			expectedErrors: []string{"type error: second argument to 'addDays' must be NUMBER, got STRING"},
		},
		{
			name:  "diffDays() works with correct types",
			input: "return diffDays('2023-10-30', '2023-10-25')",
		},
		{
			name:           "diffDays() rejects mismatched types",
			input:          "return diffDays('2023-10-30', 5)",
			expectedErrors: []string{"type error: second argument to 'diffDays' must be DATE, got NUMBER"},
		},
		{
			name:  "newDate() works with correct types",
			input: "return newDate(2023, 1, 1)",
		},
		{
			name:           "newDate() rejects mismatched types",
			input:          "return newDate(\"2023\", 1, 1)",
			expectedErrors: []string{"type error: first argument to 'newDate' must be NUMBER, got STRING"},
		},
	}
	runTestScenarios(t, tests)
}
