# encoder

## Purpose

Serializes a `.caja` entrypoint script plus all of its recursively-resolved local imports into a single portable, URL-safe, zlib-compressed base64 token — and reverses the process. Backs the CLI's `encode`/`decode` commands, letting a whole multi-file Caja program be shared or transported as one opaque string (e.g. pasted into a URL or playground) instead of a directory of files.

## Integration

- **Imports:** `internal/file` (for the `.caja` extension and `main.caja` naming convention), `internal/pipeline/ast`, `internal/pipeline/lexer`, `internal/pipeline/parser` — parses each module only far enough to walk its `ImportStatement`s and recursively discover dependencies; it does not run the analyzer.
- **Depended on by:** `cmd/cli/encode.go` and `cmd/cli/decode.go` only.

## How it works

- `Bundle{Entrypoint string; Modules map[string]string}` is the in-memory representation of a multi-file program before/after (de)serialization.
- `Encode(entryFile, baseDir string) (token string, err error)` walks imports starting from the entrypoint, collects every reachable local module's raw source into a `Bundle`, JSON-marshals it, zlib-compresses it, and base64-encodes the result.
- `Decode(token string) (*Bundle, error)` reverses that: base64-decode, zlib-decompress, JSON-unmarshal back into a `Bundle`.

## Gotchas / invariants

- `Decode` has a **legacy-format fallback**: if JSON unmarshaling the decompressed payload fails, it falls back to treating the entire payload as a single legacy raw script and wraps it as `{Entrypoint: file.MAIN_FILE, Modules: {main.caja: payload}}`. This means old single-file tokens (predating the `Bundle` format) still decode correctly — don't remove this fallback without confirming no old tokens need to keep working.
- The decode path uses a named return `err` alongside a deferred `Close()` on a reader/writer — if `Close()` itself returns an error at defer time, it can silently overwrite (clobber) a `nil` err that was already set from an earlier successful `Copy`/`Unmarshal`. Be careful preserving the original error's priority if you touch this function.
