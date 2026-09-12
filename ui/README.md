# @caja/ui

The renderer-agnostic building blocks for generating HTML and CSS from
[Caja](https://www.cajalang.com): a `View` tree, the `view`/`attr`/`style` primitives, the full
modifier chain, the event-handler surface, a catalog of structural (unstyled) components, the
`renderToString` terminal, the CSS-in-Caja APIs (`Class`, `Rule`, `MediaQuery`, `Keyframes`), and
the page/document helpers.

Two closed enums do the type-checking on the public surface: **`HTMLElement`** (every tag
`view()` accepts) and **`CSSProperty`** (every property `style()` accepts). Each turns a typo into
a compile-time error at the call site instead of an attribute or style that silently does nothing.
Events are covered the same way by a private `DOMEvent` enum behind the named handler functions.

A static page can carry real JavaScript, built as a tree by [`@caja/js`](../js) — see
[Events](#events) and [JavaScript](#javascript).

**This package has no opinion about how anything looks.** There is no palette, no spacing scale,
no default colors — every component here emits either plain structural markup or only the inline
styles its own layout semantics require (`column` sets `flex-direction`, not a `gap` you didn't
ask for). Theming is a separate package built on top of this one:

- [`@caja/siriguela`](../siriguela) is the default theme. It builds its own, smaller component
  vocabulary on top of this catalog — it does **not** re-export it, so a siriguela consumer writes
  siriguela and never sees the names below.
- A custom theme is just another `.caja` module of wrappers over these same functions. Import
  this package directly and you never pull in a palette you intend to replace.

So this README is for two audiences: someone writing a theme, and someone who wants the
structural layer with no theme at all. If you are just building a site and are happy with the
default look, [siriguela's README](../siriguela/README.md) is the one you want.

Render the same tree as an HTML string (`static-page` projects) or mount it live in the DOM
(`web-app` projects, via the separate [`@caja/dom`](../dom) package).

## Install

Not yet published to npm — a fresh `caja init` needs a manual `node_modules` setup pointing at
this package in the meantime. Once published:

```sh
npm install @caja/ui
```

## Package layout

Everything lives in a single `index.caja`, in nine numbered sections, primitives first and the
component catalog and site model last:

```
1. Enums          HTMLElement, CSSProperty, DOMEvent
2. Core types     Handler, Attr, Style, View
3. Primitives     view/customView, attr, style, addChild(ren)
4. Modifier chain one pipeable function per CSSProperty, plus the attribute modifiers
5. Events         a named handler function per event, x2 (Caja fn / JavaScript)
6. Rendering      escaping, then renderToString
7. CSS in Caja    Class, Rule, MediaQuery, Keyframes, stylesheet
8. Components     the structural catalog
9. Site model     Page, navLinks, htmlDocument
```

That order is not just tidiness: Caja has no forward references between separate bindings (only a
function may reference its *own* name from inside its own body), so the reading order **is** the
dependency order. A component that needs a `Rule` has to be declared after the `Rule` section.

Keeping the base function set in one file means it is fully visible in one place, with nothing
held back behind an internal boundary — which is exactly what a theme author needs.
## Quick start — `static-page`

A `static-page` project only ever needs this package. `renderToString` has no browser
dependency at all, so importing only `@caja/siriguela` (never `@caja/dom`) is what
keeps `caja build` compiling a plain native binary instead of `wasm` — merely importing a module
that contains any browser-touching code (even code nothing calls) forces the whole compiled
binary into `wasm`-only mode, which is why `mount()`/`sync()` live in the separate
`@caja/dom` package instead of here.

```caja
import "@caja/ui" as ui
import doc

let card = ui.view(ui.HTMLElement.Div)
	|> ui.padding("20px")
	|> ui.addChild(ui.heading(1, "Hello"))
	|> ui.addChild(ui.text("Welcome to your new project."))

doc.write("dist/index.html", ui.renderToString(card))
```

## Quick start — `web-app`

A `web-app` project additionally needs [`@caja/dom`](../dom) for `mount()`
— see that package's own README for why it's a separate package and the import-alias caveat its
hyphenated name needs.

```caja
import "@caja/ui" as ui
import "@caja/dom" as dom
import browser

let handleClick = fn() -> Nothing {
	browser.alert("hi")
}

let panel = ui.view(ui.HTMLElement.Div)
	|> ui.addChild(ui.button("Click me") |> ui.onClick(handleClick))

dom.mount(panel, browser.getElementById("app"))
```

`caja init --type static-page` / `--type web-app` scaffold exactly this shape by default.

## Component catalog

- **Tree**: `view(tag: HTMLElement)`, `customView(tag: String)`, `attr(target, name, value)`,
  `addChild(target, child)` — see "Tags are typed" below
- **Modifiers**: every one is `v |> modifier(value)`, chainable —
  `padding`, `margin`, `backgroundColor`, `color`, `fontSize`, `fontFamily`, `rounded`,
  `display`, `flexDirection`, `flexBasis`,
  `width`, `height`, `minWidth`, `maxWidth`, `minHeight`, `maxHeight`, `gap`,
  `alignItems`, `justifyContent`, `alignSelf`, `flexGrow`, `flexShrink`, `flexWrap`,
  `gridTemplateColumns`, `gridTemplateRows`,
  `fontWeight`, `lineHeight`, `textAlign`, `letterSpacing`, `textDecoration`, `whiteSpace`,
  `textTransform`,
  `background`, `paddingTop`, `paddingBottom`, `marginTop`,
  `border`, `borderWidth`, `borderColor`, `borderStyle`, `borderTop`, `borderBottom`,
  `borderCollapse`, `boxShadow`, `opacity`, `cursor`, `pointerEvents`,
  `overflow`, `overflowX`, `verticalAlign`,
  `position`, `top`, `right`, `bottom`, `left`, `zIndex`,
  `transition`, `transform`, `boxSizing`, `aspectRatio`, `animation`, `pseudoContent`,
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
- **Events**: one function per event, in two families — `onClick`/`onInput`/`onKeyDown`/… take a
  Caja function (web-app builds), and `onClick`/`onInput`/… take JavaScript (static
  pages). ~90 events each; see "Events" below
- **Terminal**: `renderToString(target) -> String`

`View`, `Handler`, `Attr`, `Style`, `style(target, property, value)` and both public enums are
also exported, for anyone building components below the catalog level — see "Adding a CSS
property" below.

## Tags are typed

`view()` takes an `HTMLElement`, the closed set of tag names from the HTML Living Standard —
grouped in `index.caja` the way the spec groups them, and with obsolete elements (`<center>`,
`<font>`, `<marquee>`) deliberately left out:

```caja
view(HTMLElement.Div)         # <div>
view(HTMLElement.Section)     # <section>
view("div")                   # compile error: argument 1 expected HTMLElement, got String
view(HTMLElement.Dvi)         # compile error: 'Dvi' is not a member of enum 'HTMLElement'
```

For anything the enum doesn't cover — a web component, an SVG or MathML child — `customView`
takes a raw, unchecked `String`:

```caja
customView("my-widget")       # <my-widget>
customView("circle")          # an SVG child
```

`customView` is the escape hatch, not the front door: nothing validates the string, so a typo
behaves exactly the way it did before the enum existed. Reach for `view()` unless you actually
need a tag outside HTML.

Enum member names never collide with anything else in the library, even where they read the same
(`HTMLElement.Style` alongside the `Style` type, `HTMLElement.Option` alongside `option()`): an
enum's members live in the enum's own namespace, reachable only through `HTMLElement.`.

## Events

This package's handlers take **JavaScript**, built with [`@caja/js`](../js), and there are 90 of
them — one per event:

```caja
button("Save") |> onClick([jsAlert(jsStr("saved"))])
```

They are emitted as an inline event attribute, so the browser runs them with no runtime of ours
involved. That means they work in **every** target: a static page, and a mounted web-app.

The other kind of handler — one taking a Caja `fn() -> Nothing` — lives in
[`@caja/dom`](../dom), not here:

| | takes | attached by | works in |
|---|---|---|---|
| `onClick`, `onInput`, … (**here**) | a `[JsStmt]` | the browser, as an inline attribute | every target |
| `onClick`, `onInput`, … (**`@caja/dom`**) | a Caja `fn() -> Nothing` | `mount()` | `web-app` (wasm) only |

**The names are identical on purpose** — `onClick` is `onClick`, and which one you get is which
package you reached into. Wildcard-importing both and writing a bare `onClick` is a clear
compile-time error, not a silent pick:

```
semantic error: ambiguous reference to 'onClick': wildcard-imported from 'ui' and 'dom'.
Suggestion: qualify it (ui.onClick or dom.onClick)
```

So the usual shape is to wildcard-import this package and qualify the other:

```caja
import * from "@caja/ui"
import "@caja/dom" as dom

button("js")   |> onClick([jsAlert(jsStr("hi"))])            # JavaScript, every target
button("caja") |> dom.onClick(fn() -> Nothing { ... })        # Caja code, wasm only
```

The split is by what makes the handler work. Running a Caja function in a browser needs a Caja
runtime there, which only a wasm build ships — so those functions live next to the `mount()` that
attaches them. A static page has no such runtime: `main.caja` ran at build time and produced HTML,
so `renderToString` drops a Caja handler and says so with a `log.warn` at `caja build` time.

Coverage is identical in both families, and is every element-attachable event: mouse, pointer,
touch, keyboard, form and input, focus, clipboard, drag-and-drop, scroll, loading,
disclosure/dialog, media, animation, transition and slots. Window- and document-scoped events
(`beforeunload`, `DOMContentLoaded`, `popstate`, `visibilitychange`, `online`) are deliberately
**absent** — a handler attaches to the element a `View` becomes, so one of those could never fire
from here; call `browser.on` directly for those.

There is no generic `on(target, event, program)` in the public API, and no exported `DOMEvent`
enum. The named function *is* the event: it is not a runtime choice, so nothing is gained by
making it a value you can pass around, and a closed enum nobody can pass anywhere is just noise.

A JavaScript handler cannot see anything from your Caja program — it is text sent to a browser, so
nothing of yours is in scope there. What it *can* do is be built from Caja values, which is the
point of `@caja/js`: the data is baked in at build time, escaped on the way.

## JavaScript

`onClick` and `scriptBlock` take a **statement tree** from [`@caja/js`](../js), not a
string. The JavaScript is built the way this package builds HTML, and rendered at the end:

```caja
import * from "@caja/js"

let out = paragraph("Nothing yet.") |> elementId("out")

let b = button("Run it")
	|> onClick([
		jsSetText(jsGetElementById("out"), jsStr("clicked"))
	])

let helper = scriptBlock([
	jsFunction("greet", ["n"], [jsLog(jsAdd(jsStr("hi "), jsIdent("n")))])
])
```

Two things make that safe, and they matter more than the syntax:

- **It is correct by construction.** Expressions and statements are separate types, so a statement
  cannot land where an expression belongs; `jsStr` escapes whatever you hand it, so Caja data
  crossing into JavaScript is inert text rather than syntax.
- **It can depend on build-time values** — a page list, a label, a generated table. That is the
  case a literal cannot reach at all, and the main reason to prefer a tree over a string.

For JavaScript you would rather just write, the `js` **builtin** module takes a literal and parses
it at compile time with a real JavaScript parser; `jsRawStmt` drops the result into a built
program:

```caja
import js

button("Save") |> onClick([jsRawStmt(js.raw("alert('saved')"))])
```

A malformed script is a Caja build error pointing at the `js.raw` call — not a page that loads and
silently does nothing:

```
invalid JavaScript in 'js.raw': Expected ")" but found end of file (line 1 of the script)
```

`js.raw`'s argument must be a string **literal**: that is what puts the source in front of the
validator while the compiler is still running, and a script assembled at runtime could not be
checked at all. A `js.Script` is also the only thing `jsRaw`/`jsRawStmt` accept, which is what
keeps the single door for arbitrary syntax a door that validates — see
[@caja/js's README](../js/README.md) for how that holds up against hand-built nodes.

One Caja-level gotcha: a JS **template literal** needs its `${...}` written `\${...}`, because Caja
interpolates `${...}` inside its own string literals. An unescaped one is evaluated at build time
— and, being an interpolated string rather than a literal, rejected by `js.raw`.

`scriptBlock` is for JavaScript that is not one element's handler — a shared helper, some setup on
load. Where it sits in the tree matters exactly as in a hand-written page: a script runs when the
parser reaches it, so one that touches elements declared after it must come after them.

### How the two are escaped

Different, and both correct:

- An **inline handler attribute** goes through ordinary attribute escaping, so a single quote in
  the JavaScript renders as `&#39;`. That is not a bug: a browser entity-decodes an attribute
  value *before* interpreting it as JavaScript, so the quotes arrive intact.
- A **`<script>` body** is emitted verbatim. `<script>`, `<style>`, `<textarea>` and `<title>` are
  HTML raw-text elements — a browser never decodes entities inside them — so escaping their
  content would corrupt it rather than protect anything.

## More components

There is no JS runtime available while a `View` tree is being built — `renderToString` and
`mount()` are the only two ways it ever becomes real HTML/DOM, and `onClick`/`onInput` are the
only hook into either. So the components below are either plain structural/styling primitives, or
lean on a native HTML behavior/CSS-only trick to get real open/closed/toggled interactivity with
**zero JS**. Two ideas come up more than once, so they're explained here instead of per-component:

- **ID-reference attributes never care about DOM tree position.** `for`, `list`, and
  `popovertarget` all work by looking up an element by `id` anywhere in the document — wrapping
  two loosely-related pieces in a plain `view(HTMLElement.Div)`/`view(HTMLElement.Span)` (as
  `popover`/`combobox`/`toggle` below all do) is cosmetically inert, never a correctness
  compromise.
- **A handful of components need static, instance-independent CSS** that can't be injected
  globally from inside a single `View`. Each exposes it twice: as a Caja value
  (`tooltipRules()`/`tabRules(...)` return `[Rule]`; `pulseKeyframes()`/`spinKeyframes()` return
  `Keyframes`) and as a ready-made `<style>` View wrapping it
  (`tooltipStyles()`/`skeletonStyles()`/`spinnerStyles()`).
  **Prefer the Caja value**: fold it into your own `stylesheet()` call and the rules ship once, in
  the `.css` file, with no per-page `<style>` block and none of the `<`/`>`/`&` escaping
  constraint. The `<style>` wrapper is for the quick case and for a `mount()`-only app with no
  stylesheet at all — call it once anywhere in your page tree; calling it more than once is
  harmless duplication, not an error.

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
- `tooltip(content, child)` / `tooltipRules()` / `tooltipStyles()` — sets a `data-tooltip`
  attribute per instance; the shared rules use CSS's own `content: attr(data-tooltip)`, so they
  need no per-instance interpolation at all.
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
  import { tab, tabs, text, renderToString } from "@caja/ui"

  let page = tabs("demo", [
  	tab("one", "One", text("Content 1")),
  	tab("two", "Two", text("Content 2"))
  ])
  ```
  The first tab starts selected. `groupName`/`id` uniqueness across independent `tabs()` calls on
  one page is your responsibility, same as `popover`'s `id`.

The generated CSS blocks in this library (`tabRules`, `tooltipRules`, `pulseKeyframes`,
`spinKeyframes`) use only space (descendant), `~` (general sibling), `+` (adjacent sibling), `,`,
`#` and `:pseudo-class`/`::pseudo-element` selectors. That used to be a hard requirement —
`<style>` text was HTML-escaped like any other, so a literal `>` came out as `&gt;`: invisible to
a human reading the output, but silently broken CSS. `<style>` is now treated as the raw-text
element it is, so `>` works fine and your own CSS-emitting components are under no such
restriction.

## Compose-style DSL syntax

Combining two Caja language features that already existed before this library did — named
imports and trailing-block call sugar — gives every `[View]`-taking component a nested,
Kotlin/Jetpack-Compose-style syntax, with **no changes needed to this library itself**:

```caja
import { column, heading, text, button } from "@caja/ui"

let page = column() {
	heading(1, "Hello")
	text("Welcome to your new project.")
	button("Get started")
}
```

`column() { ... }` desugars purely at parse time into
`column([heading(...), text(...), button(...)])` — the ordinary call you'd write by hand,
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
with ordinary code, then pass it to the component in the plain, non-block style.

A named import brings in types and enums as well as values, so `View` and `HTMLElement` can go in
the same list as the functions and no second qualified import is needed:

```caja
import { View, select, option } from "@caja/ui"
import array

const countryOptions = fn(countries: [String]) -> [View] {
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
feature itself, not specific to this library. Avoid it inside `{ }`; build any conditional child with
a `let` beforehand instead, same as the data-driven case above.

### Wildcard import — the simpler alternative

`import * from "mod"` (Caja's wildcard-import feature) supersedes the named-import-plus-qualified-
fallback dance above for full compose-style usage: it brings in **everything** unqualified —
values and types both — so there's no per-name list to maintain and no need for a second,
qualified import just to name `View`:

```caja
import * from "@caja/ui" as ui

const countryOptions = fn(countries: [String]) -> [View] {
	if (array.len(countries) == 0) {
		return []
	}
	let name = array.head(countries)
	return countryOptions(array.tail(countries)) |> array.push(option(name, name, false))
}

let dropdown = select(countryOptions(["Brazil", "Canada", "Denmark"]))
```

Combining it with an alias (`as ui`) is not required but is cheap insurance: `ui.` stays available
for the rare genuine collision, on top of every bare name. The trade-off named imports don't have:
a wildcard import brings in a name unconditionally, and a local declaration of that same name
**silently shadows** it rather than erroring (named imports, by contrast, raise a real "already
declared" conflict error if a local name collides). That means a future @caja/ui release adding a
new export could start silently shadowing — or being shadowed by — something already in your file,
with nothing to flag it at import time. Worth knowing before reaching for `import *` in a project
with a much larger, less predictable set of local names than a short scaffold.

`onClick`/`onInput` read nicely with the same language's zero-parameter trailing-lambda sugar
(`call(args) => { ... }`, no identifier before the `=>`):

```caja
button("Click")
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

A site is a list of pages. @caja/ui gives you the page value and the two things every page of a
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
import "@caja/ui" as ui
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
the `"@caja/ui"` import identically in every file (the specifier is the module cache key —
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

doc.write("dist/style.css", ui.stylesheet([spin], rules, [card], responsive))
```

- `rule(selector, template)` builds a `Rule` the same way `class` builds a `Class`; `rulesCss`
  renders a list of them (`"h1, h2, h3{line-height:1.25;}..."`); `rulesStylesheet` is the
  `renderToString`-safe `<style>` View equivalent, under the same `<`/`>`/`&` restriction as
  `classStylesheet`.
- `media(query, rules)` groups Rules under one `@media` condition; `mediaCss` renders a list of
  them. `query` is raw condition text, unchecked for the same reason a selector is.
- `stylesheet(frames, rules, classes, queries)` is the one-call way to render a whole site's CSS.
  The parameter order is the emit order, and the emit order is the cascade order a hand-written
  stylesheet would use: `@keyframes` first (at-rules, referenced by name and unaffected by the
  cascade), then rules (resets, elements, combinators), then classes, then media queries last — so
  a responsive rule still wins at equal specificity. `@caja/siriguela`'s `siteStylesheet()` builds
  a whole theme's CSS with one such call.

## Animations

`@keyframes` is the one shape in the CSS section that isn't "a selector plus declarations", so it
has its own type rather than reusing `Rule`: an `@keyframes` block nests a whole list of
declaration blocks inside itself, and a `Rule` is flat.

```caja
let spin = keyframes("spin", [
	keyframe("from", styles() |> transform("rotate(0deg)")),
	keyframe("to", styles() |> transform("rotate(360deg)"))
])

let pulse = keyframes("pulse", [
	keyframe("0%", styles() |> opacity("1")),
	keyframe("50%", styles() |> opacity("0.4")),
	keyframe("100%", styles() |> opacity("1"))
])

keyframesCss([spin, pulse])        # "@keyframes spin{from{transform:rotate(0deg);}...}"
keyframesStylesheet([spin])        # the same, as a <style> View
view(HTMLElement.Div) |> animation("spin 0.6s linear infinite")   # reference one by name
```

A `Keyframe`'s `offset` is raw text (`"0%"`, `"from"`, `"to"`) for the same reason a `Rule`'s
selector is: a keyframe selector is CSS syntax, not a set of properties. The declarations inside
each frame go through the ordinary modifier chain, so `CSSProperty` still checks them.

The library's own two animated components expose their keyframes this way —
`pulseKeyframes()` for `skeleton()` and `spinKeyframes()` for `spinner()` — so a site that uses
either adds them to its `stylesheet()` call instead of embedding a `<style>` block per page.

## Adding a CSS property

`style()`'s `property` parameter is a closed `CSSProperty` enum, not a plain string — passing an
unrecognized property name is a compile-time type error, not a silent no-op or a typo that
quietly does nothing. If a property you need doesn't have a modifier yet, add a `CSSProperty`
member and a one-line wrapper function directly in `index.caja`, following the existing pattern.

## Status

Draft, pre-1.0, developed alongside [`caja-cli`](https://github.com/caja-tech/caja-cli) itself.
