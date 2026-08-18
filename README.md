# Caja CLI

[![NPM Version](https://img.shields.io/npm/v/@caja/cli?style=for-the-badge&logo=npm)](https://www.npmjs.com/package/@caja/cli)

A command-line interface for the Caja language. The `caja` CLI allows you to execute `.caja` scripts, as well as encode and decode them into transportable token strings.

## Installation

You can install the Caja CLI globally using npm:

```bash
npm install -g @caja/cli
```

## Usage & Commands

### 1. Run a Script
Parse and evaluate a `.caja` script file to execute it.

```bash
caja run -f <file.caja>
```
- `-f, --file`: The path to the `.caja` script file.
- `-e, --export` (Optional): File name to export log values to (e.g., `data.csv`).

### 2. Encode a Script
Encode a `.caja` script file and its dependencies into a single base64-like token string.

```bash
caja -f <file.caja>
```
- `-f, --file`: The path to the script to encode.

### 3. Decode a Token
Decode a token string back into its original `.caja` script modules and save them to a directory.

```bash
caja <token> -o <output_dir>
```
- `-o, --output`: Directory path to save the decoded scripts (use `-o .` for the current directory).

### 4. Check Version
Retrieve the currently installed version of the Caja CLI.

```bash
caja --version
```

## Example Script

Create a file named `test.caja`:

```caja
import stdlib
import query

let isEven = fn(x: Number) -> Boolean {
    return x % 2 == 0
}

let powerTwo = fn(x: Number) -> Number {
    return x ^ 2
}

let add_numbers = fn(acc: Number, current: Number) -> Number {
    return acc + current
}

let result = stdlib.range(1, 10)
    |> query.filter(isEven) 
    |> query.map(powerTwo)
    |> query.reduce(add_numbers, 0)

return result
```

Run it using the CLI:

```bash
caja run -f test.caja
```
