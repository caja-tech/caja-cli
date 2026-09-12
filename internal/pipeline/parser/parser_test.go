package parser

import (
	"caja-cli/internal/pipeline/ast"
	"caja-cli/internal/pipeline/lexer"
	"caja-cli/internal/text"
	"strings"
	"testing"
)

type testScenario struct {
	name     string
	input    string
	expected string
}

func TestAnonymousFunctions(t *testing.T) {
	tests := []testScenario{
		{
			name:     "Single parameter expression body",
			input:    "p => p + 1",
			expected: "p(p: ) { ... }",
		},
		{
			name:     "Multiple parameters expression body",
			input:    "(p, q) => p + q",
			expected: "((p: , q: ) { ... }",
		},
		{
			name:     "Explicit types and block body",
			input:    "(p: Number, q: Number) => { return p + q }",
			expected: "((p: Number, q: Number) { ... }",
		},
		{
			name:     "Assignment to typed variable",
			input:    "let sum: fn(Number, Number) -> Number = (p, q) => p + q",
			expected: "let sum: fn(Number, Number) -> Number = ((p: , q: ) { ... }",
		},
	}

	runTestScenarios(t, tests)
}

// TestOperatorPrecedenceParsing verifies that the parser respects arithmetic
// operator precedence, left-to-right associativity, parenthesized grouping, and
// assignment combined with complex math.
func TestOperatorPrecedenceParsing(t *testing.T) {
	tests := []testScenario{
		{
			name:     "Addition and Multiplication",
			input:    "10 + 5 * 2",
			expected: "(10 + (5 * 2))",
		},
		{
			name:     "Left to right evaluation",
			input:    "10 + 5 - 2",
			expected: "((10 + 5) - 2)",
		},
		{
			name:     "Parentheses override precedence",
			input:    "(10 + 5) * 2",
			expected: "((10 + 5) * 2)",
		},
		{
			name:     "Assignments with complex math",
			input:    "rate = (100 / 2) + 15.5",
			expected: "rate = ((100 / 2) + 15.5)",
		},
		{
			name:     "Logical operators precedence over comparison",
			input:    "a == b and c < d",
			expected: "((a == b) and (c < d))",
		},
		{
			name:     "Logical operators left-to-right",
			input:    "true and false or true xor false",
			expected: "(((true and false) or true) xor false)",
		},
	}

	runTestScenarios(t, tests)
}

// TestSimpleExpressions verifies that the parser correctly handles single-token
// inputs and basic single-operator binary expressions without any precedent
// complexity.
func TestSimpleExpressions(t *testing.T) {
	tests := []testScenario{
		{
			name:     "Bare number",
			input:    "42",
			expected: "42",
		},
		{
			name:     "Bare identifier",
			input:    "rate",
			expected: "rate",
		},
		{
			name:     "Bare identifier with numbers",
			input:    "rate2",
			expected: "rate2",
		},
		{
			name:     "Identifier with numbers mixed",
			input:    "a1b2c3",
			expected: "a1b2c3",
		},
		{
			name:     "Identifier with underscore",
			input:    "my_rate_2",
			expected: "my_rate_2",
		},
		{
			name:     "Single binary operation",
			input:    "10 + 5",
			expected: "(10 + 5)",
		},
	}

	runTestScenarios(t, tests)
}

// TestAllOperatorsInIsolation ensures each of the four arithmetic operators
// produces the correct InfixExpression when used alone in a binary expression.
func TestAllOperatorsInIsolation(t *testing.T) {
	tests := []testScenario{
		{
			name:     "Subtraction",
			input:    "10 - 5",
			expected: "(10 - 5)",
		},
		{
			name:     "Multiplication",
			input:    "10 * 5",
			expected: "(10 * 5)",
		},
		{
			name:     "Division",
			input:    "10 / 5",
			expected: "(10 / 5)",
		},
		{
			name:     "Addition",
			input:    "10 + 5",
			expected: "(10 + 5)",
		},
		{
			name:     "Exponentiation",
			input:    "10 ^ 5",
			expected: "(10 ^ 5)",
		},
		{
			name:     "Modulo",
			input:    "10 % 3",
			expected: "(10 % 3)",
		},
	}

	runTestScenarios(t, tests)
}

// TestDeeplyNestedParentheses checks that the parser correctly strips redundant
// parentheses and handles multiply-nested grouped expressions.
func TestDeeplyNestedParentheses(t *testing.T) {
	tests := []testScenario{
		{
			name:     "Double-wrapped expression",
			input:    "((10 + 5))",
			expected: "(10 + 5)",
		},
		{
			name:     "Nested groups with infix",
			input:    "((a + b) * (c - d))",
			expected: "((a + b) * (c - d))",
		},
	}

	runTestScenarios(t, tests)
}

// TestMixedPrecedenceAllOperators verifies that all four operators interact
// correctly when combined in a single expression, respecting both precedence
// and left-to-right associativity.
func TestMixedPrecedenceAllOperators(t *testing.T) {
	tests := []testScenario{
		{
			name:     "All four operators",
			input:    "a + b * c - d / e",
			expected: "((a + (b * c)) - (d / e))",
		},
		{
			name:     "Multiplication and division only",
			input:    "a * b / c * d",
			expected: "(((a * b) / c) * d)",
		},
		{
			name:     "Addition and subtraction only",
			input:    "a + b - c + d",
			expected: "(((a + b) - c) + d)",
		},
		{
			name:     "Exponentiation precedence",
			input:    "a + b * c ^ d",
			expected: "(a + (b * (c ^ d)))",
		},
		{
			name:     "Modulo precedence",
			input:    "a + b % c - d",
			expected: "((a + (b % c)) - d)",
		},
	}

	runTestScenarios(t, tests)
}

// TestDecimalNumberLiterals ensures that floating-point number literals are
// parsed correctly as NumberLiteral nodes with their original literal preserved.
func TestDecimalNumberLiterals(t *testing.T) {
	tests := []testScenario{
		{
			name:     "Bare decimal number",
			input:    "3.14",
			expected: "3.14",
		},
		{
			name:     "Decimal in assignment",
			input:    "rate = 0.5 * 100",
			expected: "rate = (0.5 * 100)",
		},
		{
			name:     "Two decimals in expression",
			input:    "1.5 + 2.5",
			expected: "(1.5 + 2.5)",
		},
	}

	runTestScenarios(t, tests)
}

// TestMultipleStatements verifies that the parser handles multi-line input by
// producing one Statement per line and concatenating their ToString output.
func TestMultipleStatements(t *testing.T) {
	t.Run("Two assignments", func(t *testing.T) {
		input := "a = 10\nb = 20"
		tknzr := lexer.New(input)
		p := New(tknzr)
		program := p.Parse()

		checkParseErrors(t, p)

		if len(program.Statements) != 2 {
			t.Fatalf("expected 2 statements, got %d", len(program.Statements))
		}

		expected1 := "a = 10"
		if result := program.Statements[0].String(); result != expected1 {
			t.Errorf("statement 1: expected %q, got %q", expected1, result)
		}

		expected2 := "b = 20"
		if result := program.Statements[1].String(); result != expected2 {
			t.Errorf("statement 2: expected %q, got %q", expected2, result)
		}
	})
}

// TestAssignmentFollowedByExpression checks that an assignment statement on one
// line followed by a standalone expression on the next line produces two
// separate statements with the correct string representations.
func TestAssignmentFollowedByExpression(t *testing.T) {
	t.Run("Assignment followed by expression", func(t *testing.T) {
		input := "rate = 100\nrate + 50"
		tknzr := lexer.New(input)
		p := New(tknzr)
		program := p.Parse()

		checkParseErrors(t, p)

		if len(program.Statements) != 2 {
			t.Fatalf("expected 2 statements, got %d", len(program.Statements))
		}

		expected1 := "rate = 100"
		if result := program.Statements[0].String(); result != expected1 {
			t.Errorf("statement 1: expected %q, got %q", expected1, result)
		}

		expected2 := "(rate + 50)"
		if result := program.Statements[1].String(); result != expected2 {
			t.Errorf("statement 2: expected %q, got %q", expected2, result)
		}
	})
}

// TestAssignmentOnly verifies that simple assignment statements (with a number
// or identifier on the right side) are parsed correctly without any arithmetic.
func TestAssignmentOnly(t *testing.T) {
	tests := []testScenario{
		{
			name:     "Assign number to variable",
			input:    "rate = 100",
			expected: "rate = 100",
		},
		{
			name:     "Assign identifier to variable",
			input:    "rate = value",
			expected: "rate = value",
		},
		{
			name:     "Assign decimal to variable",
			input:    "tax = 15.5",
			expected: "tax = 15.5",
		},
		{
			name:     "Index assignment",
			input:    "a[0] = 5",
			expected: "a[0] = 5",
		},
	}

	runTestScenarios(t, tests)
}

// TestParseErrors validates that malformed input is detected by the parser and
// that the basic error reporting mechanism works.
func TestParseErrors(t *testing.T) {
	input := "rate = 10 + )\ntax = 5 * 2"
	tknzr := lexer.New(input)
	p := New(tknzr)
	p.Parse()

	errors := p.Errors()

	if len(errors) == 0 {
		t.Fatal("expected parser errors, but got none")
	}
}

func TestPropertyAssignmentStatement(t *testing.T) {
	input := `obj.prop = 42`
	tknzr := lexer.New(input)
	p := New(tknzr)
	program := p.Parse()

	if len(p.Errors()) != 0 {
		t.Fatalf("parser has %d errors: %v", len(p.Errors()), p.Errors())
	}

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.PropertyAssignmentStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not PropertyAssignmentStatement. got=%T", program.Statements[0])
	}

	if stmt.TokenLiteral() != "=" {
		t.Errorf("stmt.TokenLiteral not '='. got=%q", stmt.TokenLiteral())
	}
	if stmt.Property.Value != "prop" {
		t.Errorf("stmt.Property.Value not 'prop'. got=%q", stmt.Property.Value)
	}
}

// TestInvalidAssignmentTargetErrors ensures that assigning to non-identifiers
// or non-index expressions generates the correct syntax error.
func TestInvalidAssignmentTargetErrors(t *testing.T) {
	tests := []string{
		"10 = 5",
		"(10 + 5) = 5",
		"[1, 2] = [3, 4]",
		"(1 + 1) = 2",
		"\"hello\" = 5",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			tknzr := lexer.New(input)
			p := New(tknzr)
			p.Parse()

			errors := p.Errors()
			if len(errors) == 0 {
				t.Fatalf("expected parser errors for invalid assignment target %q, but got none", input)
			}

			found := false
			for _, err := range errors {
				if text.ContainsSubstring(err, "invalid assignment target") {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected error mentioning 'invalid assignment target', got: %v", errors)
			}
		})
	}
}

// TestConsecutiveOperators ensures that two operators appearing back-to-back
// (e.g. "10 + * 5") are flagged as a parse error because the second operator
// has no valid prefix parse function.
func TestConsecutiveOperators(t *testing.T) {
	t.Run("Consecutive operators produce error", func(t *testing.T) {
		input := "10 + * 5"
		tknzr := lexer.New(input)
		p := New(tknzr)
		p.Parse()

		errors := p.Errors()
		if len(errors) == 0 {
			t.Fatal("expected at least 1 error for consecutive operators, got none")
		}
	})
}

// TestStandaloneOperator verifies that an input consisting of a single operator
// with no operands (e.g. "+") produces a parse error, since operators are not
// registered as prefix parse functions.
func TestStandaloneOperator(t *testing.T) {
	t.Run("Standalone operator produces error", func(t *testing.T) {
		input := "+"
		tknzr := lexer.New(input)
		p := New(tknzr)
		p.Parse()

		errors := p.Errors()
		if len(errors) == 0 {
			t.Fatal("expected at least 1 error for standalone operator, got none")
		}
	})
}

// TestUnmatchedLeftParen confirms that an opening parenthesis without a
// corresponding closing parenthesis produces an error mentioning the missing
// RPAREN token.
func TestUnmatchedLeftParen(t *testing.T) {
	t.Run("Unmatched left paren produces RPAREN error", func(t *testing.T) {
		input := "(10 + 5"
		tknzr := lexer.New(input)
		p := New(tknzr)
		p.Parse()

		errors := p.Errors()
		if len(errors) == 0 {
			t.Fatal("expected at least 1 error, got none")
		}

		found := false
		for _, e := range errors {
			if text.ContainsSubstring(e, "RPAREN") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected error mentioning RPAREN, got: %v", errors)
		}
	})
}

// TestUnmatchedRightParen checks that a closing parenthesis appearing where an
// expression is expected produces a parse error referencing the unexpected token.
func TestUnmatchedRightParen(t *testing.T) {
	t.Run("Unmatched right paren produces error", func(t *testing.T) {
		input := "rate = 10 + )"
		tknzr := lexer.New(input)
		p := New(tknzr)
		p.Parse()

		errors := p.Errors()
		if len(errors) == 0 {
			t.Fatal("expected at least 1 error, got none")
		}

		found := false
		for _, e := range errors {
			if text.ContainsSubstring(e, "RPAREN") || text.ContainsSubstring(e, "prefix") || text.ContainsSubstring(e, "unknown") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected error mentioning unrecognized token, got: %v", errors)
		}
	})
}

// TestWhitespaceOnlyCases ensures that input composed entirely of whitespace
// characters (spaces, tabs, newlines) produces a valid empty program with no
// statements and no errors.
func TestWhitespaceOnlyCases(t *testing.T) {
	t.Run("Whitespace-only input produces empty program", func(t *testing.T) {
		input := "   \t\n  "
		tknzr := lexer.New(input)
		p := New(tknzr)
		program := p.Parse()

		checkParseErrors(t, p)

		if len(program.Statements) != 0 {
			t.Errorf("expected 0 statements, got %d", len(program.Statements))
		}
	})
}

// TestEmptyInput verifies that an empty string yields a program with zero
// statements and no parse errors.
func TestEmptyInput(t *testing.T) {
	t.Run("Empty input produces empty program", func(t *testing.T) {
		input := ""
		tknzr := lexer.New(input)
		p := New(tknzr)
		program := p.Parse()

		checkParseErrors(t, p)

		if len(program.Statements) != 0 {
			t.Errorf("expected 0 statements, got %d", len(program.Statements))
		}
	})
}

// TestMultipleErrors checks that when the first line contains a syntax error,
// the parser's synchronize mechanism recovers and successfully parses the valid
// statement on the subsequent line.
func TestMultipleErrors(t *testing.T) {
	t.Run("Multiple errors on separate lines", func(t *testing.T) {
		input := "+ 10\na = 5"
		tknzr := lexer.New(input)
		p := New(tknzr)
		program := p.Parse()

		errors := p.Errors()
		if len(errors) == 0 {
			t.Fatal("expected parser errors, but got none")
		}

		found := false
		for _, stmt := range program.Statements {
			if stmt == nil {
				continue
			}
			if es, ok := stmt.(*ast.ExpressionStatement); ok && es.Expression == nil {
				continue
			}
			if stmt.String() == "a = 5" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected recovered statement 'a = 5' but it was not found")
		}
	})
}

// TestRecoveryAfterError verifies that the parser recovers from an error caused
// by an unmatched right parenthesis on the first line and still correctly parses
// the assignment statement on the second line.
func TestRecoveryAfterError(t *testing.T) {
	t.Run("Recovers after error and parses next statement", func(t *testing.T) {
		input := "rate = 10 + )\ntax = 5 * 2"
		tknzr := lexer.New(input)
		p := New(tknzr)
		program := p.Parse()

		errors := p.Errors()
		if len(errors) == 0 {
			t.Fatal("expected parser errors on first line, but got none")
		}

		if len(program.Statements) < 2 {
			t.Fatalf("expected at least 2 statements (including recovered), got %d", len(program.Statements))
		}

		lastStmt := program.Statements[len(program.Statements)-1]
		expected := "tax = (5 * 2)"
		if result := lastStmt.String(); result != expected {
			t.Errorf("recovered statement: expected %q, got %q", expected, result)
		}
	})
}

// TestIfExpression verifies that if-else expressions are parsed correctly,
// both as standalone expressions and as right-hand values in assignments.
func TestIfExpression(t *testing.T) {
	tests := []testScenario{
		{
			name:     "If expression without else",
			input:    "if (x > y) { x }",
			expected: "if (x > y) x",
		},
		{
			name:     "If expression with else",
			input:    "if (x < y) { x } else { y }",
			expected: "if (x < y) x else y",
		},
		{
			name:     "If else in assignment",
			input:    "result = if (a == b) { 10 } else { 20 }",
			expected: "result = if (a == b) 10 else 20",
		},
	}

	runTestScenarios(t, tests)
}

// TestReturnInsideBlockAllowed verifies that return statements are now
// successfully parsed inside block statements, reflecting the updated rules.
func TestReturnInsideBlockAllowed(t *testing.T) {
	tests := []testScenario{
		{
			name:     "Return inside if block",
			input:    "if (x > y) { return x }",
			expected: "if (x > y) return x",
		},
		{
			name:     "Return inside function block",
			input:    "let f = fn() -> Number { return 10 }",
			expected: "let f = fn() -> Number { ... }",
		},
	}
	runTestScenarios(t, tests)
}

// TestPrefixExpressionParsing verifies parsing of prefix operators.
func TestPrefixExpressionParsing(t *testing.T) {
	tests := []testScenario{
		{
			name:     "Bang prefix",
			input:    "!true",
			expected: "(!true)",
		},
		{
			name:     "Minus prefix",
			input:    "-15",
			expected: "(-15)",
		},
		{
			name:     "Move prefix",
			input:    "move a",
			expected: "(movea)", // Since expected format just concatenates Operator and Right string representations: "move" + "a"
		},
	}
	runTestScenarios(t, tests)
}

// TestLetStatements verifies that let statements with various assignments parse correctly.
func TestLetStatements(t *testing.T) {
	tests := []testScenario{
		{
			name:     "Simple let declaration",
			input:    "let x = 5",
			expected: "let x = 5",
		},
		{
			name:     "Let declaration with mathematical expression",
			input:    "let result = (10 + 5) * 2",
			expected: "let result = ((10 + 5) * 2)",
		},
		{
			name:     "Let declaration with decimal",
			input:    "let rate = 15.5",
			expected: "let rate = 15.5",
		},
		{
			name:     "Let declaration with explicit type",
			input:    "let x: Number = 5",
			expected: "let x: Number = 5",
		},
		{
			name:     "Let declaration with array type and nil",
			input:    "let b: [String] = nil",
			expected: "let b: [String] = nil",
		},
	}

	runTestScenarios(t, tests)
}

func TestConstStatements(t *testing.T) {
	tests := []testScenario{
		{
			name:     "Basic const assignment",
			input:    "const a = 10",
			expected: "const a = 10",
		},
		{
			name:     "Const assignment with expression",
			input:    "const b = a + 5",
			expected: "const b = (a + 5)",
		},
		{
			name:     "Private const assignment",
			input:    "private const secret = 42",
			expected: "private const secret = 42",
		},
		{
			name:     "Const declaration with explicit type",
			input:    "const PI: Number = 3.14",
			expected: "const PI: Number = 3.14",
		},
		{
			name:     "Private const declaration with explicit type",
			input:    "private const text: String = \"hello\"",
			expected: "private const text: String = \"hello\"",
		},
	}
	runTestScenarios(t, tests)
}

func TestConstStatementErrors(t *testing.T) {
	tests := []string{
		"const import \"foo\"",
		"import const \"foo\"",
		"const return 10",
		"return const 10",
		"const",
		"let const = 10",
		"const const = 10",
		"const type MyFunc fn(Number) -> Number",
		"type const MyFunc fn(Number) -> Number",
		"type const fn(Number) -> Number",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			l := lexer.New(input)
			p := New(l)
			p.Parse()
			if !p.HasErrors() {
				t.Fatalf("expected parsing error for input: %q", input)
			}
		})
	}
}

// TestLetStatementErrors validates that malformed let statements are caught by the parser.
func TestLetStatementErrors(t *testing.T) {
	tests := []string{
		"let = 5",    // Missing identifier
		"let 5 = 10", // Identifier is a number
		"let x 10",   // Missing assign operator
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			tknzr := lexer.New(input)
			p := New(tknzr)
			p.Parse()

			errors := p.Errors()
			if len(errors) == 0 {
				t.Fatalf("expected parser errors for input %q, but got none", input)
			}
		})
	}
}

// TestFunctionParsing verifies that function literals with various parameter types and return types parse correctly.
func TestFunctionParsing(t *testing.T) {
	tests := []testScenario{
		{
			name:     "Function with Number types",
			input:    "let add = fn(a: Number, b: Number) -> Number { a + b }",
			expected: "let add = fn(a: Number, b: Number) -> Number { ... }",
		},
		{
			name:     "Function with String and Boolean types",
			input:    "let check = fn(name: String, isValid: Boolean) -> Boolean { isValid }",
			expected: "let check = fn(name: String, isValid: Boolean) -> Boolean { ... }",
		},
		{
			name:     "Function with Date type and Date return type",
			input:    "let log = fn(date: Date) -> Date { date }",
			expected: "let log = fn(date: Date) -> Date { ... }",
		},
		{
			name:     "Function with no parameters",
			input:    "let ping = fn() -> String { \"pong\" }",
			expected: "let ping = fn() -> String { ... }",
		},
		{
			name:     "Function with implicit Nothing return type",
			input:    "let f = fn() { 10 }",
			expected: "let f = fn() -> Nothing { ... }",
		},
		{
			name:     "Function with parameters and implicit Nothing return type",
			input:    "let f = fn(a: Number) { a }",
			expected: "let f = fn(a: Number) -> Nothing { ... }",
		},
		{
			name:     "Function taking an inline function as parameter",
			input:    "let apply = fn(cb: fn(Number) -> String) -> String { cb(10) }",
			expected: "let apply = fn(cb: fn(Number) -> String) -> String { ... }",
		},
	}

	runTestScenarios(t, tests)
}

// TestFunctionErrors verifies that invalid function literals (like missing return type) result in parse errors.
func TestFunctionErrors(t *testing.T) {
	tests := []string{
		"let f = fn(a: Number): { a }", // Missing return type identifier
		"let f = fn<type>(x: type) -> String {return \"a\"}", // Keyword in generic type parameters
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			tknzr := lexer.New(input)
			p := New(tknzr)
			p.Parse()

			errors := p.Errors()
			if len(errors) == 0 {
				t.Fatalf("expected parser errors for input %q, but got none", input)
			}
		})
	}
}

func TestAnonymousFunctionErrors(t *testing.T) {
	tests := []string{
		"let f = (a: ) => a",       // Missing type after colon
		"let f = (type) => 1",      // Keyword as parameter name
		"let f = (a, b => a + b",   // Missing closing paren
		"let f = => 1",             // Missing parameters
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			tknzr := lexer.New(input)
			p := New(tknzr)
			p.Parse()

			errors := p.Errors()
			if len(errors) == 0 {
				t.Fatalf("expected parser errors for input %q, but got none", input)
			}
		})
	}
}

// TestCallExpressionParsing verifies that function calls with different argument types parse correctly.
func TestCallExpressionParsing(t *testing.T) {
	tests := []testScenario{
		{
			name:     "Call with number arguments",
			input:    "add(1, 2)",
			expected: "add(1, 2)",
		},
		{
			name:     "Call with string and boolean",
			input:    "check(\"John\", true)",
			expected: "check(\"John\", true)",
		},
		{
			name:     "Call with complex expressions",
			input:    "add(1 + 2, 3 * 4)",
			expected: "add((1 + 2), (3 * 4))",
		},
		{
			name:     "Call with only named arguments",
			input:    "route(method: \"GET\", path: \"/health\")",
			expected: "route(method: \"GET\", path: \"/health\")",
		},
		{
			name:     "Call with positional then named arguments",
			input:    "route(\"GET\", path: \"/health\")",
			expected: "route(\"GET\", path: \"/health\")",
		},
	}

	runTestScenarios(t, tests)
}

// TestCallExpressionNamedArgumentErrors verifies that a positional argument
// appearing after a named argument is rejected as a syntax error.
func TestCallExpressionNamedArgumentErrors(t *testing.T) {
	tests := []string{
		"route(method: \"GET\", \"/health\")", // positional after named
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			l := lexer.New(input)
			p := New(l)
			p.Parse()

			if len(p.Errors()) == 0 {
				t.Fatalf("expected a syntax error for input %q, got none", input)
			}
		})
	}
}

// TestTypeAliasParsing verifies that type alias statements parse correctly.
func TestTypeAliasParsing(t *testing.T) {
	tests := []testScenario{
		{
			name:     "Type alias with parameters and return type",
			input:    "type BinaryOp fn(Number, Number) -> Number",
			expected: "type BinaryOp fn(Number, Number) -> Number",
		},
		{
			name:     "Type alias with no parameters",
			input:    "type Provider fn() -> String",
			expected: "type Provider fn() -> String",
		},
		{
			name:     "Type alias with Date",
			input:    "type DateFactory fn() -> Date",
			expected: "type DateFactory fn() -> Date",
		},
		{
			name:     "Type alias with simple Number",
			input:    "type Money Number",
			expected: "type Money Number",
		},
		{
			name:     "Type alias with array Number",
			input:    "type Collection [Number]",
			expected: "type Collection [Number]",
		},
		{
			name:     "Type alias function implicit Nothing return type",
			input:    "type Runnable fn()",
			expected: "type Runnable fn() -> Nothing",
		},
		{
			name:     "Type alias function with parameters implicit Nothing return type",
			input:    "type Consumer fn(String)",
			expected: "type Consumer fn(String) -> Nothing",
		},
		{
			name:     "Type alias struct with inline function property",
			input:    "type Logger struct {\n  log fn(String)\n}",
			expected: "type Logger struct {\n  log fn(String) -> Nothing\n}",
		},
		{
			name:     "Generic type alias with type parameters",
			input:    "type CustomFunc<T, R> fn(T) -> R",
			expected: "type CustomFunc<T, R> fn(T) -> R",
		},
		{
			name:     "Using generic type alias with arguments",
			input:    "let f: CustomFunc<Number, String> = nil",
			expected: "let f: CustomFunc<Number, String> = nil",
		},
		{
			name:     "Using generic type alias in an array",
			input:    "let arr: [CustomFunc<Number, String>] = []",
			expected: "let arr: [CustomFunc<Number, String>] = []",
		},
		{
			name:     "Complex nested generic type alias",
			input:    "let complexMap: map[String]CustomFunc<Number, CustomFunc<String, Boolean>> = {}",
			expected: "let complexMap: map[String]CustomFunc<Number, CustomFunc<String, Boolean>> = {}",
		},
	}

	runTestScenarios(t, tests)
}

// TestTypeAliasErrors verifies that malformed type aliases produce syntax errors.
func TestTypeAliasErrors(t *testing.T) {
	tests := []string{
		"type fn(Number) -> Number",                 // Missing alias name
		"type money Number",                         // Lowercase type name
		"type BinaryOp (Number, Number) -> Number",  // Missing fn keyword
		"type BinaryOp fn(Number, Number):",       // Missing return type
		"type CustomFunc<T, R fn(T) -> R",           // Missing closing bracket in alias definition
		"let f: CustomFunc<Number, String = nil",    // Missing closing bracket in variable instantiation
		"let f: CustomFunc<Number String> = nil",    // Missing comma separation
		"type CustomType<fn> fn(fn) -> String",      // Keyword in generic parameters
		"type CustomType<T, type> fn(T, type) -> String", // Keyword in second generic parameter
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			tknzr := lexer.New(input)
			p := New(tknzr)
			p.Parse()

			errors := p.Errors()
			if len(errors) == 0 {
				t.Fatalf("expected parser errors for input %q, but got none", input)
			}
		})
	}
}

// TestStringAndBooleanParsing verifies parsing of string and boolean literals.
func TestStringAndBooleanParsing(t *testing.T) {
	tests := []testScenario{
		{
			name:     "String literal in assignment",
			input:    "let name = \"John Doe\"",
			expected: "let name = \"John Doe\"",
		},
		{
			name:     "Boolean literal true",
			input:    "let isActive = true",
			expected: "let isActive = true",
		},
		{
			name:     "Boolean literal false",
			input:    "let isFailed = false",
			expected: "let isFailed = false",
		},
	}

	runTestScenarios(t, tests)
}

// TestDateParsing verifies parsing of date literals.
func TestDateParsing(t *testing.T) {
	tests := []testScenario{
		{
			name:     "Date literal in assignment",
			input:    "let today = '2023-10-25'",
			expected: "let today = '2023-10-25'",
		},
	}

	runTestScenarios(t, tests)
}

// TestDateParsingErrors verifies that invalid date literals produce parse errors.
func TestDateParsingErrors(t *testing.T) {
	tests := []string{
		"let today = '2023-10-32'", // Invalid day
		"let today = '10-25-2023'", // Invalid format
		"let today = 'not-a-date'", // Invalid string
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			tknzr := lexer.New(input)
			p := New(tknzr)
			p.Parse()

			errors := p.Errors()
			if len(errors) == 0 {
				t.Fatalf("expected parser errors for input %q, but got none", input)
			}
		})
	}
}

// TestArrayParsing verifies parsing of array literals and indexing.
func TestArrayParsing(t *testing.T) {
	tests := []testScenario{
		{
			name:     "Empty array",
			input:    "[]",
			expected: "[]",
		},
		{
			name:     "Array with numbers",
			input:    "[1, 2, 3]",
			expected: "[1, 2, 3]",
		},
		{
			name:     "Array with expressions",
			input:    "[1 + 2, 3 * 4]",
			expected: "[(1 + 2), (3 * 4)]",
		},
		{
			name:     "Array index expression",
			input:    "myArray[1]",
			expected: "(myArray[1])",
		},
		{
			name:     "Array index with complex expression",
			input:    "myArray[1 + 2]",
			expected: "(myArray[(1 + 2)])",
		},
	}

	runTestScenarios(t, tests)
}

// TestArrayParsingErrors verifies that malformed arrays or index expressions produce parse errors.
func TestArrayParsingErrors(t *testing.T) {
	tests := []string{
		"[1, 2",     // Missing closing bracket
		"myArray[1", // Missing closing bracket in index
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			tknzr := lexer.New(input)
			p := New(tknzr)
			p.Parse()

			errors := p.Errors()
			if len(errors) == 0 {
				t.Fatalf("expected parser errors for input %q, but got none", input)
			}
		})
	}
}

// checkParseErrors is a test helper that fails the current test immediately if
// the parser accumulated any errors, logging each error message for debugging.
func checkParseErrors(t *testing.T, p *Parser) {
	errors := p.Errors()
	if len(errors) == 0 {
		return
	}

	t.Errorf("parser has %d errors", len(errors))
	for _, msg := range errors {
		t.Errorf("parser error: %s", msg)
	}
	t.FailNow()
}

func runTestScenarios(t *testing.T, tests []testScenario) {
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tknzr := lexer.New(test.input)
			p := New(tknzr)
			program := p.Parse()

			checkParseErrors(t, p)

			testResult := program.String()
			if testResult != test.expected {
				t.Errorf("expected: %s, got: %s", test.expected, testResult)
			}
		})
	}
}

// TestKeywordVariableErrors validates that using a keyword as a variable or parameter name produces a parse error.
func TestKeywordVariableErrors(t *testing.T) {
	tests := []string{
		"let if = 10",                    // Let declaration with keyword
		"if = 5",                         // Assignment to keyword
		"let f = fn(let: Number) { 10 }", // Function parameter as keyword
		"let as = 10",                    // Let declaration with 'as' keyword
		"let and = 10",                   // Let declaration with 'and' keyword
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			tknzr := lexer.New(input)
			p := New(tknzr)
			p.Parse()

			errors := p.Errors()
			if len(errors) == 0 {
				t.Fatalf("expected parser errors for keyword usage %q, but got none", input)
			}

			// Validate the specific error message is present
			found := false
			for _, err := range errors {
				if text.ContainsSubstring(err, "cannot use keyword") {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected error mentioning 'cannot use keyword', got: %v", errors)
			}
		})
	}
}

func TestImportStatement(t *testing.T) {
	tests := []struct {
		input        string
		expectedName string
		expectedPath string
	}{
		{"import math", "math", "math"},
		{"import \"utils/math\"", "math", "utils/math"},
		{"import \"core/net/http\"", "http", "core/net/http"},
		{"import math as m", "m", "math"},
		{"import \"utils/math\" as m", "m", "utils/math"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tknzr := lexer.New(tt.input)
			p := New(tknzr)
			program := p.Parse()
			if len(p.Errors()) > 0 {
				t.Fatalf("parser errors: %v", p.Errors())
			}
			if len(program.Statements) != 1 {
				t.Fatalf("expected 1 statement, got %d", len(program.Statements))
			}
			stmt, ok := program.Statements[0].(*ast.ImportStatement)
			if !ok {
				t.Fatalf("expected ImportStatement, got %T", program.Statements[0])
			}
			if stmt.Name.Value != tt.expectedName {
				t.Errorf("expected Name %q, got %q", tt.expectedName, stmt.Name.Value)
			}
			if stmt.Path != tt.expectedPath {
				t.Errorf("expected Path %q, got %q", tt.expectedPath, stmt.Path)
			}
		})
	}
}

// TestPrivateModifierParsing verifies that the private access modifier is correctly
// parsed for let statements and type alias statements.
func TestPrivateModifierParsing(t *testing.T) {
	tests := []testScenario{
		{
			name:     "Private let statement",
			input:    "private let rate = 15.5",
			expected: "private let rate = 15.5",
		},
		{
			name:     "Private type alias",
			input:    "private type BinaryOp fn(Number, Number) -> Number",
			expected: "private type BinaryOp fn(Number, Number) -> Number",
		},
	}

	runTestScenarios(t, tests)
}

// TestPrivateModifierErrors verifies that the private access modifier generates
// correct syntax errors when applied to invalid statements like imports or other keywords.
func TestPrivateModifierErrors(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedError string
	}{
		{
			name:          "Private on import",
			input:         "private import \"foo\"",
			expectedError: "syntax error: 'private' modifier must be followed by 'let', 'const', 'type', 'define', or 'union'",
		},
		{
			name:          "Private on return",
			input:         "private return 10",
			expectedError: "syntax error: 'private' modifier must be followed by 'let', 'const', 'type', 'define', or 'union'",
		},
		{
			name:          "Return private",
			input:         "return private",
			expectedError: "unknown prefix type \"PRIVATE\"",
		},
		{
			name:          "Import private",
			input:         "import private",
			expectedError: "expected identifier or string for module name, got PRIVATE",
		},
		{
			name:          "Private as variable declaration",
			input:         "let private = 10",
			expectedError: "syntax error: cannot use keyword 'private' as a variable name",
		},
		{
			name:          "Private standalone",
			input:         "private",
			expectedError: "syntax error: 'private' modifier must be followed by 'let', 'const', 'type', 'define', or 'union'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tknzr := lexer.New(tt.input)
			p := New(tknzr)
			p.Parse()

			errors := p.Errors()
			if len(errors) == 0 {
				t.Fatalf("expected parser errors for %q, but got none", tt.input)
			}

			found := false
			for _, err := range errors {
				if text.ContainsSubstring(err, tt.expectedError) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected error mentioning %q, got: %v", tt.expectedError, errors)
			}
		})
	}
}

// TestTypeAliasUsage verifies that all primitive types and array variations can be mapped to type aliases and used securely in function parameters.
func TestTypeAliasUsage(t *testing.T) {
	tests := []testScenario{
		{
			name:     "Type alias with primitive types used in functions",
			input:    "type Money Number\ntype Moment Date\ntype Name String\ntype Custom Number\nlet process = fn(m: Money, d: Moment, n: Name, c: Custom) -> Money { m }",
			expected: "type Money Numbertype Moment Datetype Name Stringtype Custom Numberlet process = fn(m: Money, d: Moment, n: Name, c: Custom) -> Money { ... }",
		},
		{
			name:     "Type alias with array types used in functions",
			input:    "type Prices [Number]\ntype Names [String]\ntype Holidays [Date]\ntype Collection [Number]\nlet addAll = fn(p: Prices, n: Names, h: Holidays, c: Collection) -> Prices { p }",
			expected: "type Prices [Number]type Names [String]type Holidays [Date]type Collection [Number]let addAll = fn(p: Prices, n: Names, h: Holidays, c: Collection) -> Prices { ... }",
		},
	}

	runTestScenarios(t, tests)
}

func TestNullableTypesAndNavigation(t *testing.T) {
	t.Run("Nullable Struct Properties", func(t *testing.T) {
		input := `
		type Node struct {
			value Number
			next Node?
		}`
		l := lexer.New(input)
		p := New(l)
		program := p.Parse()

		if len(p.Errors()) != 0 {
			t.Fatalf("parser returned errors: %v", p.Errors())
		}

		if len(program.Statements) != 1 {
			t.Fatalf("expected 1 statement, got %d", len(program.Statements))
		}

		typeAlias, ok := program.Statements[0].(*ast.TypeAliasStatement)
		if !ok {
			t.Fatalf("statement is not TypeAliasStatement. got=%T", program.Statements[0])
		}

		structNode := typeAlias.StructDefinition
		if structNode == nil {
			t.Fatalf("target type is not StructDefinition. got=nil")
		}

		var nextType string
		for _, f := range structNode.Fields {
			if f.Name.Value == "next" {
				nextType = f.Type
			}
		}
		if nextType != "Node?" {
			t.Errorf("expected 'next' field type to be Node?, got %s", nextType)
		}
	})

	t.Run("Nullable Function Params and Return Type", func(t *testing.T) {
		input := "let f = fn(node: Node?) -> Node? { return node }"
		l := lexer.New(input)
		p := New(l)
		program := p.Parse()

		if len(p.Errors()) != 0 {
			t.Fatalf("parser returned errors: %v", p.Errors())
		}

		letStmt := program.Statements[0].(*ast.LetStatement)
		fnLit := letStmt.Value.(*ast.FunctionLiteral)
		if fnLit.Parameters[0].Type != "Node?" {
			t.Errorf("expected param type Node?, got %s", fnLit.Parameters[0].Type)
		}
		if fnLit.ReturnType != "Node?" {
			t.Errorf("expected return type Node?, got %s", fnLit.ReturnType)
		}
	})

	t.Run("Consecutive Safe Navigation", func(t *testing.T) {
		input := "user?.address?.city"
		l := lexer.New(input)
		p := New(l)
		program := p.Parse()

		if len(p.Errors()) != 0 {
			t.Fatalf("parser returned errors: %v", p.Errors())
		}

		exprStmt := program.Statements[0].(*ast.ExpressionStatement)
		propExpr := exprStmt.Expression.(*ast.PropertyExpression)
		if !propExpr.Safe {
			t.Errorf("expected Safe=true for .city")
		}
		if propExpr.Property.Value != "city" {
			t.Errorf("expected property 'city', got %s", propExpr.Property.Value)
		}
		innerProp := propExpr.Object.(*ast.PropertyExpression)
		if !innerProp.Safe {
			t.Errorf("expected Safe=true for .address")
		}
	})

	t.Run("Nullable Primitives Forbidden", func(t *testing.T) {
		inputs := []string{
			"type T struct { a Number? }",
			"type T struct { a String? }",
			"type T struct { a Boolean? }",
			"type T struct { a Date? }",
		}
		for _, input := range inputs {
			l := lexer.New(input)
			p := New(l)
			p.Parse()

			if len(p.Errors()) == 0 {
				t.Fatalf("expected parsing error for input '%s', but found none", input)
			}
			if !contains(p.Errors()[0], "cannot be nullable") {
				t.Errorf("expected primitive nullable error, got: %v", p.Errors())
			}
		}
	})
}

// contains is a helper to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || s[0:len(substr)] == substr || s[len(s)-len(substr):] == substr || func() bool {
		for i := 0; i <= len(s)-len(substr); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	}())
}

// TestStructLiteralParsing verifies that struct literals parse correctly, including turbofish type arguments.
func TestStructLiteralParsing(t *testing.T) {
	tests := []testScenario{
		{
			name:     "Struct literal with trailing comma",
			input:    "let u = User { name: \"John\", }",
			expected: "let u = User {name: \"John\"}",
		},
		{
			name:     "Generic struct literal instantiation with turbofish",
			input:    "let u = CustomStruct::<String> { f: fn(x: String) -> String { return x } }",
			expected: "let u = CustomStruct::<String> {f: fn(x: String) -> String { ... }}",
		},
	}
	runTestScenarios(t, tests)
}

// TestStructLiteralErrors verifies that invalid struct literal instantiations are rejected.
func TestStructLiteralErrors(t *testing.T) {
	tests := []string{
		"let u = CustomStruct::<String { f: 1 }", // Missing closing angle bracket in turbofish
		"let u = CustomStruct::<String> f: 1 }",  // Missing curly brace in struct literal
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			tknzr := lexer.New(input)
			p := New(tknzr)
			p.Parse()

			errors := p.Errors()
			if len(errors) == 0 {
				t.Fatalf("expected parser errors for input %q, but got none", input)
			}
		})
	}
}

// TestTrailingBlockCallParsing verifies that a `{ }` block following a call
// expression desugars into an ArrayLiteral appended as the call's final
// argument (Kotlin-style DSL sugar).
func TestTrailingBlockCallParsing(t *testing.T) {
	tests := []testScenario{
		{
			name:     "Trailing block with single call",
			input:    "foo() {\nbar()\n}",
			expected: "foo([bar()])",
		},
		{
			name:     "Trailing block with existing args and multiple statements",
			input:    "foo(1, 2) {\nbar()\nbaz()\n}",
			expected: "foo(1, 2, [bar(), baz()])",
		},
		{
			name:     "Empty trailing block",
			input:    "foo() {\n}",
			expected: "foo([])",
		},
		{
			name:     "Trailing block on turbofish call",
			input:    "f::<Number>(1) {\ng()\n}",
			expected: "f::<Number>(1, [g()])",
		},
		{
			name:     "Trailing block on property call",
			input:    "mod.foo() {\nbar()\n}",
			expected: "(mod.foo)([bar()])",
		},
		{
			name:     "Struct literal argument before trailing block",
			input:    "state(Config { retries: 3 }) {\ntransition()\n}",
			expected: "state(Config {retries: 3}, [transition()])",
		},
		{
			name:     "Nested trailing blocks",
			input:    "workflow(\"Purchase Approval\") {\nstate(\"Pending\") {\ntransition(\"Approved\")\n}\n}",
			expected: "workflow(\"Purchase Approval\", [state(\"Pending\", [transition(\"Approved\")])])",
		},
	}

	runTestScenarios(t, tests)
}

// TestTrailingBlockCallErrors verifies that non-expression statements inside
// a trailing block are rejected, and that the pre-existing struct-literal
// error path for invalid left-hand sides is unaffected.
func TestTrailingBlockCallErrors(t *testing.T) {
	tests := []string{
		"foo() {\nlet x = 1\n}", // let statement inside block
		"foo() {\nreturn 1\n}",  // return statement inside block
		"foo() {\nx = 1\n}",     // assignment inside block
		"(1 + 2) { x: 1 }",      // regression: non-identifier left is still invalid
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			tknzr := lexer.New(input)
			p := New(tknzr)
			p.Parse()

			errors := p.Errors()
			if len(errors) == 0 {
				t.Fatalf("expected parser errors for input %q, but got none", input)
			}
		})
	}
}

// TestTrailingLambdaParsing verifies Kotlin-style trailing-lambda sugar:
// "call(args) paramName => { ... }" appends a bare FunctionLiteral (not an
// ArrayLiteral, unlike TestTrailingBlockCallParsing above) as the call's
// last argument.
func TestTrailingLambdaParsing(t *testing.T) {
	tests := []testScenario{
		{
			name:     "Trailing lambda with block body",
			input:    "route(\"GET\", \"/health\") req => {\nreturn ok(req)\n}",
			expected: "route(\"GET\", \"/health\", req(req: ) { ... })",
		},
		{
			name:     "Trailing lambda with single-expression body",
			input:    "route(\"GET\", \"/health\") req => ok(req)",
			expected: "route(\"GET\", \"/health\", req(req: ) { ... })",
		},
		{
			name:     "Trailing lambda on a call with no existing args",
			input:    "middleware() next => {\nreturn next\n}",
			expected: "middleware(next(next: ) { ... })",
		},
		{
			name:     "Trailing lambda on a property-method call",
			input:    "server.get(\"/x\") req => {\nreturn ok(req)\n}",
			expected: "(server.get)(\"/x\", req(req: ) { ... })",
		},
		{
			name:     "Trailing lambda nested inside a trailing block",
			input:    "registerRoutes(server) {\nroute(\"GET\", \"/health\") req => {\nreturn ok(req)\n}\n}",
			expected: "registerRoutes(server, [route(\"GET\", \"/health\", req(req: ) { ... })])",
		},
		{
			name:     "Trailing lambda as another call's argument",
			input:    "use(route(\"GET\", \"/health\") req => {\nreturn ok(req)\n})",
			expected: "use(route(\"GET\", \"/health\", req(req: ) { ... }))",
		},
		{
			name:     "A call followed by '{' still uses the existing array-building sugar, unaffected",
			input:    "foo() {\nbar()\n}",
			expected: "foo([bar()])",
		},
	}

	runTestScenarios(t, tests)
}

// TestTrailingLambdaErrors verifies that a call followed by an identifier
// with no FAT_ARROW after it is left alone (falls through to the ordinary
// one-statement-per-line rule) rather than silently consuming a token, and
// that a trailing lambda missing its body produces a sensible parse error
// instead of a panic.
func TestTrailingLambdaErrors(t *testing.T) {
	tests := []string{
		"foo() bar",     // IDENT with no FAT_ARROW after a call, same line: falls through to the ordinary "one statement per line" error, not a silent no-op
		"foo() req =>",  // FAT_ARROW with nothing after it (EOF)
		"foo() req => }", // invalid single-expression body (a bare '}' has no prefix parse function)
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			tknzr := lexer.New(input)
			p := New(tknzr)
			p.Parse()

			errors := p.Errors()
			if len(errors) == 0 {
				t.Fatalf("expected parser errors for input %q, but got none", input)
			}
		})
	}
}

// TestStringInterpolationParsing verifies Kotlin-style string interpolation:
// a "${expr}" occurrence inside a string literal splits it into an
// InterpolatedStringLiteral of literal-text/expression segments, while a
// plain string with no "${" anywhere parses exactly as before.
func TestStringInterpolationParsing(t *testing.T) {
	tests := []testScenario{
		{
			name:     "Plain string with no interpolation is unaffected",
			input:    `"hello world"`,
			expected: `"hello world"`,
		},
		{
			name:     "Single interpolated identifier",
			input:    `"${name}"`,
			expected: `"${name}"`,
		},
		{
			name:     "Literal text around and between two interpolations",
			input:    `"${a} ${b}"`,
			expected: `"${a} ${b}"`,
		},
		{
			name:     "Interpolated property access",
			input:    `"${req.method}"`,
			expected: `"${(req.method)}"`,
		},
		{
			name:     "Interpolated call expression",
			input:    `"${len(arr)}"`,
			expected: `"${len(arr)}"`,
		},
		{
			name:     "Interpolated arithmetic expression",
			input:    `"${a + b}"`,
			expected: `"${(a + b)}"`,
		},
		{
			name:     "Interpolated string composes with ordinary '+' concatenation",
			input:    `"a" + "${x}"`,
			expected: `("a" + "${x}")`,
		},
	}

	runTestScenarios(t, tests)
}

// TestStringInterpolationErrors verifies malformed interpolations produce a
// sensible parse error rather than a panic or a silent misparse.
func TestStringInterpolationErrors(t *testing.T) {
	tests := []string{
		`"${1 +"`,  // unterminated: no closing '}' before the string itself ends
		`"${1 2}"`, // trailing content after the embedded expression
		`"${}"`,    // empty interpolation, no expression at all
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			tknzr := lexer.New(input)
			p := New(tknzr)
			p.Parse()

			errors := p.Errors()
			if len(errors) == 0 {
				t.Fatalf("expected parser errors for input %q, but got none", input)
			}
		})
	}
}

// TestStringEscapeParsing verifies escape-sequence decoding (Kotlin's set:
// \t \b \n \r \' \" \\ \$, plus \uXXXX), and its interaction with string
// interpolation: "\$" must not trigger interpolation even immediately
// before "{", and escape-decoding must not disturb interpolation boundary-
// finding for a "${...}" appearing later in the same literal.
func TestStringEscapeParsing(t *testing.T) {
	tests := []testScenario{
		{
			name:     "Newline escape decodes to a real newline",
			input:    `"a\nb"`,
			expected: "\"a\nb\"",
		},
		{
			name:     "Tab escape decodes to a real tab",
			input:    `"a\tb"`,
			expected: "\"a\tb\"",
		},
		{
			name:     "Backspace escape decodes",
			input:    `"a\bb"`,
			expected: "\"a\bb\"",
		},
		{
			name:     "Carriage return escape decodes",
			input:    `"a\rb"`,
			expected: "\"a\rb\"",
		},
		{
			name:     "Escaped double quote decodes to a literal quote",
			input:    `"a\"b"`,
			expected: `"a"b"`,
		},
		{
			name:     "Escaped single quote decodes to a literal quote",
			input:    `"a\'b"`,
			expected: `"a'b"`,
		},
		{
			name:     "Escaped backslash decodes to one literal backslash",
			input:    `"a\\b"`,
			expected: `"a\b"`,
		},
		{
			name:     "Escaped dollar not followed by '{' is just a literal dollar",
			input:    `"cost: \$5"`,
			expected: `"cost: $5"`,
		},
		{
			name:     "Escaped dollar-brace does NOT start an interpolation",
			input:    `"\${not a var}"`,
			expected: `"${not a var}"`,
		},
		{
			name:     "Escaped interpolation alongside a real one in the same string",
			input:    `"${x} \${literal}"`,
			expected: `"${x} ${literal}"`,
		},
		{
			name:     "Unicode escape decodes to the given code point",
			input:    `"I \u2764 Caja"`,
			expected: `"I ❤ Caja"`,
		},
		{
			name:     "Nested string literal inside an interpolation now parses successfully",
			input:    `"${concat(x, "-")}"`,
			expected: `"${concat(x, "-")}"`,
		},
	}

	runTestScenarios(t, tests)
}

// TestStringEscapeErrors verifies an unrecognized escape sequence reports a
// parse error rather than silently passing the backslash through — matching
// Kotlin/Go's own strictness, not the lenient behavior some languages use.
func TestStringEscapeErrors(t *testing.T) {
	tests := []string{
		`"a\zb"`, // 'z' is not a recognized escape target
		`"\u12"`, // \u needs exactly 4 hex digits
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			tknzr := lexer.New(input)
			p := New(tknzr)
			p.Parse()

			errors := p.Errors()
			if len(errors) == 0 {
				t.Fatalf("expected parser errors for input %q, but got none", input)
			}
		})
	}
}

func TestPipeOperatorParsing(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			"A |> f(B)",
			"f(A, B)",
		},
		{
			"A |> f",
			"f(A)",
		},
		{
			"A |> obj.method",
			"(obj.method)(A)",
		},
		{
			"A |> f(B) |> g(C)",
			"g(f(A, B), C)",
		},
		{
			"[1, 2, 3] |> query.where(is_even) |> query.select(times_ten)",
			"(query.select)((query.where)([1, 2, 3], is_even), times_ten)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			lexer := lexer.New(tt.input)
			parser := New(lexer)
			program := parser.Parse()
			if parser.HasErrors() {
				t.Fatalf("parser errors: %v", parser.Errors())
			}

			if len(program.Statements) != 1 {
				t.Fatalf("program.Statements does not contain 1 statements. got=%d", len(program.Statements))
			}

			stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
			if !ok {
				t.Fatalf("program.Statements[0] is not ast.ExpressionStatement. got=%T", program.Statements[0])
			}

			if stmt.Expression.String() != tt.expected {
				t.Errorf("expression string wrong. expected %q, got %q", tt.expected, stmt.Expression.String())
			}
		})
	}
}

// TestMethodCallSugarParsesLikeQualifiedCall documents the invariant the
// UFCS/extension-function analyzer feature depends on: the parser has no
// notion of "module" vs. "value" receivers, so `list.push(4)` and
// `array.push(list, 4)` must produce structurally identical CallExpression
// shapes — a CallExpression whose Function is a PropertyExpression{Object,
// Property}. Disambiguating an arbitrary identifier receiver from a real
// module is deferred entirely to the analyzer's symbol table.
func TestMethodCallSugarParsesLikeQualifiedCall(t *testing.T) {
	input := "list.push(4)"
	l := lexer.New(input)
	p := New(l)
	program := p.Parse()

	if p.HasErrors() {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ExpressionStatement. got=%T", program.Statements[0])
	}

	callExpr, ok := stmt.Expression.(*ast.CallExpression)
	if !ok {
		t.Fatalf("expression is not ast.CallExpression. got=%T", stmt.Expression)
	}

	propExpr, ok := callExpr.Function.(*ast.PropertyExpression)
	if !ok {
		t.Fatalf("CallExpression.Function is not ast.PropertyExpression. got=%T", callExpr.Function)
	}

	objIdent, ok := propExpr.Object.(*ast.Identifier)
	if !ok || objIdent.Value != "list" {
		t.Errorf("expected PropertyExpression.Object to be Identifier 'list', got %#v", propExpr.Object)
	}
	if propExpr.Property.Value != "push" {
		t.Errorf("expected PropertyExpression.Property to be 'push', got %q", propExpr.Property.Value)
	}
	if len(callExpr.Arguments) != 1 {
		t.Fatalf("expected 1 argument, got %d", len(callExpr.Arguments))
	}
	if callExpr.Arguments[0].String() != "4" {
		t.Errorf("expected argument '4', got %q", callExpr.Arguments[0].String())
	}
}

// TestMultipleStatementsOnSameLineError verifies that consecutive statements
// on the same line without a newline produce a syntax error.
func TestMultipleStatementsOnSameLineError(t *testing.T) {
	input := "let area = calculus.PI 10 * 10"
	tknzr := lexer.New(input)
	p := New(tknzr)
	p.Parse()

	errors := p.Errors()
	if len(errors) == 0 {
		t.Fatal("expected parser errors for multiple statements on same line, but got none")
	}

	expectedError := "Expected a newline between statements"
	found := false
	for _, msg := range errors {
		if strings.Contains(msg, expectedError) {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("expected error containing %q, got: %v", expectedError, errors)
	}
}

func TestTypeConstraintParsing(t *testing.T) {
	tests := []testScenario{
		{
			name:     "Type constraint simple",
			input:    "define MajorCustomer constraints Customer with: fn(c: Customer) -> Boolean {\nreturn c.age > 18\n}",
			expected: "define MajorCustomer constraints Customer with: fn(c: Customer) -> Boolean { ... }",
		},
		{
			name:     "Type constraint one-liner",
			input:    "define Even constraints Number with: fn(n: Number) -> Boolean { return n % 2 == 0 }",
			expected: "define Even constraints Number with: fn(n: Number) -> Boolean { ... }",
		},
	}

	runTestScenarios(t, tests)
}

func TestTypeConstraintErrors(t *testing.T) {
	tests := []string{
		"define constraints Customer with: fn() {}",
		"define MajorCustomer Customer with: fn() {}",
		"define MajorCustomer constraints Customer fn() {}",
		"define MajorCustomer constraints Customer with fn() {}",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			tknzr := lexer.New(input)
			p := New(tknzr)
			p.Parse()

			errors := p.Errors()
			if len(errors) == 0 {
				t.Fatalf("expected parser errors for input %q, but got none", input)
			}
		})
	}
}

func TestUnionStatementParsing(t *testing.T) {
	tests := []testScenario{
		{
			name:     "Union with two variants",
			input:    "union Animal = Cat | Dog",
			expected: "union Animal = Cat | Dog",
		},
		{
			name:     "Union with three variants",
			input:    "union Animal = Cat | Dog | Pig",
			expected: "union Animal = Cat | Dog | Pig",
		},
		{
			name:     "Private union",
			input:    "private union Animal = Cat | Dog",
			expected: "private union Animal = Cat | Dog",
		},
	}

	runTestScenarios(t, tests)
}

func TestUnionStatementErrors(t *testing.T) {
	tests := []string{
		"union = Cat | Dog",         // Missing union name
		"union Animal Cat | Dog",    // Missing '='
		"union Animal =",            // Missing first variant
		"union Animal = Cat |",      // Missing variant after '|'
		"union Animal = Cat, Dog",   // Comma instead of pipe
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			tknzr := lexer.New(input)
			p := New(tknzr)
			p.Parse()

			errors := p.Errors()
			if len(errors) == 0 {
				t.Fatalf("expected parser errors for input %q, but got none", input)
			}
		})
	}
}

func TestIsExpressionParsing(t *testing.T) {
	tests := []testScenario{
		{
			name:     "Is expression on identifier",
			input:    "animal is Cat",
			expected: "(animal is Cat)",
		},
		{
			name:     "Is expression narrowed in a let statement",
			input:    "let cat: Cat? = animal is Cat",
			expected: "let cat: Cat? = (animal is Cat)",
		},
		{
			name:     "Is expression with module-qualified type name",
			input:    "animal is animals.Cat",
			expected: "(animal is animals.Cat)",
		},
	}

	runTestScenarios(t, tests)
}

func TestIsExpressionErrors(t *testing.T) {
	tests := []string{
		"animal is",         // Missing type name
		"animal is 123",     // Non-identifier after 'is'
		"animal is animals.", // Missing type name after '.'
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			tknzr := lexer.New(input)
			p := New(tknzr)
			p.Parse()

			errors := p.Errors()
			if len(errors) == 0 {
				t.Fatalf("expected parser errors for input %q, but got none", input)
			}
		})
	}
}

func TestSafePipeOperatorParsing(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			"A ?> f(B)",
			"(A ?> f)",
		},
		{
			"A ?> f",
			"(A ?> f)",
		},
		{
			"A ?> obj.method",
			"(A ?> (obj.method))",
		},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := New(l)
			program := p.Parse()
			checkParseErrors(t, p)

			if len(program.Statements) != 1 {
				t.Fatalf("program.Statements does not contain 1 statements. got=%d", len(program.Statements))
			}

			stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
			if !ok {
				t.Fatalf("program.Statements[0] is not ast.ExpressionStatement. got=%T", program.Statements[0])
			}

			if stmt.Expression.String() != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, stmt.Expression.String())
			}
		})
	}
}

func TestStreamPipeOperatorParsing(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			"A |>> f(B)",
			"(A |>> f)",
		},
		{
			"A |>> f",
			"(A |>> f)",
		},
		{
			"A ?>> f(B)",
			"(A ?>> f)",
		},
		{
			"A |>> f |>> g",
			"((A |>> f) |>> g)",
		},
		{
			"sales |>> calcDiscount(5) |>> calcProfit",
			"((sales |>> calcDiscount) |>> calcProfit)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := New(l)
			program := p.Parse()
			checkParseErrors(t, p)

			if len(program.Statements) != 1 {
				t.Fatalf("program.Statements does not contain 1 statements. got=%d", len(program.Statements))
			}

			stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
			if !ok {
				t.Fatalf("program.Statements[0] is not ast.ExpressionStatement. got=%T", program.Statements[0])
			}

			if stmt.Expression.String() != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, stmt.Expression.String())
			}
		})
	}
}

// TestStreamPipeExpressionStructure verifies that a chained stream pipe
// parses into properly nested *ast.StreamPipeExpression nodes (rather than
// desugaring away like plain |>), with each stage's Call carrying the
// upstream expression prepended as its first argument.
func TestStreamPipeExpressionStructure(t *testing.T) {
	input := "sales |>> calcDiscount(5) |>> calcProfit"
	l := lexer.New(input)
	p := New(l)
	program := p.Parse()
	checkParseErrors(t, p)

	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ExpressionStatement. got=%T", program.Statements[0])
	}

	outer, ok := stmt.Expression.(*ast.StreamPipeExpression)
	if !ok {
		t.Fatalf("expression is not *ast.StreamPipeExpression. got=%T", stmt.Expression)
	}
	if outer.Safe {
		t.Errorf("expected outer stage to not be Safe")
	}
	if len(outer.Call.Arguments) != 1 {
		t.Fatalf("expected outer Call to have 1 argument, got %d", len(outer.Call.Arguments))
	}

	inner, ok := outer.Left.(*ast.StreamPipeExpression)
	if !ok {
		t.Fatalf("outer.Left is not *ast.StreamPipeExpression. got=%T", outer.Left)
	}
	if len(inner.Call.Arguments) != 2 {
		t.Fatalf("expected inner Call to have 2 arguments (upstream + curried arg), got %d", len(inner.Call.Arguments))
	}

	if _, ok := inner.Left.(*ast.Identifier); !ok {
		t.Fatalf("inner.Left is not *ast.Identifier (the source). got=%T", inner.Left)
	}
}

// TestStreamPipeBoundaryMixing verifies that a regular pipe may feed into a
// stream pipe chain (the array a |> chain produces becomes a stream's
// source), but a stream pipe chain may not feed back into a regular pipe.
func TestStreamPipeBoundaryMixing(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
	}{
		{
			name:      "regular pipe feeding into a stream pipe is allowed",
			input:     "sales |> filter(isEligible) |>> calcDiscount(5)",
			wantError: false,
		},
		{
			name:      "stream pipe feeding into a regular pipe is rejected",
			input:     "sales |>> calcDiscount(5) |> sumArr",
			wantError: true,
		},
		{
			name:      "stream pipe feeding into a regular safe pipe is rejected",
			input:     "sales |>> calcDiscount(5) ?> sumArr",
			wantError: true,
		},
		{
			name:      "stream pipe feeding into another stream pipe is allowed",
			input:     "sales |>> calcDiscount(5) |>> calcProfit",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := New(l)
			p.Parse()

			hasErrors := len(p.Errors()) > 0
			if hasErrors != tt.wantError {
				t.Fatalf("wantError=%v, got errors=%v", tt.wantError, p.Errors())
			}
		})
	}
}

// TestJoinGroupParsing verifies that a parallel join group (f1 & f2 & f3)
// used as a |>>/?>> stage fuses with the immediately following stage: the
// join's N calls each receive the upstream item as their own first argument,
// and the consuming stage's Call gets N reserved (nil) leading argument
// slots standing in for the join's results.
func TestJoinGroupParsing(t *testing.T) {
	input := "loans |>> (resolveCalendar & fetchIndexRate & fetchFees) |>> calculatePnl"
	l := lexer.New(input)
	p := New(l)
	program := p.Parse()
	checkParseErrors(t, p)

	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ExpressionStatement. got=%T", program.Statements[0])
	}

	outer, ok := stmt.Expression.(*ast.StreamPipeExpression)
	if !ok {
		t.Fatalf("expression is not *ast.StreamPipeExpression. got=%T", stmt.Expression)
	}

	if outer.Join == nil {
		t.Fatalf("expected outer.Join to be set")
	}
	if outer.Call == nil {
		t.Fatalf("expected outer.Call to be set (fused with the consuming stage)")
	}
	if outer.Call.Function.String() != "calculatePnl" {
		t.Fatalf("expected consuming call to be calculatePnl, got %s", outer.Call.Function.String())
	}
	if len(outer.Call.Arguments) != 3 {
		t.Fatalf("expected 3 reserved argument slots on the consuming call, got %d", len(outer.Call.Arguments))
	}
	for i, arg := range outer.Call.Arguments {
		if arg != nil {
			t.Errorf("expected outer.Call.Arguments[%d] to be a nil placeholder, got %v", i, arg)
		}
	}

	if len(outer.Join.Calls) != 3 {
		t.Fatalf("expected 3 join calls, got %d", len(outer.Join.Calls))
	}
	wantFns := []string{"resolveCalendar", "fetchIndexRate", "fetchFees"}
	for i, call := range outer.Join.Calls {
		if call.Function.String() != wantFns[i] {
			t.Errorf("join call %d: expected function %s, got %s", i, wantFns[i], call.Function.String())
		}
		if len(call.Arguments) != 1 {
			t.Fatalf("join call %d: expected 1 argument (the upstream item), got %d", i, len(call.Arguments))
		}
		if call.Arguments[0].String() != "loans" {
			t.Errorf("join call %d: expected upstream argument 'loans', got %s", i, call.Arguments[0].String())
		}
	}

	if ident, ok := outer.Left.(*ast.Identifier); !ok || ident.Value != "loans" {
		t.Fatalf("expected outer.Left to be the original 'loans' identifier, got %T", outer.Left)
	}
}

// TestJoinGroupWithCurriedArgs verifies join members can carry their own
// curried arguments (e.g. fetchFees(feeSchedule)) alongside the implicit
// upstream item, same as a normal stage.
func TestJoinGroupWithCurriedArgs(t *testing.T) {
	input := "loans |>> (fetchIndexRate & fetchFees(feeSchedule)) |>> calculatePnl(extra)"
	l := lexer.New(input)
	p := New(l)
	program := p.Parse()
	checkParseErrors(t, p)

	stmt := program.Statements[0].(*ast.ExpressionStatement)
	outer := stmt.Expression.(*ast.StreamPipeExpression)

	feesCall := outer.Join.Calls[1]
	if len(feesCall.Arguments) != 2 {
		t.Fatalf("expected fetchFees to have 2 arguments (upstream + curried), got %d", len(feesCall.Arguments))
	}
	if feesCall.Arguments[1].String() != "feeSchedule" {
		t.Errorf("expected curried arg 'feeSchedule', got %s", feesCall.Arguments[1].String())
	}

	if len(outer.Call.Arguments) != 3 {
		t.Fatalf("expected 3 arguments on the consuming call (2 join placeholders + 1 curried), got %d", len(outer.Call.Arguments))
	}
	if outer.Call.Arguments[0] != nil || outer.Call.Arguments[1] != nil {
		t.Fatalf("expected the first 2 arguments to be reserved join placeholders")
	}
	if outer.Call.Arguments[2].String() != "extra" {
		t.Errorf("expected trailing curried arg 'extra', got %s", outer.Call.Arguments[2].String())
	}
}

// TestJoinGroupDanglingParsesWithoutError verifies a join group with no
// following consuming stage parses successfully (Call left nil) — rejecting
// it as "must be followed by another stage" is the analyzer's job, not the
// parser's, since the parser has no way to know a consumer won't be added by
// further input.
func TestJoinGroupDanglingParsesWithoutError(t *testing.T) {
	input := "loans |>> (resolveCalendar & fetchIndexRate)"
	l := lexer.New(input)
	p := New(l)
	program := p.Parse()
	checkParseErrors(t, p)

	stmt := program.Statements[0].(*ast.ExpressionStatement)
	outer, ok := stmt.Expression.(*ast.StreamPipeExpression)
	if !ok {
		t.Fatalf("expression is not *ast.StreamPipeExpression. got=%T", stmt.Expression)
	}
	if outer.Join == nil {
		t.Fatalf("expected outer.Join to be set")
	}
	if outer.Call != nil {
		t.Fatalf("expected outer.Call to be nil (dangling, unconsumed join)")
	}
}

// TestAsyncExpressionParsing verifies `async <expr>` greedily captures a
// full trailing pipe chain as its operand (LOWEST_PRECEDENCE, not
// PREFIX_PRECEDENCE), when used as a let statement's value.
func TestAsyncExpressionParsing(t *testing.T) {
	input := "let p = async loans |> resolveCalendar"
	l := lexer.New(input)
	p := New(l)
	program := p.Parse()
	checkParseErrors(t, p)

	letStmt, ok := program.Statements[0].(*ast.LetStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.LetStatement. got=%T", program.Statements[0])
	}

	asyncExpr, ok := letStmt.Value.(*ast.AsyncExpression)
	if !ok {
		t.Fatalf("letStmt.Value is not *ast.AsyncExpression. got=%T", letStmt.Value)
	}

	call, ok := asyncExpr.Right.(*ast.CallExpression)
	if !ok {
		t.Fatalf("expected async's operand to swallow the whole pipe chain (a CallExpression), got=%T", asyncExpr.Right)
	}
	if call.Function.String() != "resolveCalendar" {
		t.Errorf("expected pipe-desugared call to resolveCalendar, got %s", call.Function.String())
	}
}

// TestUnwrapExpressionParsing verifies `unwrap <expr>` parses correctly as a
// let statement's value — this is the only construct that extracts a value
// out of an async handle (`await` is a pure synchronization barrier and
// never produces one, see TestAwaitStatementParsing).
func TestUnwrapExpressionParsing(t *testing.T) {
	input := "let r = unwrap pipeline"
	l := lexer.New(input)
	p := New(l)
	program := p.Parse()
	checkParseErrors(t, p)

	letStmt, ok := program.Statements[0].(*ast.LetStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.LetStatement. got=%T", program.Statements[0])
	}

	unwrapExpr, ok := letStmt.Value.(*ast.UnwrapExpression)
	if !ok {
		t.Fatalf("letStmt.Value is not *ast.UnwrapExpression. got=%T", letStmt.Value)
	}
	if ident, ok := unwrapExpr.Right.(*ast.Identifier); !ok || ident.Value != "pipeline" {
		t.Fatalf("expected unwrap's operand to be identifier 'pipeline', got %T", unwrapExpr.Right)
	}
}

// TestAwaitStatementParsing verifies the bare, unparenthesized
// WaitGroup-style join barrier `await p1 & p2 & p3` parses as a single
// *ast.AwaitStatement (a statement, never an assignable expression — await
// never produces a value, only unwrap does).
func TestAwaitStatementParsing(t *testing.T) {
	input := "await p1 & p2 & p3"
	l := lexer.New(input)
	p := New(l)
	program := p.Parse()
	checkParseErrors(t, p)

	stmt, ok := program.Statements[0].(*ast.AwaitStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not *ast.AwaitStatement. got=%T", program.Statements[0])
	}

	if len(stmt.Pipelines) != 3 {
		t.Fatalf("expected 3 joined pipelines, got %d", len(stmt.Pipelines))
	}
	wantNames := []string{"p1", "p2", "p3"}
	for i, p := range stmt.Pipelines {
		ident, ok := p.(*ast.Identifier)
		if !ok || ident.Value != wantNames[i] {
			t.Errorf("pipeline %d: expected identifier %s, got %v", i, wantNames[i], p)
		}
	}
}

// TestAwaitStatementSingleOperand verifies a bare `await <expr>` statement
// (no '&') still parses as an *ast.AwaitStatement with exactly one pipeline
// — a valid, if degenerate, single-operand synchronization barrier.
func TestAwaitStatementSingleOperand(t *testing.T) {
	input := "await pipeline"
	l := lexer.New(input)
	p := New(l)
	program := p.Parse()
	checkParseErrors(t, p)

	stmt, ok := program.Statements[0].(*ast.AwaitStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not *ast.AwaitStatement. got=%T", program.Statements[0])
	}
	if len(stmt.Pipelines) != 1 {
		t.Fatalf("expected 1 pipeline, got %d", len(stmt.Pipelines))
	}
	if ident, ok := stmt.Pipelines[0].(*ast.Identifier); !ok || ident.Value != "pipeline" {
		t.Fatalf("expected pipeline operand to be identifier 'pipeline', got %T", stmt.Pipelines[0])
	}
}

func TestNamedImportStatement(t *testing.T) {
	tests := []struct {
		input           string
		expectedName    string
		expectedPath    string
		expectedImports []string
	}{
		{"import { where } from \"@caja/query\"", "query", "@caja/query", []string{"where"}},
		{"import { a, b, c } from \"math\"", "math", "math", []string{"a", "b", "c"}},
		{"import { where } from \"@caja/query\" as q", "q", "@caja/query", []string{"where"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {

			tknzr := lexer.New(tt.input)
			p := New(tknzr)
			program := p.Parse()
			if len(p.Errors()) > 0 {
				t.Fatalf("parser errors: %v", p.Errors())
			}
			if len(program.Statements) != 1 {
				t.Fatalf("expected 1 statement, got %d", len(program.Statements))
			}
			stmt, ok := program.Statements[0].(*ast.ImportStatement)
			if !ok {
				t.Fatalf("expected ImportStatement, got %T", program.Statements[0])
			}
			if stmt.Name.Value != tt.expectedName {
				t.Errorf("expected Name %q, got %q", tt.expectedName, stmt.Name.Value)
			}
			if stmt.Path != tt.expectedPath {
				t.Errorf("expected Path %q, got %q", tt.expectedPath, stmt.Path)
			}
			if len(stmt.NamedImports) != len(tt.expectedImports) {
				t.Fatalf("expected %d named imports, got %d", len(tt.expectedImports), len(stmt.NamedImports))
			}
			for i, exp := range tt.expectedImports {
				if stmt.NamedImports[i].Value != exp {
					t.Errorf("expected named import %q, got %q", exp, stmt.NamedImports[i].Value)
				}
			}
		})
	}
}

func TestNamedImportStatementErrors(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedError string
	}{
		{
			"Missing closing brace",
			"import { a, b from \"math\"",
			"expected identifier in named import, got STRING",
		},
		{
			"Missing from keyword",
			"import { a, b } \"math\"",
			"expected 'from' after named imports",
		},
		{
			"Invalid identifier in block",
			"import { a, 1 } from \"math\"",
			"expected identifier in named import, got NUMBER",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tknzr := lexer.New(tt.input)
			p := New(tknzr)
			p.Parse()
			errors := p.Errors()
			
			if len(errors) == 0 {
				t.Fatalf("expected error '%s', but got none", tt.expectedError)
			}
			
			found := false
			for _, err := range errors {
				if strings.Contains(err, tt.expectedError) {
					found = true
					break
				}
			}
			
			if !found {
				t.Errorf("expected error '%s', but got: %v", tt.expectedError, errors)

			}
		})
	}
}

// TestWildcardImportStatement verifies `import * from mod` parses in every
// module-specifier form the grammar already supports (unquoted identifier,
// quoted string, quoted path, and with an `as` alias), sets IsWildcard, leaves
// NamedImports empty, and round-trips through String().
func TestWildcardImportStatement(t *testing.T) {
	tests := []struct {
		input          string
		expectedName   string
		expectedPath   string
		expectedString string
	}{
		{"import * from array", "array", "array", "import * from array"},
		{"import * from \"array\"", "array", "array", "import * from array"},
		{"import * from \"utils/array\"", "array", "utils/array", "import * from \"utils/array\""},
		// String() always quotes the specifier once an alias is present — a
		// pre-existing quirk shared with named imports, not wildcard-specific.
		{"import * from array as arr", "arr", "array", "import * from \"array\" as arr"},
		{"import * from \"@caja/query\"", "query", "@caja/query", "import * from \"@caja/query\""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tknzr := lexer.New(tt.input)
			p := New(tknzr)
			program := p.Parse()
			if len(p.Errors()) > 0 {
				t.Fatalf("parser errors: %v", p.Errors())
			}
			if len(program.Statements) != 1 {
				t.Fatalf("expected 1 statement, got %d", len(program.Statements))
			}
			stmt, ok := program.Statements[0].(*ast.ImportStatement)
			if !ok {
				t.Fatalf("expected ImportStatement, got %T", program.Statements[0])
			}
			if !stmt.IsWildcard {
				t.Errorf("expected IsWildcard to be true")
			}
			if len(stmt.NamedImports) != 0 {
				t.Errorf("expected no named imports, got %d", len(stmt.NamedImports))
			}
			if stmt.Name.Value != tt.expectedName {
				t.Errorf("expected Name %q, got %q", tt.expectedName, stmt.Name.Value)
			}
			if stmt.Path != tt.expectedPath {
				t.Errorf("expected Path %q, got %q", tt.expectedPath, stmt.Path)
			}
			if stmt.String() != tt.expectedString {
				t.Errorf("expected String() %q, got %q", tt.expectedString, stmt.String())
			}
		})
	}
}

// TestWildcardImportStatementErrors verifies malformed wildcard imports are
// rejected, including the case where '*' and a named-import block are combined
// (they are alternatives, never both).
func TestWildcardImportStatementErrors(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedError string
	}{
		{"Missing from", "import *", "expected 'from' after wildcard import"},
		{"Missing from before specifier", "import * array", "expected 'from' after wildcard import"},
		{"Wildcard combined with named imports", "import * { a } from math", "expected 'from' after wildcard import"},
		{"Missing module specifier", "import * from", "expected identifier or string for module name"},
		// Reserved words lex as their own token type, so they never reach the
		// IDENT branch's IsKeyword guard — the specifier branch rejects them.
		{"Keyword as module name", "import * from let", "expected identifier or string for module name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tknzr := lexer.New(tt.input)
			p := New(tknzr)
			p.Parse()
			errors := p.Errors()

			if len(errors) == 0 {
				t.Fatalf("expected error '%s', but got none", tt.expectedError)
			}

			found := false
			for _, err := range errors {
				if strings.Contains(err, tt.expectedError) {
					found = true
					break
				}
			}

			if !found {
				t.Errorf("expected error '%s', but got: %v", tt.expectedError, errors)
			}
		})
	}
}

// TestMemoModifierParsing verifies that 'memo' is parsed as a prefix modifier
// on a function literal, setting IsMemo, in both let and const bindings.
func TestMemoModifierParsing(t *testing.T) {
	tests := []testScenario{
		{
			name:     "Memo let statement",
			input:    "let powerTwo = memo fn(n: Number) -> Number { return n * n }",
			expected: "let powerTwo = memo fn(n: Number) -> Number { ... }",
		},
		{
			name:     "Memo const statement",
			input:    "const sum = memo fn(list: [Number]) -> Number { return 0 }",
			expected: "const sum = memo fn(list: [Number]) -> Number { ... }",
		},
	}

	runTestScenarios(t, tests)

	// Confirm IsMemo is actually set on the parsed FunctionLiteral node, not
	// just reflected in the rendered String().
	tknzr := lexer.New("let powerTwo = memo fn(n: Number) -> Number { return n * n }")
	p := New(tknzr)
	program := p.Parse()
	checkParseErrors(t, p)

	letStmt, ok := program.Statements[0].(*ast.LetStatement)
	if !ok {
		t.Fatalf("expected *ast.LetStatement, got %T", program.Statements[0])
	}
	fnLit, ok := letStmt.Value.(*ast.FunctionLiteral)
	if !ok {
		t.Fatalf("expected *ast.FunctionLiteral, got %T", letStmt.Value)
	}
	if !fnLit.IsMemo {
		t.Errorf("expected IsMemo to be true")
	}
}

// TestMemoModifierErrors verifies that 'memo' produces a syntax error when
// applied to anything other than a function literal.
func TestMemoModifierErrors(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedError string
	}{
		{
			name:          "Memo on number literal",
			input:         "let x = memo 5",
			expectedError: "syntax error: 'memo' modifier must be applied to a function literal",
		},
		{
			name:          "Memo on identifier",
			input:         "let x = memo y",
			expectedError: "syntax error: 'memo' modifier must be applied to a function literal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tknzr := lexer.New(tt.input)
			p := New(tknzr)
			p.Parse()

			errors := p.Errors()
			if len(errors) == 0 {
				t.Fatalf("expected parser errors for %q, but got none", tt.input)
			}

			found := false
			for _, err := range errors {
				if strings.Contains(err, tt.expectedError) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected error mentioning %q, got: %v", tt.expectedError, errors)
			}
		})
	}
}
