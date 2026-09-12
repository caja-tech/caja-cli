package ast

import (
	"reflect"
	"sort"

	"caja-cli/internal/pipeline/lexer"
)

// Children returns n's direct child nodes, in source order.
//
// This is the only place in the codebase that knows the shape of every node type.
// Consumers that need to walk the tree — the language server's position lookups, and
// anything added later — must go through here rather than carrying a type switch of their
// own. Three such private switches previously existed in internal/lsp and had drifted to
// covering 25, 11 and 18 of the 44 node types: each construct added to the language after
// a switch was written became invisible to it, silently and with no failing test.
// TestChildrenReachesEveryNodeField in this package is what now makes that impossible.
//
// Nodes whose children live in a Go map are emitted in source order, never map order, so
// two identical queries cannot disagree about what sits under the cursor.
func Children(n Node) []Node {
	if isNil(n) {
		return nil
	}

	var out []Node
	add := func(nodes ...Node) {
		for _, node := range nodes {
			if !isNil(node) {
				out = append(out, node)
			}
		}
	}

	switch n := n.(type) {
	// --- roots and blocks ---
	case *Program:
		for _, s := range n.Statements {
			add(s)
		}
	case *BlockStatement:
		for _, s := range n.Statements {
			add(s)
		}

	// --- declarations ---
	case *LetStatement:
		add(n.Name, n.ValueType, n.Value)
	case *ConstStatement:
		add(n.Name, n.ValueType, n.Value)
	case *TypeAliasStatement:
		add(n.Name)
		add(signatureTypes(n.Signature)...)
		add(n.TargetType)
		add(structFieldNodes(n.StructDefinition)...)
	case *TypeConstraintStatement:
		add(n.Name, n.BaseType, n.Predicate)
	case *UnionStatement:
		add(n.Name)
		for _, v := range n.Variants {
			add(v)
		}
	case *ImportStatement:
		add(n.Name)
		for _, imp := range n.NamedImports {
			add(imp)
		}

	// --- assignment and control flow ---
	case *AssignStatement:
		add(n.Name, n.Value)
	case *IndexAssignmentStatement:
		add(n.Left, n.Index, n.Value)
	case *PropertyAssignmentStatement:
		add(n.Object, n.Property, n.Value)
	case *ReturnStatement:
		add(n.ReturnValue)
	case *ExpressionStatement:
		add(n.Expression)
	case *AwaitStatement:
		for _, p := range n.Pipelines {
			add(p)
		}

	// --- operators ---
	case *PrefixExpression:
		add(n.Right)
	case *InfixExpression:
		add(n.Left, n.Right)
	case *IsExpression:
		add(n.Left, n.TypeName)
	case *IfExpression:
		add(n.Condition, n.Consequence, n.Alternative)

	// --- calls and access ---
	case *CallExpression:
		add(n.Function)
		for _, arg := range n.Arguments {
			add(arg)
		}
		// Named arguments sit in a slice of their own, separate from the positional
		// ones, and were reachable from no traversal at all before this.
		for _, named := range n.NamedArguments {
			add(named)
		}
	case *NamedArgument:
		add(n.Name, n.Value)
	case *IndexExpression:
		add(n.Left, n.Index)
	case *PropertyExpression:
		add(n.Object, n.Property)
	case *GenericIdentifier:
		add(n.Identifier)

	// --- pipes and concurrency ---
	case *SafePipeExpression:
		add(n.Left, n.Call)
	case *StreamPipeExpression:
		add(n.Left, n.Join, n.Call)
	case *JoinGroupExpression:
		for _, c := range n.Calls {
			add(c)
		}
	case *AsyncExpression:
		add(n.Right)
	case *UnwrapExpression:
		add(n.Right)

	// --- literals ---
	case *ArrayLiteral:
		for _, el := range n.Elements {
			add(el)
		}
	case *MapLiteral:
		for _, k := range sortedBySource(mapKeys(n.Pairs)) {
			add(k, n.Pairs[k])
		}
	case *StructLiteral:
		add(n.NameRef)
		for _, v := range sortedBySource(mapValues(n.Fields)) {
			add(v)
		}
	case *FunctionLiteral:
		for _, p := range n.Parameters {
			add(p)
		}
		add(n.ReturnType)
		add(n.Body)
	case *InterpolatedStringLiteral:
		for _, seg := range n.Segments {
			// A text segment carries no token and no identity of its own; only the
			// embedded expressions are addressable.
			add(seg.Expr)
		}

	// --- type annotations ---
	case *Parameter:
		add(n.Type)
	case *TypeExpr:
		for _, ref := range n.Refs {
			add(ref)
		}

		// --- leaves ---
		// Identifier, TypeRef and the scalar literals have no child nodes.
	}

	return out
}

// structFieldNodes reaches through StructDefinition and StructField, neither of which
// implements Node (they carry no token of their own), to the field names and type
// annotations that do. Without this, struct field declarations would be unaddressable.
func structFieldNodes(def *StructDefinition) []Node {
	if def == nil {
		return nil
	}
	out := make([]Node, 0, len(def.Fields)*2)
	for i := range def.Fields {
		if def.Fields[i].Name != nil {
			out = append(out, def.Fields[i].Name)
		}
		if def.Fields[i].Type != nil {
			out = append(out, def.Fields[i].Type)
		}
	}
	return out
}

// signatureTypes reaches through FunctionSignature, which is likewise not a Node, to the
// parameter and return annotations inside a `type Name fn(A, B) -> C` alias.
func signatureTypes(sig *FunctionSignature) []Node {
	if sig == nil {
		return nil
	}
	out := make([]Node, 0, len(sig.ParamTypes)+1)
	for _, pt := range sig.ParamTypes {
		if pt != nil {
			out = append(out, pt)
		}
	}
	if sig.ReturnType != nil {
		out = append(out, sig.ReturnType)
	}
	return out
}

// Inspect walks the tree rooted at n in depth-first, source order, calling f for every
// node. Returning false from f skips that node's subtree.
func Inspect(n Node, f func(Node) bool) {
	if isNil(n) || !f(n) {
		return
	}
	for _, child := range Children(n) {
		Inspect(child, f)
	}
}

// StartToken returns the token a node begins at. Four node types carry no token of their
// own — Program, and the three helper structs — and report the zero token; for the
// operator nodes whose own token is the operator rather than the start of the expression,
// the leftmost operand's token is used instead.
func StartToken(n Node) lexer.Token {
	switch n := n.(type) {
	case nil:
		return lexer.Token{}

	// The node's own token is the operator, which sits after the left operand.
	case *InfixExpression:
		return StartToken(n.Left)
	case *IsExpression:
		return StartToken(n.Left)
	case *IndexExpression:
		return StartToken(n.Left)
	case *PropertyExpression:
		return StartToken(n.Object)
	case *SafePipeExpression:
		return StartToken(n.Left)
	case *StreamPipeExpression:
		return StartToken(n.Left)

	case *Program:
		if len(n.Statements) > 0 {
			return StartToken(n.Statements[0])
		}
		return lexer.Token{}
	}

	if tok, ok := ownToken(n); ok {
		return tok
	}
	// Fall back to the first child that has a position.
	for _, child := range Children(n) {
		if tok := StartToken(child); tok.Line != 0 {
			return tok
		}
	}
	return lexer.Token{}
}

// Span returns the first and last token a node covers. It is derived from Children rather
// than from a second type switch, so it stays correct for any node whose Children is
// correct.
func Span(n Node) (start, end lexer.Token) {
	if isNil(n) {
		return lexer.Token{}, lexer.Token{}
	}

	start = StartToken(n)
	end = start

	// Closing delimiters are not children, so they must be considered explicitly or a
	// span would stop at the last element rather than at the bracket that closes it.
	if call, ok := n.(*CallExpression); ok {
		end = laterOf(end, call.RParenToken)
	}

	for _, child := range Children(n) {
		childStart, childEnd := Span(child)
		if start.Line == 0 || (childStart.Line != 0 && earlier(childStart, start)) {
			start = childStart
		}
		end = laterOf(end, childEnd)
	}
	return start, end
}

// ownToken reads a node's own `Token` field. Forty of the forty-four node types have one
// under exactly that name, so this is read reflectively rather than through a forty-case
// switch: a switch is the thing that drifts, and a node type added without being listed
// there would silently report no position at all.
func ownToken(n Node) (lexer.Token, bool) {
	v := reflect.ValueOf(n)
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return lexer.Token{}, false
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return lexer.Token{}, false
	}

	field := v.FieldByName("Token")
	if !field.IsValid() || field.Type() != reflect.TypeOf(lexer.Token{}) {
		return lexer.Token{}, false
	}
	return field.Interface().(lexer.Token), true
}

// isNil reports whether n carries no value, including the typed-nil case a plain
// `n == nil` misses: the parser leaves optional children (IfExpression.Alternative on an
// `if` without `else`) as a nil concrete pointer inside a non-nil Node interface. Every
// traversal here must screen for it before switching on the concrete type, or the switch
// matches and dereferences nil. See the typed-nil note in this package's CLAUDE.md.
func isNil(n Node) bool {
	if n == nil {
		return true
	}
	switch v := reflect.ValueOf(n); v.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Slice, reflect.Map, reflect.Func:
		return v.IsNil()
	default:
		return false
	}
}

func earlier(a, b lexer.Token) bool {
	if a.Line != b.Line {
		return a.Line < b.Line
	}
	return a.Column < b.Column
}

func laterOf(a, b lexer.Token) lexer.Token {
	if b.Line == 0 {
		return a
	}
	if a.Line == 0 || earlier(a, b) {
		return b
	}
	return a
}

func mapKeys(pairs map[Expression]Expression) []Expression {
	out := make([]Expression, 0, len(pairs))
	for k := range pairs {
		out = append(out, k)
	}
	return out
}

func mapValues(fields map[string]Expression) []Expression {
	out := make([]Expression, 0, len(fields))
	for _, v := range fields {
		out = append(out, v)
	}
	return out
}

func sortedBySource[T Node](nodes []T) []T {
	sort.Slice(nodes, func(i, j int) bool {
		return earlier(StartToken(nodes[i]), StartToken(nodes[j]))
	})
	return nodes
}
