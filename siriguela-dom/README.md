# @caja/siriguela-dom

`mount()` and `sync()` for [`@caja/siriguela`](../siriguela) `View` trees — the `web-app` half
of siriguela, kept in its own package.

## Why a separate package

Merely *importing* a module that contains any `browser`-touching code forces the whole compiled
binary into `wasm`-only mode, even if nothing in the program actually calls it — confirmed
directly against `internal/pipeline/compiler`'s cross-module transpilation, which emits every
top-level statement of an imported module unconditionally, not just the ones referenced. A
`static-page` project must never need `wasm` just because the same library also happens to
define `mount()`.

## Install

Not yet published to npm. Once published:

```sh
npm install @caja/siriguela @caja/siriguela-dom
```

## Import alias

`@caja/siriguela-dom`'s default import alias is its own last path segment,
`siriguela-dom` — not a valid Caja identifier (`-` lexes as a minus sign, not part of a name).
Always import it with an explicit alias:

```caja
import "@caja/siriguela-dom" as siriguelaDom
```

## Usage

```caja
import "@caja/siriguela" as siriguela
import "@caja/siriguela-dom" as siriguelaDom
import browser

let view = siriguela.text("Hello")
let root = browser.getElementById("app")

siriguelaDom.mount(view, root)

# later, after some state changes elsewhere:
let updated = siriguela.text("Hello again")
siriguelaDom.sync(updated, root)
```

- **`mount(view, parent) -> Element`** — walks the tree into live DOM nodes via
  `browser.createElement`/`appendChild`/`setAttribute`/`setStyle`/`on`. Returns the root element
  it created, so a caller can keep a handle for a later, explicit `browser.setText`/`setStyle`
  call — an opt-in escape hatch, not automatic reactivity.
- **`sync(view, mountedRoot) -> Nothing`** — explicit reconciliation. v1 is clear-and-remount
  (`browser.setHTML(mountedRoot, "")` then `mount()`), not a diff/patch — correct, but repaints
  the whole subtree and loses any focus/scroll state inside it. Call it by hand after rebuilding
  a `View` from freshly-read state; there is no automatic reactivity to hook into here at all
  (Caja's `active`/`react` system can't be reached from inside any function body, purity forbids
  it), so an event handler already has to re-fetch whatever state it needs fresh — the same
  pattern `onClick`/`onInput` handlers already require in the main package.

## Status

Draft, pre-1.0, developed alongside [`caja-cli`](https://github.com/caja-tech/caja-cli) itself.
