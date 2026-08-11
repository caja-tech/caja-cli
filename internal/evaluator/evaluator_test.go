package evaluator

import (
	"caja-cli/internal/environment"
	"caja-cli/internal/lexer"
	"caja-cli/internal/semantic"
	"caja-cli/internal/syntax"
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
	p := syntax.New(tknzr)
	program := p.Parse()

	if p.HasErrors() {
		return nil, fmt.Errorf("parser errors: %v", p.Errors())
	}

	env := environment.NewEnvironment("", "", false)
	analyzer := semantic.New(env)
	analyzer.Run(program)
	if analyzer.HasErrors() {
		return nil, fmt.Errorf("semantic errors: %v", analyzer.Errors())
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
		return obj.Inspect(), nil
	default:
		return obj, nil
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
		{"Index operator on non-array (caught by semantic)", "let a = 1\nreturn a[0]", "type error: index operator not supported for NUMBER"},
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

func TestEvaluateVariables(t *testing.T) {
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
			input:    "let add = fn(x: Number, y: Number): Number { return x + y }\nreturn add(5, 5)",
			expected: 10.0,
		},
		{
			name:     "Function returning highest number",
			input:    "let max = fn(a: Number, b: Number): Number { if (a > b) { return a } else { return b } }\nreturn max(10, 20)",
			expected: 20.0,
		},
		{
			name:     "Higher order function with type alias",
			input:    "type Op fn(Number, Number): Number\nlet applyOp = fn(a: Number, b: Number, op: Op): Number { return op(a, b) }\nlet add = fn(x: Number, y: Number): Number { return x + y }\nreturn applyOp(10, 20, add)",
			expected: 30.0,
		},
		{
			name: "Recursive function execution (factorial)",
			input: `
let factorial = fn(n: Number): Number {
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
let fib = fn(n: Number): Number {
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
let deepRecurse = fn(n: Number): Number {
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
	}

	runTestErrorScenarios(t, tests)

	// Test stack overflow separately to control the limit
	t.Run("Stack overflow triggers stack tracer", func(t *testing.T) {
		originalLimit := stackTraceLimit
		stackTraceLimit = 10
		defer func() { stackTraceLimit = originalLimit }()

		_, err := testEval("let deepRecurse = fn(n: Number): Number { if (n == 0) { return 0 } else { return 1 + deepRecurse(n - 1) } }\nreturn deepRecurse(20)")
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

// mockNode is a parser.Node implementation unknown to the evaluator,
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
	node := &syntax.InfixExpression{
		Operator: "#",
		Left:     &syntax.NumberLiteral{Value: 10},
		Right:    &syntax.NumberLiteral{Value: 3},
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
	assignNode := &syntax.LetStatement{
		Name:  &syntax.Identifier{Value: "x"},
		Value: &syntax.NumberLiteral{Value: 42},
	}

	env1 := environment.NewEnvironment("", "", false)
	_, err := Eval(assignNode, env1)
	if err != nil {
		t.Fatalf("unexpected error setting variable: %v", err)
	}

	// Second evaluator must not see the variable
	identNode := &syntax.ExpressionStatement{
		Expression: &syntax.Identifier{Value: "x"},
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
			input:    "let greet = fn(): String { return \"hello\" }\nreturn greet()",
			expected: "hello",
		},
		{
			name:     "Return boolean from function",
			input:    "let isTrue = fn(): Boolean { return true }\nreturn isTrue()",
			expected: true,
		},
		{
			name:     "Date literal",
			input:    "return '2023-10-25'",
			expected: "2023-10-25",
		},
		{
			name:     "Return date from function",
			input:    "let getToday = fn(): Date { return '2023-10-25' }\nreturn getToday()",
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
		{"append to array", "import array\nlet arr = [1, 2]\nlet new = array.append(arr, 3)\nreturn new[2]", 3.0},
		{"append does not modify original", "import array\nlet arr = [1, 2]\narray.append(arr, 3)\nreturn array.len(arr)", 2.0},
		{"head of array", "import array\nreturn array.head([5, 6, 7])", 5.0},
		{"tail of array", "import array\nlet t = array.tail([1, 2, 3])\nreturn t[0]", 2.0},
		{"tail of empty array", "import array\nreturn array.len(array.tail([]))", 0.0},
		{"last of array", "import array\nreturn array.last([5, 6, 7])", 7.0},
		{"copy of array", "import array\nlet arr = [1, 2]\nlet c = array.copy(arr)\narray.append(c, 3)\nreturn array.len(arr)", 2.0},
		{"slice of array", "import array\nlet s = array.slice([1, 2, 3, 4], 1, 3)\nreturn s[1]", 3.0},
		{"join of arrays", "import array\nlet j = array.join([1, 2], [3, 4])\nreturn j[2]", 3.0},
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
		{"parseDate", "import date\nreturn date.parseDate(\"2023-10-25\")", "2023-10-25"},
		{"addDays", "import date\nreturn date.addDays('2023-10-25', 5)", "2023-10-30"},
		{"addDays negative", "import date\nreturn date.addDays('2023-10-25', 0 - 5)", "2023-10-20"},
		{"diffDays", "import date\nreturn date.diffDays('2023-10-30', '2023-10-25')", 5.0},
		{"newDate", "import date\nreturn date.newDate(2023, 10, 25)", "2023-10-25"},
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
		{"parseDate invalid format", "import date\nreturn date.parseDate(\"invalid\")", "runtime error: invalid date format for 'parseDate'"},
		{"newDate invalid month", "import date\nreturn date.newDate(2023, 13, 1)", "runtime error: invalid date boundaries for 'newDate'"},
		{"newDate invalid leap year", "import date\nreturn date.newDate(2023, 2, 29)", "runtime error: invalid date boundaries for 'newDate'"},
		{"newDate negative year", "import date\nreturn date.newDate(0 - 1, 1, 1)", "runtime error: invalid date boundaries for 'newDate'"},
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
	p := syntax.New(tknzr)
	program := p.Parse()

	if p.HasErrors() {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	env := environment.NewEnvironment("", "", false)
	analyzer := semantic.New(env)
	analyzer.Run(program)
	if analyzer.HasErrors() {
		t.Fatalf("semantic errors: %v", analyzer.Errors())
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
	p := syntax.New(tknzr)
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

