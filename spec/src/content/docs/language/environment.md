---
title: Environment
description: Information about the runtime environment memory state.
---

The **Environment** is the state management system that acts as the "RAM" for the Caja language. During both Semantic Analysis and Evaluation, the pipeline relies on the Environment to keep track of variable bindings, lexical scopes, modules, and the call stack.

Without the Environment, the interpreter would not be able to remember variables from one statement to the next.

## Implementation Details (`/internal/pipeline/environment`)

The Environment package is defined in Go under `/internal/pipeline/environment`. It is structurally divided into three core responsibilities:

### 1. State and Scoping (`environment.go`)
This file defines the `Environment` struct, which provides a symbol table (a hash map) to store variables. 

- **Lexical Scoping**: To support block scopes (like variables declared inside an `if` statement or a function body), environments can be enclosed within an `outer` environment (using `NewEnclosedEnvironment()`). When retrieving a variable via `Get()`, the environment checks its own map first. If the variable is missing, it recursively searches the `outer` environment. This perfectly simulates nested lexical scoping and variable shadowing.
- **Call Stack Tracking**: The environment maintains a `CallStack` array of `StackFrame`s. Every time a function is called, a frame is pushed, allowing the interpreter to generate precise stack traces (`GetStackTrace()`) if an error occurs.
- **Module Caching**: It caches loaded standard and user-defined modules (`ModuleCache`), preventing circular dependencies and redundant file reads.

### 2. Runtime Data Models (`types.go`)
Before Go native values can be used in Caja, they must be wrapped into standardized objects. `types.go` defines the `Object` interface, which mandates a `Type()` and `Inspect()` method.

Every variable created in a Caja script is mapped to one of these Go structs at runtime:
- **Primitives**: `Number`, `String`, `Boolean`, `Null`, `Date`
- **Data Structures**: `Array`, `Map`, `StructObject`
- **Executables**: `Function` (a user-defined closure containing the AST and bound environment) and `Builtin` (native Go functions).
- **Internal Markers**: `ReturnValue` (to break out of block execution) and `TailCall` (to optimize recursive function memory).

### 3. Native Built-ins (`builtin.go`)
This file registers all the native Go functions that Caja scripts can call natively. It groups these functions into Standard Modules (like math, strings, arrays). When a script imports or accesses a standard module, `GetStandardModule()` binds these Go-implemented functions into the Caja environment as `Builtin` objects.
