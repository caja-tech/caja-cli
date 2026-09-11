# Changelog

All notable changes to the Caja Language and CLI will be documented in this file.

## [Unreleased]

### Added
- **Enum Types**: Added a closed `enum` type to the language (e.g. `enum CSSProperty { Padding = "padding", Margin = "margin" }`). The backing primitive type is inferred from the members' literal values, and an enum value widens implicitly to that backing type (e.g. into a `String` parameter or a `map[String]String` slot) — but a bare value of the backing type does not narrow to the enum, so a typo'd or unrelated string is a real compile-time type error at the call site, not a runtime one. Enums have no distinct Go type of their own at compile time — a member access transpiles directly to its literal value.

### Fixed
- **Non-tail-call self-recursion failed to compile**: a `let`/`const`-bound function whose recursive call wasn't a pure tail call (e.g. classic `return n * fact(n - 1)`, or the function passing its own name along as a value) transpiled to a single Go `var f T = func(...){ ...f... }` statement, which Go rejects (`undefined: f`) since a variable's own initializer can't reference it — even inside a closure literal that isn't actually invoked until later. The analyzer always accepted this as valid recursion; only the real `go build` path was affected, which is why it went unnoticed. Fixed by splitting into a declare-then-assign pair (`var f T; f = func(...){...}`) whenever a function literal's body references its own bound name — the standard Go idiom for self-referencing closures.
- **`caja build` could pick the wrong compile target for a declared project**: once a project declares its type via `cajaproj.yml`, `build.go` was still ORing that against `compiler.UsesBrowserModule` sniffing the transpiled Go source for a `syscall/js` import, rather than letting the declared type decide outright. A `static-page` (or `http-api`) project whose code merely *referenced* `browser.*` anywhere — even inside an imported library function nothing in the program ever called — got compiled to `wasm` and then crashed with `exec format error` when the static-page build path tried to run that wasm binary as a native generator. Fixed: with a declared project type, only that type decides the target (`web-app` → `js`/`wasm`, everything else → native); the `UsesBrowserModule` heuristic is now only consulted as a fallback for a standalone script with no `cajaproj.yml` at all.
- **A function parameter shadowing an unrelated sibling global resolved to the wrong binding, but only across a cross-module import**: `transpiler.go`'s identifier-resolution only checked whether a global with the *same name* existed anywhere in the module and whether this identifier's own definition lived in that module's file — both trivially true for a local parameter/`let` sharing a name with an unrelated top-level binding too, since it's declared in that same file. A parameter shadowing a same-named global elsewhere in its own module got module-prefixed and resolved to the *global* instead, real miscompilation (silent wrong-value substitution whenever the two happened to share a type, a genuine type error otherwise). Fixed by additionally requiring this occurrence's own resolved definition token to match the global scope entry's — confirming it actually refers to the global, not merely shares its name.

## [v0.1.0-alpha.5] - 2026-08-29

### Added
- **VS Code Extension**: Introduced the official Caja VS Code extension with foundational syntax highlighting (PR #73).
- **Language Server Protocol (LSP)**: Implemented full LSP support inside the Caja CLI to bring advanced IDE features to the editor (PR #74).
  - **Live Diagnostics**: Real-time syntax and semantic error highlighting as you type.
  - **Hover Information**: Instantly view inferred types, struct layouts, and function signatures.
  - **Go to Definition (Cmd+Click)**: Navigate directly to variable, function, and struct definitions, including cross-file module navigation.
  - **Signature Help**: Display parameter hints and active parameter tracking while typing function calls.

### Performance & Architecture Optimizations
- **Single Worker Pattern**: The LSP safely processes requests sequentially per document, completely eliminating CPU spikes caused by overlapping parses on rapid keystrokes.
- **Early Context Cancellation**: Intermediate keystrokes instantly cancel mid-flight background validations, freeing up CPU resources for the most recent AST changes.
- **Lock-Free Fast Reads**: ASTs and Analyzers are safely cached using `sync.RWMutex`, allowing LSP operations like Hover and Go To Definition to read from memory in `O(1)` time without triggering expensive re-parses.

### Fixed
- **Parser Crash Fix**: Fixed a critical interface typed-nil bug in the parser that caused panics when evaluating incomplete statements (e.g., `let`, `const`, `return`, `import`, `type`).
- **Syntax Enforcement**: The parser now strictly enforces newlines between statements.

### Documentation
- Created this comprehensive human-readable root `CHANGELOG.md` to document all notable changes across alpha releases.
- Standardized the VS Code extension's `CHANGELOG.md` to adhere to the "Keep a Changelog" format.

## [v0.1.0-alpha.4] - 2026-08-27

### Added
- **Encode & Decode Commands**: Added new `encode` and `decode` commands to the CLI, along with updated help documentation and test coverage (PR #71).

## [v0.1.0-alpha.3] - 2026-08-24

### Added
- **Node Modules Support**: Introduced support for requiring and using node modules directly within Caja scripts (PR #69, #70).
- **Documentation**: Added a comprehensive `README.md` to document the language and its tooling (PR #63).
- **Open Source License**: Added the MIT License to the project (PR #68).

### Fixed/Changed
- Tweaked the NPM deployment pipeline in GitHub Actions (removed provenance flag and adjusted workflows) (PR #62, #63, #66).

## [v0.1.0-alpha.2] - 2026-08-18

### Added
- **CLI Versioning**: Added a `--version` flag to the `cajac` CLI (PR #57, #58).

### Fixed/Changed
- **Pipeline Fixes**: Fixed the NPM publish GitHub Actions pipeline and upgraded the deployment Node version to 24 (PR #59, #60, #61).
- **Gitignore**: Allowed `npm/bin` in `.gitignore` (PR #57).

## [v0.1.0-alpha.1] - 2026-08-18

This was the monumental initial release of the Caja Language, laying down the fundamental compilation pipeline, language features, and standard library.

### Core Architecture
- **Pipeline Implementation**: Built the core Lexer/Tokenizer, Pratt Parser, Semantic Analyzer, and Tree-Walking Evaluator (PR #2, #4, #5, #12).
- **Refactoring & Scoping**: Refactored the compiler into an encoder and introduced block scoping (PR #13, #14).
- **NPM Integration**: Configured initial GitHub Actions for deploying the `cajac` CLI to npm (PR #1, #55).

### Language Features
- **Data Types & Structures**: Implemented Primitives, Arrays, Native Dictionary Maps, Structs, and Date types (PR #17, #22, #40, #44).
- **Advanced Typing**: Introduced Generics (for functions, structs, and aliases), Nullable Types, and a `Nothing` type for implicit returns. Enforced generic constraints and removed the permissive `Any` type (PR #40, #47, #50, #53).
- **Immutability & Purity**: Enforced deep immutability and function purity inside the semantic analyzer (PR #54).
- **Functions & Flow**: Added support for Higher-order functions, Inline arrow functions, Recursive functions with Tail Call Optimization, and declarative `if-else` expressions (PR #11, #15, #18, #19, #20, #48).
- **Variable Modifiers**: Introduced `let`, `const`, and `private` access modifiers (PR #13, #37, #38).
- **Operators**: Implemented the data-first pipe operator (`|>`), boolean logic (`and`, `or`, `xor`), prefix operators, exponential (`^`), and modulo (`%`) (PR #9, #10, #28, #29, #33, #52).

### Standard Library (Built-ins)
- **Math**: Added the `math` module with constants (like `PI`), functions, and `math.rand()` (PR #31, #35, #43).
- **Strings & Dates**: Added robust string manipulation and date processing functions (PR #26, #27).
- **Casting**: Implemented the `cast` module for safe type conversions (PR #46).
- **Collections**: Added polymorphic array functions and `map.delete` for dictionaries (PR #24, #49).
- **Logging**: Added the `log` module for console outputs (PR #36).
- Migrated many built-in modules to use the new Generics system (PR #51).
