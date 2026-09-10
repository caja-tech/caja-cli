# project

## Purpose

Reads and writes `cajaproj.yml`, the manifest a `caja init`-scaffolded project uses to declare its type (`static-page`, `http-api`, `web-app`), name, and entry file. Lets `caja build`/`run`/`serve`/`listen` detect what kind of project they're operating on when invoked without an explicit `--file`.

## Integration

- **Imports:** `internal/file` (for the `file.MAIN_FILE` default), `gopkg.in/yaml.v3`.
- **Depended on by:** `cmd/cli/init.go` (writes the manifest via `Save`) and `cmd/cli/project.go`'s `resolveProjectContext` — called from `run.go`, `build.go`, `serve.go`, and `listen.go` (reads it via `Load`).

## How it works

- `Load(dir) (*Manifest, bool, error)` — a missing `cajaproj.yml` is not an error: it returns `(nil, false, nil)` so callers fall back to requiring `--file`, matching this repo's existing behavior for standalone scripts.
- `Manifest.EntryOrDefault()` returns `Entry` if set, else `file.MAIN_FILE` — every scaffolded project currently uses the default, but a hand-edited manifest can point elsewhere.
- `Type.Valid()` is the single place the three legal type strings are enumerated; `Load` rejects anything else.

## Gotchas / invariants

- `CajaVersion` is purely informational (recorded from the CLI's `Version` var at `caja init` time) — nothing in this package or its callers currently enforces or compares it.
- `StaticPageOutputDir` (`"dist"`) is hardcoded here rather than a manifest field — if per-project output directories are ever needed, this is where that field would be added.
