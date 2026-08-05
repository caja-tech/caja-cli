package lexer

import (
	"testing"
)

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
