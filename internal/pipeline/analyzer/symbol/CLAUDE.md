# analyzer/symbol

## Purpose

Defines the symbol-table type hierarchy that `analyzer` builds while walking the AST and that `compiler` reads to decide what Go code to emit. This is the analyzer's *output vocabulary* for types — the analyzer decides which symbol a name resolves to, and every symbol type here knows how to describe itself as a Caja type.

## Integration

- **Imports:** kept minimal and self-contained; it's a leaf-ish type-hierarchy package underneath `analyzer`.
- **Depended on by:** `internal/pipeline/analyzer` (produces symbols during scope resolution), `internal/pipeline/compiler` (reads symbols via `mapSymbolToGoType` to choose concrete Go types), `internal/lsp` (imports this directly for hover/signature-help type display).
- It has its own file rather than being folded into `analyzer`'s doc specifically because `compiler` and `lsp` consume it directly as a public contract — it isn't private analyzer plumbing.

## How it works

- `symbol.Symbol` is the core interface every type-representation implements.
- Concrete symbol types, one file each: `basic.go` (primitives), `array.go`, `map.go`, `struct.go`, `function.go`, `union.go`, `generic.go` / `constraint.go` (generics), `nullable.go` / `null.go` (optional types), `async.go` (async values), `builtin.go`, `module.go`.
- `symbol.GetStandardModule` resolves the built-in/standard-library module symbols (as opposed to user-imported `.caja` modules, which go through `analyzer`'s import resolution).

## Gotchas / invariants

- Some symbols are *wrappers* that forward `Type()` calls to an underlying symbol — notably nullable and async symbols. Any code that switches on `sym.Type()` to decide behavior (as `compiler.mapSymbolToGoType` does) must check for these wrapper interfaces (`NullableSymbol`, `AsyncSymbol`) **first**, before falling through to a generic `Type()` switch, or it will silently treat a `Nullable<Number>` as a plain `Number`.
- This package defines *what a type is*, not *how it behaves at runtime* — runtime values live in `internal/pipeline/environment` (and are largely tied to the deprecated evaluator there). Don't conflate the two when adding a new type kind: it needs a `symbol` representation here for the analyzer/compiler, and separately (only if the deprecated evaluator still needs to run it) a runtime `Object` in `environment`.
