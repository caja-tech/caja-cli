---
title: Parser
description: Details about the language's parser.
---

The Syntactic Analyzer, commonly known as the **Parser**, is the second phase of the Caja compilation pipeline. Its primary responsibility is to take the linear stream of tokens produced by the Lexer and assemble them into a hierarchical data structure known as the **Abstract Syntax Tree (AST)**.

While the Lexer verifies that individual characters form valid tokens, the Parser verifies that those tokens appear in a grammatically correct order (e.g., ensuring `let x = 5` is valid, but `let 5 = x` is rejected).

## Implementation Details (`/internal/pipeline/parser`)
The parser is implemented in Go under the `internal/pipeline/parser` package. The architecture is straightforward and robust:

- **`parser.go`**: Contains the main `Parser` struct. It holds a reference to the Lexer and maintains a small window of context (the `currToken` and `peekToken`) using a 1-token lookahead strategy.
- **`parser_test.go`**: Contains extensive unit tests that feed raw code strings to the parser and assert that the resulting AST perfectly matches the expected logical tree.

## The Pratt Parsing Algorithm
The Caja parser is implemented as a **Top-Down Operator Precedence Parser**, also widely known as a **Pratt Parser**. This algorithm is highly efficient and elegant for parsing expressions with varying operator precedences without relying on deeply nested, fragile rules.

### How it builds the AST
1. **Registry Maps**: During initialization (`New()`), the parser registers two maps:
   - `prefixParseFuncs`: Functions triggered when a token appears at the start of an expression (e.g., `-` in `-5`, `!` in `!true`, or `if`).
   - `infixParseFuncs`: Functions triggered when a token appears between two expressions (e.g., `+` in `5 + 5`, `==`, or function calls `(`).
2. **Precedence Binding**: Every operator is assigned a precedence (binding power). When the parser encounters `1 + 2 * 3`, it knows to bind `2 * 3` together first because the `*` infix parse function dictates a higher binding power than `+`.
3. **Recursive Descent**: For every statement, the parser reads the prefix token, calls its registered function, and then continuously checks the next token. If the next token is an infix operator with a higher binding power, it recursively hands control over to that infix function, building out the branches of the AST nodes (found in the `ast/` package).

## Error Recovery (`synchronize`)
Rather than crashing immediately upon encountering the first syntax error, the Caja parser implements a robust error recovery mechanism. 

When a parsing rule fails (e.g., a missing parenthesis), the parser logs a `DiagnosticError` containing the exact line, column, and error message. It then calls a `synchronize()` method. This method discards tokens until it finds a reliable statement boundary (like a newline or a structural keyword). Once synchronized, it resumes parsing. This allows the compiler to report *all* syntax errors in a single run, greatly improving the developer experience.
