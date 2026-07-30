package lexer

import "testing"

type testScenario struct {
	name string
	Token
}

// runTestsOnTokens iterates over a slice of testScenario entries, advancing the
// tokenizer one step per entry and asserting that the produced token matches the
// expected Type, Literal, Line, and Column values.
func runTestsOnTokens(tknzr *tokenizer, tests []testScenario, t *testing.T) {
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			token := tknzr.nextToken()

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

// TestBaseScenario verifies that the tokenizer correctly sequences through a basic
// two-line expression ("rate = 15.5\nrate + 5"), producing the right token types,
// literals, and accurate line/column positions for each token.
func TestBaseScenario(t *testing.T) {
	input := "rate = 15.5\nrate + 5"

	tests := []testScenario{
		{"Assign variable 'rate'", Token{IDENT, "rate", 1, 1}},
		{"Assign operator", Token{ASSIGN, "=", 1, 6}},
		{"Number value", Token{NUMBER, "15.5", 1, 8}},
		{"Identifier", Token{IDENT, "rate", 2, 1}},
		{"Plus operator", Token{PLUS, "+", 2, 6}},
		{"Number value", Token{NUMBER, "5", 2, 8}},
		{"End of file", Token{EOF, "", 2, 9}},
	}

	tknzr := newTokenizer(input)
	runTestsOnTokens(tknzr, tests, t)
}

// TestLeadingWhitespace ensures that spaces at the beginning of the input are
// skipped transparently and that column positions reflect the indented offset
// rather than the raw character index from position 0.
func TestLeadingWhitespace(t *testing.T) {
	input := "     rate = 15.5"

	tests := []testScenario{
		{"Assign variable 'rate'", Token{IDENT, "rate", 1, 6}},
		{"Assign operator", Token{ASSIGN, "=", 1, 11}},
		{"Number value", Token{NUMBER, "15.5", 1, 13}},
		{"End of file", Token{EOF, "", 1, 17}},
	}

	tknzr := newTokenizer(input)
	runTestsOnTokens(tknzr, tests, t)
}

// TestTrailingWhitespace confirms that trailing spaces after the last meaningful
// token are consumed without producing extra tokens, and that the EOF position
// accounts for the whitespace characters that follow.
func TestTrailingWhitespace(t *testing.T) {
	input := "rate = 15.5     "

	tests := []testScenario{
		{"Assign variable 'rate'", Token{IDENT, "rate", 1, 1}},
		{"Assign operator", Token{ASSIGN, "=", 1, 6}},
		{"Number value", Token{NUMBER, "15.5", 1, 8}},
		{"End of file", Token{EOF, "", 1, 17}},
	}

	tknzr := newTokenizer(input)
	runTestsOnTokens(tknzr, tests, t)
}

// TestLeadingNewLine checks that leading newline characters advance the line
// counter correctly so that tokens on the first non-blank line are reported with
// the right line number (4 in this case) and column 1.
func TestLeadingNewLine(t *testing.T) {
	input := "\n\n\nrate = 15.5"

	tests := []testScenario{
		{"Assign variable 'rate'", Token{IDENT, "rate", 4, 1}},
		{"Assign operator", Token{ASSIGN, "=", 4, 6}},
		{"Number value", Token{NUMBER, "15.5", 4, 8}},
		{"End of file", Token{EOF, "", 4, 12}},
	}

	tknzr := newTokenizer(input)
	runTestsOnTokens(tknzr, tests, t)
}

// TestTrailingNewLine verifies that newlines appended after the last token are
// consumed and reflected in the EOF token's line number, while earlier tokens
// retain their original positions on line 1.
func TestTrailingNewLine(t *testing.T) {
	input := "rate = 15.5\n\n\n"

	tests := []testScenario{
		{"Assign variable 'rate'", Token{IDENT, "rate", 1, 1}},
		{"Assign operator", Token{ASSIGN, "=", 1, 6}},
		{"Number value", Token{NUMBER, "15.5", 1, 8}},
		{"End of file", Token{EOF, "", 4, 1}},
	}

	tknzr := newTokenizer(input)
	runTestsOnTokens(tknzr, tests, t)
}

// TestLeadingTab asserts that tab characters at the start of the input advance
// the column counter (each tab counts as one column unit) so that subsequent
// tokens report the correct column offset.
func TestLeadingTab(t *testing.T) {
	input := "\t\t\trate = 15.5"

	tests := []testScenario{
		{"Assign variable 'rate'", Token{IDENT, "rate", 1, 4}},
		{"Assign operator", Token{ASSIGN, "=", 1, 9}},
		{"Number value", Token{NUMBER, "15.5", 1, 11}},
		{"End of file", Token{EOF, "", 1, 15}},
	}

	tknzr := newTokenizer(input)
	runTestsOnTokens(tknzr, tests, t)
}

// TestTrailingTab confirms that tab characters trailing the last meaningful token
// are consumed without emitting additional tokens, and that the EOF column
// position accounts for each trailing tab character.
func TestTrailingTab(t *testing.T) {
	input := "rate = 15.5\t\t\t"

	tests := []testScenario{
		{"Assign variable 'rate'", Token{IDENT, "rate", 1, 1}},
		{"Assign operator", Token{ASSIGN, "=", 1, 6}},
		{"Number value", Token{NUMBER, "15.5", 1, 8}},
		{"End of file", Token{EOF, "", 1, 15}},
	}

	tknzr := newTokenizer(input)
	runTestsOnTokens(tknzr, tests, t)
}

// TestFullArithmeticExpression tokenizes a complete arithmetic expression
// involving nested parentheses and all four operators, asserting that every
// token — including two sets of parentheses and a numeric literal — is emitted
// with the correct type, literal value, and column position.
func TestFullArithmeticExpression(t *testing.T) {
	input := "(a + b) * (c - d) / 2"

	tests := []testScenario{
		{"Left paren 1", Token{LPAREN, "(", 1, 1}},
		{"Identifier a", Token{IDENT, "a", 1, 2}},
		{"Plus", Token{PLUS, "+", 1, 4}},
		{"Identifier b", Token{IDENT, "b", 1, 6}},
		{"Right paren 1", Token{RPAREN, ")", 1, 7}},
		{"Asterisk", Token{ASTERISK, "*", 1, 9}},
		{"Left paren 2", Token{LPAREN, "(", 1, 11}},
		{"Identifier c", Token{IDENT, "c", 1, 12}},
		{"Minus", Token{MINUS, "-", 1, 14}},
		{"Identifier d", Token{IDENT, "d", 1, 16}},
		{"Right paren 2", Token{RPAREN, ")", 1, 17}},
		{"Slash", Token{SLASH, "/", 1, 19}},
		{"Number 2", Token{NUMBER, "2", 1, 21}},
		{"End of file", Token{EOF, "", 1, 22}},
	}

	tknzr := newTokenizer(input)
	runTestsOnTokens(tknzr, tests, t)
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
// treated as an unrecognized character: the dot produces an ILLEGAL token with a
// corresponding syntax error message, while the following digit is lexed as a
// separate NUMBER token.
func TestLeadingDecimalNumber(t *testing.T) {
	input := ".5"

	tokens, errors := Lex(input)

	if len(errors) != 1 {
		t.Fatalf("expected 1 error for leading dot, got %d", len(errors))
	}

	expectedErr := "Syntax Error at line 1, column 1: unrecognized character '.'"
	if errors[0] != expectedErr {
		t.Errorf("expected error %q, got %q", expectedErr, errors[0])
	}

	if len(tokens) != 3 {
		t.Fatalf("expected 3 tokens (ILLEGAL + NUMBER + EOF), got %d", len(tokens))
	}

	if tokens[0].Type != ILLEGAL || tokens[0].Literal != "." || tokens[0].Column != 1 {
		t.Errorf("expected ILLEGAL '.', got %q %q col=%d", tokens[0].Type, tokens[0].Literal, tokens[0].Column)
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
// letter (e.g. "Rate") are recognised as IDENT tokens and that the literal is
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

// TestWindowsLineEndings checks that Windows-style CRLF line endings (\r\n)
// are handled correctly: the carriage-return is discarded, the line counter
// increments as expected, and subsequent tokens appear on the right line.
func TestWindowsLineEndings(t *testing.T) {
	input := "rate\r\nresult"

	tests := []testScenario{
		{"First identifier", Token{IDENT, "rate", 1, 1}},
		{"Second identifier", Token{IDENT, "result", 2, 1}},
		{"End of file", Token{EOF, "", 2, 7}},
	}

	tknzr := newTokenizer(input)
	runTestsOnTokens(tknzr, tests, t)
}

// TestEmptyInput verifies that an empty string produces exactly one EOF token
// with no errors, positioned at line 1, column 1.
func TestEmptyInput(t *testing.T) {
	tokens, errors := Lex("")

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
		tokens, errors := Lex("   ")

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
		tokens, errors := Lex("\n\n")

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
// throughout valid input ("10 @ 5 #") each generate a distinct syntax error
// message referencing the correct line and column of the offending character.
func TestInvalidCharacters(t *testing.T) {
	input := `10 @ 5 #`

	_, errors := Lex(input)

	if len(errors) != 2 {
		t.Fatalf("expected 2 lexer errors, got %d", len(errors))
	}

	expectedErr1 := "Syntax Error at line 1, column 4: unrecognized character '@'"
	if errors[0] != expectedErr1 {
		t.Errorf("expected error %q, got %q", expectedErr1, errors[0])
	}

	expectedErr2 := "Syntax Error at line 1, column 8: unrecognized character '#'"
	if errors[1] != expectedErr2 {
		t.Errorf("expected error %q, got %q", expectedErr2, errors[1])
	}
}
