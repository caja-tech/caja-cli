package lsp

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"unicode"

	"github.com/owenrumney/go-lsp/lsp"
)

// Diagnostic codes. The analyzer and parser classify every message they report with a
// prefix — "type error:", "semantic error:", "syntax error:" — across more than two
// hundred report sites. Reading that existing taxonomy is what gives diagnostics stable,
// machine-readable codes without threading a code parameter through all of them.
const (
	codeTypeError     = "type-error"
	codeSemanticError = "semantic-error"
	codeSyntaxError   = "syntax-error"
	codeUnclassified  = "caja"
)

// classifyDiagnostic derives a stable code from a message's taxonomy prefix.
func classifyDiagnostic(message string) string {
	switch {
	case strings.HasPrefix(message, "type error"):
		return codeTypeError
	case strings.HasPrefix(message, "semantic error"):
		return codeSemanticError
	case strings.HasPrefix(message, "syntax error"):
		return codeSyntaxError
	default:
		return codeUnclassified
	}
}

func diagnosticCode(message string) json.RawMessage {
	encoded, err := json.Marshal(classifyDiagnostic(message))
	if err != nil {
		return nil
	}
	return encoded
}

// Patterns for the diagnostics that already tell the user what to do. Each of these
// messages ends in advice; a quick fix turns that advice into an edit.
var (
	// "type name 'money' must start with a capital letter"
	lowercaseTypePattern = regexp.MustCompile(`type name '([^']+)' must start with a capital letter`)
	// "... Suggestion: qualify it (array.len or string.len)"
	qualifySuggestionPattern = regexp.MustCompile(`Suggestion: qualify it \(([^)]+)\)`)
	// "a variable bound to a reactive call result must be declared 'active'"
	needsActivePattern = regexp.MustCompile(`must be declared 'active'`)
	// "unnecessary safe navigation on non-nullable type"
	unnecessarySafeNavPattern = regexp.MustCompile(`unnecessary safe navigation`)
)

// CodeAction offers quick fixes for the diagnostics under the cursor.
//
// Every fix here corresponds to advice the analyzer already prints. The information to
// repair these was always present in the message; the user just had to apply it by hand.
func (h *CajaHandler) CodeAction(_ context.Context, params *lsp.CodeActionParams) (res []lsp.CodeAction, err error) {
	defer recoverInto("codeAction", params.TextDocument.URI, &res, &err)

	_, ix, ok := h.documentFor(params.TextDocument.URI)
	if !ok {
		return nil, nil
	}

	actions := make([]lsp.CodeAction, 0)
	for _, diag := range params.Context.Diagnostics {
		actions = append(actions, fixesFor(params.TextDocument.URI, ix, diag)...)
	}
	return actions, nil
}

// fixesFor produces the quick fixes that repair one diagnostic.
func fixesFor(uri lsp.DocumentURI, ix interface{ Line(int) string }, diag lsp.Diagnostic) []lsp.CodeAction {
	switch {
	case lowercaseTypePattern.MatchString(diag.Message):
		return capitaliseTypeFix(uri, diag)
	case qualifySuggestionPattern.MatchString(diag.Message):
		return qualifyNameFixes(uri, diag)
	case needsActivePattern.MatchString(diag.Message):
		return declareActiveFix(uri, ix, diag)
	case unnecessarySafeNavPattern.MatchString(diag.Message):
		return removeSafeNavigationFix(uri, ix, diag)
	default:
		return nil
	}
}

// capitaliseTypeFix repairs `type money Number`, where the language requires type names
// to be capitalised and the diagnostic names the offending identifier.
func capitaliseTypeFix(uri lsp.DocumentURI, diag lsp.Diagnostic) []lsp.CodeAction {
	match := lowercaseTypePattern.FindStringSubmatch(diag.Message)
	if len(match) < 2 {
		return nil
	}

	name := match[1]
	capitalised := capitaliseFirst(name)
	if capitalised == name {
		return nil
	}

	return []lsp.CodeAction{newQuickFix(
		"Rename to "+capitalised,
		uri, diag,
		lsp.TextEdit{Range: diag.Range, NewText: capitalised},
	)}
}

// qualifyNameFixes turns the "qualify it (array.len or string.len)" advice attached to an
// ambiguous wildcard import into one fix per candidate module.
func qualifyNameFixes(uri lsp.DocumentURI, diag lsp.Diagnostic) []lsp.CodeAction {
	match := qualifySuggestionPattern.FindStringSubmatch(diag.Message)
	if len(match) < 2 {
		return nil
	}

	var actions []lsp.CodeAction
	for _, candidate := range splitSuggestions(match[1]) {
		actions = append(actions, newQuickFix(
			"Qualify as "+candidate,
			uri, diag,
			lsp.TextEdit{Range: diag.Range, NewText: candidate},
		))
	}
	return actions
}

// splitSuggestions unpacks the "a.len, b.len or c.len" list the analyzer formats.
func splitSuggestions(list string) []string {
	list = strings.ReplaceAll(list, " or ", ",")

	var out []string
	for _, part := range strings.Split(list, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// declareActiveFix inserts the `active` keyword a reactive binding requires. The
// diagnostic points at the `let`, so the insertion goes immediately after it.
func declareActiveFix(uri lsp.DocumentURI, ix interface{ Line(int) string }, diag lsp.Diagnostic) []lsp.CodeAction {
	line := ix.Line(diag.Range.Start.Line)
	keyword := strings.Index(line, "let ")
	if keyword == -1 {
		return nil
	}

	insertAt := lsp.Position{Line: diag.Range.Start.Line, Character: keyword + len("let ")}
	return []lsp.CodeAction{newQuickFix(
		"Declare as `active`",
		uri, diag,
		lsp.TextEdit{Range: lsp.Range{Start: insertAt, End: insertAt}, NewText: "active "},
	)}
}

// removeSafeNavigationFix drops the `?` from a `?.` applied to a value that cannot be
// nil, which the language rejects as redundant.
func removeSafeNavigationFix(uri lsp.DocumentURI, ix interface{ Line(int) string }, diag lsp.Diagnostic) []lsp.CodeAction {
	line := ix.Line(diag.Range.Start.Line)

	// Search from the reported position backwards: the diagnostic points at the property,
	// and the `?.` sits just before it.
	cut := strings.LastIndex(line[:min(diag.Range.Start.Character, len(line))], "?.")
	if cut == -1 {
		return nil
	}

	return []lsp.CodeAction{newQuickFix(
		"Remove unnecessary `?`",
		uri, diag,
		lsp.TextEdit{
			Range: lsp.Range{
				Start: lsp.Position{Line: diag.Range.Start.Line, Character: cut},
				End:   lsp.Position{Line: diag.Range.Start.Line, Character: cut + 1},
			},
			NewText: "",
		},
	)}
}

func newQuickFix(title string, uri lsp.DocumentURI, diag lsp.Diagnostic, edits ...lsp.TextEdit) lsp.CodeAction {
	kind := lsp.CodeActionQuickFix
	preferred := true

	return lsp.CodeAction{
		Title:       title,
		Kind:        &kind,
		Diagnostics: []lsp.Diagnostic{diag},
		IsPreferred: &preferred,
		Edit: &lsp.WorkspaceEdit{
			Changes: map[lsp.DocumentURI][]lsp.TextEdit{uri: edits},
		},
	}
}

func capitaliseFirst(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
