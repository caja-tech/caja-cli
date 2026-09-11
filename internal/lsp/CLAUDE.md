# lsp

## Purpose

Implements a Language Server Protocol server for Caja — hover, go-to-definition, completion, signature help, folding ranges, and live diagnostics — over `github.com/owenrumney/go-lsp`. This is what powers the `editors/vscode/caja` extension's IDE features (see [[../../editors/CLAUDE.md]]).

## Integration

- **Imports:** `internal/pipeline/analyzer`, `internal/pipeline/analyzer/symbol`, `internal/pipeline/ast`, `internal/pipeline/environment`, `internal/pipeline/lexer`, `internal/pipeline/parser` — directly, not via `internal/script`. It re-implements its own parse-then-analyze sequence because it needs cancellable, per-keystroke re-analysis via `context.Context`, which `script.ParseWithDir` doesn't support.
- **Depended on by:** `cmd/cli/lsp.go` only (`lsp.Run(version)`), invoked as `caja lsp` and spawned as a subprocess by the VS Code extension over stdio.

## How it works

- `Run(version string) error` is the entry point — runs the LSP server over stdio.
- `CajaHandler` implements the handler interface: `Initialize`, `DidOpen`/`DidChange`/`DidClose`, `Hover`, `Definition`, `SignatureHelp`, `Completion`, `FoldingRange`. `go-lsp`'s `server` package auto-detects which optional handler interfaces `CajaHandler` satisfies and merges the corresponding capability flags into `Initialize`'s response (`server.buildCapabilities`, called by the framework, not by this package) — `Initialize` in `lsp.go` only needs to declare capabilities that aren't auto-detected (`CompletionOptions.TriggerCharacters`). Adding a new handler method here is often enough on its own to light up the matching client capability; check `go-lsp`'s `server/capabilities.go` before hand-declaring one.
- `folding.go`: `FoldingRange` answers `textDocument/foldingRange` with C#-style `#region [name]` / `#endregion` blocks. These are ordinary `#` comments to the rest of the pipeline — comments are swallowed inside the lexer's `skipWhitespace` and never become tokens (see [[../pipeline/lexer/CLAUDE.md]]), so there is no AST node for a region and this can't be answered from `astCache`. Instead `scanRegionMarkers` does its own lightweight pass over the raw document text (from `h.docs`, the same source `SignatureHelp` re-lexes from), tracking string/date-literal spans just enough that a `#` inside one isn't mistaken for a comment, and requiring the `#` to be the first non-whitespace character on its line (matching how a real preprocessor directive works) so a trailing `# region` comment mid-statement isn't treated as a marker. Nesting is handled with a stack keyed by line number; an unmatched `#region` (no closing `#endregion`) is left un-folded rather than guessing an end at EOF, and a stray `#endregion` with nothing open is ignored. Region *names* are accepted but not surfaced anywhere — this `go-lsp` version's `FoldingRange` has no `collapsedText` field (a newer LSP addition), so a collapsed region always shows the client's generic placeholder regardless of the name written after `#region`.
- `ast_util.go` / `autocomplete.go` provide AST position-lookup utilities used by the request handlers: `FindNodeAtPosition`, `FindCallExpressionAtPosition`, `GetNodeToken`, `GetVariablesInScope`.
- Per-document validation runs on a dedicated worker goroutine with a size-1 buffered channel of `context.Context` — a "replace pending" pattern where a new keystroke's context replaces (and thereby cancels, via the parser's `WithContext` support) any still-in-flight validation for that document, so rapid typing doesn't queue up a backlog of stale re-parses.
- An `astCache` keyed by document URI holds the last successfully-parsed-and-analyzed `*ast.Program` + `*analyzer.Analyzer`, so Hover/Definition/Completion can answer from memory in O(1) without triggering a fresh parse on every request.

## Gotchas / invariants

- There are stray debug prints left in the codebase (`fmt.Printf("resolved symbol...")` in `autocomplete.go`, `fmt.Println("PROG IS NIL")` in `lsp.go`). These are real bugs worth fixing, not intentional behavior to preserve — stdout is the JSON-RPC transport channel for stdio-based LSP, so any stray write to stdout is a protocol-corruption risk, not just console noise. Grep for `fmt.Print` before shipping changes here.
- The single-worker-per-document pattern is what keeps CPU spikes from overlapping parses on rapid keystrokes down to zero — if you're adding a new request type, make sure it goes through the same cancellation-aware path rather than triggering an ad-hoc synchronous re-parse.
- Because this package duplicates `internal/script`'s pipeline sequencing rather than reusing it, any bugfix to lexer/parser/analyzer *invocation order or options* in `internal/script` should be checked against this package too.
