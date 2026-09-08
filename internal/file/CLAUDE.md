# file

## Purpose

Leaf constants package — the single source of truth for the `.caja` file extension convention. No logic, just two constants.

## Integration

- **Imports:** none.
- **Depended on by:** `cmd/cli/{root,run,build,encode}.go`, `internal/encoder`, `internal/pipeline/modules`.

## How it works

- `EXTENSION = ".caja"`
- `MAIN_FILE = "main.caja"`

## Gotchas / invariants

- If the `.caja` extension or the default entrypoint filename convention ever needs to change repo-wide, this is the one place to change it — every consumer imports these constants rather than hardcoding the string, so a change here propagates everywhere it's needed without a repo-wide find/replace.
