# ast

## Purpose

Defines the AST node types — the shared vocabulary every other pipeline stage speaks. `parser` produces these nodes, `analyzer` walks and annotates them, `compiler` transpiles them to Go. Also carries a handful of static control-flow-analysis helpers (`guarantee.go`) that operate purely on tree shape, independent of semantic analysis.

## Integration

- **Imports:** `internal/pipeline/lexer` only (for `Token`, so every node can carry its source position/lexeme).
- **Depended on by:** `parser`, `analyzer`, `compiler`, `internal/pipeline/modules`, `internal/script`, `internal/lsp`, `internal/encoder` — essentially every package above `lexer` in the pipeline. Changing a node's shape here has the widest blast radius of any package in the repo.

## How it works

- `Node` / `Statement` / `Expression` are the three core interfaces. `Statement` and `Expression` are distinguished by unexported marker methods (`statementNode()` / `expressionNode()`) — a compile-time-only tag trick with no runtime behavior, purely so the Go type system rejects putting a statement where an expression is expected (and vice versa).
- `Program` is the root node returned by `parser.Parse()`.
- Concrete node types (`FunctionLiteral`, `CallExpression`, `IfExpression`, etc., in `ast.go`) mirror the grammar 1:1.
- `errors.go` defines `DiagnosticError`, the structured (positioned) error type used by both `parser` and `analyzer` diagnostics — this is what the LSP turns into positioned squiggles.
- `guarantee.go` implements control-flow exhaustiveness checks consumed by `analyzer`: `GuaranteesReturn` (does every path through this node definitely return?), `GuaranteesRecursiveCall` (does every path make a recursive call — used for tail-recursion/memoization validation), `MightReturn` (could any path return?).

## Gotchas / invariants

- **`MightReturn` (and similarly-shaped recursive helpers) must nil-check `n.Alternative` *before* recursing into it**, not rely on a top-of-function `node == nil` check to catch it. `IfExpression.Alternative` is a concrete `*BlockStatement`; when there's no `else` branch it's a nil pointer, but passing that nil pointer as a `Node` interface argument boxes it into a *non-nil* interface value (nil concrete type + non-nil interface = the classic Go typed-nil trap). The top-level `if node == nil` guard doesn't fire on that boxed value, so skipping the explicit `n.Alternative == nil` check before recursing panics. See `guarantee.go`'s existing handling for the pattern to follow when adding new traversal helpers here.
- Because `Statement`/`Expression` are structural marker interfaces, a new node type only needs the right marker method to satisfy them — there's no central registry to update, but conversely nothing stops a node from accidentally satisfying both if you're not careful with method naming.
