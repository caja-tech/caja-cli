# Caja for Visual Studio Code

Language support for [Caja](https://cajalang.com): syntax highlighting plus a full language
server.

## Features

- **Diagnostics** as you type, and across files — editing a module re-checks the files that
  import it.
- **Hover** for types, functions, struct fields and type annotations.
- **Go to definition** and **find all references**, including on type names.
- **Rename**, with the declaration and every use updated together.
- **Completion** for locals, parameters, struct fields, module members and keywords.
- **Signature help**, including while the call is still being typed.
- **Outline and breadcrumbs**, folding, and expand-selection.
- **Workspace symbol search** across every `.caja` file in the folder.
- **Semantic highlighting**, which distinguishes types from ordinary capitalised names,
  marks standard-library modules, and colours the expressions inside string interpolation.
- **Quick fixes** for the problems the analyzer already explains — capitalising a type
  name, qualifying an ambiguous wildcard import, adding a missing `active`.

## Requirements

The `caja` CLI must be installed and on your `PATH`:

```sh
npm install -g @caja/cli
```

If it lives somewhere else, point the extension at it with `caja.server.path`.

## Settings

| Setting | Description |
| --- | --- |
| `caja.server.path` | Path to the `caja` executable. Empty means "use `PATH`". |
| `caja.trace.server` | Log traffic between the editor and the server: `off`, `messages` or `verbose`. |

## Commands

| Command | Description |
| --- | --- |
| `Caja: Restart Language Server` | Restart the server after upgrading the CLI, or if it stops responding. |
| `Caja: Show Language Server Output` | Open the server's log. |

## Troubleshooting

If language features are missing, run **Caja: Show Language Server Output**. A server that
failed to launch reports the path it tried there, and the usual cause is `caja` not being
on `PATH`.

## Contributing

The extension lives in [caja-tech/caja-cli](https://github.com/caja-tech/caja-cli) under
`editors/vscode/caja`; the language server is the same repository's `caja lsp` command.
