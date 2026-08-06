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

// TestLetStatementBasic tests that the 'let' keyword is properly tokenized.
func TestLetStatementBasic(t *testing.T) {
	input := "let rate = 15.5"
	tknzr := New(input)

	tests := []testScenario{
		{"Let keyword", Token{LET, "let", 1, 1}},
		{"Identifier rate", Token{IDENT, "rate", 1, 5}},
		{"Assign operator", Token{ASSIGN, "=", 1, 10}},
		{"Number 15.5", Token{NUMBER, "15.5", 1, 12}},
		{"End of file", Token{EOF, "", 1, 16}},
	}

	runTestsOnTokens(tknzr, tests, t)
}

// TestLetStatementWithExpressions tests that 'let' works correctly with a mathematical expression.
func TestLetStatementWithExpressions(t *testing.T) {
	input := "let total = rate * 2"
	tknzr := New(input)

	tests := []testScenario{
		{"Let keyword", Token{LET, "let", 1, 1}},
		{"Identifier total", Token{IDENT, "total", 1, 5}},
		{"Assign operator", Token{ASSIGN, "=", 1, 11}},
		{"Identifier rate", Token{IDENT, "rate", 1, 13}},
		{"Asterisk", Token{ASTERISK, "*", 1, 18}},
		{"Number 2", Token{NUMBER, "2", 1, 20}},
		{"End of file", Token{EOF, "", 1, 21}},
	}

	runTestsOnTokens(tknzr, tests, t)
}

// TestMultipleLetStatements tests multiple let declarations separated by a newline.
func TestMultipleLetStatements(t *testing.T) {
	input := "let a = 5\nlet b = 10"
	tknzr := New(input)

	tests := []testScenario{
		{"Let keyword", Token{LET, "let", 1, 1}},
		{"Identifier a", Token{IDENT, "a", 1, 5}},
		{"Assign operator", Token{ASSIGN, "=", 1, 7}},
		{"Number 5", Token{NUMBER, "5", 1, 9}},
		{"Let keyword", Token{LET, "let", 2, 1}},
		{"Identifier b", Token{IDENT, "b", 2, 5}},
		{"Assign operator", Token{ASSIGN, "=", 2, 7}},
		{"Number 10", Token{NUMBER, "10", 2, 9}},
		{"End of file", Token{EOF, "", 2, 11}},
	}

	runTestsOnTokens(tknzr, tests, t)
}

// TestFunctionTokenization tests the tokenization of a function declaration with parameters and a return type.
func TestFunctionTokenization(t *testing.T) {
	input := "fn add(a: Number, b: Number): Number {\n return a + b\n}"
	tknzr := New(input)

	tests := []testScenario{
		{"Function keyword", Token{FN, "fn", 1, 1}},
		{"Identifier add", Token{IDENT, "add", 1, 4}},
		{"Left paren", Token{LPAREN, "(", 1, 7}},
		{"Identifier a", Token{IDENT, "a", 1, 8}},
		{"Colon", Token{COLON, ":", 1, 9}},
		{"Type Number", Token{IDENT, "Number", 1, 11}},
		{"Comma", Token{COMMA, ",", 1, 17}},
		{"Identifier b", Token{IDENT, "b", 1, 19}},
		{"Colon", Token{COLON, ":", 1, 20}},
		{"Type Number", Token{IDENT, "Number", 1, 22}},
		{"Right paren", Token{RPAREN, ")", 1, 28}},
		{"Colon", Token{COLON, ":", 1, 29}},
		{"Type Number", Token{IDENT, "Number", 1, 31}},
		{"Left brace", Token{LBRACE, "{", 1, 38}},
		{"Return keyword", Token{RETURN, "return", 2, 2}},
		{"Identifier a", Token{IDENT, "a", 2, 9}},
		{"Plus", Token{PLUS, "+", 2, 11}},
		{"Identifier b", Token{IDENT, "b", 2, 13}},
		{"Right brace", Token{RBRACE, "}", 3, 1}},
		{"End of file", Token{EOF, "", 3, 2}},
	}

	runTestsOnTokens(tknzr, tests, t)
}

// TestStringTokenization tests the tokenization of string literals.
func TestStringTokenization(t *testing.T) {
	input := "let name = \"John Doe\""
	tknzr := New(input)

	tests := []testScenario{
		{"Let keyword", Token{LET, "let", 1, 1}},
		{"Identifier name", Token{IDENT, "name", 1, 5}},
		{"Assign operator", Token{ASSIGN, "=", 1, 10}},
		{"String value", Token{STRING, "John Doe", 1, 12}},
		{"End of file", Token{EOF, "", 1, 22}},
	}

	runTestsOnTokens(tknzr, tests, t)
}

// TestBooleanTokenization tests the tokenization of boolean literals.
func TestBooleanTokenization(t *testing.T) {
	input := "let isDone = true\nlet isFailed = false"
	tknzr := New(input)

	tests := []testScenario{
		{"Let keyword", Token{LET, "let", 1, 1}},
		{"Identifier isDone", Token{IDENT, "isDone", 1, 5}},
		{"Assign operator", Token{ASSIGN, "=", 1, 12}},
		{"True keyword", Token{TRUE, "true", 1, 14}},
		{"Let keyword", Token{LET, "let", 2, 1}},
		{"Identifier isFailed", Token{IDENT, "isFailed", 2, 5}},
		{"Assign operator", Token{ASSIGN, "=", 2, 14}},
		{"False keyword", Token{FALSE, "false", 2, 16}},
		{"End of file", Token{EOF, "", 2, 21}},
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
