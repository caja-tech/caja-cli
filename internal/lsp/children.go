package lsp

import (
	"sort"

	"caja-cli/internal/pipeline/ast"
)

// childNodes returns the direct child nodes of n, in source order.
//
// This is the single place in the package that knows the shape of the AST. The position
// lookups used to each carry their own partial type switch, which is why they drifted to
// covering 25, 11 and 18 of the language's 44 node types respectively: every construct
// added to Caja after a switch was written became invisible to it, silently, with no
// failing test. Routing every traversal through one enumerator means a construct is
// either reachable by all of them or by none, which is a bug you notice.
//
// Nodes whose children are held in a Go map are emitted in source order rather than map
// order, so repeated identical requests cannot return different answers.
func childNodes(n ast.Node) []ast.Node {
	if isNilNode(n) {
		return nil
	}

	var out []ast.Node
	add := func(nodes ...ast.Node) {
		for _, node := range nodes {
			if !isNilNode(node) {
				out = append(out, node)
			}
		}
	}

	switch n := n.(type) {
	// --- roots and blocks ---
	case *ast.Program:
		for _, s := range n.Statements {
			add(s)
		}
	case *ast.BlockStatement:
		for _, s := range n.Statements {
			add(s)
		}

	// --- declarations ---
	case *ast.LetStatement:
		add(n.Name, n.Value)
	case *ast.ConstStatement:
		add(n.Name, n.Value)
	case *ast.TypeAliasStatement:
		add(n.Name)
		if n.StructDefinition != nil {
			for i := range n.StructDefinition.Fields {
				add(n.StructDefinition.Fields[i].Name)
			}
		}
	case *ast.TypeConstraintStatement:
		add(n.Name, n.BaseType, n.Predicate)
	case *ast.UnionStatement:
		add(n.Name)
		for _, v := range n.Variants {
			add(v)
		}
	case *ast.ImportStatement:
		add(n.Name)
		for _, imp := range n.NamedImports {
			add(imp)
		}

	// --- assignments and jumps ---
	case *ast.AssignStatement:
		add(n.Name, n.Value)
	case *ast.IndexAssignmentStatement:
		add(n.Left, n.Index, n.Value)
	case *ast.PropertyAssignmentStatement:
		add(n.Object, n.Property, n.Value)
	case *ast.ReturnStatement:
		add(n.ReturnValue)
	case *ast.ExpressionStatement:
		add(n.Expression)
	case *ast.AwaitStatement:
		for _, p := range n.Pipelines {
			add(p)
		}

	// --- operators ---
	case *ast.PrefixExpression:
		add(n.Right)
	case *ast.InfixExpression:
		add(n.Left, n.Right)
	case *ast.IsExpression:
		add(n.Left)
	case *ast.IfExpression:
		add(n.Condition, n.Consequence, n.Alternative)

	// --- calls and access ---
	case *ast.CallExpression:
		add(n.Function)
		for _, arg := range n.Arguments {
			add(arg)
		}
		// Named arguments live in a separate slice from positional ones, and were
		// reachable from no traversal at all before this.
		for _, named := range n.NamedArguments {
			add(named)
		}
	case *ast.NamedArgument:
		add(n.Name, n.Value)
	case *ast.IndexExpression:
		add(n.Left, n.Index)
	case *ast.PropertyExpression:
		add(n.Object, n.Property)

	// --- pipes and concurrency ---
	case *ast.SafePipeExpression:
		add(n.Left, n.Call)
	case *ast.StreamPipeExpression:
		add(n.Left, n.Join, n.Call)
	case *ast.JoinGroupExpression:
		for _, c := range n.Calls {
			add(c)
		}
	case *ast.AsyncExpression:
		add(n.Right)
	case *ast.UnwrapExpression:
		add(n.Right)

	// --- literals ---
	case *ast.ArrayLiteral:
		for _, el := range n.Elements {
			add(el)
		}
	case *ast.MapLiteral:
		for _, k := range sortedKeys(n.Pairs) {
			add(k, n.Pairs[k])
		}
	case *ast.StructLiteral:
		for _, v := range sortedValues(n.Fields) {
			add(v)
		}
	case *ast.FunctionLiteral:
		for _, p := range n.Parameters {
			add(p)
		}
		add(n.Body)
	case *ast.InterpolatedStringLiteral:
		for _, seg := range n.Segments {
			// Text segments carry no token and no identity; only the embedded
			// expressions are addressable.
			add(seg.Expr)
		}
	}

	return out
}

// sortedKeys orders a map literal's keys by their position in the source, so traversal is
// deterministic. Go randomizes map iteration, which would otherwise let two identical
// requests disagree about which node sits under the cursor.
func sortedKeys(pairs map[ast.Expression]ast.Expression) []ast.Expression {
	keys := make([]ast.Expression, 0, len(pairs))
	for k := range pairs {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return beforeInSource(keys[i], keys[j]) })
	return keys
}

// sortedValues orders a struct literal's field values by source position. Ordering by the
// value rather than the field name is deliberate: the cursor is somewhere in the source,
// not somewhere in the alphabet.
func sortedValues(fields map[string]ast.Expression) []ast.Expression {
	values := make([]ast.Expression, 0, len(fields))
	for _, v := range fields {
		values = append(values, v)
	}
	sort.Slice(values, func(i, j int) bool { return beforeInSource(values[i], values[j]) })
	return values
}

func beforeInSource(a, b ast.Node) bool {
	at, bt := GetNodeToken(a), GetNodeToken(b)
	if at.Line != bt.Line {
		return at.Line < bt.Line
	}
	return at.Column < bt.Column
}
