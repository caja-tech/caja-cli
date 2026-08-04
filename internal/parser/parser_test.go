package parser

import (
	"caja-cli/internal/text"
	"caja-cli/internal/tokenizer"
	"testing"
)

type testScenario struct {
	name     string
	input    string
	expected string
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
		tknzr := tokenizer.New(input)
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
		tknzr := tokenizer.New(input)
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
	}

	runTestScenarios(t, tests)
}

// TestParseErrors validates that malformed input is detected by the parser and
// that the basic error reporting mechanism works.
func TestParseErrors(t *testing.T) {
	input := "rate = 10 + )\ntax = 5 * 2"
	tknzr := tokenizer.New(input)
	p := New(tknzr)
	p.Parse()

	errors := p.Errors()

	if len(errors) == 0 {
		t.Fatal("expected parser errors, but got none")
	}
}

// TestConsecutiveOperators ensures that two operators appearing back-to-back
// (e.g. "10 + * 5") are flagged as a parse error because the second operator
// has no valid prefix parse function.
func TestConsecutiveOperators(t *testing.T) {
	t.Run("Consecutive operators produce error", func(t *testing.T) {
		input := "10 + * 5"
		tknzr := tokenizer.New(input)
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
		tknzr := tokenizer.New(input)
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
		tknzr := tokenizer.New(input)
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
		tknzr := tokenizer.New(input)
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
		tknzr := tokenizer.New(input)
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
		tknzr := tokenizer.New(input)
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
		tknzr := tokenizer.New(input)
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
			if es, ok := stmt.(*ExpressionStatement); ok && es.Expression == nil {
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
		tknzr := tokenizer.New(input)
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

// TestReturnInsideBlockError validates the architectural rule that return
// statements are prohibited inside block statements, ensuring that the parser
// flags it as an error.
func TestReturnInsideBlockError(t *testing.T) {
	input := "if (x > y) { return x }"
	tknzr := tokenizer.New(input)
	p := New(tknzr)
	p.Parse()

	errors := p.Errors()
	if len(errors) == 0 {
		t.Fatal("expected parser error for return inside block statement, but got none")
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
			tknzr := tokenizer.New(test.input)
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
