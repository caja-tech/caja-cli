# editors

## Purpose

The Caja VS Code extension (`editors/vscode/caja`) — the only editor integration in the repo today. Provides syntax highlighting and wires the editor up to the Caja language server. Unlike every other package documented in this repo, this is **not Go code** — it's a TypeScript VS Code extension with its own npm-based build.

## Integration

- **Imports (Go):** none — no direct Go dependency.
- **Connects to `internal/lsp` indirectly**, at runtime, by spawning the compiled `caja` binary as a subprocess with `caja lsp` and speaking LSP over stdio via `vscode-languageclient`. See [[../internal/lsp/CLAUDE.md]] for the server side of this connection.
- **Depended on by:** nothing else in the repo — this is a standalone client artifact, published separately to the VS Code Marketplace.

## How it works

- `syntaxes/caja.tmLanguage.json` — the TextMate grammar providing syntax highlighting for `.caja` files.
- `language-configuration.json` — editor behavior config (comment tokens, bracket matching, etc.).
- `src/extension.ts` — `activate()`/`deactivate()` lifecycle. On activation, spawns the language server process (`command: 'caja', args: ['lsp']`) and connects `vscode-languageclient` to it.

## Gotchas / invariants

- In dev mode, `extension.ts` points at a relative path (`../../../bin/caja`) instead of a `caja` binary on `PATH` — this assumes a specific repo layout where a locally-built binary lands at `bin/caja` relative to the repo root. If you change the build output location for local development, update this path or local extension testing will silently fail to find the language server.
- Has its own `README.md`/`CHANGELOG.md`/`LICENSE.md` (VS Code Marketplace convention) rather than relying solely on this file — this CLAUDE.md is for orienting a contributor working on the extension's *integration* with the rest of the repo, not a replacement for the extension's own docs.
