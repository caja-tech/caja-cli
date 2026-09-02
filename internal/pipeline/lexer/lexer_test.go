package lexer

import (
	"testing"
)

type testScenario struct {
	name string
	Token
}

// TestPrefixOperators ensures that prefix operators are correctly identified.
func TestPrefixOperators(t *testing.T) {
	input := "!-5"
	tknzr := New(input)

	tests := []testScenario{
		{"Bang operator", Token{BANG, "!", 1, 1}},
		{"Minus operator", Token{MINUS, "-", 1, 2}},
		{"Number value", Token{NUMBER, "5", 1, 3}},
		{"End of file", Token{EOF, "", 1, 4}},
	}

	runTestsOnTokens(tknzr, tests, t)
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

func TestArrowToken(t *testing.T) {
	input := "->"
	tknzr := New(input)
	
	tests := []testScenario{
		{"Arrow token", Token{ARROW, "->", 1, 1}},
		{"End of file", Token{EOF, "", 1, 3}},
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

// TestDateTokenization tests the tokenization of date literals.
func TestDateTokenization(t *testing.T) {
	input := "let today = '2023-10-25'"
	tknzr := New(input)

	tests := []testScenario{
		{"Let keyword", Token{LET, "let", 1, 1}},
		{"Identifier today", Token{IDENT, "today", 1, 5}},
		{"Assign operator", Token{ASSIGN, "=", 1, 11}},
		{"Date value", Token{DATE, "2023-10-25", 1, 13}},
		{"End of file", Token{EOF, "", 1, 25}},
	}

	runTestsOnTokens(tknzr, tests, t)
}

func TestTypeAliasTokenization(t *testing.T) {
	input := "type Add = fn(Number, Number): Number"
	tknzr := New(input)

	tests := []testScenario{
		{"Type keyword", Token{TYPE, "type", 1, 1}},
		{"Identifier Add", Token{IDENT, "Add", 1, 6}},
		{"Assign operator", Token{ASSIGN, "=", 1, 10}},
		{"Fn keyword", Token{FN, "fn", 1, 12}},
		{"Left paren", Token{LPAREN, "(", 1, 14}},
		{"Type Number", Token{IDENT, "Number", 1, 15}},
		{"Comma", Token{COMMA, ",", 1, 21}},
		{"Type Number", Token{IDENT, "Number", 1, 23}},
		{"Right paren", Token{RPAREN, ")", 1, 29}},
		{"Colon", Token{COLON, ":", 1, 30}},
		{"Return Type Number", Token{IDENT, "Number", 1, 32}},
		{"End of file", Token{EOF, "", 1, 38}},
	}

	runTestsOnTokens(tknzr, tests, t)
}

// TestArrayTokenization tests the tokenization of array literals and indexing.
func TestArrayTokenization(t *testing.T) {
	input := "let arr = [1, 2]\narr[0]"
	tknzr := New(input)

	tests := []testScenario{
		{"Let keyword", Token{LET, "let", 1, 1}},
		{"Identifier arr", Token{IDENT, "arr", 1, 5}},
		{"Assign operator", Token{ASSIGN, "=", 1, 9}},
		{"Left bracket", Token{LBRACKET, "[", 1, 11}},
		{"Number 1", Token{NUMBER, "1", 1, 12}},
		{"Comma", Token{COMMA, ",", 1, 13}},
		{"Number 2", Token{NUMBER, "2", 1, 15}},
		{"Right bracket", Token{RBRACKET, "]", 1, 16}},
		{"Identifier arr", Token{IDENT, "arr", 2, 1}},
		{"Left bracket", Token{LBRACKET, "[", 2, 4}},
		{"Number 0", Token{NUMBER, "0", 2, 5}},
		{"Right bracket", Token{RBRACKET, "]", 2, 6}},
		{"End of file", Token{EOF, "", 2, 7}},
	}

	runTestsOnTokens(tknzr, tests, t)
}

// TestImportTokenization tests the tokenization of the import keyword.
func TestImportTokenization(t *testing.T) {
	input := "import \"module_name\""
	tknzr := New(input)

	tests := []testScenario{
		{"Import keyword", Token{IMPORT, "import", 1, 1}},
		{"String module", Token{STRING, "module_name", 1, 8}},
		{"End of file", Token{EOF, "", 1, 21}},
	}

	runTestsOnTokens(tknzr, tests, t)
}

// TestAliasTokenization tests the tokenization of the as keyword.
func TestAliasTokenization(t *testing.T) {
	input := "import \"module_name\" as m"
	tknzr := New(input)

	tests := []testScenario{
		{"Import keyword", Token{IMPORT, "import", 1, 1}},
		{"String module", Token{STRING, "module_name", 1, 8}},
		{"As keyword", Token{AS, "as", 1, 22}},
		{"Identifier alias", Token{IDENT, "m", 1, 25}},
		{"End of file", Token{EOF, "", 1, 26}},
	}

	runTestsOnTokens(tknzr, tests, t)
}

// TestDotOperatorTokenization tests the tokenization of the dot operator for property access.
func TestDotOperatorTokenization(t *testing.T) {
	input := "module.Property"
	tknzr := New(input)

	tests := []testScenario{
		{"Identifier module", Token{IDENT, "module", 1, 1}},
		{"Dot operator", Token{DOT, ".", 1, 7}},
		{"Identifier Property", Token{IDENT, "Property", 1, 8}},
		{"End of file", Token{EOF, "", 1, 16}},
	}

	runTestsOnTokens(tknzr, tests, t)
}

// TestPackageUsageScenario tests a full package usage scenario with import and dot operators.
func TestPackageUsageScenario(t *testing.T) {
	input := "import math\nlet result = math.add(5, 10)\nreturn result"
	tknzr := New(input)

	tests := []testScenario{
		{"Import keyword", Token{IMPORT, "import", 1, 1}},
		{"Identifier math", Token{IDENT, "math", 1, 8}},
		{"Let keyword", Token{LET, "let", 2, 1}},
		{"Identifier result", Token{IDENT, "result", 2, 5}},
		{"Assign operator", Token{ASSIGN, "=", 2, 12}},
		{"Identifier math", Token{IDENT, "math", 2, 14}},
		{"Dot operator", Token{DOT, ".", 2, 18}},
		{"Identifier add", Token{IDENT, "add", 2, 19}},
		{"Left paren", Token{LPAREN, "(", 2, 22}},
		{"Number 5", Token{NUMBER, "5", 2, 23}},
		{"Comma", Token{COMMA, ",", 2, 24}},
		{"Number 10", Token{NUMBER, "10", 2, 26}},
		{"Right paren", Token{RPAREN, ")", 2, 28}},
		{"Return keyword", Token{RETURN, "return", 3, 1}},
		{"Identifier result", Token{IDENT, "result", 3, 8}},
		{"End of file", Token{EOF, "", 3, 14}},
	}

	runTestsOnTokens(tknzr, tests, t)
}

func runTestsOnTokens(tknzr *Lexer, tests []testScenario, t *testing.T) {
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

// TestIntegerNumber uses the public Lex API to verify that a plain integer
// literal ("42") is tokenized as a NUMBER token with the correct literal and
// position, followed by an EOF token at the expected column.
func TestIntegerNumber(t *testing.T) {
	input := "42"
	tokens, errors := Lex(input)

	if len(errors) != 0 {
		t.Fatalf("expected no errors, got %d: %v", len(errors), errors)
	}
	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens (NUMBER + EOF), got %d", len(tokens))
	}

	num := tokens[0]
	if num.Type != NUMBER {
		t.Errorf("expected NUMBER, got %q", num.Type)
	}
	if num.Literal != "42" {
		t.Errorf("expected literal %q, got %q", "42", num.Literal)
	}
	if num.Line != 1 || num.Column != 1 {
		t.Errorf("expected line=1 col=1, got line=%d col=%d", num.Line, num.Column)
	}

	eof := tokens[1]
	if eof.Type != EOF {
		t.Errorf("expected EOF, got %q", eof.Type)
	}
	if eof.Line != 1 || eof.Column != 3 {
		t.Errorf("EOF: expected line=1 col=3, got line=%d col=%d", eof.Line, eof.Column)
	}
}

// TestLeadingDecimalNumber ensures that a leading decimal point (".5") is
// treated as a DOT token followed by a NUMBER token.
func TestLeadingDecimalNumber(t *testing.T) {
	input := ".5"
	tokens, errors := Lex(input)

	if len(errors) != 0 {
		t.Fatalf("expected 0 errors for leading dot, got %d", len(errors))
	}

	if len(tokens) != 3 {
		t.Fatalf("expected 3 tokens (DOT + NUMBER + EOF), got %d", len(tokens))
	}

	if tokens[0].Type != DOT || tokens[0].Literal != "." || tokens[0].Column != 1 {
		t.Errorf("expected DOT '.', got %q %q col=%d", tokens[0].Type, tokens[0].Literal, tokens[0].Column)
	}
	if tokens[1].Type != NUMBER || tokens[1].Literal != "5" || tokens[1].Column != 2 {
		t.Errorf("expected NUMBER '5' at col 2, got %q %q col=%d", tokens[1].Type, tokens[1].Literal, tokens[1].Column)
	}
	if tokens[2].Type != EOF || tokens[2].Literal != "" || tokens[2].Column != 3 {
		t.Errorf("expected EOF '' at col 3, got %q %q col=%d", tokens[2].Type, tokens[2].Literal, tokens[2].Column)
	}
}

// TestUnderscoreIdentifier verifies that identifiers containing an underscore
// (e.g. "my_rate") are tokenized as a single IDENT token with the full literal
// preserved, and that the EOF position immediately follows the last character.
func TestUnderscoreIdentifier(t *testing.T) {
	input := "my_rate"
	tokens, errors := Lex(input)

	if len(errors) != 0 {
		t.Fatalf("expected no errors, got %d: %v", len(errors), errors)
	}
	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens (IDENT + EOF), got %d", len(tokens))
	}

	ident := tokens[0]
	if ident.Type != IDENT {
		t.Errorf("expected IDENT, got %q", ident.Type)
	}
	if ident.Literal != "my_rate" {
		t.Errorf("expected literal %q, got %q", "my_rate", ident.Literal)
	}
	if ident.Line != 1 || ident.Column != 1 {
		t.Errorf("expected line=1 col=1, got line=%d col=%d", ident.Line, ident.Column)
	}

	eof := tokens[1]
	if eof.Type != EOF {
		t.Errorf("expected EOF, got %q", eof.Type)
	}
	if eof.Line != 1 || eof.Column != 8 {
		t.Errorf("EOF: expected line=1 col=8, got line=%d col=%d", eof.Line, eof.Column)
	}
}

// TestUppercaseIdentifier confirms that identifiers starting with an uppercase
// letter (e.g. "Rate") are recognized as IDENT tokens and that the literal is
// preserved with its original casing intact.
func TestUppercaseIdentifier(t *testing.T) {
	input := "Rate"
	tokens, errors := Lex(input)

	if len(errors) != 0 {
		t.Fatalf("expected no errors, got %d: %v", len(errors), errors)
	}
	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens (IDENT + EOF), got %d", len(tokens))
	}

	ident := tokens[0]
	if ident.Type != IDENT {
		t.Errorf("expected IDENT, got %q", ident.Type)
	}
	if ident.Literal != "Rate" {
		t.Errorf("expected literal %q, got %q", "Rate", ident.Literal)
	}
	if ident.Line != 1 || ident.Column != 1 {
		t.Errorf("expected line=1 col=1, got line=%d col=%d", ident.Line, ident.Column)
	}

	eof := tokens[1]
	if eof.Type != EOF {
		t.Errorf("expected EOF, got %q", eof.Type)
	}
	if eof.Line != 1 || eof.Column != 5 {
		t.Errorf("EOF: expected line=1 col=5, got line=%d col=%d", eof.Line, eof.Column)
	}
}

// TestEmptyInput verifies that an empty string produces exactly one EOF token
// with no errors, positioned at line 1, column 1.
func TestEmptyInput(t *testing.T) {
	input := ""
	tokens, errors := Lex(input)

	if len(errors) != 0 {
		t.Fatalf("expected no errors, got %d: %v", len(errors), errors)
	}
	if len(tokens) != 1 {
		t.Fatalf("expected exactly 1 token (EOF), got %d", len(tokens))
	}

	eof := tokens[0]
	if eof.Type != EOF {
		t.Errorf("expected EOF, got %q", eof.Type)
	}
	if eof.Line != 1 || eof.Column != 1 {
		t.Errorf("EOF: expected line=1 col=1, got line=%d col=%d", eof.Line, eof.Column)
	}
}

// TestWhitespaceOnlyInput checks that input composed entirely of space characters
// produces a single EOF token with no errors, and that the EOF column reflects
// the total number of whitespace characters consumed.
func TestWhitespaceOnlyInput(t *testing.T) {
	t.Run("Spaces only", func(t *testing.T) {
		input := "   "
		tokens, errors := Lex(input)

		if len(errors) != 0 {
			t.Fatalf("expected no errors, got %d: %v", len(errors), errors)
		}
		if len(tokens) != 1 {
			t.Fatalf("expected exactly 1 token (EOF), got %d", len(tokens))
		}

		eof := tokens[0]
		if eof.Type != EOF {
			t.Errorf("expected EOF, got %q", eof.Type)
		}

		if eof.Line != 1 || eof.Column != 4 {
			t.Errorf("EOF: expected line=1 col=4, got line=%d col=%d", eof.Line, eof.Column)
		}
	})
}

// TestNewLinesOnlyInput ensures that input composed solely of newline characters
// yields a single EOF token with no errors, with the EOF's line counter
// incremented for each newline consumed.
func TestNewLinesOnlyInput(t *testing.T) {
	t.Run("Newlines only", func(t *testing.T) {
		input := "\n\r"
		tokens, errors := Lex(input)

		if len(errors) != 0 {
			t.Fatalf("expected no errors, got %d: %v", len(errors), errors)
		}
		if len(tokens) != 1 {
			t.Fatalf("expected exactly 1 token (EOF), got %d", len(tokens))
		}

		eof := tokens[0]
		if eof.Type != EOF {
			t.Errorf("expected EOF, got %q", eof.Type)
		}

		if eof.Line != 3 || eof.Column != 1 {
			t.Errorf("EOF: expected line=3 col=1, got line=%d col=%d", eof.Line, eof.Column)
		}
	})
}

// TestConsecutiveIllegalCharacters verifies that each unrecognized character in
// a sequence ("@@") produces its own ILLEGAL token and a corresponding syntax
// error message with the correct column, followed by an EOF token.
func TestConsecutiveIllegalCharacters(t *testing.T) {
	input := "@@"
	tokens, errors := Lex(input)

	if len(errors) != 2 {
		t.Fatalf("expected 2 errors, got %d", len(errors))
	}

	expectedErr1 := "Syntax Error at line 1, column 1: unrecognized character '@'"
	if errors[0] != expectedErr1 {
		t.Errorf("error 1: expected %q, got %q", expectedErr1, errors[0])
	}

	expectedErr2 := "Syntax Error at line 1, column 2: unrecognized character '@'"
	if errors[1] != expectedErr2 {
		t.Errorf("error 2: expected %q, got %q", expectedErr2, errors[1])
	}

	if len(tokens) != 3 {
		t.Fatalf("expected 3 tokens (ILLEGAL + ILLEGAL + EOF), got %d", len(tokens))
	}
	if tokens[0].Type != ILLEGAL || tokens[0].Column != 1 {
		t.Errorf("token 0: expected ILLEGAL at col 1, got %q col=%d", tokens[0].Type, tokens[0].Column)
	}
	if tokens[1].Type != ILLEGAL || tokens[1].Column != 2 {
		t.Errorf("token 1: expected ILLEGAL at col 2, got %q col=%d", tokens[1].Type, tokens[1].Column)
	}
	if tokens[2].Type != EOF || tokens[2].Column != 3 {
		t.Errorf("token 2: expected EOF, got %q col=%d", tokens[2].Type, tokens[2].Column)
	}
}

// TestInvalidCharacters ensures that multiple illegal characters scattered
// throughout valid input ("10 @ 5 $") each generate a distinct syntax error
// message referencing the correct line and column of the offending character.
func TestInvalidCharacters(t *testing.T) {
	input := `10 @ 5 $`
	_, errors := Lex(input)

	if len(errors) != 2 {
		t.Fatalf("expected 2 lexer errors, got %d", len(errors))
	}

	expectedErr1 := "Syntax Error at line 1, column 4: unrecognized character '@'"
	if errors[0] != expectedErr1 {
		t.Errorf("expected error %q, got %q", expectedErr1, errors[0])
	}

	expectedErr2 := "Syntax Error at line 1, column 8: unrecognized character '$'"
	if errors[1] != expectedErr2 {
		t.Errorf("expected error %q, got %q", expectedErr2, errors[1])
	}
}

// TestSingleLineComments verifies that comments starting with '#' are ignored by the tokenizer.
func TestSingleLineComments(t *testing.T) {
	input := `
# This is a comment at the top
let a = 10 # This is a comment at the end of a line
# Another full line comment
return a
`
	tknzr := New(input)

	tests := []testScenario{
		{"Let keyword", Token{LET, "let", 3, 1}},
		{"Identifier 'a'", Token{IDENT, "a", 3, 5}},
		{"Assign operator", Token{ASSIGN, "=", 3, 7}},
		{"Number value", Token{NUMBER, "10", 3, 9}},
		{"Return keyword", Token{RETURN, "return", 5, 1}},
		{"Identifier 'a'", Token{IDENT, "a", 5, 8}},
		{"End of file", Token{EOF, "", 6, 1}},
	}

	runTestsOnTokens(tknzr, tests, t)
}

// TestBooleanOperatorKeywords verifies that 'and', 'or', and 'xor' are tokenized as keywords.
func TestBooleanOperatorKeywords(t *testing.T) {
	input := `true and false or true xor false`
	tknzr := New(input)

	tests := []testScenario{
		{"Boolean true", Token{TRUE, "true", 1, 1}},
		{"AND operator", Token{AND, "and", 1, 6}},
		{"Boolean false", Token{FALSE, "false", 1, 10}},
		{"OR operator", Token{OR, "or", 1, 16}},
		{"Boolean true", Token{TRUE, "true", 1, 19}},
		{"XOR operator", Token{XOR, "xor", 1, 24}},
		{"Boolean false", Token{FALSE, "false", 1, 28}},
		{"End of file", Token{EOF, "", 1, 33}},
	}

	runTestsOnTokens(tknzr, tests, t)
}

func TestPrivateVariableTokenization(t *testing.T) {
	input := "private let rate = 15.5"
	tknzr := New(input)

	tests := []testScenario{
		{"Private keyword", Token{PRIVATE, "private", 1, 1}},
		{"Let keyword", Token{LET, "let", 1, 9}},
		{"Identifier rate", Token{IDENT, "rate", 1, 13}},
		{"Assign operator", Token{ASSIGN, "=", 1, 18}},
		{"Number 15.5", Token{NUMBER, "15.5", 1, 20}},
		{"End of file", Token{EOF, "", 1, 24}},
	}

	runTestsOnTokens(tknzr, tests, t)
}

func TestPrivateTypeTokenization(t *testing.T) {
	input := "private type Alias = Number"
	tknzr := New(input)

	tests := []testScenario{
		{"Private keyword", Token{PRIVATE, "private", 1, 1}},
		{"Type keyword", Token{TYPE, "type", 1, 9}},
		{"Identifier Alias", Token{IDENT, "Alias", 1, 14}},
		{"Assign operator", Token{ASSIGN, "=", 1, 20}},
		{"Type Number", Token{IDENT, "Number", 1, 22}},
		{"End of file", Token{EOF, "", 1, 28}},
	}

	runTestsOnTokens(tknzr, tests, t)
}

// TestStructTokenization verifies the tokenization of the struct definition and struct literal block
func TestStructTokenization(t *testing.T) {
	input := "type User struct {\n const name String\n}\nlet u = User { name: \"Bob\" }"
	tknzr := New(input)

	tests := []testScenario{
		{"Type keyword", Token{TYPE, "type", 1, 1}},
		{"Identifier User", Token{IDENT, "User", 1, 6}},
		{"Struct keyword", Token{STRUCT, "struct", 1, 11}},
		{"Left brace", Token{LBRACE, "{", 1, 18}},

		{"Const keyword", Token{CONST, "const", 2, 2}},
		{"Identifier name", Token{IDENT, "name", 2, 8}},
		{"Type String", Token{IDENT, "String", 2, 13}},

		{"Right brace", Token{RBRACE, "}", 3, 1}},

		{"Let keyword", Token{LET, "let", 4, 1}},
		{"Identifier u", Token{IDENT, "u", 4, 5}},
		{"Assign operator", Token{ASSIGN, "=", 4, 7}},
		{"Identifier User", Token{IDENT, "User", 4, 9}},
		{"Left brace", Token{LBRACE, "{", 4, 14}},
		{"Identifier name", Token{IDENT, "name", 4, 16}},
		{"Colon", Token{COLON, ":", 4, 20}},
		{"String value", Token{STRING, "Bob", 4, 22}},
		{"Right brace", Token{RBRACE, "}", 4, 28}},
		{"End of file", Token{EOF, "", 4, 29}},
	}

	runTestsOnTokens(tknzr, tests, t)
}

// TestQuestionTokens verifies that ? and ?. are correctly tokenized, including edge cases.
func TestQuestionTokens(t *testing.T) {
	input := "Node? node?.next user?.address?.city [String]? Node ?"
	tknzr := New(input)

	tests := []testScenario{
		// Baseline
		{"Identifier Node", Token{IDENT, "Node", 1, 1}},
		{"Question mark", Token{QUESTION, "?", 1, 5}},
		{"Identifier node", Token{IDENT, "node", 1, 7}},
		{"Question dot", Token{QUESTIONDOT, "?.", 1, 11}},
		{"Identifier next", Token{IDENT, "next", 1, 13}},

		// Consecutive safe navigation
		{"Identifier user", Token{IDENT, "user", 1, 18}},
		{"Question dot", Token{QUESTIONDOT, "?.", 1, 22}},
		{"Identifier address", Token{IDENT, "address", 1, 24}},
		{"Question dot", Token{QUESTIONDOT, "?.", 1, 31}},
		{"Identifier city", Token{IDENT, "city", 1, 33}},

		// Nullable array type
		{"Left Bracket", Token{LBRACKET, "[", 1, 38}},
		{"Identifier String", Token{IDENT, "String", 1, 39}},
		{"Right Bracket", Token{RBRACKET, "]", 1, 45}},
		{"Question mark", Token{QUESTION, "?", 1, 46}},

		// Spacing edge case
		{"Identifier Node", Token{IDENT, "Node", 1, 48}},
		{"Question mark", Token{QUESTION, "?", 1, 53}},

		{"End of file", Token{EOF, "", 1, 54}},
	}

	runTestsOnTokens(tknzr, tests, t)
}

// TestDoubleColonTokenization ensures that the turbofish generic syntax '::' is correctly identified.
func TestDoubleColonTokenization(t *testing.T) {
	input := "f::<Number>()"
	tknzr := New(input)

	tests := []testScenario{
		{"Identifier f", Token{IDENT, "f", 1, 1}},
		{"Double colon", Token{DOUBLE_COLON, "::", 1, 2}},
		{"Left angle bracket (less than)", Token{LT, "<", 1, 4}},
		{"Identifier Number", Token{IDENT, "Number", 1, 5}},
		{"Right angle bracket (greater than)", Token{GT, ">", 1, 11}},
		{"Left paren", Token{LPAREN, "(", 1, 12}},
		{"Right paren", Token{RPAREN, ")", 1, 13}},
		{"End of file", Token{EOF, "", 1, 14}},
	}

	runTestsOnTokens(tknzr, tests, t)
}

func TestPipeToken(t *testing.T) {
	input := "|>"
	tknzr := New(input)

	tests := []testScenario{
		{"Pipe operator", Token{PIPE, "|>", 1, 1}},
		{"End of file", Token{EOF, "", 1, 3}},
	}

	runTestsOnTokens(tknzr, tests, t)
}

// TestTypeConstraintTokenization verifies the tokenization of type constraints.
func TestTypeConstraintTokenization(t *testing.T) {
	input := `define MajorCustomer constraints Customer with: fn(c: Customer) -> Boolean {
		return c.age > 18
	}`

	tknzr := New(input)

	tests := []testScenario{
		{"define", Token{DEFINE, "define", 1, 1}},
		{"ident", Token{IDENT, "MajorCustomer", 1, 8}},
		{"constraints", Token{CONSTRAINTS, "constraints", 1, 22}},
		{"ident", Token{IDENT, "Customer", 1, 34}},
		{"with", Token{WITH, "with", 1, 43}},
		{"colon", Token{COLON, ":", 1, 47}},
		{"fn", Token{FN, "fn", 1, 49}},
		{"lparen", Token{LPAREN, "(", 1, 51}},
		{"ident", Token{IDENT, "c", 1, 52}},
		{"colon", Token{COLON, ":", 1, 53}},
		{"ident", Token{IDENT, "Customer", 1, 55}},
		{"rparen", Token{RPAREN, ")", 1, 63}},
		{"arrow", Token{ARROW, "->", 1, 65}},
		{"ident", Token{IDENT, "Boolean", 1, 68}},
		{"lbrace", Token{LBRACE, "{", 1, 76}},
		{"return", Token{RETURN, "return", 2, 3}},
		{"ident", Token{IDENT, "c", 2, 10}},
		{"dot", Token{DOT, ".", 2, 11}},
		{"ident", Token{IDENT, "age", 2, 12}},
		{"gt", Token{GT, ">", 2, 16}},
		{"number", Token{NUMBER, "18", 2, 18}},
		{"rbrace", Token{RBRACE, "}", 3, 2}},
		{"EOF", Token{EOF, "", 3, 3}},
	}

	runTestsOnTokens(tknzr, tests, t)
}

func TestSafePipeToken(t *testing.T) {
	input := "?>"
	tknzr := New(input)

	tests := []testScenario{
		{"Safe pipe operator", Token{SAFE_PIPE, "?>", 1, 1}},
		{"End of file", Token{EOF, "", 1, 3}},
	}

	runTestsOnTokens(tknzr, tests, t)
}
// TestNamedImportTokenization tests the tokenization of named imports syntax.
func TestNamedImportTokenization(t *testing.T) {
	input := "import { where, select } from \"@caja/query\" as q"
	tknzr := New(input)

	tests := []testScenario{
		{"Import keyword", Token{IMPORT, "import", 1, 1}},
		{"Left brace", Token{LBRACE, "{", 1, 8}},
		{"Identifier where", Token{IDENT, "where", 1, 10}},
		{"Comma", Token{COMMA, ",", 1, 15}},
		{"Identifier select", Token{IDENT, "select", 1, 17}},
		{"Right brace", Token{RBRACE, "}", 1, 24}},
		{"Identifier from", Token{IDENT, "from", 1, 26}},
		{"String module", Token{STRING, "@caja/query", 1, 31}},
		{"As keyword", Token{AS, "as", 1, 45}},
		{"Identifier q", Token{IDENT, "q", 1, 48}},
		{"End of file", Token{EOF, "", 1, 49}},
	}

	runTestsOnTokens(tknzr, tests, t)
}
