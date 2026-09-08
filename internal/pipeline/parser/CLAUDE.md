# parser

## Purpose

Converts the `lexer`'s token stream into an `ast.Program` — the tree the rest of the pipeline (`analyzer`, `compiler`) operates on. This is where Caja's grammar actually lives: precedence, statement structure, and syntax-sugar desugaring.

## Integration

- **Imports:** `internal/pipeline/lexer` (consumes tokens), `internal/pipeline/ast` (builds nodes).
- **Depended on by:** `internal/pipeline/modules` (re-parses each imported file), `internal/script` (`ParseWithDir`, the CLI's `run`/`build` entry point), and `internal/lsp` (re-implements its own parse-per-document flow directly against this package rather than going through `internal/script`).

## How it works

- `New(*lexer.Lexer) *Parser` builds a parser; `(*Parser).Parse() *ast.Program` runs it to completion.
- Diagnostics: `.Errors()` (strings), `.DiagnosticErrors()` (`ast.DiagnosticError`, structured — used by the LSP for positioned squiggles), `.HasErrors()`.
- `.WithContext(ctx)` threads a `context.Context` through parsing so a caller (the LSP) can cancel a parse mid-flight, e.g. when a new keystroke supersedes an in-progress validation.
- This is a Pratt / top-down operator-precedence parser: `prefixParseFuncs` and `infixParseFuncs` are maps keyed by `lexer.TokenType`, registered once at construction, and dispatched on during expression parsing instead of a hand-rolled recursive-descent grammar per operator.
- Pipe operators (`PIPE`, `SAFE_PIPE`, `STREAM_PIPE`, `SAFE_STREAM_PIPE`) aren't a distinct AST node family — they're desugared at parse time into rewritten `CallExpression`s (e.g. a safe-pipe threads the left-hand value in as an argument to the right-hand call). If you're looking for pipe semantics, look at how these tokens are handled in the infix table, not in `ast.go`.
- Panic-mode error recovery: `synchronize()` skips forward to the next `IDENT`/`NUMBER` token after a parse error, so one bad statement doesn't cascade into hundreds of spurious errors.

## Gotchas / invariants

- The grammar enforces **one statement per line** — two statements landing on the same source line is a parse error, not just a style complaint. This interacts with the lexer's `\r\n`-as-one-newline rule.
- Because pipe operators are desugared into `CallExpression`s during parsing, `analyzer` and `compiler` never see a "pipe" node — don't add pipe-specific handling downstream; extend the desugaring here instead.
- `WithContext` cancellation exists specifically for the LSP's rapid-edit case — a batch CLI parse (`internal/script`) doesn't need it but gets it for free; don't assume an uncancelled context if you're calling into LSP-triggered code paths.
