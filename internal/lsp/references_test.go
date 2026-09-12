package lsp

import (
	"strings"
	"testing"

	"github.com/owenrumney/go-lsp/lsp"
)

// TestReferencesFindsEveryUse checks that find-references reports the declaration and all
// of its uses, and nothing that merely shares the name in a different scope.
func TestReferencesFindsEveryUse(t *testing.T) {
	src := `let total = 1
let doubled = total + total
let shadow = fn(total: Number) -> Number {
    return total
}
let after = total
`
	_, s, uri := openInline(t, "main.caja", src)

	// On the declaration of the outer `total`.
	locs, err := s.References(uri, 0, 4, true)
	if err != nil {
		t.Fatalf("References: %v", err)
	}

	lines := referencedLines(locs)
	// Declaration on 0, two uses on 1, one use on 5. The parameter on line 2 and its use
	// on line 3 are a different binding and must not appear.
	for _, want := range []int{0, 1, 5} {
		if lines[want] == 0 {
			t.Errorf("no reference reported on line %d", want)
		}
	}
	if lines[1] != 2 {
		t.Errorf("line 1 uses `total` twice, got %d references", lines[1])
	}
	if lines[3] != 0 {
		t.Error("the parameter `total` inside the function was reported; it is a different binding")
	}
}

// TestReferencesRespectsIncludeDeclaration checks the flag the editor sends when it wants
// only the uses, not the declaration itself.
func TestReferencesRespectsIncludeDeclaration(t *testing.T) {
	src := "let total = 1\nlet doubled = total\n"
	_, s, uri := openInline(t, "main.caja", src)

	with, err := s.References(uri, 0, 4, true)
	if err != nil {
		t.Fatalf("References: %v", err)
	}
	without, err := s.References(uri, 0, 4, false)
	if err != nil {
		t.Fatalf("References: %v", err)
	}

	if len(with) != len(without)+1 {
		t.Errorf("including the declaration gave %d results and excluding it gave %d; "+
			"expected exactly one more", len(with), len(without))
	}
	for _, loc := range without {
		if loc.Range.Start.Line == 0 {
			t.Error("the declaration was returned even though includeDeclaration was false")
		}
	}
}

// TestReferencesOnTypeName covers references to a type, which only became possible once
// type annotations carried positions.
func TestReferencesOnTypeName(t *testing.T) {
	src := `type Money Number
let price: Money = 1
let fee: Money = 2
let convert = fn(m: Money) -> Money { return m }
`
	_, s, uri := openInline(t, "main.caja", src)

	locs, err := s.References(uri, 0, 5, true) // on the declaration of Money
	if err != nil {
		t.Fatalf("References: %v", err)
	}

	// Declaration, two annotations, plus the parameter and return annotations.
	if len(locs) < 5 {
		t.Errorf("found %d references to the type Money, expected its declaration and all "+
			"four annotations; got %v", len(locs), locs)
	}
}

func referencedLines(locs []lsp.Location) map[int]int {
	counts := make(map[int]int)
	for _, loc := range locs {
		counts[loc.Range.Start.Line]++
	}
	return counts
}

// TestDocumentHighlightSeparatesReadsFromWrites checks that the editor can shade the
// place a value is assigned differently from the places it is read.
func TestDocumentHighlightSeparatesReadsFromWrites(t *testing.T) {
	src := "let total = 1\nlet doubled = total + 1\n"
	_, s, uri := openInline(t, "main.caja", src)

	highlights, err := s.DocumentHighlight(uri, 1, 14) // on the use of `total`
	if err != nil {
		t.Fatalf("DocumentHighlight: %v", err)
	}
	if len(highlights) != 2 {
		t.Fatalf("got %d highlights, want the declaration and the one use", len(highlights))
	}

	var reads, writes int
	for _, hl := range highlights {
		if hl.Kind == nil {
			t.Fatal("highlight has no kind")
		}
		switch *hl.Kind {
		case lsp.DocumentHighlightKindWrite:
			writes++
		case lsp.DocumentHighlightKindRead:
			reads++
		}
	}
	if writes != 1 || reads != 1 {
		t.Errorf("got %d writes and %d reads, want the declaration shaded as a write and "+
			"the use as a read", writes, reads)
	}
}

// TestRenameRewritesEveryOccurrence is the core rename guarantee: every use moves, and
// only the uses of that one binding.
func TestRenameRewritesEveryOccurrence(t *testing.T) {
	src := `let total = 1
let doubled = total + total
let shadow = fn(total: Number) -> Number {
    return total
}
`
	_, s, uri := openInline(t, "main.caja", src)

	edit, err := s.Rename(uri, 0, 4, "grandTotal")
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if edit == nil {
		t.Fatal("Rename returned no edit")
	}

	edits := edit.Changes[uri]
	if len(edits) != 3 {
		t.Fatalf("got %d edits, want the declaration plus its two uses on line 1 "+
			"(the shadowing parameter must not move)", len(edits))
	}
	for _, e := range edits {
		if e.NewText != "grandTotal" {
			t.Errorf("edit inserts %q, want the new name", e.NewText)
		}
		if e.Range.Start.Line == 3 {
			t.Error("an edit targets the shadowed parameter's use, which is a different binding")
		}
	}
}

// TestRenameRejectsInvalidNames covers the guard that stops a refactor from turning into
// a syntax or semantic error at every use at once.
func TestRenameRejectsInvalidNames(t *testing.T) {
	cases := []struct {
		name       string
		src        string
		line, char int
		newName    string
		wantErr    string
	}{
		{"keyword", "let total = 1\n", 0, 4, "return", "keyword"},
		{"not an identifier", "let total = 1\n", 0, 4, "two words", "not a valid"},
		{"leading digit", "let total = 1\n", 0, 4, "1st", "not a valid"},
		{"empty", "let total = 1\n", 0, 4, "", "empty"},
		{"lowercase type", "type Money Number\nlet p: Money = 1\n", 0, 5, "money", "capital letter"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, s, uri := openInline(t, "main.caja", tc.src)

			_, err := s.Rename(uri, tc.line, tc.char, tc.newName)
			if err == nil {
				t.Fatalf("renaming to %q was accepted; expected it to be refused", tc.newName)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q does not explain the problem (want it to mention %q)",
					err, tc.wantErr)
			}
		})
	}
}

// TestPrepareRenameReportsTheNameSpan checks that the rename box opens pre-filled with
// the name and anchored to it, rather than to whatever surrounds the caret.
func TestPrepareRenameReportsTheNameSpan(t *testing.T) {
	_, s, uri := openInline(t, "main.caja", "let total = 1\nlet doubled = total\n")

	res, err := s.PrepareRename(uri, 1, 14) // inside the use of `total`
	if err != nil {
		t.Fatalf("PrepareRename: %v", err)
	}
	if res == nil {
		t.Fatal("PrepareRename returned nothing for a renameable name")
	}
	if res.Placeholder != "total" {
		t.Errorf("placeholder = %q, want the current name", res.Placeholder)
	}
	if res.Range.Start.Line != 1 || res.Range.Start.Character != 14 {
		t.Errorf("range starts at %d:%d, want the start of the name at 1:14",
			res.Range.Start.Line, res.Range.Start.Character)
	}
	if res.Range.End.Character-res.Range.Start.Character != len("total") {
		t.Errorf("range covers %d characters, want %d",
			res.Range.End.Character-res.Range.Start.Character, len("total"))
	}
}

// TestRenameOnNonNameIsDeclined checks that asking to rename a literal or an operator
// answers with nothing rather than producing a nonsense edit.
func TestRenameOnNonNameIsDeclined(t *testing.T) {
	_, s, uri := openInline(t, "main.caja", "let total = 42\n")

	edit, err := s.Rename(uri, 0, 12, "other") // on the literal `42`
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if edit != nil && len(edit.Changes) != 0 {
		t.Errorf("renaming a number literal produced edits: %v", edit.Changes)
	}
}
