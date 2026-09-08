# analyzer

## Purpose

Semantic analysis for Caja: scope resolution, type inference/checking, purity enforcement, move-semantics tracking, and import resolution. It's the stage between parsing (syntax) and compiling (codegen) — `compiler` cannot run without a fully-executed `Analyzer`, because it reads the analyzer's symbol table to decide what Go types to emit.

## Integration

- **Imports:** `internal/pipeline/ast`, `internal/pipeline/lexer`, `internal/pipeline/environment` (only `ObjectType` constants and `Environment`/`Module` bookkeeping — see [[../environment/CLAUDE.md]] for why that's the load-bearing subset), `internal/pipeline/modules` (to load imported `.caja` files during import resolution).
- **Depended on by:** `internal/pipeline/compiler` (reads the symbol table via `analyzer/symbol` to map Caja types to Go types), `internal/lsp` (runs its own analyzer per document for hover/definition/completion), `internal/script` (`ParseWithDir`), and the deprecated evaluator.
- Its own `symbol` subpackage (see [[symbol/CLAUDE.md]]) defines the symbol-table type hierarchy this package produces and that `compiler` consumes directly.

## How it works

- `New(*environment.Environment) *Analyzer` constructs an analyzer bound to a global environment; `(*Analyzer).Run()` walks the AST.
- Diagnostics: `.Errors()`, `.DiagnosticErrors()`. Lookups: `.GetSymbol()`, `.GetDefinition()` (used by the LSP for go-to-definition), `.GlobalScope()`, `.GlobalEnv()`.
- **Function purity enforcement**: a function body cannot read or mutate anything from an enclosing scope. `pushFunctionBoundary`/`currentFunctionBoundary` (in `scope.go`) track this — a boundary caps *redeclaration* lookups at the function edge but does not cap general read lookups, so purity violations are actively checked rather than structurally prevented by scoping alone.
- **Move-semantics tracking**: `markVarMoved` plus snapshot/restore/union logic tracks which variables have been moved-from, unioning move state across branches (e.g. both arms of an `if`) so a variable moved in only one branch is correctly flagged as possibly-moved after the branch merges.
- **Import resolution** (`analyzeImportStatement`) is recursive and stateful: each imported module gets a brand-new `Analyzer` + `Environment`, a `loading` map is shared across the whole import chain to detect circular imports, and results are cached in `globalEnv.ModuleAnalyzers` / `globalEnv.ModuleASTs` — the compiler reuses these caches rather than re-resolving imports itself.
- **Type scoping**: types are tracked in a `types` stack kept in lockstep with the value-scope stack, giving real lexical scoping to type declarations, not just a flat global type table.

## Gotchas / invariants

- Don't assume `environment.Environment` gives you interpreter semantics here — the analyzer only uses it for `ObjectType` tagging and module bookkeeping, never for evaluating anything.
- Import cycles are only caught because the `loading` map is *shared* across the recursive per-module analyzer instances — if you refactor import resolution to not thread that map through, circular-import detection silently breaks.
- `ModuleAnalyzers`/`ModuleASTs` caches populated here are read later by `compiler` — if you change when/how they're populated, check the compiler's import handling too.
