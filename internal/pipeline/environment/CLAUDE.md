# environment

## ⚠ Deprecation boundary — read this first

This package is **straddling deprecation**. It was originally the interpreter's full runtime object model + symbol table, back when Caja programs were evaluated directly rather than transpiled. That evaluator (`internal/pipeline/evaluator`) is deprecated and being phased out. In the live, compiled pipeline (`analyzer` → `compiler`), only two things from this package are actually load-bearing:

1. The `ObjectType` enum and its constants (`NUMBER_OBJ`, `ANY_OBJ`, etc.) — used purely as type tags.
2. `Environment` / `Module` bookkeeping fields — `FileName`, `ModuleASTs`, `ModuleAnalyzers`, private-name tracking, `ExportedValues`.

Everything else — the `Object` interface, concrete runtime value types (`Number`, `String`, `Function`, `Array`, `Map`, `Async`, `Builtin`, etc.), and the `newXModule()` built-in implementations in `builtin.go` — exists only to support the deprecated evaluator. This was verified by grepping `analyzer` and `compiler`: both only ever reference `ObjectType`/its constants and the `Environment`/`Module` struct fields, never the runtime `Object` machinery. **Do not assume the runtime object model is central to how Caja programs run today** — it isn't, for anything going through `build`/transpilation. New work on the compiled pipeline's type story belongs in `internal/pipeline/analyzer/symbol`, not here.

## Purpose

Historically: the interpreter's runtime value representation and lexical-scope symbol table. Today: primarily a shared bookkeeping struct (`Environment`) that `analyzer` and `compiler` use to track per-module state (imports, exports, file names) as they walk a program and its dependency graph.

## Integration

- **Imports:** `internal/pipeline/ast` only.
- **Depended on by:** `analyzer` and `compiler` (for the narrow surface described above), and the deprecated `evaluator` (for everything else).

## How it works

- `NewEnvironment()` / `NewEnclosedEnvironment(outer)` construct scopes; `Environment.Get`/`Set`/`Assign` are the interpreter-era variable-access API (only meaningful to the evaluator).
- `Module` and the `Environment` fields `FileName`, `ModuleASTs`, `ModuleAnalyzers`, `ModuleCache`, `Loading`, `ExportedValues`, and private-name tracking are the part `analyzer`'s import resolution actually populates and reads (see [[../analyzer/CLAUDE.md]]).
- `GetStandardModule` and the `Object`/`Builtin`/`BuiltinFunction` types + `builtin.go` module implementations back the deprecated evaluator's standard library.

## Gotchas / invariants

- `NewEnclosedEnvironment` shares several mutable maps (`ModuleCache`, `Loading`, `ExportedValues`, `privates`) with its outer environment **by reference, not by copy**. A nested environment mutating one of these maps mutates the outer environment's map too — this is relied upon by module bookkeeping (so a nested import-resolution pass can register itself in the shared `Loading`/cache maps), so don't "fix" it into a copy without checking every caller that depends on the sharing.
- If you're touching this package for a compiler/analyzer feature, resist the urge to also "clean up" the evaluator-only runtime types in the same change — they're intentionally left alone pending `evaluator`'s removal.
