# Changelog

All notable changes to the "caja" VS Code extension will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]
### Added
- Syntax highlighting for the new `active` (storage modifier) and `react` (control keyword) reactive-programming keywords.
- Highlighting for the `union` and `is` keywords, for the `^`, `%`, `!`, `?`, `::` and `|`
  operators, and for date literals (`'2024-01-01'`), none of which were recognised before.
- Highlighting for string interpolation: the code inside `${...}` is now highlighted as
  code rather than as part of the surrounding string.
- Language server features surfaced by the editor: document outline and breadcrumbs,
  folding, expand-selection, find all references, rename, document highlight, workspace
  symbol search, semantic highlighting and quick fixes.
- Settings: `caja.server.path` to point at a `caja` executable that is not on `PATH`, and
  `caja.trace.server` for troubleshooting.
- Commands: **Caja: Restart Language Server** and **Caja: Show Language Server Output**.
- A file watcher for `**/*.caja`, so the server re-checks open files when a module they
  import changes on disk.

### Fixed
- A failed server start is now reported with the path it tried and a link to the setting,
  instead of failing silently — previously a missing `caja` binary just meant nothing
  happened.
- `from` is no longer highlighted as a keyword; Caja's lexer treats it as an ordinary
  identifier.
- Quotes no longer auto-close inside comments and strings.

### Changed
- The development-mode popup announcing the server path is now a line in the output
  channel rather than a modal notification on every launch.
- The Marketplace page now documents the extension rather than reproducing the CLI's
  README.

## [0.1.2] - 2026-08-29
### Added
- Comprehensive Caja Language Server Protocol (LSP) support, which includes:
  - **Live Diagnostics**: Real-time syntax and semantic error highlighting.
  - **Hover Information**: Show inferred types and signatures when hovering over symbols.
  - **Go to Definition**: Navigate directly to variable, function, and struct definitions.
  - **Signature Help**: Display parameter hints while typing function calls.

> **Note**: To utilize the LSP features, you must have the Caja CLI version `v0.1.0-alpha.5` or greater installed on your system.

## [0.1.1] - 2026-08-28
### Changed
- Adjusted the extension's icon size for better display in the VS Code marketplace and sidebar.

## [0.1.0] - 2026-08-28
### Added
- Initial release of the Caja VS Code extension.
- Added foundational syntax highlighting for Caja scripts (`.caja` files).
- Added basic language configuration (brackets, comments, and auto-closing pairs).
