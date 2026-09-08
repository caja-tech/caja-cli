# text

## Purpose

A small substring-search helper package (`ContainsSubstring`, `SearchSubstring`), implemented as a naive manual character-loop rather than using `strings.Contains`.

## Integration

- **Imports:** none.
- **Depended on by:** `internal/pipeline/parser/parser_test.go` only, to check substrings in expected error messages.

## How it works

- `SearchSubstring(s, substr string) bool` does the actual O(n·m) manual scan.
- `ContainsSubstring(s, substr string) bool` adds a redundant length pre-check on top of `SearchSubstring`.

## Gotchas / invariants

- This is **test-support code only** — it is not used anywhere in a production code path, despite living under `internal/` alongside real runtime packages. Don't assume it's load-bearing when tracing dependencies.
- It duplicates what `strings.Contains` already does from the standard library; it's a reasonable candidate for deletion in favor of `strings.Contains` if this package is ever revisited, rather than a pattern to extend.
