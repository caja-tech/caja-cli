# script

## Purpose

A thin orchestration facade over the front-end pipeline: turns raw `.caja` source text into a validated AST + environment in one call. It exists so the CLI doesn't need to know how to wire lexer → parser → analyzer itself — `cmd/cli/run.go` and `cmd/cli/build.go` both go through this package rather than calling the pipeline stages directly.

## Integration

- **Imports:** `internal/pipeline/lexer`, `internal/pipeline/parser`, `internal/pipeline/ast`, `internal/pipeline/environment`, `internal/pipeline/analyzer`, `internal/pipeline/evaluator` (deprecated, used only by `Run`).
- **Depended on by:** `cmd/cli/run.go` (uses both `ParseWithDir` and `Run`) and `cmd/cli/build.go` (uses only `ParseWithDir` — it transpiles instead of evaluating, so it never touches the deprecated evaluator).
- Contrast with `internal/lsp`, which needs cancellable per-keystroke re-analysis and so re-implements this same lexer→parser→analyzer sequence itself rather than calling into `script` — if you change the sequencing here, check whether `internal/lsp` needs the same change made independently.

## How it works

- `ParseWithDir(input, baseDir, filePath string) (*ast.Program, *environment.Environment, *analyzer.Analyzer, error)` runs the lexer, then the parser, then the analyzer in sequence, short-circuiting with an error if either the parser or the analyzer reports errors.
- `Run(program *ast.Program, globalEnv *environment.Environment) (environment.Object, error)` hands the parsed/analyzed program to `evaluator.Eval` — this is the only place in the non-deprecated code paths that still invokes the evaluator, and only for the `run` command's direct-execution path (as opposed to `build`, which transpiles to a binary instead).

## Gotchas / invariants

- `ParseWithDir` doesn't return raw diagnostics to the caller — the parser and analyzer print their own errors to stdout/stderr via `PrintErrors()` as a side effect *before* this function returns, and the caller only gets back a generic wrapped error. If you're changing error-reporting/formatting, the actual diagnostic text lives in the parser/analyzer, not here — this package just decides *whether* to short-circuit, not *what* gets printed.
