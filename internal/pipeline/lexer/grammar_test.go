package lexer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// grammarPath is the VS Code extension's TextMate grammar, relative to this package.
const grammarPath = "../../../editors/vscode/caja/syntaxes/caja.tmLanguage.json"

// tmGrammar is the subset of the TextMate format this test needs: every `match`, `begin`
// and `end` regex anywhere in the file.
type tmGrammar struct {
	Patterns   []tmRule            `json:"patterns"`
	Repository map[string][]tmRule `json:"-"`
}

type tmRule struct {
	Match    string   `json:"match"`
	Begin    string   `json:"begin"`
	End      string   `json:"end"`
	Name     string   `json:"name"`
	Patterns []tmRule `json:"patterns"`
}

// loadGrammarRegexes returns every regex the grammar uses, flattened.
func loadGrammarRegexes(t *testing.T) []string {
	t.Helper()

	raw, err := os.ReadFile(filepath.FromSlash(grammarPath))
	if err != nil {
		t.Fatalf("reading the editor grammar at %s: %v", grammarPath, err)
	}

	// The repository is a map of arbitrary rule names, so decode generically and walk.
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parsing the editor grammar: %v", err)
	}

	var regexes []string
	var walk func(node any)
	walk = func(node any) {
		switch n := node.(type) {
		case map[string]any:
			for _, key := range []string{"match", "begin", "end"} {
				if value, ok := n[key].(string); ok {
					regexes = append(regexes, value)
				}
			}
			for _, value := range n {
				walk(value)
			}
		case []any:
			for _, item := range n {
				walk(item)
			}
		}
	}
	walk(doc)

	if len(regexes) == 0 {
		t.Fatal("the grammar contains no patterns; the scan is broken")
	}
	return regexes
}

// TestEditorGrammarCoversEveryKeyword keeps the VS Code grammar honest against the
// language it claims to highlight.
//
// The grammar is a hand-maintained list with nothing tying it to this package, and it had
// drifted: `union` and `is` were added to the language and never to the grammar, so they
// rendered as plain identifiers. This test is the tie.
func TestEditorGrammarCoversEveryKeyword(t *testing.T) {
	regexes := loadGrammarRegexes(t)

	for _, keyword := range GetKeywords() {
		if !appearsAsAlternative(regexes, keyword) {
			t.Errorf("the editor grammar does not highlight the keyword %q; "+
				"it will render as a plain identifier", keyword)
		}
	}
}

// TestEditorGrammarHasNoPhantomKeywords catches the opposite drift: a word the grammar
// highlights as a keyword that the lexer does not treat as one.
//
// `from` was exactly this — highlighted as control flow, but lexed as an ordinary
// identifier, so `import { a } from mod` coloured a non-keyword and any variable named
// `from` was coloured wrongly too.
func TestEditorGrammarHasNoPhantomKeywords(t *testing.T) {
	real := make(map[string]bool)
	for _, keyword := range GetKeywords() {
		real[keyword] = true
	}

	// Only word-boundary alternations are keyword lists; operator and punctuation rules
	// are matched by symbol and are checked separately.
	wordList := regexp.MustCompile(`^\\b\(([a-z|]+)\)\\b$`)

	for _, pattern := range loadGrammarRegexes(t) {
		match := wordList.FindStringSubmatch(pattern)
		if match == nil {
			continue
		}
		for _, word := range strings.Split(match[1], "|") {
			if word != "" && !real[word] {
				t.Errorf("the grammar highlights %q as a keyword, but the lexer does not "+
					"recognise it as one — it lexes as a plain identifier", word)
			}
		}
	}
}

// TestEditorGrammarCoversEveryOperator checks the symbols the lexer produces are
// highlighted. Operators drift the same way keywords do: `^`, `%`, `::` and the union
// separator `|` were all unhighlighted.
func TestEditorGrammarCoversEveryOperator(t *testing.T) {
	// Every operator and delimiter the lexer can emit, from its deciders.
	operators := []string{
		"=", "==", "=>", "+", "-", "->", "*", "/", "^", "%",
		"<", "<=", ">", ">=", "!", "!=",
		"(", ")", "{", "}", "[", "]", ",", ":", "::", ".",
		"?", "?.", "?>", "?>>", "|", "|>", "|>>", "&",
	}

	regexes := loadGrammarRegexes(t)
	for _, op := range operators {
		if !matchesLiteral(regexes, op) {
			t.Errorf("the editor grammar does not highlight the operator %q", op)
		}
	}
}

// appearsAsAlternative reports whether a keyword is listed in one of the grammar's
// word-boundary alternations.
func appearsAsAlternative(regexes []string, keyword string) bool {
	for _, pattern := range regexes {
		for _, alternative := range strings.Split(pattern, "|") {
			trimmed := strings.Trim(alternative, `\b()`)
			if trimmed == keyword {
				return true
			}
		}
	}
	return false
}

// matchesLiteral reports whether some grammar regex actually matches an operator. Testing
// the compiled regex rather than the source text is what makes this robust to however the
// pattern happens to be escaped.
func matchesLiteral(regexes []string, literal string) bool {
	for _, pattern := range regexes {
		// Go's regexp does not accept every Oniguruma construct TextMate allows; a
		// pattern this package cannot compile simply cannot be checked here.
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			continue
		}
		if found := compiled.FindString(literal); found == literal {
			return true
		}
	}
	return false
}
