---
title: Semantic Analyzer
description: Details about the language's semantic analysis phase.
---

The Semantic Analyzer (or Type Checker) is the third phase of the Caja compilation pipeline. While the Lexer ensures characters are valid, and the Parser ensures tokens are in a grammatically correct order, the **Semantic Analyzer** ensures that the code actually makes logical sense. 

It validates rules like ensuring variables are declared before they are used, type-checking function arguments, preventing variable redeclarations in the same scope, and handling imports.

## Implementation Details (`/internal/pipeline/analyzer`)
The semantic analysis logic is implemented in Go under the `internal/pipeline/analyzer` package. The architecture is modular and highly integrated with the Language Server Protocol (LSP).

- **`analyzer.go`**: Contains the core `Analyzer` struct. It manages lexical `scopes`, tracks user-defined `types`, and maps every AST node to its inferred type and original definition token.
- **`scope.go`**: Handles the pushing and popping of variable environments. Every time the code enters a block (like an `if` statement or a `fn` body), a new scope is pushed.
- **`builtin.go`**: Defines the semantic signatures for all standard built-in functions, allowing the analyzer to type-check native calls accurately.
- **`generics.go`**: Contains logic for resolving generic type constraints during type checking.
- **`find.go`**: Provides traversal helpers for locating specific nodes or resolving deep property access chains.
- **`symbol/` (Subpackage)**: Contains the data models for semantic types (e.g., `FunctionSymbol`, `StructSymbol`, `AnySymbol`), defining how types interact and cast.

## How AST Traversal Works
The Semantic Analyzer implements a recursive AST traversal algorithm, often referred to as a **Visitor Pattern**.

1. **Initialization**: The pipeline invokes `analyzer.Run(astProgram)`. The analyzer begins traversing the root AST node.
2. **Recursive Analysis**: For every node type (e.g., `LetStatement`, `InfixExpression`, `Identifier`), the analyzer executes specific validation logic.
   - If it encounters a `LetStatement`, it evaluates the right-hand side, infers its `symbol.Symbol` (Type), and inserts the variable name into the current scope.
   - If it encounters an `Identifier`, it looks up the variable in the current scope. If it doesn't exist, it checks the parent scope. If it reaches the global scope without finding it, it logs an "Undeclared Variable" `DiagnosticError`.
3. **Symbol Tracking**: As the analyzer traverses the tree, it populates two crucial maps: `nodeSymbols` (mapping every AST node to its inferred Type) and `nodeDefinitions` (mapping every AST node to the exact file and token where it was declared).
4. **LSP Integration**: Because the analyzer explicitly remembers *where* a symbol was declared and *what* its type is, the Caja LSP can directly query the `Analyzer` struct to power rich editor features like **Go to Definition** and **Hover Signatures** without needing to re-parse the code.
