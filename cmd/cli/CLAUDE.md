# cmd/cli

## Purpose

The `caja` binary's entrypoint — a Cobra-based CLI that composes every other package in the repo into the commands end users actually run (`run`, `build`, `encode`, `decode`, `lsp`). This is the composition root: nothing in the repo depends on it, it depends on almost everything.

## Integration

- **Imports:** `internal/file`, `internal/script`, `internal/lsp`, `internal/encoder`, `internal/pipeline/compiler`, `internal/pipeline/environment`.
- **Depended on by:** nothing — this is the leaf/executable package.

## How it works

- `main.go` wires every `New*Cmd()` constructor into the root command and calls `root.Execute()`. Holds the `Version` var, overridden via `-ldflags` at release build time, and passed through to `lsp.Run`.
- `root.go`: `NewRootCmd()` — base `caja` command; shows help when run with no subcommand.
- `run.go`: `NewRunCmd()` — reads a `.caja` file, calls `script.ParseWithDir` then `script.Run`, prints the result via `environment.PrintObject`. The `--export`/`-e` flag reads `globalEnv.ExportedValues` and writes them to CSV, special-casing `*environment.Array` values (flattened into CSV columns) versus scalar values (written as a single column) — a Caja-specific convention, not a generic CSV writer.
- `build.go`: `NewBuildCmd()` — `script.ParseWithDir` → `compiler.Transpile` → `go/format.Source` → `compiler.Compile`, which triggers `toolchain.EnsureToolchain()` internally. `--emit-go` dumps the intermediate transpiled Go source instead of (or alongside) building.
- `encode.go` / `decode.go`: thin wrappers around `internal/encoder.Encode`/`Decode`.
- `lsp.go`: `NewLspCmd()` — calls `lsp.Run(Version)`.

## Gotchas / invariants

- The `.caja`-extension validation (`filepath.Ext(filePath) == file.EXTENSION`) is duplicated across `run.go`, `build.go`, and `encode.go` rather than centralized in one helper. This is a known repetition, not an oversight to silently "fix" as a side effect of an unrelated change — if you do centralize it, update all three call sites deliberately.
- `run.go`'s CSV export logic is the one place in the CLI layer that has Caja-value-shape-aware behavior (array-flattening) baked directly into command code rather than pipeline code — worth knowing before assuming all `environment.Object` formatting lives in `internal/pipeline/environment`.
