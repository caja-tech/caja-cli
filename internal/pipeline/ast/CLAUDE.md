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
- `inspect.go` is the **single traversal**: `Children(Node) []Node` knows the shape of every node type, and `Inspect`, `StartToken` and `Span` are all derived from it. Nothing else in the codebase should carry its own type switch over node shapes — `internal/lsp` used to have five, which had decayed to covering 25, 11 and 18 of the node types because nothing connected "a new syntax node exists" to "the walkers know about it".
- `types.go` defines `TypeExpr` and `TypeRef`, the positioned form of a type annotation. `TypeExpr.Name` is the canonical rendering (byte-identical to the plain strings these fields used to hold, which is what the analyzer compares, map-keys and concatenates); `TypeExpr.Refs` carries the individual named types inside it, which is what makes `Money` resolvable inside `[Money]`.
- `errors.go` defines `DiagnosticError`, the structured (positioned) error type used by both `parser` and `analyzer` diagnostics — this is what the LSP turns into positioned squiggles.
- `guarantee.go` implements control-flow exhaustiveness checks consumed by `analyzer`: `GuaranteesReturn` (does every path through this node definitely return?), `GuaranteesRecursiveCall` (does every path make a recursive call — used for tail-recursion/memoization validation), `MightReturn` (could any path return?).

## Gotchas / invariants

- **`MightReturn` (and similarly-shaped recursive helpers) must nil-check `n.Alternative` *before* recursing into it**, not rely on a top-of-function `node == nil` check to catch it. `IfExpression.Alternative` is a concrete `*BlockStatement`; when there's no `else` branch it's a nil pointer, but passing that nil pointer as a `Node` interface argument boxes it into a *non-nil* interface value (nil concrete type + non-nil interface = the classic Go typed-nil trap). The top-level `if node == nil` guard doesn't fire on that boxed value, so skipping the explicit `n.Alternative == nil` check before recursing panics. See `guarantee.go`'s existing handling for the pattern to follow when adding new traversal helpers here.
- Because `Statement`/`Expression` are structural marker interfaces, a new node type only needs the right marker method to satisfy them — there's no central registry to update, but conversely nothing stops a node from accidentally satisfying both if you're not careful with method naming.
- **Adding a node type means teaching `Children` about it.** `inspect_test.go` enforces this two ways: `TestAllNodesListIsComplete` parses this package and refuses to let a new struct go unclassified (either a `Node` in `allNodes`, or a documented entry in `nonNodeHelpers` explaining why it carries no position), and `TestChildrenReachesEveryNodeField` plants a sentinel in every node-holding field of every node type and asserts `Children` hands them all back. Both were verified to fail when deliberately broken. A field `Children` misses is syntax that is invisible to hover, go-to-definition, folding and highlighting — silently.
- **Closing delimiters are not children**, so `Span` consults `closingToken` for them. `BlockStatement.RBrace`, `ArrayLiteral.RBracket`, `MapLiteral.RBrace`, `StructLiteral.RBrace` and `StructDefinition.RBrace` exist for exactly this: without them a span derived from children ends at the last element, so a folded function body left its closing brace outside the fold.
- `Children` emits map-held children (`MapLiteral.Pairs`, `StructLiteral.Fields`) in **source order**, not map order. Go randomizes map iteration, and without sorting two identical requests can disagree about what sits under the cursor.
