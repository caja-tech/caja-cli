---
title: Project Folder Structure
description: Details about the project's folder structure.
---

This section details the organization of the project's codebase. The repository is divided into several top-level directories, each with a specific responsibility.

## `cmd/`
The `cmd/` directory is the standard entry point for Go applications. It contains the main executable for the project's CLI. This is where the application starts and where CLI flags and arguments are parsed before calling into the core logic.

## `editors/`
The `editors/` directory contains editor integrations and plugins. This is where configurations and extensions for tools like VS Code are kept, providing features such as syntax highlighting, snippets, and language server client implementations.

## `grammar/`
The `grammar/` directory houses the formal specification of the language. This usually includes parser generator files (like Tree-sitter or ANTLR grammars) which define the lexical and syntactic rules of the language. These files are used to generate the lexer and parser.

## `internal/`
The `internal/` directory contains the core compiler, interpreter, and tooling logic. In Go, packages inside an `internal` folder are private and cannot be imported by external modules, ensuring the core implementation remains encapsulated. 

It is divided into several key subfolders:
- **`encoder/`**: Responsible for encoding operations, such as formatting output or bytecode generation.
- **`file/`**: Handles file system interactions, module resolution, and reading source files.
- **`lsp/`**: Contains the Language Server Protocol (LSP) server implementation, which powers rich editor features like autocomplete, go-to-definition, and real-time diagnostics.
- **`pipeline/`**: Orchestrates the compilation and execution pipeline. This directory represents the core lifecycle of a Caja program and is further divided into:
  - **`analyzer/`**: Performs semantic analysis, type checking, and symbol resolution on the AST to catch logical and scope errors before execution.
  - **`ast/`**: Defines the data structures for the Abstract Syntax Tree (AST), which represents the hierarchical structure of the parsed source code.
  - **`environment/`**: Manages the runtime state, managing lexical scopes, variable bindings, and memory tracking during execution.
  - **`evaluator/`**: The core interpreter engine that traverses the AST and evaluates the program logic.
  - **`lexer/`**: The scanner that reads raw source code characters and converts them into a sequence of meaningful structural tokens.
  - **`modules/`**: Handles the loading, caching, and resolution of imported standard libraries or external module files.
  - **`parser/`**: Takes the stream of tokens from the lexer and constructs the Abstract Syntax Tree based on the language's grammar rules.
- **`script/`**: Contains the runtime execution logic and interpreter environment for evaluating the language.
- **`text/`**: Provides text processing utilities, handling source code manipulation, line/column tracking, and formatting precise error messages.
