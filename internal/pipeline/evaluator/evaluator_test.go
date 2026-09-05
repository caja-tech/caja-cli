package evaluator

import (
	"caja-cli/internal/pipeline/analyzer"
	"caja-cli/internal/pipeline/ast"
	"caja-cli/internal/pipeline/environment"
	"caja-cli/internal/pipeline/lexer"
	"caja-cli/internal/pipeline/parser"
	"fmt"
	"math"
	"strings"
	"testing"
)

type testScenario struct {
	name     string
	input    string
	expected interface{}
}

type testErrorScenario struct {
	name          string
	input         string
	expectedError string
}

// testEval is a helper that tokenizes, parses, and evaluates the given input
// string through the full pipeline, returning the final result or the
// first error encountered (from the parser or evaluator).
func testEval(input string) (interface{}, error) {
	tknzr := lexer.New(input)
	p := parser.New(tknzr)
	program := p.Parse()

	if p.HasErrors() {
		return nil, fmt.Errorf("parser errors: %v", p.Errors())
	}

	env := environment.NewEnvironment("", "", false)
	a := analyzer.New(env)
	a.Run(program)
	if a.HasErrors() {
		return nil, fmt.Errorf("semantic errors: %v", a.Errors())
	}

	result, err := Eval(program, env)
	if err != nil {
		return nil, err
	}

	switch obj := result.(type) {
	case *environment.Number:
		return obj.Value, nil
	case *environment.String:
		return obj.Value, nil
	case *environment.Boolean:
		return obj.Value, nil
	case *environment.Date:
		return obj.Value.Format("2006-01-02"), nil
	case *environment.Array:
		var elements []string
		for _, el := range obj.Elements {
			elements = append(elements, el.Inspect())
		}
		return "[" + strings.Join(elements, ", ") + "]", nil
	case *environment.Function:
		return obj.Inspect(), nil
	case *environment.Module:
		return obj.Inspect(), nil
	case *environment.StructObject:
		return obj.Inspect(), nil
	default:
		return nil, nil
	}
}

// TestEvaluateArrays verifies the runtime execution of array literals and array indexing.
func TestEvaluateArrays(t *testing.T) {
	var tests = []testScenario{
		{"Evaluate array literal", "return [1, 2 * 2, 3 + 3]", "[1, 4, 6]"},
		{"Index array literal directly", "return [1, 2, 3][1]", 2.0},
		{"Index array from variable", "let a = [10, 20, 30]\nreturn a[2]", 30.0},
		{"Index array with expression", "let a = [10, 20, 30]\nreturn a[1 + 1]", 30.0},
		{"Nested array literal", "return [[1, 2], [3, 4]]", "[[1, 2], [3, 4]]"},
		{"Index nested array", "let a = [[1, 2], [3, 4]]\nreturn a[1][0]", 3.0},
		{"Modify array element", "let a = [1]\na[0] = 5\nreturn a[0]", 5.0},
		{"Modify nested array element", "let a = [[1, 2]]\na[0][1] = 5\nreturn a[0][1]", 5.0},
		{"Modify array element using expression", "let a = [1]\na[0] = a[0] + 9\nreturn a[0]", 10.0},
		{"Modify array element using variable as index", "let a = [1, 2]\nlet i = 1\na[i] = 10\nreturn a[1]", 10.0},
	}

	runTestScenarios(t, tests)
}

// TestErrorArrays verifies runtime errors during array evaluation like out of bounds.
func TestErrorArrays(t *testing.T) {
	var tests = []testErrorScenario{
		{"Index out of bounds positive", "let a = [1, 2]\nreturn a[2]", "runtime error: array index out of bounds"},
		{"Index operator on non-array (caught by semantic)", "let a = 1\nreturn a[0]", "type error: index operator not supported for Number"},
		{"Assign index out of bounds", "let a = [1, 2]\na[2] = 5", "runtime error: array index out of bounds"},
		{"Assign index out of bounds negative", "let a = [1, 2]\na[-1] = 5", "runtime error: array index out of bounds"},
	}

	runTestErrorScenarios(t, tests)
}

// runTestScenarios iterates over a slice of testScenario entries, evaluating
// each input and asserting that the result matches the expected value.
func runTestScenarios(t *testing.T, tests []testScenario) {
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			evaluated, err := testEval(tc.input)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if evaluated != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, evaluated)
			}
		})
	}
}

// runTestErrorScenarios iterates over a slice of testErrorScenario entries,
// evaluating each input and asserting that the evaluation fails with the
// expected error message.
func runTestErrorScenarios(t *testing.T, tests []testErrorScenario) {
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := testEval(tc.input)

			if err == nil {
				t.Fatalf("expected an error but got none")
			}

			if !strings.Contains(err.Error(), tc.expectedError) {
				t.Errorf("expected error to contain %q, got %q", tc.expectedError, err.Error())
			}
		})
	}
}

// TestEvaluateMath verifies core arithmetic evaluation including single
// numbers, decimals, the four basic operators, operator precedence, grouped
// expressions with parentheses, and a complex combined expression.
func TestEvaluateMath(t *testing.T) {
	var tests = []testScenario{
		{"Single Number", "return 5", 5.0},
		{"Decimal Number", "return 15.5", 15.5},
		{"Addition", "return 5 + 5", 10.0},
		{"Subtraction", "return 10 - 2", 8.0},
		{"Multiplication", "return 5 * 2", 10.0},
		{"Division", "return 10 / 2", 5.0},
		{"Modulo", "return 10 % 3", 1.0},
		{"Exponentiation", "return 2 ^ 3", 8.0},
		{"Order of Operations", "return 5 + 5 * 2", 15.0},
		{"Order of Operations with Exponent", "return 2 * 3 ^ 2", 18.0},
		{"Parentheses", "return (5 + 5) * 2", 20.0},
		{"Complex Math", "return 100 / (10 + 10) * 5", 25.0},
		{"Comparison >", "return 10 > 5", true},
		{"Comparison <", "return 10 < 5", false},
		{"Comparison >=", "return 10 >= 10", true},
		{"Comparison <=", "return 10 <= 5", false},
		{"Comparison ==", "return 10 == 10", true},
		{"Comparison !=", "return 10 != 10", false},
		{"Prefix Minus", "return -5", -5.0},
		{"Logical AND true", "return true and true", true},
		{"Logical AND false", "return true and false", false},
		{"Logical OR true", "return false or true", true},
		{"Logical OR false", "return false or false", false},
		{"Logical XOR true", "return true xor false", true},
		{"Logical XOR false", "return true xor true", false},
		{"Short-circuit AND", "return false and 1 / 0 == 1", false},
		{"Short-circuit OR", "return true or 1 / 0 == 1", true},
		{"Logical complex", "return (10 > 5) and (5 < 10) or false", true},

		{"Prefix Minus Double", "return --5", 5.0},
		{"Prefix Bang True", "return !true", false},
		{"Prefix Bang False", "return !false", true},
		{"Prefix Bang Grouped Logic", "return !(true and false)", true},
		{"Prefix Bang Double", "return !!true", true},
	}

	runTestScenarios(t, tests)
}

// TestEvaluateVariables verifies variable assignment and lookup, including
// simple assign-and-read, arithmetic with multiple variables, self-referencing
// reassignment, and cascading assignments where later variables depend on
// earlier ones.
func TestEvaluateConst(t *testing.T) {
	tests := []testScenario{
		{"Return const", "const a = 5\nreturn a", 5.0},
		{"Math with const", "const a = 5 * 5\nreturn a", 25.0},
		{"Const from const", "const a = 5\nconst b = a\nreturn b", 5.0},
		{"Cascading consts", "const a = 5\nconst b = a + 5\nconst c = a + b + 5\nreturn c", 20.0},
		{"Private const", "private const a = 42\nreturn a", 42.0},
	}

	runTestScenarios(t, tests)
}

func TestGenericFunctions(t *testing.T) {
	tests := []testScenario{
		{
			name: "Generic function with turbofish syntax",
			input: `
			let identity = fn<T>(x: T) -> T {
				return x
			}
			return identity::<Number>(10)
			`,
			expected: 10.0,
		},
		{
			name: "Generic function without turbofish syntax (inferred)",
			input: `
			let wrap = fn<T>(x: T) -> [T] {
				return [x]
			}
			return wrap("caja")[0]
			`,
			expected: "caja",
		},
		{
			name: "Generic struct instantiation (simulated with function)",
			input: `
			type Box<T> struct {
				value T
			}
			let makeBox = fn<T>(val: T) -> Box<T> {
				return Box::<T> { value: val }
			}
			return makeBox::<String>("hello").value
			`,
			expected: "hello",
		},
		{
			name: "Generic map passed as argument with turbofish",
			input: `
			let f = fn<T, P>(x: T, m: map[T]P) -> P { return m[x] }
			let m: map[String]Number = { "1": 10 }
			return f::<String, Number>("1", m)
			`,
			expected: 10.0,
		},
		{
			name: "Generic map passed as argument implicitly",
			input: `
			let f = fn<T, P>(x: T, m: map[T]P) -> P { return m[x] }
			let m: map[String]Number = { "1": 10 }
			return f("1", m)
			`,
			expected: 10.0,
		},
	}
	runTestScenarios(t, tests)
}

func TestEvaluateLetStatements(t *testing.T) {
	var tests = []testScenario{
		{"Assign and return", "let rate = 15.5\nreturn rate", 15.5},
		{"Math with variables", "let rate = 10\nlet tax = 5\nreturn rate * tax", 50.0},
		{"Reassignment", "let x = 5\nx = x + 5\nreturn x", 10.0},
		{"Cascading variables", "let a = 5\nlet b = a\nlet c = a + b + 5\nreturn c", 15.0},
	}

	runTestScenarios(t, tests)
}

// TestEvaluateFunctions verifies that function declarations, closures, and calls
// are evaluated correctly, and that return statements inside functions properly
// halt block execution and return the correct value.
func TestEvaluateFunctions(t *testing.T) {
	var tests = []testScenario{
		{
			name:     "Simple function call",
			input:    "let add = fn(x: Number, y: Number) -> Number { return x + y }\nreturn add(5, 5)",
			expected: 10.0,
		},
		{
			name:     "Function returning highest number",
			input:    "let max = fn(a: Number, b: Number) -> Number { if (a > b) { return a } else { return b } }\nreturn max(10, 20)",
			expected: 20.0,
		},
		{
			name:     "Higher order function with type alias",
			input:    "type Op fn(Number, Number) -> Number\nlet applyOp = fn(a: Number, b: Number, op: Op) -> Number { return op(a, b) }\nlet add = fn(x: Number, y: Number) -> Number { return x + y }\nreturn applyOp(10, 20, add)",
			expected: 30.0,
		},
		{
			name:     "Higher order function with inline function parameter",
			input:    "let applyOp = fn(a: Number, b: Number, op: fn(Number, Number) -> Number) -> Number { return op(a, b) }\nlet add = fn(x: Number, y: Number) -> Number { return x + y }\nreturn applyOp(10, 20, add)",
			expected: 30.0,
		},
		{
			name:     "Higher order function returning inline function",
			input:    "let getMultiplier = fn(factor: Number) -> fn(Number) -> Number { return fn(x: Number) -> Number { return x * factor } }\nlet timesTwo = getMultiplier(2)\nreturn timesTwo(5)",
			expected: 10.0,
		},
		{
			name:     "Array of inline functions",
			input:    "let funcs = [fn(x: Number) -> Number { return x * 2 }, fn(x: Number) -> Number { return x * 3 }]\nreturn funcs[1](5)",
			expected: 15.0,
		},
		{
			name:     "Function returning Nothing implicitly",
			input:    "let doNothing = fn() -> Nothing { let a = 1 }\nreturn doNothing()",
			expected: "Nothing {  }",
		},
		{
			name:     "Function returning Nothing explicitly empty",
			input:    "let doNothing = fn() -> Nothing { return }\nreturn doNothing()",
			expected: "Nothing {  }",
		},
		{
			name: "Recursive function execution (factorial)",
			input: `
let factorial = fn(n: Number) -> Number {
	if (n == 0) {
		return 1
	} else {
		return n * factorial(n - 1)
	}
}
return factorial(5)
`,
			expected: 120.0,
		},
		{
			name: "Recursive function execution (fibonacci)",
			input: `
let fib = fn(n: Number) -> Number {
	if (n < 2) {
		return n
	} else {
		return fib(n - 1) + fib(n - 2)
	}
}
return fib(6)
`,
			expected: 8.0,
		},
		{
			name: "Tail recursive function execution",
			input: `
let deepRecurse = fn(n: Number) -> Number {
	if (n == 0) {
		return 0
	} else {
		return deepRecurse(n - 1)
	}
}
return deepRecurse(10005)
`,
			expected: 0.0,
		},
	}

	runTestScenarios(t, tests)
}

// TestEvaluateIfElseExpressions verifies the evaluation of conditional if/else
// blocks. It checks that the consequence block evaluates when the condition is
// truthy, the alternative block evaluates when falsy, and a falsy condition
// without an else block evaluates to 0.0.
func TestEvaluateIfElseExpressions(t *testing.T) {
	var tests = []testScenario{
		{"Condition is true", "return if (10 > 5) { 100 } else { 200 }", 100.0},
		{"Condition is false", "return if (5 > 10) { 100 } else { 200 }", 200.0},
		{"No else, condition is true", "return if (10 > 5) { 100 }", 100.0},
		{"No else, condition is false", "return if (5 > 10) { 100 }", 0.0},
		{"Complex condition true", "let x = 10\nlet y = 20\nreturn if (x < y) { 1 } else { 0 }", 1.0},
		{"If else inside assignment", "let val = if (10 == 10) { 42 } else { 0 }\nreturn val", 42.0},
	}

	runTestScenarios(t, tests)
}

// TestErrorHandling verifies that the evaluator surfaces the correct error
// messages for common failure cases: referencing an undefined variable and
// dividing by a literal zero.
func TestErrorHandling(t *testing.T) {
	var tests = []testErrorScenario{
		{"Undefined variable", "10 + rate", "semantic error: undeclared variable 'rate'"},
		{"Division by zero", "10 / 0", "division by zero"},
		{"Modulo by zero", "10 % 0", "modulo by zero"},
		{
			"Generic map passed as argument (invalid generic instantiation mismatch)", 
			"let f = fn<T, P>(x: T, m: map[T]P) -> P { return m[x] }\nlet m: map[String]Number = { \"1\": 10 }\nlet a = f::<Boolean, Number>(true, m)", 
			"[Line 3, Column 10] type error: argument 2 expected map[Boolean]Number, got map[String]Number",
		},
	}

	runTestErrorScenarios(t, tests)

	// Test stack overflow separately to control the limit
	t.Run("Stack overflow triggers stack tracer", func(t *testing.T) {
		originalLimit := stackTraceLimit
		stackTraceLimit = 10
		defer func() { stackTraceLimit = originalLimit }()

		_, err := testEval("let deepRecurse = fn(n: Number) -> Number { if (n == 0) { return 0 } else { return 1 + deepRecurse(n - 1) } }\nreturn deepRecurse(20)")
		if err == nil {
			t.Fatalf("expected an error but got none")
		}

		if !strings.Contains(err.Error(), "stack overflow") {
			t.Errorf("expected error to contain 'stack overflow', got %q", err.Error())
		}
	})
}

// TestEvaluateBlockScoping verifies that isolated environments created by
// block statements correctly handle variable modification and shadowing.
func TestEvaluateBlockScoping(t *testing.T) {
	var tests = []testScenario{
		{
			name:     "Outer scope modification",
			input:    "let x = 10\nif (10 > 5) {\n x = 20 \n}\nreturn x",
			expected: 20.0,
		},
	}

	runTestScenarios(t, tests)
}

// TestErrorBlockScoping verifies that variables declared inside an inner block
// do not leak into the outer environment when the block terminates.
func TestErrorBlockScoping(t *testing.T) {
	var tests = []testErrorScenario{
		{
			name:          "Inner scope leak prevention",
			input:         "if (10 > 5) {\n let x = 20 \n}\nreturn x",
			expectedError: "semantic error: undeclared variable 'x'",
		},
		{
			name:          "Environment shadowing",
			input:         "let x = 10\nif (10 > 5) {\n let x = 20 \n}\nreturn x",
			expectedError: "semantic error: variable 'x' is already declared",
		},
	}

	runTestErrorScenarios(t, tests)
}

// ---------------------------------------------------------------------------
// Error Paths
// ---------------------------------------------------------------------------

// mockNode is an ast.Node implementation unknown to the evaluator,
// used to exercise the "unknown node type" fallback.
type mockNode struct{}

// TokenLiteral returns an empty string since mockNode carries no real token.
func (m *mockNode) TokenLiteral() string { return "" }

// ToString returns an empty string since mockNode has no meaningful representation.
func (m *mockNode) String() string { return "" }

// TestErrorUnknownOperator verifies that evaluating an InfixExpression with an
// unsupported operator (e.g. "#") returns an "unknown operator" error. The AST
// node is constructed directly to bypass the parser, which would never produce
// such an operator.
func TestErrorUnknownOperator(t *testing.T) {
	node := &ast.InfixExpression{
		Operator: "#",
		Left:     &ast.NumberLiteral{Value: 10},
		Right:    &ast.NumberLiteral{Value: 3},
	}

	env := environment.NewEnvironment("", "", false)
	_, err := Eval(node, env)
	if err == nil {
		t.Fatal("expected an error but got none")
	}

	expected := "unknown operator: #"
	if err.Error() != expected {
		t.Errorf("expected error %q, got %q", expected, err.Error())
	}
}

// TestErrorUnknownNodeType verifies that passing a Node implementation not
// recognized by the evaluator's type switch returns an "unknown node type"
// error. A mockNode is used to simulate this scenario.
func TestErrorUnknownNodeType(t *testing.T) {
	env := environment.NewEnvironment("", "", false)
	_, err := Eval(&mockNode{}, env)
	if err == nil {
		t.Fatal("expected an error but got none")
	}

	expected := "unknown node type: *evaluator.mockNode"
	if err.Error() != expected {
		t.Errorf("expected error %q, got %q", expected, err.Error())
	}
}

// TestErrorInLeftOperand verifies that an error in the left-hand side of an
// infix expression (an undefined variable) propagates correctly.
func TestErrorInLeftOperand(t *testing.T) {
	_, err := testEval("return unknown_var + 5")
	if err == nil {
		t.Fatal("expected an error but got none")
	}

	expected := "semantic error: undeclared variable 'unknown_var'"
	if !strings.Contains(err.Error(), expected) {
		t.Errorf("expected error to contain %q, got %q", expected, err.Error())
	}
}

// TestErrorInRightOperand verifies that an error in the right-hand side of an
// infix expression (an undefined variable) propagates correctly.
func TestErrorInRightOperand(t *testing.T) {
	_, err := testEval("return 5 + missing_var")
	if err == nil {
		t.Fatal("expected an error but got none")
	}

	expected := "semantic error: undeclared variable 'missing_var'"
	if !strings.Contains(err.Error(), expected) {
		t.Errorf("expected error to contain %q, got %q", expected, err.Error())
	}
}

// TestErrorInAssignmentValue verifies that an error in the value expression of
// an assignment statement (referencing an undefined variable) propagates and
// prevents the assignment from completing.
func TestErrorInAssignmentValue(t *testing.T) {
	_, err := testEval("x = undefined_var")
	if err == nil {
		t.Fatal("expected an error but got none")
	}

	expected1 := "undeclared variable 'undefined_var'"
	expected2 := "undeclared variable 'x'. Use 'let' to declare it."
	if !strings.Contains(err.Error(), expected1) || !strings.Contains(err.Error(), expected2) {
		t.Errorf("expected error to contain %q and %q, got %q", expected1, expected2, err.Error())
	}
}

// TestErrorMidProgramHaltsExecution verifies that when a multi-statement
// program encounters an error in an intermediate statement, execution stops
// immediately and subsequent statements are not evaluated.
func TestErrorMidProgramHaltsExecution(t *testing.T) {
	_, err := testEval("let x = 5\nlet y = unknown\nx + 1")
	if err == nil {
		t.Fatal("expected an error but got none")
	}

	expected := "semantic error: undeclared variable 'unknown'"
	if !strings.Contains(err.Error(), expected) {
		t.Errorf("expected error to contain %q, got %q", expected, err.Error())
	}
}

// TestErrorDivisionByZeroWithVariable verifies that dividing by a variable
// whose value is zero produces a "division by zero" error, testing the runtime
// check rather than a static literal check.
func TestErrorDivisionByZeroWithVariable(t *testing.T) {
	_, err := testEval("let x = 0\nreturn 10 / x")
	if err == nil {
		t.Fatal("expected an error but got none")
	}

	expected := "division by zero"
	if err.Error() != expected {
		t.Errorf("expected error %q, got %q", expected, err.Error())
	}
}

// ---------------------------------------------------------------------------
// Arithmetic Edge Cases
// ---------------------------------------------------------------------------

// TestNegativeResult verifies that subtraction producing a negative value
// returns the correct result.
func TestNegativeResult(t *testing.T) {
	evaluated, err := testEval("return 3 - 10")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evaluated != -7.0 {
		t.Errorf("expected -7, got %f", evaluated)
	}
}

// TestZeroResult verifies that subtracting a number from itself correctly
// evaluates to zero.
func TestZeroResult(t *testing.T) {
	evaluated, err := testEval("return 5 - 5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evaluated != 0.0 {
		t.Errorf("expected 0, got %f", evaluated)
	}
}

// TestFloatingPointArithmetic verifies that addition of two decimal numbers
// produces the correct float64 result.
func TestFloatingPointArithmetic(t *testing.T) {
	evaluated, err := testEval("return 0.5 + 0.25")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evaluated != 0.75 {
		t.Errorf("expected 0.75, got %f", evaluated)
	}
}

// TestLargeNumbers verifies that the evaluator handles multiplication of large
// numeric values without overflow or precision loss within float64 range.
func TestLargeNumbers(t *testing.T) {
	evaluated, err := testEval("return 1000000 * 1000000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evaluated != 1e12 {
		t.Errorf("expected 1e12, got %f", evaluated)
	}
}

// TestDivisionProducingDecimal verifies that dividing two integers that do not
// divide evenly produces the correct decimal result.
func TestDivisionProducingDecimal(t *testing.T) {
	evaluated, err := testEval("return 1 / 4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evaluated != 0.25 {
		t.Errorf("expected 0.25, got %f", evaluated)
	}
}

// TestChainedOperations verifies that a long chain of same-precedence
// additions is evaluated correctly via left-to-right associativity.
func TestChainedOperations(t *testing.T) {
	evaluated, err := testEval("return 1 + 2 + 3 + 4 + 5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evaluated != 15.0 {
		t.Errorf("expected 15, got %f", evaluated)
	}
}

// TestNestedParentheses verifies that parenthesized sub-expressions on both
// sides of an operator are evaluated before the outer operation.
func TestNestedParentheses(t *testing.T) {
	evaluated, err := testEval("return ((2 + 3) * (4 - 1))")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evaluated != 15.0 {
		t.Errorf("expected 15, got %f", evaluated)
	}
}

// TestDeeplyNestedParentheses verifies that multiple levels of redundant
// parentheses around a single number are handled correctly.
func TestDeeplyNestedParentheses(t *testing.T) {
	evaluated, err := testEval("return (((5)))")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evaluated != 5.0 {
		t.Errorf("expected 5, got %f", evaluated)
	}
}

// TestMixedPrecedenceChain verifies correct evaluation of an expression that
// combines addition, subtraction, multiplication, and division, requiring
// proper operator precedence handling (2 + 3*4 - 6/2 = 11).
func TestMixedPrecedenceChain(t *testing.T) {
	evaluated, err := testEval("return 2 + 3 * 4 - 6 / 2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evaluated != 11.0 {
		t.Errorf("expected 11, got %f", evaluated)
	}
}

// ---------------------------------------------------------------------------
// Variable / Environment Edge Cases
// ---------------------------------------------------------------------------

// TestVariableInComplexExpression verifies that a stored variable can be used
// inside a grouped arithmetic expression.
func TestVariableInComplexExpression(t *testing.T) {
	evaluated, err := testEval("let x = 10\nreturn (x + 5) * 2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evaluated != 30.0 {
		t.Errorf("expected 30, got %f", evaluated)
	}
}

// TestMultipleReassignments verifies that a variable can be overwritten
// multiple times and that reading it returns the most recent value.
func TestMultipleReassignments(t *testing.T) {
	evaluated, err := testEval("let x = 1\nx = 2\nx = 3\nreturn x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evaluated != 3.0 {
		t.Errorf("expected 3, got %f", evaluated)
	}
}

// TestVariableOverwriteWithExpression verifies that a variable can be
// reassigned using its own current value in the right-hand expression
// (x = x * x).
func TestVariableOverwriteWithExpression(t *testing.T) {
	evaluated, err := testEval("let x = 10\nx = x * x\nreturn x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evaluated != 100.0 {
		t.Errorf("expected 100, got %f", evaluated)
	}
}

// TestManyVariablesInOneExpression verifies that multiple distinct variables
// can be referenced together in a single arithmetic expression.
func TestManyVariablesInOneExpression(t *testing.T) {
	evaluated, err := testEval("let a = 1\nlet b = 2\nlet c = 3\nreturn a + b + c")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evaluated != 6.0 {
		t.Errorf("expected 6, got %f", evaluated)
	}
}

// TestAssignmentRequiresReturn verifies that an assignment statement alone
// does not produce a valid program without a return statement.
func TestAssignmentRequiresReturn(t *testing.T) {
	_, err := testEval("let x = 42")
	if err == nil {
		t.Fatal("expected an error but got none")
	}
	expected := "execution error: script finished without a return statement"
	if err.Error() != expected {
		t.Errorf("expected error %q, got %q", expected, err.Error())
	}
}

// TestChainOfDependentAssignments verifies a sequence of assignments where
// each variable depends on previously assigned ones (a=2, b=a*3, c=b+a),
// ensuring correct evaluation order and environment state.
func TestChainOfDependentAssignments(t *testing.T) {
	evaluated, err := testEval("let a = 2\nlet b = a * 3\nlet c = b + a\nreturn c")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evaluated != 8.0 {
		t.Errorf("expected 8, got %f", evaluated)
	}
}

// ---------------------------------------------------------------------------
// Program Structure
// ---------------------------------------------------------------------------

// TestEmptyProgram verifies that evaluating an empty input returns an error
// because there is no return statement.
func TestEmptyProgram(t *testing.T) {
	_, err := testEval("")
	if err == nil {
		t.Fatal("expected an error but got none")
	}
	expected := "execution error: script finished without a return statement"
	if err.Error() != expected {
		t.Errorf("expected error %q, got %q", expected, err.Error())
	}
}

// TestProgramReturnsReturnStatement verifies that a multi-statement program
// returns the result of the return statement.
func TestProgramReturnsReturnStatement(t *testing.T) {
	evaluated, err := testEval("1\n2\nreturn 3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evaluated != 3.0 {
		t.Errorf("expected 3, got %f", evaluated)
	}
}

// TestSingleAssignmentProgram verifies that a program consisting of a single
// assignment statement and a return returns the assigned value as its result.
func TestSingleAssignmentProgram(t *testing.T) {
	evaluated, err := testEval("let x = 99\nreturn x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evaluated != 99.0 {
		t.Errorf("expected 99, got %f", evaluated)
	}
}

// ---------------------------------------------------------------------------
// Evaluator Constructor
// ---------------------------------------------------------------------------

// TestIndependentEnvironments verifies that two evaluators created with New
// have completely isolated environments: setting a variable in one must not
// make it visible in the other.
func TestIndependentEnvironments(t *testing.T) {
	// Set variable in first evaluator
	assignNode := &ast.LetStatement{
		Name:  &ast.Identifier{Value: "x"},
		Value: &ast.NumberLiteral{Value: 42},
	}

	env1 := environment.NewEnvironment("", "", false)
	_, err := Eval(assignNode, env1)
	if err != nil {
		t.Fatalf("unexpected error setting variable: %v", err)
	}

	// Second evaluator must not see the variable
	identNode := &ast.ExpressionStatement{
		Expression: &ast.Identifier{Value: "x"},
	}

	env2 := environment.NewEnvironment("", "", false)
	_, err = Eval(identNode, env2)
	if err == nil {
		t.Fatal("expected error from second evaluator but got none")
	}
}

// TestEvaluateTypes verifies the evaluation of types other than Number,
// such as Strings, Booleans, and potentially Dates.
func TestEvaluateTypes(t *testing.T) {
	var tests = []testScenario{
		{
			name:     "String literal",
			input:    "return \"hello world\"",
			expected: "hello world",
		},
		{
			name:     "Boolean true",
			input:    "return true",
			expected: true,
		},
		{
			name:     "Boolean false",
			input:    "return false",
			expected: false,
		},
		{
			name:     "Return string from function",
			input:    "let greet = fn() -> String { return \"hello\" }\nreturn greet()",
			expected: "hello",
		},
		{
			name:     "Return boolean from function",
			input:    "let isTrue = fn() -> Boolean { return true }\nreturn isTrue()",
			expected: true,
		},
		{
			name:     "Date literal",
			input:    "return '2023-10-25'",
			expected: "2023-10-25",
		},
		{
			name:     "Return date from function",
			input:    "let getToday = fn() -> Date { return '2023-10-25' }\nreturn getToday()",
			expected: "2023-10-25",
		},
	}

	runTestScenarios(t, tests)
}

func TestEvaluateBuiltins(t *testing.T) {
	var tests = []testScenario{
		{"len on number array", "import array\nreturn array.len([1, 2, 3])", 3.0},
		{"len on string array", "import array\nreturn array.len([\"a\", \"b\"])", 2.0},
		{"len on empty array", "import array\nreturn array.len([])", 0.0},
		{"push to array", "import array\nlet arr = [1, 2]\nlet new = array.push(arr, 3)\nreturn new[2]", 3.0},
		{"push does not modify original", "import array\nlet arr = [1, 2]\narray.push(arr, 3)\nreturn array.len(arr)", 2.0},
		{"pop from array", "import array\nlet arr = [1, 2, 3]\nlet popped = array.pop(arr)\nreturn array.len(popped)", 2.0},
		{"pop from empty array", "import array\nreturn array.len(array.pop([]))", 0.0},
		{"head of array", "import array\nreturn array.head([5, 6, 7])", 5.0},
		{"tail of array", "import array\nlet t = array.tail([1, 2, 3])\nreturn t[0]", 2.0},
		{"tail of empty array", "import array\nreturn array.len(array.tail([]))", 0.0},
		{"last of array", "import array\nreturn array.last([5, 6, 7])", 7.0},
		{"copy of array", "import array\nlet arr = [1, 2]\nlet c = array.copy(arr)\narray.push(c, 3)\nreturn array.len(arr)", 2.0},
		{"slice of array", "import array\nlet s = array.slice([1, 2, 3, 4], 1, 3)\nreturn s[1]", 3.0},
		{"join of arrays", "import array\nlet j = array.join([1, 2], [3, 4])\nreturn j[2]", 3.0},
		{"generic array function variable assignment", "import array\nlet p = array.push\nlet arr = [1, 2]\nlet res = p(arr, 3)\nreturn res[2]", 3.0},
		{"explicit generic array turbofish", "import array\nlet arr = [1, 2]\nlet res = array.push::<Number>(arr, 3)\nreturn res[2]", 3.0},
		{"generic array function on array of generic structs", "import array\ntype Container<T> struct { val T }\nlet arr = [Container::<Number> { val: 1 }]\nlet pushFunc = array.push\nlet res = pushFunc(arr, Container::<Number> { val: 2 })\nreturn array.len(res)", 2.0},
		{"join strings", "import string\nreturn string.join([\"a\", \"b\"], \",\")", "a,b"},
		{"join strings empty delimiter", "import string\nreturn string.join([\"a\", \"b\"], \"\")", "ab"},
		{"charAt string", "import string\nreturn string.charAt(\"hello\", 1)", "e"},
		{"substring of string", "import string\nreturn string.substring(\"hello\", 1, 4)", "ell"},
		{"concat of strings", "import string\nreturn string.concat(\"hello\", \" world\")", "hello world"},
		{"split string by comma", "import string\nreturn string.split(\"a,b,c\", \",\")[1]", "b"},
		{"contains returns true", "import string\nreturn string.contains(\"hello\", \"ell\")", true},
		{"contains returns false", "import string\nreturn string.contains(\"hello\", \"world\")", false},
		{"startsWith true", "import string\nreturn string.startsWith(\"hello\", \"he\")", true},
		{"endsWith true", "import string\nreturn string.endsWith(\"hello\", \"lo\")", true},
		{"replace string", "import string\nreturn string.replace(\"hello world\", \"world\", \"caja\")", "hello caja"},
		{"toUpper", "import string\nreturn string.toUpper(\"hello\")", "HELLO"},
		{"toLower", "import string\nreturn string.toLower(\"HELLO\")", "hello"},
		{"trim", "import string\nreturn string.trim(\"   hello \")", "hello"},
		{"strlen", "import string\nreturn string.len(\"hello\")", 5.0},
		{"year", "import date\nreturn date.year('2023-10-25')", 2023.0},
		{"month", "import date\nreturn date.month('2023-10-25')", 10.0},
		{"day", "import date\nreturn date.day('2023-10-25')", 25.0},
		{"weekday", "import date\nreturn date.weekday('2023-10-25')", 3.0},
		{"parse", "import date\nreturn date.parse(\"2023-10-25\")", "2023-10-25"},
		{"addDays", "import date\nreturn date.addDays('2023-10-25', 5)", "2023-10-30"},
		{"addDays negative", "import date\nreturn date.addDays('2023-10-25', 0 - 5)", "2023-10-20"},
		{"diffDays", "import date\nreturn date.diffDays('2023-10-30', '2023-10-25')", 5.0},
		{"new", "import date\nreturn date.new(2023, 10, 25)", "2023-10-25"},
		{"today is recent", "import date\nreturn date.year(date.today()) >= 2024", true},
		{"math abs", "import math\nreturn math.abs(0 - 5.5)", 5.5},
		{"math sqrt", "import math\nreturn math.sqrt(16)", 4.0},
		{"math pow", "import math\nreturn math.pow(2, 3)", 8.0},
		{"math floor", "import math\nreturn math.floor(4.9)", 4.0},
		{"math ceil", "import math\nreturn math.ceil(4.1)", 5.0},
		{"math round", "import math\nreturn math.round(4.5)", 5.0},
		{"math min", "import math\nreturn math.min(10, 5)", 5.0},
		{"math max", "import math\nreturn math.max(10, 5)", 10.0},
		{"math log", "import math\nreturn math.log(100, 10)", 2.0},
		{"math PI", "import math\nreturn math.PI", math.Pi},
		{"math E", "import math\nreturn math.E", math.E},
		{"math SQRT2", "import math\nreturn math.SQRT2", math.Sqrt2},
		{"math LN2", "import math\nreturn math.LN2", math.Ln2},
		{"math LN10", "import math\nreturn math.LN10", math.Ln10},
		{"math LOG2E", "import math\nreturn math.LOG2E", math.Log2E},
		{"math LOG10E", "import math\nreturn math.LOG10E", math.Log10E},
		{"math rand", "import math\nlet r = math.rand()\nreturn r >= 0 and r < 1", true},
		{"map containsKey true", "import map\ntype CustomStruct struct {\nkey map.KeyFunc\nvalue Number\n}\nlet d: map[CustomStruct]Number = {}\nlet s = CustomStruct{key: fn() -> String { return \"a\" }, value: 10}\nd[s] = 100\nreturn map.containsKey(d, s)", true},
		{"map containsKey false", "import map\nlet d: map[String]Number = {}\nd[\"a\"] = 1\nreturn map.containsKey(d, \"b\")", false},
		// Cast tests
		{"cast to success", "import cast\nreturn cast.to(\"123.45\", 0)", 123.45},
		{"cast to fallback", "import cast\nreturn cast.to(\"abc\", 0 - 1)", -1.0},
		{"cast to string from number", "import cast\nreturn cast.to(123, \"err\")", "123"},
		{"cast to string from boolean", "import cast\nreturn cast.to(true, \"err\")", "true"},
		{"cast to boolean success", "import cast\nreturn cast.to(\"true\", false)", true},
		{"cast to boolean fallback", "import cast\nreturn cast.to(\"foo\", true)", true},
	}
	runTestScenarios(t, tests)
}

func TestErrorBuiltins(t *testing.T) {
	var tests = []testErrorScenario{
		{"charAt index out of bounds (negative)", "import string\nreturn string.charAt(\"hello\", 0 - 1)", "runtime error: charAt index -1 out of bounds"},
		{"charAt index out of bounds (too large)", "import string\nreturn string.charAt(\"hello\", 5)", "runtime error: charAt index 5 out of bounds"},
		{"substring start out of bounds", "import string\nreturn string.substring(\"hello\", 0 - 1, 4)", "runtime error: substring start index -1 out of bounds"},
		{"substring end out of bounds", "import string\nreturn string.substring(\"hello\", 0, 6)", "runtime error: substring end index 6 out of bounds"},
		{"substring start > end", "import string\nreturn string.substring(\"hello\", 4, 1)", "runtime error: substring start index 4 is greater than end index 1"},
		{"split delimiter empty", "import string\nreturn string.split(\"hello\", \"\")", "runtime error: split delimiter must be a single character string"},
		{"split delimiter too long", "import string\nreturn string.split(\"hello\", \"ll\")", "runtime error: split delimiter must be a single character string"},
		{"parse invalid format", "import date\nreturn date.parse(\"invalid\")", "runtime error: invalid date format for 'parse'"},
		{"new invalid month", "import date\nreturn date.new(2023, 13, 1)", "runtime error: invalid date boundaries for 'new'"},
		{"new invalid leap year", "import date\nreturn date.new(2023, 2, 29)", "runtime error: invalid date boundaries for 'new'"},
		{"new negative year", "import date\nreturn date.new(0 - 1, 1, 1)", "runtime error: invalid date boundaries for 'new'"},
		{"math sqrt negative", "import math\nreturn math.sqrt(0 - 16)", "runtime error: sqrt of negative number is undefined"},
		{"math log non-positive", "import math\nreturn math.log(0, 10)", "runtime error: log of non-positive number is undefined"},
		{"math log base <= 0", "import math\nreturn math.log(100, 0)", "runtime error: log base must be positive and not equal to 1"},
		{"math log base 1", "import math\nreturn math.log(100, 1)", "runtime error: log base must be positive and not equal to 1"},
	}
	runTestErrorScenarios(t, tests)
}

func TestEvaluateLogFunctions(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"log.info output format", "import log\nreturn log.info(\"test message\", 123)", "[Info]: test message: 123"},
		{"log.warn output format", "import log\nreturn log.warn(\"warning msg\", true)", "[Warning]: warning msg: true"},
		{"log.error output format", "import log\nreturn log.error(\"error occurred\", [1, 2])", "[Error]: error occurred: [1, 2]"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			evaluated, err := testEval(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			resultStr, ok := evaluated.(string)
			if !ok {
				t.Fatalf("expected result to be a string, got %T", evaluated)
			}
			if !strings.Contains(resultStr, tc.expected) {
				t.Errorf("expected result to contain '%s', got '%s'", tc.expected, resultStr)
			}
		})
	}
}

func TestEvaluateLogExport(t *testing.T) {
	input := `
	import log
	log.export("First export")
	log.export(42)
	log.export([1, 2, 3])
	return 1
	`
	tknzr := lexer.New(input)
	p := parser.New(tknzr)
	program := p.Parse()

	if p.HasErrors() {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	env := environment.NewEnvironment("", "", false)
	a := analyzer.New(env)
	a.Run(program)
	if a.HasErrors() {
		t.Fatalf("semantic errors: %v", a.Errors())
	}

	_, err := Eval(program, env)
	if err != nil {
		t.Fatalf("evaluation error: %v", err)
	}

	if env.ExportedValues == nil {
		t.Fatalf("expected env.ExportedValues to not be nil")
	}

	if len(*env.ExportedValues) != 3 {
		t.Errorf("expected 3 exported values, got %d", len(*env.ExportedValues))
	}

	val1 := environment.FormatObject((*env.ExportedValues)[0])
	val2 := environment.FormatObject((*env.ExportedValues)[1])
	val3 := (*env.ExportedValues)[2].Inspect() // Array inspects differently than FormatObject

	if val1 != "First export" {
		t.Errorf("expected 'First export', got '%s'", val1)
	}
	if val2 != "42" {
		t.Errorf("expected '42', got '%s'", val2)
	}
	if val3 != "[1, 2, 3]" {
		t.Errorf("expected '[1, 2, 3]', got '%s'", val3)
	}
}

// TestEvaluatePrivateLet verifies that a private let statement evaluates normally within its own module.
func TestEvaluatePrivateLet(t *testing.T) {
	var tests = []testScenario{
		{"Private let works in local scope", "private let a = 10\nreturn a", 10.0},
		{"Private let re-assignment", "private let a = 10\na = 20\nreturn a", 20.0},
	}

	runTestScenarios(t, tests)
}

// TestErrorPrivatePropertyAccess verifies that accessing a private property on a module
// from outside of that module produces a runtime error.
func TestErrorPrivatePropertyAccess(t *testing.T) {
	// Create a module environment with a private property
	moduleEnv := environment.NewEnvironment("test_module", "", false)
	moduleEnv.Set("secret", &environment.Number{Value: 42})
	moduleEnv.MarkPrivate("secret")

	module := &environment.Module{
		Name: "test_module",
		Env:  moduleEnv,
	}

	// Create main environment and inject the module directly
	mainEnv := environment.NewEnvironment("", "", false)
	mainEnv.Set("test_module", module)

	input := "return test_module.secret"

	tknzr := lexer.New(input)
	p := parser.New(tknzr)
	program := p.Parse()

	if p.HasErrors() {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	// Evaluate directly to bypass semantic analysis which would also need the module injected
	_, err := Eval(program, mainEnv)
	if err == nil {
		t.Fatal("expected an error but got none")
	}

	expected := "runtime error: property 'secret' is private and cannot be accessed from outside module 'test_module'"
	if !strings.Contains(err.Error(), expected) {
		t.Errorf("expected error to contain %q, got %q", expected, err.Error())
	}
}

func TestEvaluateTypeAliasUsage(t *testing.T) {
	tests := []testScenario{
		{
			name:     "Type alias with primitive types",
			input:    "type Money Number\ntype Moment Date\ntype Name String\ntype Flag Boolean\ntype Custom Number\nlet process = fn(m: Money, d: Moment, n: Name, f: Flag, c: Custom) -> Money { return m }\nreturn process(100, '2023-01-01', \"John\", true, 42)",
			expected: 100.0,
		},
		{
			name:     "Type alias with array types",
			input:    "type Prices [Number]\ntype Names [String]\ntype Holidays [Date]\ntype Flags [Boolean]\ntype Collection [Number]\nlet addAll = fn(p: Prices, n: Names, h: Holidays, f: Flags, c: Collection) -> Prices { return p }\nreturn addAll([1, 2], [\"a\"], ['2023-01-01'], [true, false], [1, 2])[1]",
			expected: 2.0,
		},
		{
			name:     "Explicitly typed variable with function alias",
			input:    "type CustomFunc fn(Number) -> Number\nlet f: CustomFunc = fn(x: Number) -> Number { return x + 2 }\nreturn f(5)",
			expected: 7.0,
		},
	}
	runTestScenarios(t, tests)
}

func TestStructs(t *testing.T) {
	tests := []testScenario{
		{
			name: "Struct with inline function property",
			input: `
				type Action struct {
					run fn(Number) -> Number
				}
				let a = Action {
					run: fn(x: Number) -> Number { return x * 2 }
				}
				return a.run(5)
			`,
			expected: 10.0,
		},
		{
			name: "Generic struct with turbofish instantiation",
			input: `
				type CustomStruct<T> struct {
					run fn(T) -> T
				}
				let a = CustomStruct::<Number> {
					run: fn(x: Number) -> Number { return x * 3 }
				}
				return a.run(5)
			`,
			expected: 15.0,
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

				return f::<String>("hello")
			`,
			expected: "hello",
		},
		{
			name: "Struct creation and property access",
			input: `
				type User struct {
					name String
					age Number
				}
				let u = User { name: "Bob", age: 30 }
				return u.name
			`,
			expected: "Bob",
		},
		{
			name: "Struct property mutation",
			input: `
				type Point struct {
					x Number
					y Number
				}
				let p = Point { x: 0, y: 0 }
				p.x = 10
				p.y = 20
				return p.x + p.y
			`,
			expected: 30.0,
		},
		{
			name: "Struct passed to and returned from function",
			input: `
				type Point struct {
					x Number
					y Number
				}
				let offset = fn(p: Point) -> Point {
					return Point { x: p.x + 5, y: p.y + 5 }
				}
				let p1 = Point { x: 10, y: 10 }
				let p2 = offset(p1)
				return p2.x
			`,
			expected: 15.0,
		},
		{
			name: "Nested structs",
			input: `
				type Metadata struct {
					id Number
					tag String
				}
				type Item struct {
					meta Metadata
					value Number
				}
				let it = Item {
					meta: Metadata { id: 100, tag: "box" },
					value: 99
				}
				return it.meta.tag
			`,
			expected: "box",
		},
		{
			name: "Deeply nested structs",
			input: `
				type Level5 struct { v Number }
				type Level4 struct { next Level5 }
				type Level3 struct { next Level4 }
				type Level2 struct { next Level3 }
				type Level1 struct { next Level2 }

				let l = Level1 { 
					next: Level2 { 
						next: Level3 { 
							next: Level4 { 
								next: Level5 { v: 42 } 
							} 
						} 
					} 
				}
				return l.next.next.next.next.v
			`,
			expected: 42.0,
		},
	}
	runTestScenarios(t, tests)
}

func TestStructErrors(t *testing.T) {
	tests := []testErrorScenario{
		{
			name:          "Undefined struct",
			input:         `let p = Unknown { x: 1 }`,
			expectedError: "semantic error: undefined struct 'Unknown'",
		},
		{
			name: "Null Pointer Exception on property read",
			input: `
				type Node struct {
					value Number
					next Node?
				}
				let n = Node {
					value: 1,
					next: nil
				}
				return n.next.value
			`,
			expectedError: "semantic errors: [[Line 10, Column 18] semantic error: property access on nullable type requires safe navigation operator '?.' (property: value)]",
		},
		{
			name: "Missing struct field",
			input: `
				type Point struct {
					x Number
					y Number
				}
				let p = Point { x: 1 }
			`,
			expectedError: "semantic error: missing required field 'y' in struct literal",
		},
		{
			name: "Type mismatch in struct literal",
			input: `
				type Point struct {
					x Number
				}
				let p = Point { x: "not-a-number" }
			`,
			expectedError: "type error: field 'x' expects Number, got String",
		},
		{
			name: "Assign to const property",
			input: `
				type Point struct {
					const x Number
				}
				let p = Point { x: 1 }
				p.x = 2
			`,
			expectedError: "semantic error: cannot assign to constant property 'x' on struct 'Point'",
		},
	}
	runTestErrorScenarios(t, tests)
}

func TestNullableTypesEvaluator(t *testing.T) {
	tests := []testScenario{
		{
			name: "Safe navigation on null struct returns null",
			input: `
				type B struct { c Number }
				type A struct { b B? }
				let a = A { b: nil }
				let x = a.b?.c
				return x
			`,
			expected: nil,
		},
		{
			name: "Safe navigation on instantiated struct returns value",
			input: `
				type B struct { c Number }
				type A struct { b B? }
				let a = A { b: B { c: 42 } }
				let x = a.b?.c
				return x
			`,
			expected: 42.0,
		},
		{
			name: "Consecutive safe navigation on null",
			input: `
				type C struct { d Number }
				type B struct { c C? }
				type A struct { b B? }
				let a = A { b: B { c: nil } }
				let x = a.b?.c?.d
				return x
			`,
			expected: nil,
		},
		{
			name: "Consecutive safe navigation on instantiated struct",
			input: `
				type C struct { d Number }
				type B struct { c C? }
				type A struct { b B? }
				let a = A { b: B { c: C { d: 100 } } }
				let x = a.b?.c?.d
				return x
			`,
			expected: 100.0,
		},
		{
			name: "Safe assignment on null struct is ignored",
			input: `
				type B struct { c Number }
				type A struct { b B? }
				let a = A { b: nil }
				a.b?.c = 2
				return a.b
			`,
			expected: nil,
		},
		{
			name: "Safe assignment on instantiated struct applies value",
			input: `
				type B struct { c Number }
				type A struct { b B? }
				let a = A { b: B { c: 1 } }
				a.b?.c = 99
				return a.b?.c
			`,
			expected: 99.0,
		},
	}
	runTestScenarios(t, tests)
}

// TestEvaluateNothingType verifies the runtime evaluation of the built-in Nothing type.
func TestEvaluateNothingType(t *testing.T) {
	var tests = []testScenario{
		{
			name:     "Return Nothing struct at top level",
			input:    "return Nothing {}",
			expected: "Nothing {  }",
		},
		{
			name:     "Return Nothing implicitly at top level",
			input:    "return",
			expected: "Nothing {  }",
		},
	}
	runTestScenarios(t, tests)
}

func TestEvaluateTypeConstraints(t *testing.T) {
	tests := []testScenario{
		{
			name: "Type constraint success",
			input: `
type Customer struct { age Number }
define MajorCustomer constraints Customer with: fn(c: Customer) -> Boolean { return c.age > 18 }
let m: MajorCustomer? = Customer { age: 20 }
return m?.age
`,
			expected: 20.0,
		},
		{
			name: "Type constraint fallback to null",
			input: `
type Customer struct { age Number }
define MajorCustomer constraints Customer with: fn(c: Customer) -> Boolean { return c.age > 18 }
let m: MajorCustomer? = Customer { age: 10 }
return m
`,
			expected: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := testEval(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, res)
			}
		})
	}
}



func TestSafePipeEvaluator(t *testing.T) {
	tests := []testScenario{
		{
			name: "Safe pipe executes correctly when not null",
			input: `
type Wrapper struct { value Number }
let double = fn(w: Wrapper) -> Number { return w.value * 2 }
let m: Wrapper? = Wrapper { value: 5 }
return m ?> double
`,
			expected: 10.0,
		},
		{
			name: "Safe pipe short-circuits to null",
			input: `
type Wrapper struct { value Number }
let double = fn(w: Wrapper) -> Number { return w.value * 2 }
let m: Wrapper? = nil
return m ?> double
`,
			expected: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := testEval(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, res)
			}
		})
	}
}

func TestStreamPipeEvaluator(t *testing.T) {
	tests := []testScenario{
		{
			name: "Stream pipe applies each stage per-item in order",
			input: `
type Sale struct { amount Number }

let calcDiscount = fn(s: Sale, pct: Number) -> Sale {
    return Sale { amount: s.amount - (s.amount * pct / 100) }
}
let calcProfit = fn(s: Sale) -> Number { return s.amount * 0.3 }

let sales = [Sale { amount: 100 }, Sale { amount: 200 }, Sale { amount: 300 }]

let result = sales |>> calcDiscount(5) |>> calcProfit
return result
`,
			expected: "[28.5, 57, 85.5]",
		},
		{
			name: "Safe stream pipe drops nil items before calling the stage function",
			input: `
type Sale struct { amount Number }
define BigSale constraints Sale with: fn(s: Sale) -> Boolean { return s.amount > 150 }

let calcProfit = fn(s: BigSale) -> Number { return s.amount * 0.3 }

let s1: BigSale? = nil
let s2: BigSale? = Sale { amount: 200 }
let s3: BigSale? = nil
let s4: BigSale? = Sale { amount: 300 }

let sales = [s1, s2, s3, s4]
let result = sales ?>> calcProfit
return result
`,
			expected: "[60, 90]",
		},
	}

	runTestScenarios(t, tests)
}

func TestJoinGroupEvaluator(t *testing.T) {
	tests := []testScenario{
		{
			name: "Parallel join results are all passed to the consuming stage",
			input: `
type Loan struct { principal Number }
type Calendar struct { isHoliday Boolean }

let resolveCalendar = fn(loan: Loan) -> Calendar { return Calendar { isHoliday: false } }
let fetchIndexRate = fn(loan: Loan) -> Number { return loan.principal * 0.05 }
let fetchFees = fn(loan: Loan) -> Number { return 10 }
let calculatePnl = fn(cal: Calendar, rate: Number, fees: Number) -> Number { return rate + fees }

let loans = [Loan { principal: 100 }, Loan { principal: 200 }]
let pnl = loans |>> (resolveCalendar & fetchIndexRate & fetchFees) |>> calculatePnl
return pnl
`,
			expected: "[15, 20]",
		},
		{
			name: "Safe stream pipe skips the whole join for nil items",
			input: `
type Loan struct { principal Number }
define BigLoan constraints Loan with: fn(l: Loan) -> Boolean { return l.principal > 150 }

let fetchIndexRate = fn(loan: BigLoan) -> Number { return loan.principal * 0.05 }
let fetchFees = fn(loan: BigLoan) -> Number { return 10 }
let calculatePnl = fn(rate: Number, fees: Number) -> Number { return rate + fees }

let l1: BigLoan? = nil
let l2: BigLoan? = Loan { principal: 200 }
let loans = [l1, l2]
let pnl = loans ?>> (fetchIndexRate & fetchFees) |>> calculatePnl
return pnl
`,
			expected: "[20]",
		},
		{
			name: "Join member curried arguments work alongside the upstream item",
			input: `
type Loan struct { principal Number }

let fetchIndexRate = fn(loan: Loan) -> Number { return loan.principal * 0.05 }
let fetchFees = fn(loan: Loan, flat: Number) -> Number { return flat }
let calculatePnl = fn(rate: Number, fees: Number) -> Number { return rate + fees }

let loans = [Loan { principal: 100 }]
let pnl = loans |>> (fetchIndexRate & fetchFees(7)) |>> calculatePnl
return pnl
`,
			expected: "[12]",
		},
	}

	runTestScenarios(t, tests)
}

// TestEvaluateNamedImports verifies that named imports accurately extract the target values from a module.
func TestEvaluateNamedImports(t *testing.T) {
	var tests = []testScenario{
		{"Named import from math", "import { max } from \"math\"\nreturn max(10, 20)", 20.0},
		{"Named import and alias access", "import { max } from \"math\" as m\nlet a = max(5, 10)\nlet b = m.min(10, 20)\nreturn a + b", 20.0},
		{"Multiple named imports", "import { max, min } from \"math\"\nreturn max(5, 10) + min(15, 20)", 25.0},
	}

	runTestScenarios(t, tests)
}

func TestEvaluateMoveKeyword(t *testing.T) {
	var tests = []testScenario{
		{"Move primitives", "let a = 10\nreturn move a", 10.0},
		{"Move arrays", "let a = [1, 2]\nreturn move a[0]", 1.0},
		{"Move in pipelines", "import \"array\"\nlet a = [1, 2, 3]\nreturn move a |> array.head()", 1.0},
		{"Move with struct", `
			type Point struct { x Number }
			let p = Point { x: 5 }
			return move p.x
		`, 5.0},
	}
	runTestScenarios(t, tests)
}
