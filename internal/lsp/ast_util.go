package lsp

import (
	"reflect"

	"caja-cli/internal/lsp/posmap"
	"caja-cli/internal/pipeline/ast"
	"caja-cli/internal/pipeline/lexer"
)

// isNilNode reports whether node carries no value, including the typed-nil case that a
// plain `node == nil` misses: the parser leaves optional children as a nil concrete
// pointer boxed in a non-nil ast.Node interface — IfExpression.Alternative on an `if`
// without `else` is the common one. Without this guard the type switches below match the
// concrete case and dereference nil, so hover, definition and signature help crash
// anywhere in a file containing an unpaired `if`.
func isNilNode(node ast.Node) bool {
	if node == nil {
		return true
	}
	switch v := reflect.ValueOf(node); v.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Slice, reflect.Map, reflect.Func:
		return v.IsNil()
	default:
		return false
	}
}

// FindNodeAtPosition traverses the AST and returns the innermost Node
// that encapsulates the given line and column (0-indexed).
func FindNodeAtPosition(node ast.Node, line, col int) ast.Node {
	if isNilNode(node) {
		return nil
	}

	// 1-indexed conversion
	targetLine := line + 1
	targetCol := col + 1

	return findTightestNode(node, targetLine, targetCol)
}

func findTightestNode(node ast.Node, line, col int) ast.Node {
	if isNilNode(node) {
		return nil
	}

	// Recursively search children based on node type
	var bestChild ast.Node

	switch n := node.(type) {
	case *ast.Program:
		for _, s := range n.Statements {
			if child := findTightestNode(s, line, col); child != nil {
				return child
			}
		}
	case *ast.BlockStatement:
		for _, s := range n.Statements {
			if child := findTightestNode(s, line, col); child != nil {
				return child
			}
		}
	case *ast.TypeConstraintStatement:
		if child := findTightestNode(n.Name, line, col); child != nil {
			bestChild = child
		} else if child := findTightestNode(n.BaseType, line, col); child != nil {
			bestChild = child
		} else if child := findTightestNode(n.Predicate, line, col); child != nil {
			bestChild = child
		}
	case *ast.UnionStatement:
		if child := findTightestNode(n.Name, line, col); child != nil {
			bestChild = child
		} else {
			for _, variant := range n.Variants {
				if child := findTightestNode(variant, line, col); child != nil {
					bestChild = child
					break
				}
			}
		}
	case *ast.IsExpression:
		if child := findTightestNode(n.Left, line, col); child != nil {
			bestChild = child
		}
	case *ast.SafePipeExpression:
		if child := findTightestNode(n.Left, line, col); child != nil {
			bestChild = child
		} else if child := findTightestNode(n.Call, line, col); child != nil {
			bestChild = child
		}
	case *ast.LetStatement:
		if child := findTightestNode(n.Name, line, col); child != nil {
			bestChild = child
		} else if child := findTightestNode(n.Value, line, col); child != nil {
			bestChild = child
		}
	case *ast.ConstStatement:
		if child := findTightestNode(n.Name, line, col); child != nil {
			bestChild = child
		} else if child := findTightestNode(n.Value, line, col); child != nil {
			bestChild = child
		}
	case *ast.AssignStatement:
		if child := findTightestNode(n.Name, line, col); child != nil {
			bestChild = child
		} else if child := findTightestNode(n.Value, line, col); child != nil {
			bestChild = child
		}
	case *ast.IndexAssignmentStatement:
		if child := findTightestNode(n.Left, line, col); child != nil {
			bestChild = child
		} else if child := findTightestNode(n.Index, line, col); child != nil {
			bestChild = child
		} else if child := findTightestNode(n.Value, line, col); child != nil {
			bestChild = child
		}
	case *ast.PropertyAssignmentStatement:
		if child := findTightestNode(n.Object, line, col); child != nil {
			bestChild = child
		} else if child := findTightestNode(n.Property, line, col); child != nil {
			bestChild = child
		} else if child := findTightestNode(n.Value, line, col); child != nil {
			bestChild = child
		}
	case *ast.ImportStatement:
		if child := findTightestNode(n.Name, line, col); child != nil {
			bestChild = child
		} else {
			for _, named := range n.NamedImports {
				if child := findTightestNode(named, line, col); child != nil {
					bestChild = child
					break
				}
			}
		}
	case *ast.ReturnStatement:
		if child := findTightestNode(n.ReturnValue, line, col); child != nil {
			bestChild = child
		}
	case *ast.ExpressionStatement:
		if child := findTightestNode(n.Expression, line, col); child != nil {
			bestChild = child
		}
	case *ast.PrefixExpression:
		if child := findTightestNode(n.Right, line, col); child != nil {
			bestChild = child
		}
	case *ast.InfixExpression:
		if child := findTightestNode(n.Left, line, col); child != nil {
			bestChild = child
		} else if child := findTightestNode(n.Right, line, col); child != nil {
			bestChild = child
		}
	case *ast.IfExpression:
		if child := findTightestNode(n.Condition, line, col); child != nil {
			bestChild = child
		} else if child := findTightestNode(n.Consequence, line, col); child != nil {
			bestChild = child
		} else if child := findTightestNode(n.Alternative, line, col); child != nil {
			bestChild = child
		}
	case *ast.CallExpression:
		if child := findTightestNode(n.Function, line, col); child != nil {
			bestChild = child
		}
		for _, arg := range n.Arguments {
			if child := findTightestNode(arg, line, col); child != nil {
				bestChild = child
			}
		}
	case *ast.IndexExpression:
		if child := findTightestNode(n.Left, line, col); child != nil {
			bestChild = child
		} else if child := findTightestNode(n.Index, line, col); child != nil {
			bestChild = child
		}
	case *ast.PropertyExpression:
		if child := findTightestNode(n.Object, line, col); child != nil {
			bestChild = child
		} else if child := findTightestNode(n.Property, line, col); child != nil {
			bestChild = child
		}
	case *ast.ArrayLiteral:
		for _, el := range n.Elements {
			if child := findTightestNode(el, line, col); child != nil {
				bestChild = child
			}
		}
	case *ast.InterpolatedStringLiteral:
		for _, seg := range n.Segments {
			if seg.Expr == nil {
				continue
			}
			if child := findTightestNode(seg.Expr, line, col); child != nil {
				bestChild = child
			}
		}
	case *ast.MapLiteral:
		for k, v := range n.Pairs {
			if child := findTightestNode(k, line, col); child != nil {
				bestChild = child
			} else if child := findTightestNode(v, line, col); child != nil {
				bestChild = child
			}
		}
	case *ast.StructLiteral:
		for _, v := range n.Fields {
			if child := findTightestNode(v, line, col); child != nil {
				bestChild = child
			}
		}
	case *ast.FunctionLiteral:
		for _, p := range n.Parameters {
			if child := findTightestNode(p, line, col); child != nil {
				bestChild = child
			}
		}
		if child := findTightestNode(n.Body, line, col); child != nil {
			bestChild = child
		}
	}

	if bestChild != nil {
		return bestChild
	}

	// Base case: verify if this node itself encapsulates the position.
	if containsPosition(node, line, col) {
		return node
	}

	return nil
}

func containsPosition(node ast.Node, line, col int) bool {
	if isNilNode(node) {
		return false
	}

	var t lexer.Token
	// Since node interface only has TokenLiteral(), we need to use type assertion or reflection
	// to get the actual token line and column.
	switch n := node.(type) {
	case *ast.TypeConstraintStatement:
		t = n.Token
	case *ast.SafePipeExpression:
		t = n.Token
	case *ast.Identifier:
		t = n.Token
	case *ast.NumberLiteral:
		t = n.Token
	case *ast.StringLiteral:
		t = n.Token
	case *ast.InterpolatedStringLiteral:
		t = n.Token
	case *ast.BooleanLiteral:
		t = n.Token
	case *ast.NilLiteral:
		t = n.Token
	case *ast.GenericIdentifier:
		t = n.Token
	case *ast.CallExpression:
		t = n.Token
	case *ast.Parameter:
		t = n.Token
	default:
		return false // Many nodes span multiple lines. For exact hover we only care about terminal tokens usually.
	}

	// posmap.TokenByteLen, not len(t.Literal): the lexer strips the delimiters from a
	// quoted literal, so `"abc"` reports a literal of `abc`. Measuring the span with the
	// literal's own length leaves the last character and the closing quote outside the
	// node, and hovering there finds nothing.
	length := posmap.TokenByteLen(t)
	if length == 0 {
		length = 1
	}

	return t.Line == line && col >= t.Column && col < t.Column+length
}

// FindCallExpressionAtPosition recursively searches the AST for the tightest CallExpression
// that encapsulates the given line and column, including the area between the '(' and ')'.
func FindCallExpressionAtPosition(node ast.Node, line, col int) *ast.CallExpression {
	if isNilNode(node) {
		return nil
	}
	targetLine := line + 1
	targetCol := col + 1

	return findCallExpression(node, targetLine, targetCol)
}

func findCallExpression(node ast.Node, line, col int) *ast.CallExpression {
	if isNilNode(node) {
		return nil
	}

	// Depth-first, children before self, so the innermost enclosing call wins. The
	// previous implementation carried its own partial type switch and assigned the last
	// match rather than the innermost, so signature help went missing inside const
	// bindings, array/map/struct literals, trailing lambdas, property chains and string
	// interpolation — every expression position its switch had never been taught about.
	for _, child := range childNodes(node) {
		if found := findCallExpression(child, line, col); found != nil {
			return found
		}
	}

	if call, ok := node.(*ast.CallExpression); ok && callSpansPosition(call, line, col) {
		return call
	}
	return nil
}

// callSpansPosition reports whether line/col falls within a call's parentheses, which is
// the region where signature help applies. Token is the opening paren and RParenToken the
// closing one; either may be zero in a partially-typed buffer, in which case the call has
// no usable span.
func callSpansPosition(call *ast.CallExpression, line, col int) bool {
	start, end := call.Token, call.RParenToken
	if start.Line == 0 || end.Line == 0 {
		return false
	}

	switch {
	case line < start.Line || line > end.Line:
		return false
	case start.Line == end.Line:
		return col >= start.Column && col <= end.Column
	case line == start.Line:
		return col >= start.Column
	case line == end.Line:
		return col <= end.Column
	default: // strictly between the two boundary lines
		return true
	}
}

// GetNodeToken extracts the starting token of an AST node.
func GetNodeToken(node ast.Node) lexer.Token {
	var t lexer.Token
	if isNilNode(node) {
		return t
	}

	switch n := node.(type) {
	case *ast.TypeConstraintStatement:
		t = n.Token
	case *ast.SafePipeExpression:
		t = n.Token
	case *ast.Identifier:
		t = n.Token
	case *ast.NumberLiteral:
		t = n.Token
	case *ast.StringLiteral:
		t = n.Token
	case *ast.InterpolatedStringLiteral:
		t = n.Token
	case *ast.BooleanLiteral:
		t = n.Token
	case *ast.NilLiteral:
		t = n.Token
	case *ast.GenericIdentifier:
		t = n.Token
	case *ast.CallExpression:
		t = n.Token
	case *ast.PropertyExpression:
		t = n.Token
	case *ast.IndexExpression:
		t = n.Token
	case *ast.InfixExpression:
		t = GetNodeToken(n.Left)
	case *ast.IsExpression:
		t = GetNodeToken(n.Left)
	case *ast.PrefixExpression:
		t = n.Token
	case *ast.UnionStatement:
		t = n.Token
	case *ast.ArrayLiteral:
		t = n.Token
	case *ast.MapLiteral:
		t = n.Token
	case *ast.FunctionLiteral:
		t = n.Token
	case *ast.Parameter:
		t = n.Token
	}
	return t
}
