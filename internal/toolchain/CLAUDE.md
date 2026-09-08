# toolchain

## Purpose

Downloads and caches a self-contained, per-user Go toolchain so `caja build` can shell out to `go build` without requiring the user to have Go installed themselves. This is what lets the `caja` CLI be a single npm-installed binary that still produces native compiled output.

## Integration

- **Imports:** none from this repo — pure stdlib (`net/http`, `archive/tar`, `archive/zip`, etc.).
- **Depended on by:** `internal/pipeline/compiler` only (`compiler.go` calls `EnsureToolchain()` before invoking `go build`/`go mod init`). See [[../pipeline/compiler/CLAUDE.md]].

## How it works

- `GoVersion` is a hardcoded version constant (currently `"1.22.0"`).
- `EnsureToolchain() (string, error)` checks `~/.caja/toolchain/go<GoVersion>/` for a cached `go` binary; if absent, downloads the platform-appropriate archive from `go.dev/dl/` (tar.gz on Unix via `extractTarGz`, zip on Windows via `extractZip`), extracts it, and returns the path to the `go` binary.

## Gotchas / invariants

- `GoVersion` is hardcoded — bumping the Go version the toolchain uses requires a code change and a new release, not a config flag.
- No checksum/signature verification is performed on the downloaded archive — this is a trust-the-network-and-go.dev design, worth being aware of before treating it as hardened against a compromised mirror or MITM.
- Downloads happen silently into the user's home directory the first time `build` runs — there's no offline fallback, so `caja build` requires network access at least once per machine per Go version.
