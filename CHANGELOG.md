# Changelog

All notable changes to the Caja Language and CLI will be documented in this file.

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
