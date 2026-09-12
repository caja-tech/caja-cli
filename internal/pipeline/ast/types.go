package ast

import "caja-cli/internal/pipeline/lexer"

// TypeExpr is a type annotation, with the source positions the language server needs.
//
// Type annotations used to be stored as bare strings — `Parameter.Type string` and
// friends — which meant no type reference anywhere in the language had a position. Hover,
// go-to-definition, find-references, rename and semantic tokens could therefore never
// resolve a type, not in `let x: Money`, not in `fn(c: Customer) -> Boolean`, not in a
// struct field, and no amount of work inside internal/lsp could fix it.
//
// Name is the canonical rendering and is byte-identical to what the old string field
// held, so type comparison, map keying and generic substitution all keep working
// unchanged. Refs carries the individual named types inside the annotation, which is what
// makes `Money` resolvable inside `[Money]` or `fn(Money) -> Boolean`.
type TypeExpr struct {
	Token lexer.Token // first token: '[', 'fn', or the leading identifier
	End   lexer.Token // last token: ']', '?', '>', or the trailing identifier
	Name  string
	Refs  []*TypeRef
}

func (t *TypeExpr) TokenLiteral() string { return t.Token.Literal }
func (t *TypeExpr) String() string       { return t.Text() }

// Text returns the canonical rendering, or "" when the annotation is absent. A nil
// receiver is the normal case — most declarations carry no explicit type, and a parse
// error also leaves the field nil — so every read goes through here rather than
// dereferencing.
func (t *TypeExpr) Text() string {
	if t == nil {
		return ""
	}
	return t.Name
}

// TypeRef is a single named type appearing inside an annotation: `Money` in `[Money]`,
// or both halves of `animals.Cat`. A reference in a type-parameter list is a binding
// occurrence; everywhere else it is a use.
type TypeRef struct {
	Token     lexer.Token  // the type identifier itself
	Qualifier *lexer.Token // the module name in `mod.Type`, else nil
	Name      string       // "Money", or "animals.Cat" when qualified
}

func (t *TypeRef) TokenLiteral() string { return t.Token.Literal }
func (t *TypeRef) String() string {
	if t == nil {
		return ""
	}
	return t.Name
}

// TypeExprText is a convenience for the many call sites that only need the rendering of
// an annotation that may be absent.
func TypeExprText(t *TypeExpr) string { return t.Text() }

// TypeExprsText renders a list of annotations, for signatures built by joining them.
func TypeExprsText(list []*TypeExpr) []string {
	out := make([]string, len(list))
	for i, t := range list {
		out[i] = t.Text()
	}
	return out
}
