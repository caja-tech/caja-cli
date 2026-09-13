package analyzer

import (
	"caja-cli/internal/pipeline/analyzer/symbol"
	"caja-cli/internal/pipeline/environment"
	"testing"
)

// TestValidateJavaScript covers the parser integration directly, rather than
// only through analyzeJsRawFunction: these are the exact mistakes js.raw
// exists to turn into build failures instead of pages that load and quietly
// do nothing.
func TestValidateJavaScript(t *testing.T) {
	valid := []struct {
		name string
		code string
	}{
		{"simple statement", `document.title = "hi"`},
		{"single quotes", `alert('hi')`},
		{"multi statement", `var a = 1; console.log(a)`},
		{"function declaration", "function f(x) {\n\treturn x + 1\n}\nf(2)"},
		{"template literal", "console.log(`a ${1 + 1}`)"},
		{"arrow and optional chaining", `const f = (x) => x?.y ?? 0; f(null)`},
		{"empty", ``},
		{"comment only", `// nothing to do`},
		{"regex literal with a slash", `var re = /a\/b/; re.test("a/b")`},
	}
	for _, tc := range valid {
		t.Run("valid/"+tc.name, func(t *testing.T) {
			if problem := validateJavaScript(tc.code); problem != "" {
				t.Errorf("expected %q to be valid, got problem: %s", tc.code, problem)
			}
		})
	}

	invalid := []struct {
		name string
		code string
	}{
		{"unbalanced paren", `alert("hi"`},
		{"unclosed brace", `function f() { return 1`},
		{"unterminated string", `var a = "oops`},
		{"stray operator", `var a = 1 +`},
		{"reserved word as binding", `var class = 1`},
	}
	for _, tc := range invalid {
		t.Run("invalid/"+tc.name, func(t *testing.T) {
			if problem := validateJavaScript(tc.code); problem == "" {
				t.Errorf("expected %q to be rejected, but it validated", tc.code)
			}
		})
	}
}

// TestValidateJavaScriptRejectsScriptEndTag pins the one rule that is about
// HTML rather than JavaScript. A "</script" inside a raw-text element ends
// the element there, whatever the JavaScript context, and entity-escaping
// cannot rescue it because a browser does not decode entities in raw text.
// esbuild considers these strings perfectly valid JS, so this check has to
// be ours.
func TestValidateJavaScriptRejectsScriptEndTag(t *testing.T) {
	cases := []string{
		`var a = "</script>"`,
		`var a = '</SCRIPT>'`, // case-insensitive: the HTML parser is
		`// </script`,         // and it does not care that this is a comment
	}
	for _, code := range cases {
		problem := validateJavaScript(code)
		if problem == "" {
			t.Errorf("expected %q to be rejected for containing a script end tag", code)
		}
	}

	// The inverse: "</" followed by anything else is ordinary JavaScript and
	// must not be rejected.
	if problem := validateJavaScript(`var a = "</div>"`); problem != "" {
		t.Errorf("expected a non-script end tag to be allowed, got: %s", problem)
	}
}

// TestSemanticAnalysisJsRaw covers analyzeJsRawFunction's own gatekeeping,
// which TestValidateJavaScript above deliberately bypasses by calling the
// validator directly. The string-literal restriction is the load-bearing
// part: it is what guarantees the source is in front of the validator while
// the caja binary is still running, so every rejection here is a script
// that could never have been checked at all.
func TestSemanticAnalysisJsRaw(t *testing.T) {
	const literalRequired = "type error: the argument to 'js.raw' must be a string literal"

	tests := []testScenario{
		{
			name: "Valid js.raw with a string literal",
			input: `import js
let s = js.raw("alert('hi')")
`,
			expectedErrors: []string{},
		},
		{
			name: "Empty script is valid",
			input: `import js
let s = js.raw("")
`,
			expectedErrors: []string{},
		},
		{
			name: "Arity error with no arguments",
			input: `import js
let s = js.raw()
`,
			expectedErrors: []string{"arity error: expected 1 argument for 'raw', got 0"},
		},
		{
			name: "Arity error with two arguments",
			input: `import js
let s = js.raw("var a = 1", "var b = 2")
`,
			expectedErrors: []string{"arity error: expected 1 argument for 'raw', got 2"},
		},
		{
			// A String-typed variable is still not a literal. This is the
			// case that would be silently accepted if the check were on the
			// argument's TYPE rather than its AST shape — and it is exactly
			// the unvalidated script the module exists to prevent.
			name: "Rejects a String variable, not just a non-String",
			input: `import js
let code = "alert('hi')"
let s = js.raw(code)
`,
			expectedErrors: []string{literalRequired},
		},
		{
			// An interpolated string is a different AST node from a plain
			// literal, and must be rejected for the same reason: the value
			// is assembled at runtime, so there is nothing to parse now.
			name: "Rejects an interpolated string",
			input: `import js
let name = "world"
let s = js.raw("alert('${name}')")
`,
			expectedErrors: []string{literalRequired},
		},
		{
			name: "Rejects a string built by a call",
			input: `import js
import string
let s = js.raw(string.concat("aler", "t(1)"))
`,
			expectedErrors: []string{literalRequired},
		},
		{
			name: "Rejects a non-String argument",
			input: `import js
let s = js.raw(42)
`,
			expectedErrors: []string{literalRequired},
		},
		{
			// The validator reached through the analyzer, positioned at the
			// call site — this is what `caja run`, `caja build` and the LSP
			// all surface.
			name: "Malformed JavaScript is a semantic error at the call site",
			input: `import js
let s = js.raw("alert(")
`,
			expectedErrors: []string{"invalid JavaScript in 'js.raw':"},
		},
		{
			name: "Script end tag is rejected through the analyzer too",
			input: `import js
let s = js.raw("var a = \"</script>\"")
`,
			expectedErrors: []string{`invalid JavaScript in 'js.raw': a script may not contain "</script"`},
		},
	}
	runTestScenarios(t, tests)
}

// TestSemanticAnalysisScriptWidening drives the one-directional Script/String
// widening through real Caja source, across every construct that performs a
// type comparison: builtin parameters (acceptsStringArg), user function
// parameters and struct fields (BasicSymbol.Equals), and let annotations.
//
// Each pair is written both ways round on purpose. A Script flowing into a
// String is the convenience; a String refused where a Script is declared is
// the guarantee — js.raw stays the only producer of a Script, and js.raw is
// what runs the JavaScript parser. Losing the second half would leave the
// first half looking like it still worked.
func TestSemanticAnalysisScriptWidening(t *testing.T) {
	tests := []testScenario{
		{
			name: "Script satisfies a builtin String parameter",
			input: `import js
import string
let s = js.raw("var a = 1")
let n = string.len(s)
let c = string.contains(s, "var")
`,
			expectedErrors: []string{},
		},
		{
			name: "Script satisfies a String element in a builtin array parameter",
			input: `import js
import string
let joined = string.join([js.raw("var a = 1")], ",")
`,
			expectedErrors: []string{},
		},
		{
			name: "Script satisfies a user function's String parameter",
			input: `import js
let wrap = fn(body: String) -> String { return body }
let out = wrap(js.raw("var a = 1"))
`,
			expectedErrors: []string{},
		},
		{
			name: "String does NOT satisfy a Script parameter",
			input: `import js
let embed = fn(code: js.Script) -> Number { return 1 }
let out = embed("var a = 1")
`,
			expectedErrors: []string{"type error: argument 1 expected Script, got String"},
		},
		{
			name: "Script satisfies a Script parameter",
			input: `import js
let embed = fn(code: js.Script) -> Number { return 1 }
let out = embed(js.raw("var a = 1"))
`,
			expectedErrors: []string{},
		},
		{
			name: "Script satisfies a String struct field",
			input: `import js
type Page struct {
	body String
}
let p = Page{ body: js.raw("var a = 1") }
`,
			expectedErrors: []string{},
		},
		{
			name: "String does NOT satisfy a Script struct field",
			input: `import js
type Page struct {
	body js.Script
}
let p = Page{ body: "var a = 1" }
`,
			expectedErrors: []string{"type error: field 'body' expects Script, got String"},
		},
		{
			name: "Script satisfies a String let annotation and interpolates",
			input: `import js
let s = js.raw("var a = 1")
let asText: String = s
let wrapped = "<script>${s}</script>"
`,
			expectedErrors: []string{},
		},
		{
			name: "String does NOT satisfy a Script let annotation",
			input: `import js
let s: js.Script = "var a = 1"
`,
			expectedErrors: []string{"type error: cannot assign String to Script"},
		},
		{
			// The named-import form of the same thing, which is how
			// @caja/js itself declares `fn(code: js.Script)`.
			name: "Named-imported Script type keeps the same asymmetry",
			input: `import { raw, Script } from "js"
let embed = fn(code: Script) -> Number { return 1 }
let ok = embed(raw("var a = 1"))
let bad = embed("var a = 1")
`,
			expectedErrors: []string{"type error: argument 1 expected Script, got String"},
		},
	}
	runTestScenarios(t, tests)
}

// TestAcceptsStringArg unit-tests the predicate that replaced ~21 duplicated
// inline `!= STRING_OBJ && != ANY_OBJ` checks across builtin.go. Because it
// is now the single gate for every builtin String parameter, a wrong answer
// here is wrong in twenty places at once — and the table is the cheapest
// place to state which types are and are not string-like.
func TestAcceptsStringArg(t *testing.T) {
	tests := []struct {
		objType environment.ObjectType
		want    bool
	}{
		{environment.STRING_OBJ, true},
		// Any is permissive so one unresolved type doesn't cascade errors.
		{environment.ANY_OBJ, true},
		// Script is validated JavaScript source, which genuinely is text.
		{environment.SCRIPT_OBJ, true},

		{environment.NUMBER_OBJ, false},
		{environment.BOOLEAN_OBJ, false},
		{environment.ARRAY_OBJ, false},
		{environment.MAP_OBJ, false},
		{environment.NULL_OBJ, false},
		{environment.DATE_OBJ, false},
		// Instant also erases to a non-string Go type and must not slip
		// through just because it is another opaque builtin type.
		{environment.INSTANT_OBJ, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.objType), func(t *testing.T) {
			got := acceptsStringArg(symbol.NewBasicSymbol(tt.objType))
			if got != tt.want {
				t.Errorf("acceptsStringArg(%s) = %v, want %v", tt.objType, got, tt.want)
			}
		})
	}
}
