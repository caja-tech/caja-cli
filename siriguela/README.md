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
import doc

let card = siriguela.el("div")
	|> siriguela.padding("20px")
	|> siriguela.addChild(siriguela.heading(1, "Hello"))
	|> siriguela.addChild(siriguela.text("Welcome to your new project."))

doc.write("dist/index.html", siriguela.renderToString(card))
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
  `transition`, `boxSizing`, `aspectRatio`, `animation`,
  plus the attribute-modifiers `className`, `elementId`, `rtl`, `ltr`
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

## More components

siriguela has no JS runtime available while a `View` tree is being built — `renderToString` and
`mount()` are the only two ways it ever becomes real HTML/DOM, and `onClick`/`onInput` are the
only hook into either. So the components below are either plain structural/styling primitives, or
lean on a native HTML behavior/CSS-only trick to get real open/closed/toggled interactivity with
**zero JS**. Two ideas come up more than once, so they're explained here instead of per-component:

- **ID-reference attributes never care about DOM tree position.** `for`, `list`, and
  `popovertarget` all work by looking up an element by `id` anywhere in the document — wrapping
  two loosely-related pieces in a plain `el("div")`/`el("span")` (as `popover`/`combobox`/`toggle`
  below all do) is cosmetically inert, never a correctness compromise.
- **A handful of components need one static, instance-independent CSS rule** (`tooltipStyles`,
  `skeletonStyles`, `spinnerStyles`) that can't be injected globally from inside a single `View`.
  Call the matching `*Styles()` function once anywhere in your page tree — calling it more than
  once is harmless duplication, not an error.

**Structural/styling** (no interactivity): `avatar(src, alt)`, `badge(label)`, `alert(children)`,
`breadcrumb(items)`, `buttonGroup(children)`, `cardHeader/cardTitle/cardDescription/cardContent/
cardFooter` (composable `card` sub-parts), `empty(children)`,
`field(labelText, forId, control, description)`, `input(inputType, placeholder)` (text-like types
only — `email`/`password`/`number`/`date`/`tel`/`url`/`search`; `file`/`checkbox`/`radio` need
different attrs and already have their own components), `inputGroup(children)`, `item(children)`,
`kbd(content)`, `marker(content)`, `code(content)`/`codeBlock(content)` (inline code and a
`<pre><code>` block — samples are HTML-escaped for you, and newlines/tabs inside them survive
verbatim), `pagination(items)`, `progress(value, max)`,
`radioGroup(name, options)`, `scrollArea(maxHeightPx, child)` (the easy 80% — a scrollable,
height-capped box; not shadcn's actual differentiator, cross-browser scrollbar *styling*, which
needs pseudo-elements no `View` can represent), `separator()`, `slider(min, max, value)`,
`skeleton(widthPx, heightPx)`/`skeletonStyles()`, `spinner()`/`spinnerStyles()`,
`lead(content)`/`muted(content)`/`blockquote(content)` (typography presets), and the
attribute-modifiers `rtl(view)`/`ltr(view)`/`className(view, name)`/`elementId(view, value)`
(these wrap `attr()` rather than `style()` — `className`/`elementId` are how a tree hooks up to a
stylesheet rule or an anchor target, which is what inline styles alone can't express). A native
`<input type="date">` covers a basic Date Picker — just `input("date", placeholder)`, no dedicated
component.

**CSS-only interactive** (zero JS, via native HTML behavior or a well-known trick):
- `details(summary, children)` / `accordion(items)` — native `<details>`/`<summary>`, the browser
  handles open/close entirely on its own. Pipe `details(...) |> attr("name", "faq-group")` for
  true single-open-at-a-time behavior across several of them (a native `<details>` feature).
- `popover(id, trigger, content)` — the native `popover` attribute + `popovertarget`; the browser
  handles show/hide/click-outside-dismiss/Escape with no JS.
- `sheet(id, trigger, content)` — the same `popover` mechanism, styled as a slide-in panel.
  Explicitly **non-modal**: it doesn't trap Tab-key focus or block the rest of the page the way a
  true modal dialog would.
- `switchInput(checked)` — `checkbox` styled as a toggle switch, no new mechanism.
- `toggle(id, toggleLabel, pressed)` / `toggleGroup(items)` — a hidden `checkbox` + `label`,
  styleable via `input:checked + label {...}`.
- `tooltip(content, child)` / `tooltipStyles()` — sets a `data-tooltip` attribute per instance;
  the one shared rule uses CSS's own `content: attr(data-tooltip)`, so it needs no per-instance
  interpolation at all.
- `combobox(inputPlaceholder, datalistId, options)` — `<input list=...>` + `<datalist>`, native
  browser autocomplete. `options` reuses the existing `option(...)` component unchanged.
- `inputOtp(groupId, length)` — ships the static N-single-char-input markup only, with
  predictable ids (`"${groupId}-0"`, `"${groupId}-1"`, ...). Does **not** implement
  auto-advance-focus between boxes — wire that yourself via your own `onInput` handler calling
  `browser.getElementById`/`browser.focus`, the same "handlers re-fetch state fresh" pattern
  `onClick`/`onInput` already use everywhere else.
- `tab(id, label, content)` / `tabs(groupName, items)` — the classic hidden-radio-inputs +
  CSS-sibling-selector trick:
  ```caja
  import { tab, tabs, text, renderToString } from "@caja/siriguela"

  let page = tabs("demo", [
  	tab("one", "One", text("Content 1")),
  	tab("two", "Two", text("Content 2"))
  ])
  ```
  The first tab starts selected. `groupName`/`id` uniqueness across independent `tabs()` calls on
  one page is your responsibility, same as `popover`'s `id`.

Every generated CSS block in this file (`tabStyleRules`, `tooltipStyles`, `skeletonStyles`,
`spinnerStyles`) is written to only ever use space (descendant), `~` (general sibling), `+`
(adjacent sibling), `,`, `#`, and `:pseudo-class`/`::pseudo-element` selectors — never `>`, `<`,
or `&`. `renderToString`'s HTML-escaping runs on `<style>` text the same as any other tag's, and a
literal `>` would come out as `&gt;` in the page source — invisible to a human reading the output,
but silently broken CSS. If you write your own CSS-emitting component, follow the same rule.

## Compose-style DSL syntax

Combining two Caja language features that already existed before this library did — named
imports and trailing-block call sugar — gives every `[View]`-taking component a nested,
Kotlin/Jetpack-Compose-style syntax, with **no changes needed to siriguela itself**:

```caja
import { card, pageHeading, text, primaryButton } from "@caja/siriguela"

let page = card() {
	pageHeading("Hello")
	text("Welcome to your new project.")
	primaryButton("Get started")
}
```

`card() { ... }` desugars purely at parse time into
`card([pageHeading(...), text(...), primaryButton(...)])` — the ordinary call you'd write by hand,
just without the `[...]` and `|>` plumbing. This works for any component whose only parameter is
`[View]`: `column`, `row`, `card`, `section`, `article`, `nav`, `header`, `footer`, `mainEl`,
`aside`, `form`, `unorderedList`, `orderedList`, `table`, `tableRow`, `select` — and it nests:

```caja
column() {
	row() {
		text("left")
		text("right")
	}
}
```

**The parens before `{` are required.** `card { ... }` (no parens) is not the same syntax — a bare
identifier followed by `{` parses as an unrelated (and here invalid) struct literal instead.
Always write `card()`, `column()`, `select()`, etc., even with no other arguments.

**Only bare expression statements are allowed inside `{ }`** — no `let`, `return`, or assignment;
each line becomes one array element. For anything computed — a data-driven `select`/`option` list
built from a collection, or a conditionally-included child — build the `[View]` array beforehand
with ordinary code, then pass it to the component in the plain, non-block style. Note named
imports only bring in *values* (functions/constants), not the `View` type itself — keep a plain
`import "@caja/siriguela" as siriguela` alongside a named import when a helper function's own
signature needs to name the type, as below:

```caja
import "@caja/siriguela" as siriguela
import { select, option } from "@caja/siriguela"
import array

const countryOptions = fn(countries: [String]) -> [siriguela.View] {
	if (array.len(countries) == 0) {
		return []
	}
	let name = array.head(countries)
	return countryOptions(array.tail(countries)) |> array.push(option(name, name, false))
}

let dropdown = select(countryOptions(["Brazil", "Canada", "Denmark"]))
```

One gotcha worth knowing: an `if`/`if-else` written as a bare statement inside `{ }` is **not**
rejected by the parser (Caja treats `if` as an expression, not only a statement), but currently
fails to compile once it reaches Go codegen — a pre-existing gap in the trailing-block language
feature itself, not specific to siriguela. Avoid it inside `{ }`; build any conditional child with
a `let` beforehand instead, same as the data-driven case above.

`onClick`/`onInput` read nicely with the same language's zero-parameter trailing-lambda sugar
(`call(args) => { ... }`, no identifier before the `=>`):

```caja
primaryButton("Click")
	|> onClick() => {
		browser.alert("hi")
	}
```

instead of `|> onClick(fn() -> Nothing { browser.alert("hi") })`. A handler still can't close over
an outer `let`/`const` (see "Events" above) — the sugar only changes how the function literal is
spelled, not its semantics.

`box(child: View)` is the one catalog function that does **not** support the trailing-block sugar
— it takes a single `View`, not `[View]`, so `box() { ... }` would try to pass an array where a
single view is expected (a type error). By design: `box` is a single-child passthrough, not a
container.

## Multi-page sites

A site is a list of pages. siriguela gives you the page value and the two things every page of a
site shares — navigation and the HTML document shell — and deliberately nothing that writes a file:

```caja
type Page struct { title String, path String, content View }

definePage(title, path, content) -> Page      # path is the output file AND the href: "about.html"
navLinks(pages, currentPath) -> View          # <nav> of links; the current one gets aria-current='page'
htmlDocument(title, stylesheetHref, body) -> String   # "<!doctype html>" + html/head/body, body last (trailing block works)
```

Writing is the `doc` builtin module's job, and it must stay in the project's `main.caja`: `caja run`
refuses any program whose transpiled output calls `doc.write`, and cross-module transpilation
emits every top-level statement of every import — so a `doc.write` anywhere in this library would
make every program that imports it un-runnable. The split is one page module per file under
`pages/`, each exporting `let page = definePage(...)`, and a `main.caja` that lists them and loops:

```caja
import "@caja/siriguela" as ui
import doc
import array
import "pages/index" as home
import "pages/about" as about

let site: [ui.Page] = [home.page, about.page]

const layout = fn(current: ui.Page, all: [ui.Page]) -> [ui.View] {
	let shell: [ui.View] = [ui.navLinks(all, current.path), current.content]
	return shell
}

const writeSite = fn(remaining: [ui.Page], all: [ui.Page]) -> Nothing {
	if (array.len(remaining) == 0) {
		return
	}
	let current = array.head(remaining)
	doc.write("dist/${current.path}", ui.htmlDocument(current.title, "style.css", layout(current, all)))
	return writeSite(array.tail(remaining), all)
}

writeSite(site, site)
```

A page module's own `definePage` call reads better with named arguments — `title`/`path` are two
adjacent strings, and only the parameter names tell them apart:

```caja
let page = ui.definePage(title: "About", path: "about.html", content: content)
```

`caja init --type static-page` scaffolds exactly this shape (plus an `assets/` folder that
`caja build`/`serve` copy into `dist/assets/`). Two constraints the module system imposes: spell
the `"@caja/siriguela"` import identically in every file (the specifier is the module cache key —
two spellings compile the library twice, with two incompatible `Page` types), and don't define
another struct named `Page` in a page module (struct identity is by name).

## CSS classes from Caja

Inline styles can't express a `:hover`, a `:focus`, a pseudo-element or a media query. A `Class`
is a stylesheet rule defined in Caja, built by the **same modifier chain** that styles a View — so
the closed `CSSProperty` enum still catches a misspelled property at compile time, and nothing is
duplicated:

```caja
let card = ui.class("card", ui.styles() |> ui.padding("8px") |> ui.rounded("6px"))
	|> ui.variant("hover", ui.styles() |> ui.boxShadow("0 4px 12px #0002"))

let hero = ui.box(content) |> ui.withClass(card)          # class='card'
let both = ui.box(content) |> ui.withClasses([card, pill]) # class='card pill' — ONE attribute

doc.write("dist/style.css", ui.classCss([card, pill]))     # ".card{padding:8px;border-radius:6px;}.card:hover{...}.pill{...}"
ui.classStylesheet([card])                                 # the same rules as a <style> View, for inside a page
```

- `styles()` is an empty View whose only purpose is to collect modifier output; `class(name,
  template)` lifts its `.styles` into a `Class`; `variant(cls, pseudo, template)` appends a
  `.name:pseudo` rule (it takes the Class first so it pipes).
- `withClass` appends a `class` attribute each call — two calls give two attributes. Use
  `withClasses` to combine several.
- `classCss` written to a `.css` file has no character restrictions. `classStylesheet` goes through
  `renderToString`'s escaping like any other `<style>` View, so its values must avoid `<`, `>`, `&`
  (the same rule `tabs`/`tooltipStyles` live under). A `'` survives both routes, so
  `fontFamily("'Inter', sans-serif")` works everywhere.

## Arbitrary CSS rules

`Class`/`variant` cover one class name plus its pseudo-states — the case with a fixed shape to hang
a modifier chain off of. A real stylesheet also needs element selectors, comma-grouped selectors,
descendant/child/sibling combinators, attribute selectors and media queries, none of which have a
set of properties to validate — a selector isn't CSS declarations, it's arbitrary CSS syntax. `Rule`
keeps the type-checked half (declarations, via the same modifier chain and `CSSProperty` enum) and
takes the selector as plain text, unchecked, exactly like a selector inside a template literal in
any CSS-in-`<language>` library:

```caja
let rules: [ui.Rule] = [
	ui.rule("h1, h2, h3", ui.styles() |> ui.lineHeight("1.25")),
	ui.rule(".tabs > input[type=radio]", ui.styles() |> ui.position("absolute") |> ui.opacity("0")),
	ui.rule(".site-nav a[aria-current=page]", ui.styles() |> ui.color("#7A2048"))
]

let responsive: [ui.MediaQuery] = [
	ui.media("(max-width: 640px)", [ui.rule(".page", ui.styles() |> ui.gap("28px"))])
]

doc.write("dist/style.css", ui.stylesheet(rules, [card], responsive))
```

- `rule(selector, template)` builds a `Rule` the same way `class` builds a `Class`; `rulesCss`
  renders a list of them (`"h1, h2, h3{line-height:1.25;}..."`); `rulesStylesheet` is the
  `renderToString`-safe `<style>` View equivalent, under the same `<`/`>`/`&` restriction as
  `classStylesheet`.
- `media(query, rules)` groups Rules under one `@media` condition; `mediaCss` renders a list of
  them. `query` is raw condition text, unchecked for the same reason a selector is.
- `stylesheet(rules, classes, queries)` is the one-call way to render a whole site's CSS: rules
  first (resets, elements, combinators), then classes, then media queries last — the cascade order
  a hand-written stylesheet would use, so a responsive rule still wins at equal specificity. The
  static-page scaffold's `main.caja` builds its entire `dist/style.css` this way.

## Default theme

Named after the fruit itself — deep plum-red skin, warm amber-orange flesh:

| Constant | Hex |
|---|---|
| `siriguelaSkin` | `#7A2048` |
| `siriguelaFlesh` | `#F0A23A` |
| `siriguelaLeaf` | `#6E8B3D` |
| `siriguelaCream` | `#FBF3E6` |
| `siriguelaInk` | `#2B1710` |
| `siriguelaDanger` | `#B3261E` |

Plus pre-styled wrapper functions built on the catalog above: `primaryButton(label)`,
`secondaryButton(label)`, `card(children)`, `pageHeading(content)`, `themedLink(href, label)`.
None of this is a required layer — a project that never calls any of it is exactly as valid as
one that uses it everywhere.

Two of the components from "More components" above also get a palette-aware variant:
`themedAlert(variant, message)` and `themedBadge(variant, label)`, where `variant` is a closed
`Variant` enum (`Variant.Info`/`Success`/`Warning`/`Danger`, the same closed-enum mechanism
`CSSProperty` itself uses — an unrecognized variant is a compile-time error, not a typo that
silently falls through). Kept deliberately small — two variant-aware components, not a
variant-per-function explosion — everything else in "More components" stays unstyled/structural
by design, matching this theme layer's existing scope.

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
