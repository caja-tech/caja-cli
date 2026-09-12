# Caja CLI

[![NPM Version](https://img.shields.io/npm/v/@caja/cli?style=for-the-badge&logo=npm)](https://www.npmjs.com/package/@caja/cli) [![Official Docs](https://img.shields.io/badge/docs-cajalang.com-orange?style=for-the-badge)](https://www.cajalang.com)

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
caja encode -f <file.caja>
```
- `-f, --file`: The path to the script to encode.

### 3. Decode a Token
Decode a token string back into its original `.caja` script modules and save them to a directory.

```bash
caja decode <token> -o <output_dir>
```
- `-o, --output`: Directory path to save the decoded scripts (use `-o .` for the current directory).

### 4. Check Version
Retrieve the currently installed version of the Caja CLI.

```bash
caja --version
```

## Example Script

::: tip
Before running this script, make sure to install the std and query packages:

npm install @caja/std
npm install @caja/query
:::

Create a file named `test.caja`:

```caja
import "@caja/std"
import "@caja/query"

let isEven = fn(x: Number) -> Boolean {
    return x % 2 == 0
}

let powerTwo = fn(x: Number) -> Number {
    return x ^ 2
}

let add_numbers = fn(acc: Number, current: Number) -> Number {
    return acc + current
}

let result = std.range(1, 10)
    |> query.filter(isEven) 
    |> query.map(powerTwo)
    |> query.reduce(add_numbers, 0)

return result
```

Run it using the CLI:

```bash
caja run -f test.caja
```

### Extension-Function ("UFCS") Syntax

Any function whose first parameter's type matches a value's type can be
called as if it were a method on that value — a builtin module's function,
a function exported by one of your own imported `.caja` modules, or a
plain top-level function declared in the same file:

```caja
import "array"

let numbers = [1, 2, 3]
let updated = numbers.push(4) # same as array.push(numbers, 4)

let double = fn(x: Number) -> Number { return x * 2 }
let ten = 5.double()          # same as double(5) — works for your own functions too
```

This includes your own struct types, so extension functions read like methods:

```caja
type Point struct { x Number, y Number }

let mag = fn(p: Point) -> Number { return p.x + p.y }

let p = Point { x: 1, y: 2 }
let m = p.mag()               # same as mag(p)
```

If the struct already has a function-typed field of that name, both stay
available and the call picks by argument count, then by argument types — so a
field `scale fn(Number)` and a function `scale(p: Point, f: Number, o: Number)`
coexist, and `p.scale(2)` calls the field while `p.scale(2, 3)` calls the
function. Only a genuinely indistinguishable pair — the same parameter list —
is rejected, with a suggestion of how to say which one you meant.
