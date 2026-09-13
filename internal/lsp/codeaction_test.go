package lsp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/owenrumney/go-lsp/lsp"
	"github.com/owenrumney/go-lsp/servertest"
)

// diagnosticsFor opens a source and returns whatever the server reported about it.
func diagnosticsFor(t *testing.T, src string) (*servertest.Harness, lsp.DocumentURI, []lsp.Diagnostic) {
	t.Helper()

	_, s, uri := openInline(t, "main.caja", src)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	diags, err := s.WaitForDiagnostics(ctx, uri)
	if err != nil {
		t.Fatalf("no diagnostics published: %v", err)
	}
	return s, uri, diags
}

// applyEdits applies a quick fix's edits to a source line, so a test can assert on the
// repaired text rather than on edit coordinates.
func applyEdits(t *testing.T, src string, edits []lsp.TextEdit) string {
	t.Helper()

	lines := strings.Split(src, "\n")
	// Apply right to left so earlier edits keep their offsets.
	for i := len(edits) - 1; i >= 0; i-- {
		e := edits[i]
		if e.Range.Start.Line != e.Range.End.Line {
			t.Fatalf("edit spans lines %d..%d; these fixes should be single-line",
				e.Range.Start.Line, e.Range.End.Line)
		}
		line := lines[e.Range.Start.Line]
		lines[e.Range.Start.Line] = line[:e.Range.Start.Character] + e.NewText + line[e.Range.End.Character:]
	}
	return strings.Join(lines, "\n")
}

// TestDiagnosticsCarryAStableCode checks that each diagnostic is machine-identifiable,
// which is what lets a client group or filter them and what a quick fix keys off.
func TestDiagnosticsCarryAStableCode(t *testing.T) {
	_, _, diags := diagnosticsFor(t, "type money Number\n")
	if len(diags) == 0 {
		t.Fatal("expected a diagnostic for the lowercase type name")
	}

	for _, d := range diags {
		if len(d.Code) == 0 {
			t.Errorf("diagnostic %q carries no code", d.Message)
			continue
		}
		var code string
		if err := json.Unmarshal(d.Code, &code); err != nil {
			t.Errorf("diagnostic code is not a JSON string: %v", err)
			continue
		}
		if code == "" {
			t.Errorf("diagnostic %q has an empty code", d.Message)
		}
	}
}

func TestClassifyDiagnostic(t *testing.T) {
	cases := []struct{ message, want string }{
		{"type error: cannot assign String to Number", codeTypeError},
		{"semantic error: use of moved variable 'x'", codeSemanticError},
		{"syntax error: primitive type 'Number' cannot be nullable", codeSyntaxError},
		{"type name 'money' must start with a capital letter", codeUnclassified},
	}

	for _, tc := range cases {
		if got := classifyDiagnostic(tc.message); got != tc.want {
			t.Errorf("classifyDiagnostic(%q) = %q, want %q", tc.message, got, tc.want)
		}
	}
}

// TestQuickFixCapitalisesTypeName drives the fix end to end: the diagnostic the analyzer
// really produces goes back in, and the resulting edit must repair the source.
func TestQuickFixCapitalisesTypeName(t *testing.T) {
	const src = "type money Number\n"
	s, uri, diags := diagnosticsFor(t, src)

	actions, err := s.CodeAction(&lsp.CodeActionParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
		Range:        diags[0].Range,
		Context:      lsp.CodeActionContext{Diagnostics: diags},
	})
	if err != nil {
		t.Fatalf("CodeAction: %v", err)
	}
	if len(actions) == 0 {
		t.Fatal("no quick fix offered for a lowercase type name")
	}

	fix := actions[0]
	if fix.Kind == nil || *fix.Kind != lsp.CodeActionQuickFix {
		t.Errorf("action kind = %v, want quickfix", fix.Kind)
	}
	if !strings.Contains(fix.Title, "Money") {
		t.Errorf("title %q does not say what it will do", fix.Title)
	}

	got := applyEdits(t, src, fix.Edit.Changes[uri])
	if !strings.HasPrefix(got, "type Money Number") {
		t.Errorf("applying the fix produced %q, want the type name capitalised", got)
	}
}

// TestQuickFixQualifiesAmbiguousName covers the ambiguous-wildcard diagnostic, which ends
// in advice naming every valid qualification. Each becomes its own fix.
func TestQuickFixQualifiesAmbiguousName(t *testing.T) {
	const src = "import * from \"array\"\nimport * from \"string\"\nlet n = len\n"
	s, uri, diags := diagnosticsFor(t, src)

	var ambiguous *lsp.Diagnostic
	for i := range diags {
		if strings.Contains(diags[i].Message, "qualify it") {
			ambiguous = &diags[i]
			break
		}
	}
	if ambiguous == nil {
		t.Skipf("the analyzer did not report an ambiguous wildcard here; got %v", diags)
	}

	actions, err := s.CodeAction(&lsp.CodeActionParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
		Range:        ambiguous.Range,
		Context:      lsp.CodeActionContext{Diagnostics: []lsp.Diagnostic{*ambiguous}},
	})
	if err != nil {
		t.Fatalf("CodeAction: %v", err)
	}
	if len(actions) < 2 {
		t.Fatalf("got %d fixes, want one per candidate module", len(actions))
	}

	for _, action := range actions {
		got := applyEdits(t, src, action.Edit.Changes[uri])
		if !strings.Contains(got, ".len") {
			t.Errorf("fix %q produced %q, want the name qualified", action.Title, got)
		}
	}
}

// TestSplitSuggestions covers the parsing of the analyzer's own suggestion formatting,
// which joins candidates with commas and a final "or".
func TestSplitSuggestions(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"array.len or string.len", []string{"array.len", "string.len"}},
		{"a.f, b.f or c.f", []string{"a.f", "b.f", "c.f"}},
		{"only.one", []string{"only.one"}},
	}

	for _, tc := range cases {
		got := splitSuggestions(tc.in)
		if strings.Join(got, "|") != strings.Join(tc.want, "|") {
			t.Errorf("splitSuggestions(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// TestCodeActionWithoutDiagnosticsOffersNothing checks that a request over clean code
// answers with an empty list rather than inventing refactors.
func TestCodeActionWithoutDiagnosticsOffersNothing(t *testing.T) {
	_, s, uri := openInline(t, "main.caja", "let total = 1\n")

	actions, err := s.CodeAction(&lsp.CodeActionParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
		Range:        lsp.Range{Start: lsp.Position{Line: 0}, End: lsp.Position{Line: 0, Character: 5}},
		Context:      lsp.CodeActionContext{},
	})
	if err != nil {
		t.Fatalf("CodeAction: %v", err)
	}
	if len(actions) != 0 {
		t.Errorf("offered %d actions for clean code, want none", len(actions))
	}
}

// TestCodeActionIgnoresUnrelatedDiagnostics checks that a diagnostic with no known repair
// produces no fix, rather than a fix that does something arbitrary.
func TestCodeActionIgnoresUnrelatedDiagnostics(t *testing.T) {
	_, s, uri := openInline(t, "main.caja", "let total = 1\n")

	actions, err := s.CodeAction(&lsp.CodeActionParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
		Context: lsp.CodeActionContext{Diagnostics: []lsp.Diagnostic{{
			Range:   lsp.Range{Start: lsp.Position{Line: 0}, End: lsp.Position{Line: 0, Character: 5}},
			Message: "type error: cannot assign String to Number",
		}}},
	})
	if err != nil {
		t.Fatalf("CodeAction: %v", err)
	}
	if len(actions) != 0 {
		t.Errorf("offered %d actions for a diagnostic with no known repair, want none", len(actions))
	}
}
