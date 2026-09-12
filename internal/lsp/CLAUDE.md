# lsp

## Purpose

Implements a Language Server Protocol server for Caja over `github.com/owenrumney/go-lsp`. This is what powers the `editors/vscode/caja` extension's IDE features (see [[../../editors/CLAUDE.md]]).

## Integration

- **Imports:** `internal/pipeline/analyzer`, `internal/pipeline/analyzer/symbol`, `internal/pipeline/ast`, `internal/pipeline/environment`, `internal/pipeline/lexer`, `internal/pipeline/modules`, `internal/pipeline/parser` — directly, not via `internal/script`. It re-implements its own parse-then-analyze sequence because it needs cancellable, per-keystroke re-analysis via `context.Context`, which `script.ParseWithDir` doesn't support.
- **Depended on by:** `cmd/cli/lsp.go` only (`lsp.Run(version)`), invoked as `caja lsp` and spawned as a subprocess by the VS Code extension over stdio.
- **Subpackage:** [[posmap/CLAUDE.md]] owns every conversion between Caja's source coordinates and LSP positions.

## What it implements

Text sync (`didOpen`/`didChange`/`didClose`/`didSave`), push diagnostics, and: hover, definition, completion, signature help, document symbols, folding ranges, selection ranges, document highlight, references, rename (+ prepare), code actions, semantic tokens, workspace symbols, and `didChangeWatchedFiles`.

**Capabilities are derived, not declared.** `go-lsp` registers a method only if `CajaHandler` satisfies the matching interface, so renaming a handler method silently un-registers the feature — no compile error, no failing test, the editor just stops asking. `capabilities_test.go` holds compile-time assertions for all of them; that file is the guard. The two things no interface can express — the semantic token legend and completion trigger characters — are stated explicitly in `Initialize`.

## How it works

- `Run(version string) error` runs the server over stdio.
- **Per-document validation worker.** Each open document gets a goroutine fed by a size-1 buffered channel of `context.Context`. A new keystroke cancels the in-flight pass (via the parser's and analyzer's `WithContext`) and replaces any queued one, so rapid typing costs one analysis rather than a backlog. `queueValidationLocked` is the single entry point; `requestValidation` is the same thing for callers not already holding the lock.
- **`astCache`** keyed by URI holds the last parsed `*ast.Program` + `*analyzer.Analyzer`, so read requests answer from memory.
- **`workspaceIndex`** (`workspace.go`) is what the server knows about unopened files: what each declares, and which files import which. Built by *parsing* each file on a background scan — parsing yields both the declarations and the import list, and analyzing every file on startup would not be affordable. The reverse edges drive cross-file revalidation.
- **Position lookup** goes through `PathAt`, built on `ast.Children`. See the ast package's [[../pipeline/ast/CLAUDE.md]] for why that single traversal exists.

## Gotchas / invariants

- **stdout is the JSON-RPC transport.** The server runs on `server.RunStdio()`, so any stray write corrupts the protocol stream rather than merely printing noise. `TestNoStdoutWrites` parses the package and fails on `fmt.Print*`/`os.Stdout`; diagnostics go to `slog`, which writes to stderr. This was a live bug — a debug print fired on every dot-completion.
- **Panics must be contained per-request, not per-worker.** `validateDocument` runs on our own goroutine, outside the transport's recover, so an uncaught panic there kills the whole process. `safeValidate` recovers **per loop iteration**: recovering around the loop would end the goroutine, and since `DidChange` drains the size-1 channel before sending, the sender would never block — the document would silently stop updating forever. Read handlers use `recoverInto`, which degrades to an empty result rather than the transport's `CodeInternalError` (that surfaces as an error toast on every keystroke). `CAJA_LSP_STRICT` re-panics; `TestMain` sets it.
- **Never construct an `lsp.Position` or `lsp.Range` outside `posmap`.** `lexer.Token.Column` is a 1-based *byte* column; LSP characters are 0-based UTF-16 code units. They agree only on ASCII, which is why the mismatch went unnoticed for so long.
- **The analyzer's `ModuleASTs` is keyed by the import string** the source wrote (`"./calculus"`), *not* by the file it resolved to — `ModuleFilePaths` holds that. Reading dependency edges from `ModuleASTs` produces paths matching no file. Both the workspace scan and open documents resolve imports through `importedFiles`, which uses `modules.Resolve`.
- **Cross-file revalidation only touches open documents.** Diagnostics published for a file the editor never opened have nothing to attach to and no way to be cleared.
- `References` and `Rename` are scoped to the current document, and `Rename` declines when the declaration lives elsewhere. The index knows what files *declare*, not where every name is *used*, so a cross-file answer would be partial — and a rename that updates only some uses is worse than one that refuses.
- Because this package duplicates `internal/script`'s pipeline sequencing rather than reusing it, any bugfix to lexer/parser/analyzer *invocation order or options* there should be checked against this package too. `TestCompilerSamplesProduceNoDiagnostics` is the tripwire: it asserts all 46 compiler samples analyze clean through this path.

## Testing

- `corpus_test.go` drives every request across every token boundary of all 132 `.caja` files in the repo (`internal/pipeline/compiler/samples` and `internal/script/tests`). This sweep has found more defects than any other test here — including a typed-nil crash hitting 5388 probe positions. Recovered panics are invisible to a caller by design, so the sweep observes the `onRecover` hook directly and aggregates by crash site.
- Notifications are fire-and-forget: a test that inspects server state right after sending one is racing the handler. Use `waitFor`.
