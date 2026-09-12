# @caja/siriguela

The default theme for [`@caja/ui`](../ui), for [Caja](https://www.cajalang.com).

A palette, the components that apply it, the page/site model, and the stylesheet that ties them
together — so a project gets a coherent look without writing any CSS.

```caja
import * from "@caja/siriguela"
import * from "@caja/ui"
import doc

let content = pageContent() {
	pageHeading("Hello")
	paragraph("Welcome.")
	primaryButton("Get started")
}

let home = definePage(title: "Home", path: "index.html", content: content)
let site: [Page] = [home]

doc.write("dist/index.html", renderPage(home, site, "A footer note.", "style.css"))
doc.write("dist/style.css", siteStylesheet())
```

## This package does not re-export @caja/ui

Every one of the 42 names siriguela exports is **its own declaration**. It does not forward
@caja/ui's ~280-name structural catalog.

That is a deliberate design choice, not an omission. A theme that forwarded the whole catalog
would be a theme in name only: your code would depend on which names happen to be forwarded, and
the line the two packages exist to draw would blur immediately. Composing instead means every name
here is a decision — `docSection` is a section *with* a heading, an anchor and a class, not a
`section` you still have to attach those to.

siriguela's surface is therefore much smaller than @caja/ui's, on purpose. **Import both.** That
is the intended shape and what the scaffold does: reach for a themed component here, and for
anything structural — `Page`, an event handler, `column`, a `Rule` — reach into @caja/ui directly.
The two export surfaces are disjoint (verified, not assumed), so wildcard-importing both is
collision-free.

How "re-exports nothing" holds up mechanically: this package reaches @caja/ui through
`import * from "@caja/ui"`, and a wildcard import is deliberately **not** forwarded to the
importing module's own consumers — only a named import is. So there is nothing to leak, by
construction rather than by discipline.

The one rule that comes with a wildcard import: a same-named local declaration *silently* shadows
the imported name, with none of the "already declared" error a named import would give. So nothing
in this package may declare a name @caja/ui already exports — a `const onClick` here would shadow
ui's and turn `return onClick(...)` into infinite self-recursion rather than the delegation it
looks like.

## Install

Neither package is published to npm yet. `caja init --type static-page` does scaffold a
`package.json` declaring both and runs your package manager's install — but until publication
that install can only fail (with a warning, not a fatal error), so a fresh project still needs a
manual `node_modules` setup pointing at **both** packages:

```
node_modules/@caja/siriguela   ->  this package
node_modules/@caja/ui          ->  ../ui
```

Once published, the scaffolded `package.json` is all it takes:

```sh
npm install @caja/siriguela @caja/ui
```

## Palette

Named after the fruit itself — deep plum-red skin, warm amber-orange flesh:

| Constant | Hex |
|---|---|
| `siriguelaSkin` | `#7A2048` |
| `siriguelaFlesh` | `#F0A23A` |
| `siriguelaLeaf` | `#6E8B3D` |
| `siriguelaCream` | `#FBF3E6` |
| `siriguelaInk` | `#2B1710` |
| `siriguelaDanger` | `#B3261E` |

## Components

**Themed** — the parts that carry the look:
`primaryButton(label)`, `secondaryButton(label)`, `card(children)`, `pageHeading(content)`,
`themedLink(href, label)`, `heroPanel(src, alt, title, subtitle)`, and the two that vary by
meaning, `themedAlert(variant, message)` and `themedBadge(variant, label)`, where `variant` is the
closed `Variant` enum (`Info`/`Success`/`Warning`/`Danger` — an unknown one is a compile-time
error, not a typo that silently falls through).

Event handlers are **not** in this list, and deliberately so: a theme has no opinion about an
event. `onClick`/`onInput`/… take JavaScript built with @caja/js and come from
@caja/ui; `onClick`/`onInput`/… take a Caja function and come from @caja/dom, which is the package
that can attach one.

**Prose and layout** — a structural element plus this theme's spacing/typography decision:
`pageContent`, `sectionBody`, `stack`, `inlineRow`, `paragraph`, `leadText`, `mutedText`,
`sectionHeading`, `subheading`, `inlineCode`, `codeSample`, `divider`, `bulletList`,
`guideSection`, `imageBlock`.

**Documentation** — the units a reference page is written in:
`docSection(anchor, title, children)`, `componentDoc(names, summary, sample, demo)`,
`masthead(title, tagline, badges)`, `tocLink`/`tocNav`, `paletteRow`/`paletteTable`,
`docTab`/`docTabs`.

Anything taking a list of children accepts Caja's trailing-block sugar, so the brackets and commas
disappear:

```caja
stack() {
	subheading("Layout")
	paragraph("Body copy.")
}
```

## The site chrome

`Page`, `definePage` and `navLinks` live in [@caja/ui](../ui), not here. A page with a title, an
output path and some content — and a nav link that knows which page is current — are universal,
with nothing theme-specific to decide. What *is* this theme's decision is how the chrome around
them looks, which is all that remains here:

```caja
siteNav(pages, currentPath) -> View          # ui's navLinks with this theme's class on it
renderPage(current, all, footerNote, stylesheetHref) -> String
siteStylesheet() -> String
```

`Page` and `definePage` come from @caja/ui, so a project that writes pages imports both packages
(which is why the example at the top of this file has two wildcard imports).

`renderPage` is everything between a `Page` and the file you write: the shared chrome (navigation,
footer) wrapped in a complete HTML document. `siteStylesheet` is the theme's entire CSS — base
rules, chrome, the CSS-only interactive components, hover states and the responsive block.

Neither writes a file. Writing stays in the consuming project's `main.caja`, which is what keeps a
program that imports this library runnable with `caja run` (a library calling `doc.write` would
make every consumer un-runnable, since cross-module transpilation emits every top-level statement
of an import).

## Building your own theme

A theme is an ordinary `.caja` module that imports @caja/ui and exports its own components —
exactly what this package is. Build on @caja/ui, not on siriguela, so you are not inheriting a
palette you are about to replace:

```caja
import "@caja/ui" as ui

const brandPrimary = "#1D7A85"

const brandButton = fn(label: String) -> ui.View {
	return ui.button(label)
		|> ui.backgroundColor(brandPrimary)
		|> ui.color("#FFFFFF")
		|> ui.rounded("6px")
		|> ui.padding("8px 16px")
}
```

Two things that shape how this file is written, both worth knowing before you copy it:

- `brandPrimary` is a `const`, not a `let`. Caja's purity rules let a function read an outer
  `const` but never an outer `let`.
- Everything from @caja/ui is reached through a wildcard import, which keeps a theme from
  re-exporting the layer it is built on: a named import IS forwarded to your theme's consumers, a
  wildcard one is not. A plain qualified import (`import "@caja/ui" as ui`, then `ui.button`,
  `-> ui.View`) forwards nothing either and reads more explicitly — pick whichever you prefer;
  only a *named* import leaks.

See [@caja/ui's README](../ui/README.md) for the structural catalog you are building on.

## Status

Draft, pre-1.0, developed alongside [`caja-cli`](https://github.com/caja-tech/caja-cli) itself.
