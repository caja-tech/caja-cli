---
name: add-language-feature
description: Guide for adding a new syntax feature, operator, keyword, or builtin type to the Caja language end-to-end across lexer, parser, ast, analyzer, and compiler. Use when the user wants to add new syntax, a new operator, a new type-system rule, or extend the Caja grammar in caja-cli.
---

Use this skill whenever the user wants to add a new syntax feature to Caja — a new operator, keyword, statement form, built-in type, or type-system rule. It gives the touch-order and the per-stage checklist; consult each package's own CLAUDE.md (linked below) for the actual mechanics and gotchas of that stage.

## Pipeline order — touch these in this order

1. `internal/pipeline/lexer` — only if new token shapes are needed
2. `internal/pipeline/parser`
3. `internal/pipeline/ast`
4. `internal/pipeline/analyzer` (+ `internal/pipeline/analyzer/symbol` if introducing a new type)
5. `internal/pipeline/compiler`

Skip `internal/pipeline/environment` and `internal/pipeline/evaluator` — deprecated; don't add new feature support there.

## Step-by-step

### 1. Lexer
- Add new `TokenType` constant(s) in `tokens.go`.
- If the new syntax needs new character-level recognition, add a "decider" function in `deciders.go` (chain-of-responsibility — order matters; insert relative to existing deciders that might otherwise shadow it).
- Add/extend `lexer_test.go` cases for the new token(s).
- Skip this stage entirely if the feature is new grammar built from *existing* tokens.

### 2. Parser
- Register a `prefixParseFuncs`/`infixParseFuncs` entry for any new token that starts or infixes an expression.
- If it's an operator, get its precedence right relative to existing operators.
- The grammar enforces one-statement-per-line — new statement forms must respect that.
- If the feature is sugar over something simpler (like the existing pipe operators desugaring into `CallExpression`), prefer desugaring here over adding a new AST node.
- Add `parser_test.go` cases, including a malformed-input case to confirm `synchronize()` recovers sensibly.

### 3. AST
- Add the new node type in `ast.go` with the correct marker method (`statementNode()` or `expressionNode()`) and a doc comment.
- If the node can hold a possibly-nil branch (like `IfExpression.Alternative`), check `guarantee.go`'s existing nil-then-recurse pattern before writing any traversal helper over it — a nil concrete pointer boxed into the `Node` interface is a non-nil interface value, so a naive `node == nil` check won't catch it.
- If the node affects `GuaranteesReturn`/`GuaranteesRecursiveCall`/`MightReturn` semantics, add a case for it in `guarantee.go`.

### 4. Analyzer
- Add scope/type-checking logic — `scope.go` for lexical/purity/move-semantics interactions, `analyzer.go` for the main AST walk.
- New value type? Add a corresponding symbol type under `analyzer/symbol/` (one file per kind, following the existing pattern), implementing `symbol.Symbol`.
- If the new type wraps another type (nullable-like, async-like), make sure any code switching on it checks the wrapper as an interface *before* falling through to a generic `Type()` switch — the same trap `compiler.mapSymbolToGoType` already has to avoid for `NullableSymbol`/`AsyncSymbol`.
- Consider whether the feature interacts with function purity or move-semantics tracking — most new expression/statement kinds do.
- Add `analyzer_test.go` cases for both valid and invalid usage.

### 5. Compiler
- Extend `transpiler.go`'s AST walk to emit Go source for the new node.
- If a new symbol type was added, extend `mapSymbolToGoType` — wrapper-interface check first, generic switch second.
- Add a `compiler_test.go`/`transpiler_test.go` case, and consider a sample `.caja` fixture under `compiler/samples/` for significant features.

### 6. Optional: LSP
- `internal/lsp` re-implements its own parse+analyze path. Most new `ast`/`analyzer` types "just work" for hover/definition since they walk the same trees, but check `ast_util.go`'s position-lookup helpers if the new node has an unusual shape.

### 7. Docs
- Update `README.md`'s example/feature list if the feature is user-facing.
- Add a `CHANGELOG.md` entry following this repo's existing "Keep a Changelog" style.

## Verification checklist

- [ ] `go test ./...` passes
- [ ] `go build ./...` succeeds
- [ ] `caja run -f <sample>.caja` executes a script using the new feature correctly
- [ ] `caja build -f <sample>.caja --emit-go` — inspect the emitted Go source for correctness, not just that it compiles
- [ ] A deliberately invalid usage produces a sensible parser/analyzer diagnostic, not a panic

## Reference

Each stage's package has its own CLAUDE.md with the concrete mechanics and known gotchas — read the relevant one before editing:
- `internal/pipeline/lexer/CLAUDE.md`
- `internal/pipeline/parser/CLAUDE.md`
- `internal/pipeline/ast/CLAUDE.md`
- `internal/pipeline/analyzer/CLAUDE.md`
- `internal/pipeline/analyzer/symbol/CLAUDE.md`
- `internal/pipeline/compiler/CLAUDE.md`
- `internal/lsp/CLAUDE.md` (if touching IDE behavior)
