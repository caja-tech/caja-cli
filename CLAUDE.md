# Caja CLI

A command-line toolchain for the Caja language: parses/analyzes `.caja` scripts and either runs them or transpiles + compiles them into a native Go binary. Distributed as `@caja/cli` via npm.

This file is a map into the per-package `CLAUDE.md` files that document *why* each package exists, *how* it works, and *how* it fits with the rest of the system. Read the linked file for the package you're actually touching — this file only covers the shape of the whole system.

## The core pipeline

```
lexer → parser → ast (shared vocabulary) → analyzer → compiler
```

- **[internal/pipeline/lexer](internal/pipeline/lexer/CLAUDE.md)** — source text → token stream.
- **[internal/pipeline/parser](internal/pipeline/parser/CLAUDE.md)** — tokens → AST (Pratt parser; desugars pipe operators here).
- **[internal/pipeline/ast](internal/pipeline/ast/CLAUDE.md)** — the node types every other stage shares, plus control-flow-shape helpers (`guarantee.go`).
- **[internal/pipeline/analyzer](internal/pipeline/analyzer/CLAUDE.md)** — semantic analysis: scopes, types, purity, move semantics, import resolution.
  - **[internal/pipeline/analyzer/symbol](internal/pipeline/analyzer/symbol/CLAUDE.md)** — the type/symbol hierarchy the analyzer produces and the compiler reads.
- **[internal/pipeline/compiler](internal/pipeline/compiler/CLAUDE.md)** — transpiles the analyzed AST to Go source, then shells out to `go build`. Terminal stage; no separate IR or bytecode VM.

Two cross-cutting support packages sit underneath/beside that chain:

- **[internal/pipeline/environment](internal/pipeline/environment/CLAUDE.md)** — ⚠ mostly a legacy runtime object model tied to the deprecated evaluator; only its `ObjectType` constants and `Environment`/`Module` bookkeeping matter to the live pipeline.
- **[internal/pipeline/modules](internal/pipeline/modules/CLAUDE.md)** — resolves and loads imported `.caja` files (Node-style resolution), used by the analyzer during import handling.

`internal/pipeline/evaluator` is deprecated and intentionally undocumented here — don't build new features against it.

## What consumes the pipeline

- **[internal/script](internal/script/CLAUDE.md)** — the orchestration facade (`ParseWithDir`) that runs lexer→parser→analyzer in one call. Used by the CLI's `run` and `build` commands.
- **[internal/lsp](internal/lsp/CLAUDE.md)** — the Language Server Protocol implementation. Re-implements the parse+analyze sequence itself (rather than calling `internal/script`) because it needs cancellable, per-keystroke re-analysis. Powers the VS Code extension.
- **[internal/encoder](internal/encoder/CLAUDE.md)** — bundles a script plus its local imports into a single portable compressed token, for the `encode`/`decode` CLI commands.
- **[internal/toolchain](internal/toolchain/CLAUDE.md)** — downloads/caches a self-contained Go toolchain so `caja build` doesn't require a system Go install.
- **[internal/file](internal/file/CLAUDE.md)** — the `.caja` extension / `main.caja` naming constants, shared across the CLI, encoder, and modules.
- **[internal/text](internal/text/CLAUDE.md)** — trivial substring helper; test-only, not load-bearing production code.

## The composition root and the client

- **[cmd/cli](cmd/cli/CLAUDE.md)** — the Cobra-based `caja` binary: `run`, `build`, `encode`, `decode`, `lsp` subcommands, wiring together everything above.
- **[editors](editors/CLAUDE.md)** — the VS Code extension (TypeScript, not Go). Talks to `internal/lsp` indirectly by spawning `caja lsp` as a subprocess over stdio.

## Orienting yourself

- If you're changing language syntax: start at `parser`, check `ast` for the node shape, then `analyzer` for semantics, then `compiler` for codegen.
- If you're changing IDE behavior: start at `lsp`; it has its own parse/analyze path independent of `internal/script`.
- If you're changing CLI UX: start at `cmd/cli`; the actual work is delegated to `internal/script`, `internal/encoder`, or `internal/pipeline/compiler`.
- If you land in `internal/pipeline/environment`, read its deprecation-boundary note before assuming any runtime `Object` code you find there is still in use.
- After an implementation's own tests pass, run `/refine` to run the refactor-then-test loop (`.claude/agents/refactor-agent.md` + `.claude/agents/test-agent.md`) before considering the change done.
