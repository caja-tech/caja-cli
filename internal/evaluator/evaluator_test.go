package evaluator

import (
	"caja-cli/internal/parser"
	"caja-cli/internal/tokenizer"
	"fmt"
	"testing"
)

type testScenario struct {
	name     string
	input    string
	expected float64
}

type testErrorScenario struct {
	name          string
	input         string
	expectedError string
}

// testEval is a helper that tokenizes, parses, and evaluates the given input
// string through the full pipeline, returning the final numeric result or the
// first error encountered (from the parser or evaluator).
func testEval(input string) (float64, error) {
	tknzr := tokenizer.New(input)
	p := parser.New(tknzr)
	program := p.Parse()

	if len(p.Errors()) > 0 {
		return 0, fmt.Errorf("parser errors: %v", p.Errors())
	}

	evaluator := New()
	return evaluator.Eval(program)
}

// runTestScenarios iterates over a slice of testScenario entries, evaluating
// each input and asserting that the result matches the expected float64 value.
func runTestScenarios(t *testing.T, tests []testScenario) {
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			evaluated, err := testEval(tc.input)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if evaluated != tc.expected {
				t.Errorf("expected %f, got %f", tc.expected, evaluated)
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

			if err.Error() != tc.expectedError {
				t.Errorf("expected error %q, got %q", tc.expectedError, err.Error())
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
	}

	runTestScenarios(t, tests)
}

// TestEvaluateVariables verifies variable assignment and lookup, including
// simple assign-and-read, arithmetic with multiple variables, self-referencing
// reassignment, and cascading assignments where later variables depend on
// earlier ones.
func TestEvaluateVariables(t *testing.T) {
	var tests = []testScenario{
		{"Assign and return", "rate = 15.5\nreturn rate", 15.5},
		{"Math with variables", "rate = 10\ntax = 5\nreturn rate * tax", 50.0},
		{"Reassignment", "x = 5\nx = x + 5\nreturn x", 10.0},
		{"Cascading variables", "a = 5\nb = a\nc = a + b + 5\nreturn c", 15.0},
	}

	runTestScenarios(t, tests)
}

// TestErrorHandling verifies that the evaluator surfaces the correct error
// messages for common failure cases: referencing an undefined variable and
// dividing by a literal zero.
func TestErrorHandling(t *testing.T) {
	var tests = []testErrorScenario{
		{"Undefined variable", "10 + rate", "identifier not found: rate"},
		{"Division by zero", "10 / 0", "division by zero"},
		{"Modulo by zero", "10 % 0", "modulo by zero"},
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
func (m *mockNode) ToString() string     { return "" }

// TestErrorUnknownOperator verifies that evaluating an InfixExpression with an
// unsupported operator (e.g. "#") returns an "unknown operator" error. The AST
// node is constructed directly to bypass the parser, which would never produce
// such an operator.
func TestErrorUnknownOperator(t *testing.T) {
	evaluator := New()
	node := &parser.InfixExpression{
		Operator: "#",
		Left:     &parser.NumberLiteral{Value: 10},
		Right:    &parser.NumberLiteral{Value: 3},
	}

	_, err := evaluator.Eval(node)
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
	evaluator := New()

	_, err := evaluator.Eval(&mockNode{})
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

	expected := "identifier not found: unknown_var"
	if err.Error() != expected {
		t.Errorf("expected error %q, got %q", expected, err.Error())
	}
}

// TestErrorInRightOperand verifies that an error in the right-hand side of an
// infix expression (an undefined variable) propagates correctly.
func TestErrorInRightOperand(t *testing.T) {
	_, err := testEval("return 5 + missing_var")
	if err == nil {
		t.Fatal("expected an error but got none")
	}

	expected := "identifier not found: missing_var"
	if err.Error() != expected {
		t.Errorf("expected error %q, got %q", expected, err.Error())
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

	expected := "identifier not found: undefined_var"
	if err.Error() != expected {
		t.Errorf("expected error %q, got %q", expected, err.Error())
	}
}

// TestErrorMidProgramHaltsExecution verifies that when a multi-statement
// program encounters an error in an intermediate statement, execution stops
// immediately and subsequent statements are not evaluated.
func TestErrorMidProgramHaltsExecution(t *testing.T) {
	_, err := testEval("x = 5\ny = unknown\nx + 1")
	if err == nil {
		t.Fatal("expected an error but got none")
	}

	expected := "identifier not found: unknown"
	if err.Error() != expected {
		t.Errorf("expected error %q, got %q", expected, err.Error())
	}
}

// TestErrorDivisionByZeroWithVariable verifies that dividing by a variable
// whose value is zero produces a "division by zero" error, testing the runtime
// check rather than a static literal check.
func TestErrorDivisionByZeroWithVariable(t *testing.T) {
	_, err := testEval("x = 0\nreturn 10 / x")
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
	evaluated, err := testEval("x = 10\nreturn (x + 5) * 2")
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
	evaluated, err := testEval("x = 1\nx = 2\nx = 3\nreturn x")
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
	evaluated, err := testEval("x = 10\nx = x * x\nreturn x")
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
	evaluated, err := testEval("a = 1\nb = 2\nc = 3\nreturn a + b + c")
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
	_, err := testEval("x = 42")
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
	evaluated, err := testEval("a = 2\nb = a * 3\nc = b + a\nreturn c")
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
	evaluated, err := testEval("x = 99\nreturn x")
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

// TestNewReturnsNonNil verifies that the New constructor returns a valid,
// non-nil Evaluator instance.
func TestNewReturnsNonNil(t *testing.T) {
	evaluator := New()
	if evaluator == nil {
		t.Fatal("expected New() to return a non-nil evaluator")
	}
}

// TestIndependentEnvironments verifies that two evaluators created with New
// have completely isolated environments: setting a variable in one must not
// make it visible in the other.
func TestIndependentEnvironments(t *testing.T) {
	eval1 := New()
	eval2 := New()

	// Set variable in first evaluator
	assignNode := &parser.AssignStatement{
		Name:  &parser.Identifier{Value: "x"},
		Value: &parser.NumberLiteral{Value: 42},
	}

	_, err := eval1.Eval(assignNode)
	if err != nil {
		t.Fatalf("unexpected error setting variable: %v", err)
	}

	// Second evaluator must not see the variable
	identNode := &parser.ExpressionStatement{
		Expression: &parser.Identifier{Value: "x"},
	}

	_, err = eval2.Eval(identNode)
	if err == nil {
		t.Fatal("expected error from second evaluator but got none")
	}
}

