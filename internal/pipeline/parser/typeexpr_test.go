package parser

import (
	"strings"
	"testing"

	"caja-cli/internal/pipeline/ast"
	"caja-cli/internal/pipeline/lexer"
)

// parseDeclaredType parses `let v: <type> = nil` and returns the annotation node.
func parseDeclaredType(t *testing.T, typeText string) *ast.TypeExpr {
	t.Helper()

	src := "let v: " + typeText + " = nil\n"
	p := New(lexer.New(src))
	prog := p.Parse()

	if len(prog.Statements) == 0 {
		t.Fatalf("parsing %q produced no statements (errors: %v)", src, p.Errors())
	}
	let, ok := prog.Statements[0].(*ast.LetStatement)
	if !ok {
		t.Fatalf("parsing %q produced %T, want *ast.LetStatement", src, prog.Statements[0])
	}
	return let.ValueType
}

// TestTypeExprRendering pins the canonical rendering of every type syntax the language
// accepts. TypeExpr.Name must stay byte-identical to what the old string field held: the
// analyzer compares these strings, keys maps by them, and builds generic substitutions by
// concatenating them, so a change in spelling here is a change in type-system behaviour.
func TestTypeExprRendering(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"Number", "Number"},
		{"String", "String"},
		{"Boolean", "Boolean"},
		{"Date", "Date"},
		{"Money", "Money"},
		{"Money?", "Money?"},
		{"[Number]", "[Number]"},
		{"[[String]]", "[[String]]"},
		{"[Money]?", "[Money]?"},
		{"map[String]Number", "map[String]Number"},
		{"map[String]Money", "map[String]Money"},
		{"fn(Number) -> Boolean", "fn(Number) -> Boolean"},
		{"fn(Number, String) -> Money", "fn(Number, String) -> Money"},
		{"fn() -> Nothing", "fn() -> Nothing"},
		{"fn(Number)", "fn(Number) -> Nothing"},
		{"Box<String>", "Box<String>"},
		{"Pair<String, Number>", "Pair<String, Number>"},
		{"animals.Cat", "animals.Cat"},
		{"animals.Cat?", "animals.Cat?"},
		{"[animals.Cat]", "[animals.Cat]"},
	}

	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			te := parseDeclaredType(t, tc.in)
			if te.Text() != tc.want {
				t.Errorf("rendering of %q = %q, want %q", tc.in, te.Text(), tc.want)
			}
		})
	}
}

// TestTypeExprCapturesLeafReferences is the point of the whole change: each named type
// inside an annotation must be individually addressable, or go-to-definition on `Money`
// inside `[Money]` has nothing to resolve.
func TestTypeExprCapturesLeafReferences(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"Money", []string{"Money"}},
		{"[Money]", []string{"Money"}},
		{"[[Money]]", []string{"Money"}},
		{"map[String]Money", []string{"map", "String", "Money"}},
		{"fn(Customer) -> Boolean", []string{"Customer", "Boolean"}},
		{"fn(A, B) -> C", []string{"A", "B", "C"}},
		{"Box<String>", []string{"Box<String>", "String"}},
		{"animals.Cat", []string{"animals.Cat"}},
	}

	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			te := parseDeclaredType(t, tc.in)

			var got []string
			for _, ref := range te.Refs {
				got = append(got, ref.Name)
			}
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("references in %q = %v, want %v", tc.in, got, tc.want)
			}

			for _, ref := range te.Refs {
				if ref.Token.Line == 0 {
					t.Errorf("reference %q carries no position, so it cannot be resolved", ref.Name)
				}
			}
		})
	}
}

// TestQualifiedTypeRefSplitsModuleAndName checks that `animals.Cat` is recorded as one
// reference with both halves positioned, so go-to-definition works whichever half the
// cursor is on.
func TestQualifiedTypeRefSplitsModuleAndName(t *testing.T) {
	te := parseDeclaredType(t, "animals.Cat")

	if len(te.Refs) != 1 {
		t.Fatalf("expected one reference for a qualified type, got %d", len(te.Refs))
	}
	ref := te.Refs[0]

	if ref.Qualifier == nil {
		t.Fatal("qualified type recorded no module qualifier")
	}
	if ref.Qualifier.Literal != "animals" {
		t.Errorf("qualifier = %q, want \"animals\"", ref.Qualifier.Literal)
	}
	if ref.Token.Literal != "Cat" {
		t.Errorf("reference token = %q, want \"Cat\"", ref.Token.Literal)
	}
	if ref.Qualifier.Column >= ref.Token.Column {
		t.Error("qualifier should be positioned before the type name")
	}
}

// TestTypeExprSpansTheAnnotation checks the outer span, which selection range, inlay
// hints and folding all need in addition to the leaf positions.
func TestTypeExprSpansTheAnnotation(t *testing.T) {
	te := parseDeclaredType(t, "[Money]")

	if te.Token.Type != lexer.LBRACKET {
		t.Errorf("annotation starts at %v, want the opening bracket", te.Token.Type)
	}
	if te.End.Type != lexer.RBRACKET {
		t.Errorf("annotation ends at %v, want the closing bracket", te.End.Type)
	}
}
