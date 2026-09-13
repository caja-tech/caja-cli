# modules

## Purpose

Resolves and loads an imported `.caja` module file from disk, mirroring Node.js's module resolution so Caja's `import` statement feels familiar. Deliberately does the minimum: locate the file, lex it, parse it, hand back an AST — nothing more.

## Integration

- **Imports:** `internal/pipeline/ast`, `internal/pipeline/lexer`, `internal/pipeline/parser`, `internal/file` (for the `.caja` extension and `main.caja` conventions).
- **Depended on by:** `internal/pipeline/analyzer`, specifically `analyzeImportStatement` — this is the *only* caller. See [[../analyzer/CLAUDE.md]] for how the analyzer wraps this with circular-import detection and caching.

## How it works

- `Load(baseDir, moduleName string) (*ast.Program, string, error)` locates, reads and parses a module, returning its AST and the resolved on-disk path.
- `Resolve(baseDir, moduleName string) (string, error)` is just the locating half. `internal/lsp` builds its workspace dependency graph with it: without a resolution-only entry point it would have to parse every module simply to learn which file an import names.
- Resolution order mimics Node: first try `baseDir/moduleName.caja` directly; if that doesn't exist, walk up parent directories looking for `node_modules/<moduleName>`, honoring that package's `package.json` `"main"` field, defaulting to `index.caja` if `package.json` doesn't specify one.

## Gotchas / invariants

- `Load` returns immediately with a parse error if the module has syntax errors — but it deliberately **does not run the analyzer** on the loaded module itself. Semantic analysis, circular-import detection, and result caching are all the caller's (`analyzer`'s) responsibility by design. If you're debugging a "module not found" vs. "module has errors" issue, check which layer you're actually in — this package only ever produces resolution or syntax errors, never semantic ones.
- Smallest package in the pipeline (one file, under 100 lines) — resist adding analyzer-ish logic here; it belongs in `analyzer.analyzeImportStatement` instead, which already owns the loading map and module caches.
