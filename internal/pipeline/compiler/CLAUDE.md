# compiler

## Purpose

The backend: transpiles an analyzed Caja AST into Go source (`transpiler.go`), then shells out to `go build` to produce a native binary (`compiler.go`). Caja does not have its own bytecode VM or IR — Go itself is the codegen target and Go's toolchain is the final compiler.

## Integration

- **Imports:** `internal/pipeline/ast`, `internal/pipeline/analyzer` and `internal/pipeline/analyzer/symbol` (reads the resolved symbol table to choose Go types), `internal/pipeline/environment` (constants only — see [[../environment/CLAUDE.md]]), `internal/toolchain` (provisions a Go toolchain to build with).
- **Depended on by:** `cmd/cli/build.go` (primary consumer: `Transpile` → `go/format` → `Compile`) and `cmd/cli/run.go` in some paths.
- This is the terminal stage of the pipeline — nothing downstream depends on `compiler`.
- Requires a fully-`Run()` `*analyzer.Analyzer` as input; it does not itself do any semantic analysis.

## How it works

- `Transpile(*ast.Program, *analyzer.Analyzer) (string, error)` walks the AST and the analyzer's symbol table together, emitting Go source as a string.
- `Compile(goSource, outputBin string) error` writes that source into a throwaway temp directory with its own generated `go.mod`, calls `internal/toolchain.EnsureToolchain()` to get a `go` binary, then shells out to `go build`.
- `mapSymbolToGoType` (in `transpiler.go`) is the core type-mapping function — see the `analyzer/symbol` gotcha about wrapper symbols; this function is exactly where that ordering matters.
- `memo`-annotated functions compile to a wrapper/impl pair backed by `sync.Map` for caching. Parameters that aren't naturally comparable (so can't be map keys directly) are hashed via a generated `caja_memo_hash` helper, string-templated directly into the emitted Go source (JSON-encode the args, then FNV64a-hash the bytes). This helper never exists as real, independently-importable Go code — it's only exercised via `memo_hash_test.go`, which re-derives the same template to test the hashing logic in isolation.
- `async` values always transpile to a type-erased `*asyncTask`, regardless of the underlying Caja type — the underlying type is recovered via the symbol table at the call site, not encoded in the Go type itself.

## Gotchas / invariants

- `mapSymbolToGoType` must check `symbol.NullableSymbol` / `symbol.AsyncSymbol` (interface checks) before falling through to a generic `sym.Type()` switch — those wrapper symbols forward `Type()` to their underlying symbol, so a naive switch silently unwraps them and emits the wrong Go type.
- `Compile` builds in an isolated temp directory with a fresh `go.mod` — it is not building inside this repo's own module, so don't assume repo-relative import paths resolve inside generated code.
- Because there's no separate IR, any new language feature that needs codegen changes goes directly into `transpiler.go`'s AST-walking logic — there's no intermediate representation to insert a pass into.
