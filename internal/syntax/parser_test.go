package syntax

import (
	"caja-cli/internal/lexer"
	"caja-cli/internal/text"
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

	stmt, ok := program.Statements[0].(*PropertyAssignmentStatement)
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
			input:    "let f = fn(): Number { return 10 }",
			expected: "let f = fn(): Number { ... }",
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
	}

	runTestScenarios(t, tests)
}

func TestConstStatements(t *testing.T) {
	tests := []testScenario{
		{
			name:  "Basic const assignment",
			input: "const a = 10",
			expected: "const a = 10",
		},
		{
			name:  "Const assignment with expression",
			input: "const b = a + 5",
			expected: "const b = (a + 5)",
		},
		{
			name:  "Private const assignment",
			input: "private const secret = 42",
			expected: "private const secret = 42",
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
		"const type MyFunc fn(Number): Number",
		"type const MyFunc fn(Number): Number",
		"type const fn(Number): Number",
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
			input:    "let add = fn(a: Number, b: Number): Number { a + b }",
			expected: "let add = fn(a: Number, b: Number): Number { ... }",
		},
		{
			name:     "Function with String and Boolean types",
			input:    "let check = fn(name: String, isValid: Boolean): Boolean { isValid }",
			expected: "let check = fn(name: String, isValid: Boolean): Boolean { ... }",
		},
		{
			name:     "Function with Date type and Date return type",
			input:    "let log = fn(date: Date): Date { date }",
			expected: "let log = fn(date: Date): Date { ... }",
		},
		{
			name:     "Function with no parameters",
			input:    "let ping = fn(): String { \"pong\" }",
			expected: "let ping = fn(): String { ... }",
		},
	}

	runTestScenarios(t, tests)
}

// TestFunctionErrors verifies that invalid function literals (like missing return type) result in parse errors.
func TestFunctionErrors(t *testing.T) {
	tests := []string{
		"let f = fn() { 10 }",          // Missing return type completely
		"let f = fn(a: Number) { a }",  // Missing return type with parameters
		"let f = fn(a: Number): { a }", // Missing return type identifier
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
	}

	runTestScenarios(t, tests)
}

// TestTypeAliasParsing verifies that type alias statements parse correctly.
func TestTypeAliasParsing(t *testing.T) {
	tests := []testScenario{
		{
			name:     "Type alias with parameters and return type",
			input:    "type BinaryOp fn(Number, Number): Number",
			expected: "type BinaryOp fn(Number, Number): Number",
		},
		{
			name:     "Type alias with no parameters",
			input:    "type Provider fn(): String",
			expected: "type Provider fn(): String",
		},
		{
			name:     "Type alias with Date",
			input:    "type DateFactory fn(): Date",
			expected: "type DateFactory fn(): Date",
		},
		{
			name:     "Type alias with simple Number",
			input:    "type money Number",
			expected: "type money Number",
		},
		{
			name:     "Type alias with array Any",
			input:    "type collection [Any]",
			expected: "type collection [Any]",
		},
	}

	runTestScenarios(t, tests)
}

// TestTypeAliasErrors verifies that malformed type aliases produce syntax errors.
func TestTypeAliasErrors(t *testing.T) {
	tests := []string{
		"type fn(Number): Number",                 // Missing alias name
		"type BinaryOp (Number, Number): Number",  // Missing fn keyword
		"type BinaryOp fn(Number, Number) Number", // Missing colon
		"type BinaryOp fn(Number, Number):",       // Missing return type
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
			stmt, ok := program.Statements[0].(*ImportStatement)
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
			input:    "private type BinaryOp fn(Number, Number): Number",
			expected: "private type BinaryOp fn(Number, Number): Number",
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
			expectedError: "syntax error: 'private' modifier must be followed by 'let', 'const' or 'type'",
		},
		{
			name:          "Private on return",
			input:         "private return 10",
			expectedError: "syntax error: 'private' modifier must be followed by 'let', 'const' or 'type'",
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
			expectedError: "syntax error: 'private' modifier must be followed by 'let', 'const' or 'type'",
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
			input:    "type money Number\ntype moment Date\ntype name String\ntype custom Any\nlet process = fn(m: money, d: moment, n: name, c: custom): money { m }",
			expected: "type money Numbertype moment Datetype name Stringtype custom Anylet process = fn(m: money, d: moment, n: name, c: custom): money { ... }",
		},
		{
			name:     "Type alias with array types used in functions",
			input:    "type prices [Number]\ntype names [String]\ntype holidays [Date]\ntype collection [Any]\nlet addAll = fn(p: prices, n: names, h: holidays, c: collection): prices { p }",
			expected: "type prices [Number]type names [String]type holidays [Date]type collection [Any]let addAll = fn(p: prices, n: names, h: holidays, c: collection): prices { ... }",
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

		typeAlias, ok := program.Statements[0].(*TypeAliasStatement)
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
		input := "let f = fn(node: Node?): Node? { return node }"
		l := lexer.New(input)
		p := New(l)
		program := p.Parse()
		
		if len(p.Errors()) != 0 {
			t.Fatalf("parser returned errors: %v", p.Errors())
		}

		letStmt := program.Statements[0].(*LetStatement)
		fnLit := letStmt.Value.(*FunctionLiteral)
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

		exprStmt := program.Statements[0].(*ExpressionStatement)
		propExpr := exprStmt.Expression.(*PropertyExpression)
		if !propExpr.Safe {
			t.Errorf("expected Safe=true for .city")
		}
		if propExpr.Property.Value != "city" {
			t.Errorf("expected property 'city', got %s", propExpr.Property.Value)
		}
		innerProp := propExpr.Object.(*PropertyExpression)
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
