// Package lexer implements a handwritten lexer (tokenizer) for the Caja
// financial DSL. It converts raw source-code text into a flat sequence of
// tokens that can be consumed by a parser. Each token carries its type,
// literal text, and the source position (line and column) where it was found.
package lexer

import "caja-cli/internal/tokenizer"

// Lex tokenizes the entire input string and returns two slices: the ordered
// sequence of tokens (always terminated by an EOF token) and a list of
// formatted syntax-error messages for any unrecognized characters encountered
// during scanning. The caller should check the error slice before using the
// token slice.
func Lex(input string) ([]tokenizer.Token, []string) {
	t := tokenizer.New(input)
	var tokens []tokenizer.Token

	for {
		token := t.NextToken()
		tokens = append(tokens, token)

		if token.Type == tokenizer.EOF {
			break
		}
	}

	return tokens, t.Errors
}
