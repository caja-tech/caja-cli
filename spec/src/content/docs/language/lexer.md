---
title: Lexer
description: Details about the language's lexer.
---

The Lexer is the first phase of the Caja compilation pipeline. Its primary responsibility is to scan the raw source code text character-by-character and group them into meaningful, categorized sequences called **Tokens**. 

By stripping away insignificant data like whitespace and comments (`#`), the Lexer provides a clean, structural stream of tokens that makes the subsequent Parsing phase much simpler and faster.

## Implementation Details (`/internal/pipeline/lexer`)
The lexer is implemented in Go under the `internal/pipeline/lexer` package. The architecture is spread across several focused files:

- **`lexer.go`**: Contains the core `Lexer` state machine. It manages the reading position, advances through characters, tracks line and column numbers for precise error reporting, and skips whitespaces/comments.
- **`tokens.go`**: Defines the `Token` struct and the `TokenType` enumeration. It also holds the lookup table for reserved keywords to efficiently distinguish between generic identifiers and keywords (like `if` or `let`).
- **`deciders.go`**: Implements the pattern-matching rules (deciders) that evaluate the current character to determine which token is being formed (e.g., matching `>` vs `>=`).
- **`precedence.go`**: Maps operator tokens to their mathematical binding power (precedence).
- **`lexer_test.go`**: Extensive unit tests ensuring that character streams are reliably converted into the expected token sequences.

## Accepted Tokens
The Lexer recognizes and categorizes code into several specific `TokenType`s:

### Literals & Identifiers
- **Identifiers**: `IDENT` (e.g., variable or function names)
- **Literals**: `NUMBER`, `STRING`, `DATE`

### Keywords
Reserved words that dictate the language structure:
- `let`, `const`, `fn`, `return`, `if`, `else`
- `import`, `as`, `private`
- `type`, `struct`
- `true`, `false`, `nil`
- `and`, `or`, `xor`

### Operators
- **Math/Logic**: `PLUS` (`+`), `MINUS` (`-`), `ASTERISK` (`*`), `SLASH` (`/`), `POWER` (`^`), `MODULO` (`%`)
- **Comparison**: `LT` (`<`), `GT` (`>`), `LTEQ` (`<=`), `GTEQ` (`>=`), `EQ` (`==`), `NEQ` (`!=`), `BANG` (`!`)
- **Assignment & Flow**: `ASSIGN` (`=`), `ARROW` (`->`), `PIPE` (`|`)

### Delimiters
Structural punctuation:
- **Brackets**: `LPAREN` `(` , `RPAREN` `)` , `LBRACE` `{` , `RBRACE` `}` , `LBRACKET` `[` , `RBRACKET` `]`
- **Punctuation**: `DOT` (`.`), `COMMA` (`,`), `COLON` (`:`), `DOUBLE_COLON` (`::`), `QUESTION` (`?`), `QUESTIONDOT` (`?.`)

## Performance and Parser Communication
A key architectural decision in the Caja pipeline is **Lazy Tokenization**. 

While the package provides a helper `Lex()` function that can tokenize an entire string into an array, the standard compilation pipeline does **not** do this. Instead, the `Parser` is instantiated with a reference to the `Lexer` and continuously calls the `NextToken()` method on-demand.

**Why this improves performance:**
1. **Memory Efficiency**: For massive source files, generating an array of tens of thousands of `Token` structs upfront wastes memory. Lazy evaluation ensures only the current token being analyzed is kept in the active CPU cache.
2. **Error Accumulation & Streaming**: When the Parser encounters a syntax error, it doesn't immediately abort. Instead, it attempts to recover and continues calling `NextToken()` to accumulate all errors across the file (tracking exact lines and columns) so the user gets a comprehensive error report in one go. Because the tokens are streamed rather than pre-allocated, the parser can handle these recovery phases gracefully without memory bloat.
