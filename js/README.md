# @caja/js

JavaScript as [Caja](https://www.cajalang.com) data — an expression/statement tree and the
renderer that turns it into source.

The same idea as [`@caja/ui`](../ui)'s `View` tree, one language over: build a tree, render it at
the end. This is what lets a static page ship JavaScript that depends on values the build
computed.

```caja
import * from "@caja/js"
import * from "@caja/ui"

let pages: [String] = ["index.html", "about.html"]

let setup = scriptBlock([
	jsConst("PAGES", jsArray(toExprs(pages))),
	jsForOf("p", jsIdent("PAGES"), [
		jsLog(jsIdent("p"))
	])
])
```

## Why this exists next to `js.raw`

The `js` **builtin** module (`js.raw`) and this **package** are complements, and they fail in
opposite directions:

| | `js.raw("…")` | `@caja/js` |
|---|---|---|
| correctness comes from | actually parsing it with esbuild, at compile time | being built that way |
| the source must be | a string literal | anything, including runtime Caja data |
| good for | JavaScript you'd rather just write | embedding build-time data, composition |

A literal cannot interpolate, so `js.raw` alone cannot embed a site's page list, a build
timestamp, or a generated lookup table. That is the gap this package fills. Use both: `jsRawStmt`
takes a `js.Script`, so a validated literal drops straight into a built program.

## Install

Not yet published to npm. `@caja/ui` depends on this package, so a project that uses `@caja/ui`
already has it.

```sh
npm install @caja/js
```

## How "valid by construction" is enforced

Two mechanisms, both load-bearing:

**Expressions and statements are separate types.** `jsCall` takes `[JsExpr]`, `jsIf` takes
`[JsStmt]`, so a statement cannot land where an expression belongs — the main way generated code
turns into syntax errors.

**Every leaf that could carry syntax is escaped, except one.** `jsStr` escapes quotes,
backslashes, newlines and `</`; `jsIdent` and property names are checked against a plain-identifier
alphabet and anything else is routed through bracket access, which quotes it. The single exception
is `jsRaw`/`jsRawStmt`, whose payload is a `js.Script` — and a `js.Script` can only come from
`js.raw`, which parses it. So the one door for arbitrary syntax is a door that validates.

That door also cannot be forged. Struct literals are public in Caja, so a consumer *can*
hand-build a node — but not one carrying arbitrary text:

```caja
JsExpr{kind: "raw", script: "); alert(1); ("}
# type error: field 'script' expects Script, got String
```

Every other kind is escaped or quoted on render, so the worst a hand-built node can produce is a
strange string literal, not injected code.

## Expressions

- **Literals**: `jsStr(s)` (escaped), `jsNum(n)`, `jsBool(b)`, `jsNull()`, `jsUndefined()`
- **Names**: `jsIdent(name)` — rejects anything that is not a plain identifier
- **Composite**: `jsArray(items)`, `jsObject(keys, values)`, `jsMember(target, prop)`,
  `jsIndex(target, key)`, `jsCall(callee, args)`, `jsNew(callee, args)`,
  `jsArrow(params, body)`
- **Operators**: `jsAdd`, `jsSub`, `jsMul`, `jsDiv`, `jsMod`, `jsEq` (`===`), `jsNeq` (`!==`),
  `jsLt`, `jsLte`, `jsGt`, `jsGte`, `jsAnd`, `jsOr`, `jsCoalesce` (`??`), `jsNot`, `jsNegate`,
  `jsTypeof`, `jsTernary(cond, a, b)`
- **Escape hatch**: `jsRaw(code: js.Script)`

`jsObject` takes keys and values as two parallel arrays rather than a list of pairs. Not a style
choice: a pair type would have to hold a `JsExpr` and `JsExpr` would have to hold pairs — mutual
recursion between two struct types, which Caja does not support.

Every composite expression renders parenthesised, so operator precedence is never consulted and
never wrong. `(a + b)` where `a + b` would read better is the right trade for generated source
nobody edits by hand.

## Statements

`jsConst(name, value)`, `jsLet(name, value)`, `jsAssign(target, value)`, `jsExprStmt(value)`,
`jsReturn(value)`, `jsReturnVoid()`, `jsIf(cond, body)`, `jsIfElse(cond, body, otherwise)`,
`jsFunction(name, params, body)`, `jsForOf(binding, iterable, body)`, and
`jsRawStmt(code: js.Script)`.

`renderProgram(program: [JsStmt]) -> String` is the terminal. Output is single-line and
semicolon-terminated, so it drops into an inline event attribute unchanged.

## DOM shorthands

Not a DOM binding — just the expressions you would otherwise spell out every time:
`jsDocument()`, `jsWindow()`, `jsConsole()`, `jsGetElementById(id)`, `jsQuerySelector(sel)`,
`jsQuerySelectorAll(sel)`, `jsLog(value)`, `jsAlert(message)`, `jsSetText(target, value)`,
`jsSetValue(target, value)`, `jsAddClass`/`jsRemoveClass`/`jsToggleClass(target, name)`.

All ordinary composition of the builders above; nothing is special-cased in the renderer.

## Limitations

- **No arrow function with a statement body.** `jsArrow` takes a single expression. A statement
  body would need `JsExpr` to hold `[JsStmt]`, and Caja supports only self-recursive struct types,
  not two that reference each other. Declare a `jsFunction` and pass its name instead.
- **`for…of` is the only loop.** It covers iteration over an array or a NodeList, which is what
  generated code needs; anything else is reachable via `jsRawStmt`.
- **No `==`.** Only `===`/`!==` are exposed — coercion is a bug source and a generator has no
  reason to emit one.
- **Nothing is type-checked as JavaScript.** The tree guarantees *syntax*, not that the
  identifiers you reference exist at runtime.

## Status

Draft, pre-1.0, developed alongside [`caja-cli`](https://github.com/caja-tech/caja-cli) itself.
