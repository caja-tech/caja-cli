# @caja/siriguela

A renderer-agnostic UI component DSL for [Caja](https://www.cajalang.com). Build a `View` tree
once with a small catalog of components and modifiers; render it as an HTML string
(`static-page` projects) or mount it live in the DOM (`web-app` projects, via the separate
[`@caja/siriguela-dom`](../siriguela-dom) package) — the same tree, the same catalog, either way.

## Install

Not yet published to npm — a fresh `caja init` needs a manual `node_modules` setup pointing at
this package in the meantime. Once published:

```sh
npm install @caja/siriguela
```

## Package layout

Everything lives in a single `index.caja` — the tree engine (`View`, `el`, `attr`, `style`), the
full modifier chain, the component catalog, the `renderToString` terminal, and the default theme
all in one file. A custom theme is nothing more than another `.caja` module built on top of this
same base function set (see "Building a custom theme" below) — keeping it all in one file means
that base set is fully visible in one place, with nothing held back behind an internal package
boundary.

## Quick start — `static-page`

A `static-page` project only ever needs this package. `renderToString` has no browser
dependency at all, so importing only `@caja/siriguela` (never `@caja/siriguela-dom`) is what
keeps `caja build` compiling a plain native binary instead of `wasm` — merely importing a module
that contains any browser-touching code (even code nothing calls) forces the whole compiled
binary into `wasm`-only mode, which is why `mount()`/`sync()` live in the separate
`@caja/siriguela-dom` package instead of here.

```caja
import "@caja/siriguela" as siriguela
import page

let card = siriguela.el("div")
	|> siriguela.padding("20px")
	|> siriguela.addChild(siriguela.heading(1, "Hello"))
	|> siriguela.addChild(siriguela.text("Welcome to your new project."))

page.write("dist/index.html", siriguela.renderToString(card))
```

## Quick start — `web-app`

A `web-app` project additionally needs [`@caja/siriguela-dom`](../siriguela-dom) for `mount()`
— see that package's own README for why it's a separate package and the import-alias caveat its
hyphenated name needs.

```caja
import "@caja/siriguela" as siriguela
import "@caja/siriguela-dom" as siriguelaDom
import browser

let handleClick = fn() -> Nothing {
	browser.alert("hi")
}

let card = siriguela.el("div")
	|> siriguela.addChild(siriguela.primaryButton("Click me") |> siriguela.onClick(handleClick))

siriguelaDom.mount(card, browser.getElementById("app"))
```

`caja init --type static-page` / `--type web-app` scaffold exactly this shape by default.

## Component catalog

- **Tree**: `el(tag)`, `attr(view, name, value)`, `addChild(view, child)`
- **Modifiers**: every one is `view |> modifier(value)`, chainable —
  `padding`, `margin`, `backgroundColor`, `color`, `fontSize`, `fontFamily`, `rounded`,
  `display`, `flexDirection`, `flexBasis`,
  `width`, `height`, `minWidth`, `maxWidth`, `minHeight`, `maxHeight`, `gap`,
  `alignItems`, `justifyContent`, `alignSelf`, `flexGrow`, `flexShrink`, `flexWrap`,
  `gridTemplateColumns`, `gridTemplateRows`,
  `fontWeight`, `lineHeight`, `textAlign`, `letterSpacing`, `textDecoration`, `whiteSpace`,
  `textTransform`,
  `border`, `borderWidth`, `borderColor`, `borderStyle`, `boxShadow`, `opacity`, `cursor`,
  `overflow`,
  `position`, `top`, `right`, `bottom`, `left`, `zIndex`,
  `transition`, `boxSizing`
- **Leaf components**: `text(content)`, `heading(level, content)`, `button(label)`,
  `textField(placeholder)`, `image(src, alt)`, `link(href, label)`, `span(content)`,
  `listItem(content)`, `tableCell(content)`, `tableHeaderCell(content)`, `label(forId, content)`
- **Layout**: `column(children)`, `row(children)`, `box(child)`, `spacer(size)`,
  `addChildren(view, children)`
- **Structural/semantic**: unstyled multi-child wrappers — `section`, `article`, `nav`, `header`,
  `footer`, `mainEl`, `aside`, `form` (all `fn(children) -> View`), plus list/table helpers
  `unorderedList(items)`, `orderedList(items)`, `table(children)`, `tableRow(children)`
- **Stateful form elements**: `checkbox(checked)`, `radio(name, value, checked)`,
  `option(value, label, selected)`, `select(options)`, `textArea(placeholder, rows)` — HTML's
  `checked`/`selected` are boolean-by-presence, so these just conditionally attach the attribute;
  no extra `View` state is needed
- **Events**: `onClick(view, handler)`, `onInput(view, handler)` — deferred to `mount()`
  (`@caja/siriguela-dom`); a `View` rendered via `renderToString` instead just drops them, with a
  `log.warn` at `caja build` time telling you so
- **Terminal**: `renderToString(view) -> String`

`View`, `Handler`, `Attr`, `Style`, `style(view, property, value)`, and the `CSSProperty` enum it
validates against are also exported, for anyone building components below the catalog level — see
"Adding a CSS property" below.

## Default theme

Named after the fruit itself — deep plum-red skin, warm amber-orange flesh:

| Constant | Hex |
|---|---|
| `siriguelaSkin` | `#7A2048` |
| `siriguelaFlesh` | `#F0A23A` |
| `siriguelaLeaf` | `#6E8B3D` |
| `siriguelaCream` | `#FBF3E6` |
| `siriguelaInk` | `#2B1710` |

Plus pre-styled wrapper functions built on the catalog above: `primaryButton(label)`,
`secondaryButton(label)`, `card(children)`, `pageHeading(level, content)`,
`themedLink(href, label)`. None of this is a required layer — a project that never calls any of
it is exactly as valid as one that uses it everywhere.

## Building a custom theme

A design system is nothing more than a `.caja` module that imports `@caja/siriguela` and
re-exports its own styled wrappers — the same pattern the default theme above uses, just yours
instead of the fruit's:

```caja
import "@caja/siriguela" as siriguela

const brandPrimary = "#1D7A85"

let brandButton = fn(label: String) -> siriguela.View {
	return siriguela.button(label)
		|> siriguela.backgroundColor(brandPrimary)
		|> siriguela.color("#FFFFFF")
		|> siriguela.rounded("6px")
		|> siriguela.padding("8px 16px")
}
```

Multiple themes can coexist in one project — they're just names in different modules, nothing
about this needs a registry or compiler awareness of "themes" as a concept. Note
`brandPrimary` is a `const`, not a `let`: Caja's purity rules let a function read an outer
`const` but not an outer `let` ("pure functions cannot capture global/module variable") — a
real compile error `brandButton` would hit immediately if `brandPrimary` were declared `let`.

## Adding a CSS property

`style()`'s `property` parameter is a closed `CSSProperty` enum, not a plain string — passing an
unrecognized property name is a compile-time type error, not a silent no-op or a typo that
quietly does nothing. If a property you need doesn't have a modifier yet, add a `CSSProperty`
member and a one-line wrapper function directly in `index.caja`, following the existing pattern.

## Status

Draft, pre-1.0, developed alongside [`caja-cli`](https://github.com/caja-tech/caja-cli) itself.
