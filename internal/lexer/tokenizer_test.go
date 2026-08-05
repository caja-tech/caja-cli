package lexer

import "testing"

type testScenario struct {
	name string
	Token
}

// TestBaseScenario verifies that the tokenizer correctly sequences through a
// script with an assignment and a return statement ("rate = 15.5\nreturn rate + 5"),
// producing the right token types, literals, and accurate line/column positions.
func TestBaseScenario(t *testing.T) {
	input := "rate = 15.5\nreturn rate + 5"
	tknzr := New(input)

	tests := []testScenario{
		{"Assign variable 'rate'", Token{IDENT, "rate", 1, 1}},
		{"Assign operator", Token{ASSIGN, "=", 1, 6}},
		{"Number value", Token{NUMBER, "15.5", 1, 8}},
		{"Return keyword", Token{RETURN, "return", 2, 1}},
		{"Identifier", Token{IDENT, "rate", 2, 8}},
		{"Plus operator", Token{PLUS, "+", 2, 13}},
		{"Number value", Token{NUMBER, "5", 2, 15}},
		{"End of file", Token{EOF, "", 2, 16}},
	}

	runTestsOnTokens(tknzr, tests, t)
}

// TestLeadingWhitespace ensures that spaces at the beginning of the input are
// skipped transparently and that column positions reflect the indented offset
// rather than the raw character index from position 0.
func TestLeadingWhitespace(t *testing.T) {
	input := "     return 15.5"
	tknzr := New(input)

	tests := []testScenario{
		{"Return keyword", Token{RETURN, "return", 1, 6}},
		{"Number value", Token{NUMBER, "15.5", 1, 13}},
		{"End of file", Token{EOF, "", 1, 17}},
	}

	runTestsOnTokens(tknzr, tests, t)
}

// TestTrailingWhitespace confirms that trailing spaces after the last meaningful
// token are consumed without producing extra tokens, and that the EOF position
// accounts for the whitespace characters that follow.
func TestTrailingWhitespace(t *testing.T) {
	input := "return 15.5     "
	tknzr := New(input)

	tests := []testScenario{
		{"Return keyword", Token{RETURN, "return", 1, 1}},
		{"Number value", Token{NUMBER, "15.5", 1, 8}},
		{"End of file", Token{EOF, "", 1, 17}},
	}

	runTestsOnTokens(tknzr, tests, t)
}

// TestLeadingNewLine checks that leading newline characters advance the line
// counter correctly so that tokens on the first non-blank line are reported with
// the right line number (4 in this case) and column 1.
func TestLeadingNewLine(t *testing.T) {
	input := "\n\n\nreturn 15.5"
	tknzr := New(input)

	tests := []testScenario{
		{"Return keyword", Token{RETURN, "return", 4, 1}},
		{"Number value", Token{NUMBER, "15.5", 4, 8}},
		{"End of file", Token{EOF, "", 4, 12}},
	}

	runTestsOnTokens(tknzr, tests, t)
}

// TestTrailingNewLine verifies that newlines appended after the last token are
// consumed and reflected in the EOF token's line number, while earlier tokens
// retain their original positions on line 1.
func TestTrailingNewLine(t *testing.T) {
	input := "return 15.5\n\n\n"
	tknzr := New(input)

	tests := []testScenario{
		{"Return keyword", Token{RETURN, "return", 1, 1}},
		{"Number value", Token{NUMBER, "15.5", 1, 8}},
		{"End of file", Token{EOF, "", 4, 1}},
	}

	runTestsOnTokens(tknzr, tests, t)
}

// TestLeadingTab asserts that tab characters at the start of the input advance
// the column counter (each tab counts as one column unit) so that subsequent
// tokens report the correct column offset.
func TestLeadingTab(t *testing.T) {
	input := "\t\t\treturn 15.5"
	tknzr := New(input)

	tests := []testScenario{
		{"Return keyword", Token{RETURN, "return", 1, 4}},
		{"Number value", Token{NUMBER, "15.5", 1, 11}},
		{"End of file", Token{EOF, "", 1, 15}},
	}

	runTestsOnTokens(tknzr, tests, t)
}

// TestTrailingTab confirms that tab characters trailing the last meaningful token
// are consumed without emitting additional tokens, and that the EOF column
// position accounts for each trailing tab character.
func TestTrailingTab(t *testing.T) {
	input := "return 15.5\t\t\t"
	tknzr := New(input)

	tests := []testScenario{
		{"Return keyword", Token{RETURN, "return", 1, 1}},
		{"Number value", Token{NUMBER, "15.5", 1, 8}},
		{"End of file", Token{EOF, "", 1, 15}},
	}

	runTestsOnTokens(tknzr, tests, t)
}

// TestFullArithmeticExpression tokenizes a complete arithmetic expression
// involving nested parentheses and all four operators, asserting that every
// token — including the return keyword, two sets of parentheses, and a numeric
// literal — is emitted with the correct type, literal value, and column.
func TestFullArithmeticExpression(t *testing.T) {
	input := "return (a + b) * (c - d) / 2"
	tknzr := New(input)

	tests := []testScenario{
		{"Return keyword", Token{RETURN, "return", 1, 1}},
		{"Left paren 1", Token{LPAREN, "(", 1, 8}},
		{"Identifier a", Token{IDENT, "a", 1, 9}},
		{"Plus", Token{PLUS, "+", 1, 11}},
		{"Identifier b", Token{IDENT, "b", 1, 13}},
		{"Right paren 1", Token{RPAREN, ")", 1, 14}},
		{"Asterisk", Token{ASTERISK, "*", 1, 16}},
		{"Left paren 2", Token{LPAREN, "(", 1, 18}},
		{"Identifier c", Token{IDENT, "c", 1, 19}},
		{"Minus", Token{MINUS, "-", 1, 21}},
		{"Identifier d", Token{IDENT, "d", 1, 23}},
		{"Right paren 2", Token{RPAREN, ")", 1, 24}},
		{"Slash", Token{SLASH, "/", 1, 26}},
		{"Number 2", Token{NUMBER, "2", 1, 28}},
		{"End of file", Token{EOF, "", 1, 29}},
	}

	runTestsOnTokens(tknzr, tests, t)
}

// TestExponentialExpression tokenizes an arithmetic expression involving
// the power operator, asserting that the caret token is correctly parsed.
func TestExponentialExpression(t *testing.T) {
	input := "rate = 10 ^ 2"
	tknzr := New(input)

	tests := []testScenario{
		{"Identifier rate", Token{IDENT, "rate", 1, 1}},
		{"Assign operator", Token{ASSIGN, "=", 1, 6}},
		{"Number 10", Token{NUMBER, "10", 1, 8}},
		{"Power operator", Token{POWER, "^", 1, 11}},
		{"Number 2", Token{NUMBER, "2", 1, 13}},
		{"End of file", Token{EOF, "", 1, 14}},
	}

	runTestsOnTokens(tknzr, tests, t)
}

// TestModuloExpression tokenizes an arithmetic expression involving
// the modulo operator, asserting that the percent token is correctly parsed.
func TestModuloExpression(t *testing.T) {
	input := "remainder = 10 % 3"
	tknzr := New(input)

	tests := []testScenario{
		{"Identifier remainder", Token{IDENT, "remainder", 1, 1}},
		{"Assign operator", Token{ASSIGN, "=", 1, 11}},
		{"Number 10", Token{NUMBER, "10", 1, 13}},
		{"Modulo operator", Token{MODULO, "%", 1, 16}},
		{"Number 3", Token{NUMBER, "3", 1, 18}},
		{"End of file", Token{EOF, "", 1, 19}},
	}

	runTestsOnTokens(tknzr, tests, t)
}

// TestIfElseExpression tokenizes an if-else expression involving comparison
// operators and block braces, asserting that the new tokens are parsed correctly.
func TestIfElseExpression(t *testing.T) {
	input := "if (a <= 10) {\n a == 5 \n} else {\n a != 5 \n}"
	tknzr := New(input)

	tests := []testScenario{
		{"If keyword", Token{IF, "if", 1, 1}},
		{"Left paren", Token{LPAREN, "(", 1, 4}},
		{"Identifier a", Token{IDENT, "a", 1, 5}},
		{"Less than or equal", Token{LTEQ, "<=", 1, 7}},
		{"Number 10", Token{NUMBER, "10", 1, 10}},
		{"Right paren", Token{RPAREN, ")", 1, 12}},
		{"Left brace", Token{LBRACE, "{", 1, 14}},

		{"Identifier a", Token{IDENT, "a", 2, 2}},
		{"Equal", Token{EQ, "==", 2, 4}},
		{"Number 5", Token{NUMBER, "5", 2, 7}},

		{"Right brace", Token{RBRACE, "}", 3, 1}},
		{"Else keyword", Token{ELSE, "else", 3, 3}},
		{"Left brace", Token{LBRACE, "{", 3, 8}},

		{"Identifier a", Token{IDENT, "a", 4, 2}},
		{"Not equal", Token{NEQ, "!=", 4, 4}},
		{"Number 5", Token{NUMBER, "5", 4, 7}},

		{"Right brace", Token{RBRACE, "}", 5, 1}},
		{"End of file", Token{EOF, "", 5, 2}},
	}

	runTestsOnTokens(tknzr, tests, t)
}

// TestWindowsLineEndings checks that Windows-style CRLF line endings (\r\n)
// are handled correctly: the carriage-return is discarded, the line counter
// increments as expected, and subsequent tokens appear on the right line.
func TestWindowsLineEndings(t *testing.T) {
	input := "rate = 10\r\nreturn rate"
	tknzr := New(input)

	tests := []testScenario{
		{"Identifier rate", Token{IDENT, "rate", 1, 1}},
		{"Assign operator", Token{ASSIGN, "=", 1, 6}},
		{"Number value", Token{NUMBER, "10", 1, 8}},
		{"Return keyword", Token{RETURN, "return", 2, 1}},
		{"Identifier rate", Token{IDENT, "rate", 2, 8}},
		{"End of file", Token{EOF, "", 2, 12}},
	}

	runTestsOnTokens(tknzr, tests, t)
}

func runTestsOnTokens(tknzr *Tokenizer, tests []testScenario, t *testing.T) {
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			token := tknzr.NextToken()

			if token.Type != test.Type {
				t.Errorf("token type wrong. expected=%q, got=%q", test.Type, token.Type)
			}

			if token.Literal != test.Literal {
				t.Errorf("literal wrong. expected=%q, got=%q", test.Literal, token.Literal)
			}

			if token.Line != test.Line {
				t.Errorf("line wrong for %q. expected=%d, got=%d", token.Literal, test.Line, token.Line)
			}

			if token.Column != test.Column {
				t.Errorf("column wrong for %q. expected=%d, got=%d", token.Literal, test.Column, token.Column)
			}
		})
	}
}
