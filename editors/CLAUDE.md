# editors

## Purpose

The Caja VS Code extension (`editors/vscode/caja`) — the only editor integration in the repo today. Provides syntax highlighting and wires the editor up to the Caja language server. Unlike every other package documented in this repo, this is **not Go code** — it's a TypeScript VS Code extension with its own npm-based build.

## Integration

- **Imports (Go):** none — no direct Go dependency.
- **Connects to `internal/lsp` indirectly**, at runtime, by spawning the compiled `caja` binary as a subprocess with `caja lsp` and speaking LSP over stdio via `vscode-languageclient`. See [[../internal/lsp/CLAUDE.md]] for the server side of this connection.
- **Depended on by:** nothing else in the repo — this is a standalone client artifact, published separately to the VS Code Marketplace.

## How it works

- `syntaxes/caja.tmLanguage.json` — the TextMate grammar. Covers the purely lexical layer: keywords, operators, literals, comments, string interpolation. Everything requiring knowledge of the program — telling a type from a capitalised variable, marking stdlib modules — comes from the server's semantic tokens instead, which the client layers on top.
- `language-configuration.json` — brackets, auto-closing pairs, `wordPattern`, indentation and on-enter rules.
- `src/extension.ts` — `activate()`/`deactivate()`, resolving which binary to launch, starting the client, and registering the restart/show-output commands.
- `package.json` `contributes` — the language and grammar, two commands, two settings (`caja.server.path`, `caja.trace.server`), and `semanticTokenScopes` giving the server's semantic tokens fallback TextMate scopes.

## Gotchas / invariants

- **The grammar is verified against the lexer, in Go.** `internal/pipeline/lexer/grammar_test.go` parses this JSON and asserts every keyword in the lexer's table is highlighted, that no *phantom* keyword is highlighted that the lexer doesn't recognise, and that every operator the lexer emits is matched. This exists because the grammar had drifted: `union` and `is` were added to the language and never here, while `from` was highlighted as a keyword despite lexing as a plain identifier. **If you add a keyword or operator to the lexer, that Go test tells you to update this file** — it is the only thing connecting the two.
- **Single quotes are date literals, not strings.** The lexer emits a distinct `DATE` token for `'...'`. The grammar scopes them as `string.quoted.single.date.caja`; do not fold them into the double-quoted string rule.
- **`synchronize.fileEvents` is load-bearing, not decoration.** The server maintains a workspace index and re-checks open files when a module they import changes on disk — but it only learns about those changes from the client's file watcher. Remove it and cross-file diagnostics silently stop updating.
- In dev mode, `extension.ts` falls back to a relative path (`../../../bin/caja`) — this assumes a locally-built binary at `bin/caja` relative to the repo root. If you change that output location, update the path or local testing silently finds no server. The path it tried is written to the output channel, which is where to look when features are missing.
- **`client.start()` must stay awaited.** Leaving it unawaited is why a missing binary used to produce nothing at all: the rejection went unhandled and the extension simply appeared inert.
- Has its own `README.md`/`CHANGELOG.md`/`LICENSE.md` (VS Code Marketplace convention) rather than relying solely on this file. The README is the Marketplace page, so it documents the *extension*; it previously duplicated the CLI's README, which meant the extension page never mentioned the extension.
- `.vscodeignore` keeps `src/`, `tsconfig.json` and the build state out of the VSIX — the extension runs from `out/`.
