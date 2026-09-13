# posmap

## Purpose

Converts between Caja's internal source coordinates and LSP positions. Exists so that arithmetic happens exactly once, in one place.

## Integration

- **Imports:** `internal/pipeline/lexer` (for `Token`) and `go-lsp`'s `lsp` types.
- **Depended on by:** `internal/lsp` only.

## How it works

The two coordinate systems disagree on both axes:

| | Caja (`lexer.Token`) | LSP (`lsp.Position`) |
|---|---|---|
| Line | 1-based | 0-based |
| Column | 1-based **byte** offset | 0-based **UTF-16 code unit** offset |

`lexer.readChar` advances the column once per *byte*, so a line containing any non-ASCII character desynchronizes every position after it. For pure ASCII the only difference is the off-by-one — which is precisely why the mismatch survived unnoticed.

- `New(text)` builds an index for one document version. Immutable, safe to share.
- `ByteColumn` / `UTF16Column` convert in each direction, short-circuiting on lines recorded as pure ASCII (the overwhelmingly common case, since Caja identifiers are ASCII by construction — only string literals, date literals and comments can carry anything else).
- `TokenRange` / `SpanRange` / `Position` turn tokens into LSP ranges.
- `TokenByteLen` is the single rule for how much source a token occupies.

## Gotchas / invariants

- **The rule this package exists to enforce: no `lsp.Position` or `lsp.Range` is constructed anywhere in `internal/lsp` except through here.** It is grep-checkable, and the whole point — every range the server emits is only as correct as this conversion.
- **`TokenByteLen` is not `len(tok.Literal)`.** The lexer stores a quoted literal's *inner* text — `"abc"` has the literal `abc` — while `Column` points at the opening quote. Measuring with the literal's own length leaves the closing quote and the character before it outside the token, so hovering there finds nothing.
- **`positionEncoding: "utf-8"` is deliberately not negotiated.** `go-lsp`'s `document.Document.offsetAt` converts UTF-16 unconditionally with no encoding switch, so under utf-8 every incremental `didChange` range on a non-ASCII line would silently corrupt the buffer; and `vscode-languageclient` v9 advertises utf-16 only. Revisit only if a client that offers utf-8 (Neovim does) becomes a first-class target.
- `Line` strips a trailing `\r`, so CRLF documents do not leak the carriage return into column arithmetic.
