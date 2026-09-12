package analyzer

import (
	"caja-cli/internal/pipeline/analyzer/symbol"
	"caja-cli/internal/pipeline/ast"
	"caja-cli/internal/pipeline/environment"
	"fmt"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
)

// This file is the whole of the `js` builtin module's analysis, and the only
// place in the repo that depends on a JavaScript parser.
//
// Why the validation lives HERE, in the analyzer, rather than in the
// compiler: the analyzer is the one stage every entry point runs — `caja
// run`, `caja build`, and the LSP's per-keystroke re-analysis — so a
// malformed script is reported identically by all three, positioned at the
// call site, alongside every other semantic error. Putting it in the
// transpiler would have made `caja run` and the editor blind to it.
//
// Why it cannot live in the generated program instead: `caja build` compiles
// its output as a stdlib-only Go module (compiler.Compile does `go mod init
// caja_build` with CGO_ENABLED=0 and no dependency resolution at all), so
// emitted code has no way to import a parser. Validation therefore has to
// happen while the caja binary itself is still running, which is also why
// js.raw only accepts a string literal — see analyzeJsRawFunction.

// scriptEndTag is rejected outright in a script payload. Inside an HTML
// raw-text element the parser ends the element at the first "</script"
// regardless of JavaScript context — string literal, comment, anything — and
// entity-escaping cannot help, because a browser never decodes entities in
// raw text. So this is genuinely unrepresentable rather than merely awkward,
// and failing at compile time beats emitting a page that truncates itself.
const scriptEndTag = "</script"

// validateJavaScript parses code with esbuild and returns a human-readable
// problem description, or "" when the source is valid.
//
// esbuild is used purely as a parser here: Transform's output is discarded
// and only its error list is read. Parsing is what catches the mistake this
// module exists to catch — an unbalanced brace or a stray quote in a script
// that would otherwise reach a browser, fail silently in the console, and
// leave the page looking merely inert.
//
// Note what this does NOT check: nothing here knows whether the identifiers
// the script references actually exist at runtime. A syntactically perfect
// script calling a misspelled function still compiles. Syntax is the part a
// build step can be certain about.
func validateJavaScript(code string) string {
	if idx := strings.Index(strings.ToLower(code), scriptEndTag); idx >= 0 {
		return fmt.Sprintf("a script may not contain %q (at offset %d): it would end the <script> element early, and raw text cannot be escaped", scriptEndTag, idx)
	}

	result := api.Transform(code, api.TransformOptions{
		Loader: api.LoaderJS,
		// Report problems rather than writing them to stderr, and keep the
		// output unmodified — this is a syntax check, not a build step.
		LogLevel: api.LogLevelSilent,
	})
	if len(result.Errors) == 0 {
		return ""
	}

	first := result.Errors[0]
	if first.Location != nil {
		return fmt.Sprintf("%s (line %d of the script)", first.Text, first.Location.Line)
	}
	return first.Text
}

// analyzeJsRawFunction type-checks js.raw(code) and validates the JavaScript
// it carries.
//
// The argument must be a string LITERAL, not merely a String-typed
// expression. That restriction is the feature, not a shortcut: it is what
// puts the source in front of the validator at compile time. A script built
// by interpolation or returned from a function could not be checked at all,
// so accepting one would mean silently downgrading the guarantee this module
// advertises for every other call site. The literal is read straight off the
// AST, the same technique cast.to and string.format already use to make
// codegen decisions from an argument's shape.
func (a *Analyzer) analyzeJsRawFunction(n *ast.CallExpression) symbol.Symbol {
	scriptSymbol := symbol.NewBasicSymbol(environment.SCRIPT_OBJ)

	if len(n.Arguments) != 1 {
		a.reportError(n.Token, fmt.Sprintf("arity error: expected 1 argument for 'raw', got %d", len(n.Arguments)))
		return scriptSymbol
	}

	literal, isLiteral := n.Arguments[0].(*ast.StringLiteral)
	if !isLiteral {
		// Analyze the argument anyway so its own errors still surface and it
		// doesn't look unreferenced.
		a.analyze(n.Arguments[0])
		a.reportError(n.Token, "type error: the argument to 'js.raw' must be a string literal, so the JavaScript can be validated at compile time — a script assembled at runtime cannot be checked")
		return scriptSymbol
	}

	if problem := validateJavaScript(literal.Value); problem != "" {
		a.reportError(n.Token, "invalid JavaScript in 'js.raw': "+problem)
	}

	return scriptSymbol
}
