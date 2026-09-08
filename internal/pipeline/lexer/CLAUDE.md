# lexer

## Purpose

Turns raw `.caja` source text into a flat stream of tokens. It's the first stage of the pipeline (`lexer → parser → ast → analyzer → compiler`) and knows nothing about grammar or meaning — only about recognizing the smallest meaningful chunks of text (identifiers, numbers, strings, dates, operators, keywords, punctuation).

## Integration

- **Imports:** none from this repo — the lexer is a leaf package with no dependency on `ast`, `parser`, or anything downstream.
- **Depended on by:** `internal/pipeline/parser` (consumes the token stream), `internal/pipeline/modules` (re-lexes each imported `.caja` file from scratch), and indirectly everything above them (`analyzer`, `compiler`, `internal/script`, `internal/lsp`).

## How it works

- `New(input string) *Lexer` constructs a lexer over source text; `(*Lexer).NextToken() Token` pulls one token at a time (pull-based, not a single big pass).
- `Lex(input string) ([]Token, []string)` is the convenience entry point most callers use — drains the lexer into a full `[]Token` slice plus any lexer-level error strings.
- `Token` / `TokenType` (`tokens.go`) define the vocabulary; `GetKeywords()` / `IsKeyword()` expose the keyword table.
- `VerifyPrecedenceLevel` (`precedence.go`) is used by the parser to validate operator precedence assumptions against the lexer's token types.
- Token recognition (`deciders.go`) uses an ordered slice of "decider" functions — a chain-of-responsibility over the current character — rather than one large switch statement. Each decider owns one token shape (number, string, date, identifier/keyword, operator, etc.) and the lexer tries them in order per character.
- `(*Lexer).Clone()` produces an independent copy of the lexer's cursor state. The parser uses this for lookahead — e.g. disambiguating anonymous-function syntax — without mutating the original lexer or polluting its shared error list with speculative parse attempts.

## Gotchas / invariants

- `#` starts a line comment; comments are consumed inside `skipWhitespace`, not surfaced as tokens.
- Date literals use single quotes (`'...'`); strings use double quotes — don't assume single quotes are just alternate string syntax.
- `\r\n` is counted as a single newline via `isCurrentCharANewLine`, not two — matters for anything doing line-based diagnostics (see the parser's one-statement-per-line rule).
- Because deciders run in order, adding a new token shape means placing it correctly relative to existing deciders that might otherwise shadow it — order is semantically significant, not just stylistic.
