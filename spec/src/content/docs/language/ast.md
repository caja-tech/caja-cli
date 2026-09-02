---
title: Abstract Syntax Tree
description: Information on the AST structure.
---

The Abstract Syntax Tree (AST) is the core data structure that acts as the bridge between parsing and execution.

## What is an AST and why does it exist?
When the Lexer processes source code, it produces a flat, linear array of Tokens. However, programming languages are not linear; they are deeply hierarchical. 

Consider the expression `1 + 2 * 3`. A flat token array doesn't inherently understand that `2 * 3` must be evaluated before adding `1`. The AST exists to convert that flat stream into a logical tree structure, abstracting away superficial syntax (like parentheses or whitespace) and enforcing operator precedence, control flow, and nested scoping.

## Implementation Details (`/internal/pipeline/ast`)
The Caja AST is defined in Go under the `internal/pipeline/ast` package. It strongly leverages Go's interface system to create a strictly typed, polymorphic tree.

### Core Interfaces
Every element in the tree implements the base `Node` interface. However, the language strictly differentiates between **Statements** (which perform an action but produce no value) and **Expressions** (which compute and produce a value).

- **`Node`**: The base interface. Every node must be able to return its literal text (`TokenLiteral()`) and a human-readable string representation of itself (`String()`).
- **`Statement`**: Embeds `Node` and adds a dummy `statementNode()` method. This is used by the Go compiler to strictly enforce that an expression cannot be accidentally passed where a statement is expected. Examples include variable declarations (`let x = 5`) or return statements.
- **`Expression`**: Embeds `Node` and adds a dummy `expressionNode()` method. Examples include literals (`5`, `"hello"`), function calls, or binary operations (`x + y`).

### The Root Node
The absolute top of the AST is the **`Program`** node.
When the Parser finishes its job, it returns a single `*ast.Program`, which contains an ordered array of top-level `Statement`s. The Semantic Analyzer and Evaluator simply take this `Program` node and iterate through its statements to execute the file.

### Error Handling
The package also contains structural definitions for diagnostics (found in `errors.go`). This allows AST nodes to inherently carry contextual information about where they originated (exact line and column numbers), making it easy to generate rich, precise errors if something goes wrong during evaluation.
