---
title: CLI Commands
description: Documentation for available Command Line Interface tools.
---

The Caja programming language provides a powerful Command Line Interface (CLI) to interact with scripts, encode/decode them, and power developer tooling.

## Commands

### `run`
Parses and evaluates a Caja script file.
- **Usage**: `caja run --file <script.caja>` or `caja run -f <script.caja>`
- **Flags**:
  - `-f, --file`: (Required) Path to the `.caja` file you want to execute.
  - `-e, --export`: (Optional) Path to export the global variables and exported environment values into a CSV file (e.g., `-e data.csv`).

### `encode`
Encodes a Caja script file and all of its dependencies into a single base64-like token string. This is extremely useful for transmitting a whole project bundle safely or executing it remotely without a full file system.
- **Usage**: `caja encode --file <script.caja>`
- **Flags**:
  - `-f, --file`: (Required) Path to the `.caja` file to encode.

### `decode`
Takes a previously encoded token and decodes it back into its original source code modules, saving them to a specified directory.
- **Usage**: `caja decode <token> --output <dir>`
- **Arguments**: 
  - `[token]`: The encoded token string.
- **Flags**:
  - `-o, --output`: (Required) The directory where the decoded modules should be saved (e.g., `-o .` for the current directory).

### `lsp`
Starts the Caja Language Server Protocol (LSP). This command is generally not invoked manually by the user, but rather spawned automatically by code editors (like VS Code) to provide features like autocomplete, go-to-definition, and diagnostics over standard input/output.
- **Usage**: `caja lsp`

---

## How the Go CLI works (`/cmd` folder)

The CLI is implemented in Go and resides in the `/cmd/cli/` directory. By standard Go conventions, the `cmd` folder serves as the main entry point for binaries.

Each command is encapsulated in its own file (e.g., `run.go`, `encode.go`), which exposes a constructor like `NewRunCmd()` or `NewEncodeCmd()`. Inside `main.go`, the application instantiates the root command (`caja`) and mounts all subcommands onto it before calling `Execute()`.

### The Cobra Library
The project relies on [Cobra](https://github.com/spf13/cobra) to structure and parse CLI commands. 

**Why Cobra?**
Cobra is the industry standard for Go CLI applications (used by tools like Kubernetes, GitHub CLI, and Docker). It is used in this project because it provides:
1. **Subcommands**: Easy nested command routing (e.g., routing `caja run` to the run logic and `caja encode` to the encoder).
2. **Flag Parsing**: Automatic POSIX-compliant flag parsing (handling `--file` and `-f` identically) natively out of the box.
3. **Help Generation**: It automatically generates polished help menus and usage text when a user makes a mistake or passes `--help`.
4. **Validation**: Built-in methods like `RunE` allow commands to return Go errors directly, which Cobra will format and print nicely for the user without panic.
