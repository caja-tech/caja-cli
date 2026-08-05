package lexer

// Lex tokenizes the entire input string and returns two slices: the ordered
// sequence of tokens (always terminated by an EOF token) and a list of
// formatted syntax-error messages for any unrecognized characters encountered
// during scanning. The caller should check the error slice before using the
// token slice.
func Lex(input string) ([]Token, []string) {
	t := New(input)
	var tokens []Token

	for {
		token := t.NextToken()
		tokens = append(tokens, token)

		if token.Type == EOF {
			break
		}
	}

	return tokens, t.Errors
}
