# @caja/dom

`mount()`, `sync()` and the Caja event handlers for [`@caja/ui`](../ui) `View` trees — the
`web-app` renderer, kept in its own package.

It depends on @caja/ui, not on a theme: `mount()` only ever walks the four structural types
(`View`, `Attr`, `Style`, `Handler`), so a project using [`@caja/siriguela`](../siriguela), a
theme of its own, or no theme at all mounts the same way. A `View` is the same nominal type
whichever package you reached it through.

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
npm install @caja/ui @caja/dom
```

## Import alias

`@caja/dom`'s default import alias is its own last path segment,
`dom` — not a valid Caja identifier (`-` lexes as a minus sign, not part of a name).
Always import it with an explicit alias:

```caja
import "@caja/dom" as dom
```

## Usage

```caja
import * from "@caja/ui"
import "@caja/dom" as dom
import browser

let tree = text("Hello")
let root = browser.getElementById("app")

dom.mount(tree, root)

# later, after some state changes elsewhere:
let updated = text("Hello again")
dom.sync(updated, root)
```

Note the local is `tree`, not `view`: `view` is @caja/ui's `View` constructor, and a wildcard
import lets a same-named local silently shadow it.

- **`mount(target, parent) -> Element`** — walks the tree into live DOM nodes via
  `browser.createElement`/`appendChild`/`setAttribute`/`setStyle`/`on`. Returns the root element
  it created, so a caller can keep a handle for a later, explicit `browser.setText`/`setStyle`
  call — an opt-in escape hatch, not automatic reactivity.
- **`onClick(target, handler)`, `onInput`, `onKeyDown`, … (90 of them)** — record a **Caja
  function** as an event handler. These share their names with `@caja/ui`'s JavaScript handlers
  deliberately: `onClick` is `onClick`, and which one you get is which package you reached into.
  Wildcard-importing both packages and writing a bare `onClick` raises an "ambiguous reference"
  error naming both, so import this one qualified (`import "@caja/dom" as dom`, then
  `dom.onClick`). These live here rather than in `@caja/ui` because this is the
  package that can honour them: calling a Caja function from a browser event needs a Caja runtime
  in the browser, which only a wasm build ships. `@caja/ui`'s `onClick` family takes
  JavaScript instead and works in every target, including this one — reach for it unless the
  handler genuinely needs to run Caja code.

  A handler cannot close over an outer `let`/`const` (Caja's purity rule applies to it like any
  other function value). Read what you need *inside* the handler — `browser.getElementById`,
  `browser.getValue`, `localStorageGet` — which is also what makes it correct, since the handler
  runs long after the tree was built.

- **`sync(target, mountedRoot) -> Nothing`** — explicit reconciliation. v1 is clear-and-remount
  (`browser.setHTML(mountedRoot, "")` then `mount()`), not a diff/patch — correct, but repaints
  the whole subtree and loses any focus/scroll state inside it. Call it by hand after rebuilding
  a `View` from freshly-read state; there is no automatic reactivity to hook into here at all
  (Caja's `active`/`react` system can't be reached from inside any function body, purity forbids
  it), so an event handler already has to re-fetch whatever state it needs fresh — the same
  pattern `onClick` handlers already require in the main package.

## Status

Draft, pre-1.0, developed alongside [`caja-cli`](https://github.com/caja-tech/caja-cli) itself.
