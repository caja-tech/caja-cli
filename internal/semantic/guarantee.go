package semantic

import "caja-cli/internal/syntax"

// guaranteesReturn checks if a given AST node is guaranteed to execute a return
// statement on all of its code paths.
func guaranteesReturn(node syntax.Node) bool {
	switch n := node.(type) {
	case *syntax.ReturnStatement:
		return true

	case *syntax.BlockStatement:
		if len(n.Statements) == 0 {
			return false
		}
		lastStatement := n.Statements[len(n.Statements)-1]
		return guaranteesReturn(lastStatement)

	case *syntax.ExpressionStatement:
		return guaranteesReturn(n.Expression)

	case *syntax.IfExpression:
		if n.Alternative != nil {
			return guaranteesReturn(n.Consequence) && guaranteesReturn(n.Alternative)
		}
		return false

	default:
		return false
	}
}

// guaranteesRecursiveCall determines if evaluating the given AST node guarantees
// that the function named targetName will be recursively called unconditionally
// on all of its code execution paths.
func guaranteesRecursiveCall(node syntax.Node, targetName string) bool {
	if node == nil {
		return false
	}

	switch n := node.(type) {
	case *syntax.BlockStatement:
		for _, stmt := range n.Statements {
			if guaranteesRecursiveCall(stmt, targetName) {
				return true
			}
		}
		return false

	case *syntax.CallExpression:
		if ident, ok := n.Function.(*syntax.Identifier); ok && ident.Value == targetName {
			return true
		}

		for _, arg := range n.Arguments {
			if guaranteesRecursiveCall(arg, targetName) {
				return true
			}
		}
		return guaranteesRecursiveCall(n.Function, targetName)

	case *syntax.IfExpression:
		if guaranteesRecursiveCall(n.Condition, targetName) {
			return true
		}
		if n.Alternative == nil {
			return false
		}
		return guaranteesRecursiveCall(n.Consequence, targetName) &&
			guaranteesRecursiveCall(n.Alternative, targetName)

	case *syntax.ReturnStatement:
		return guaranteesRecursiveCall(n.ReturnValue, targetName)

	case *syntax.LetStatement:
		return guaranteesRecursiveCall(n.Value, targetName)

	case *syntax.ExpressionStatement:
		return guaranteesRecursiveCall(n.Expression, targetName)

	case *syntax.InfixExpression:
		return guaranteesRecursiveCall(n.Left, targetName) || guaranteesRecursiveCall(n.Right, targetName)

	case *syntax.FunctionLiteral:
		return false

	case *syntax.ArrayLiteral:
		for _, el := range n.Elements {
			if guaranteesRecursiveCall(el, targetName) {
				return true
			}
		}
		return false

	case *syntax.IndexExpression:
		return guaranteesRecursiveCall(n.Left, targetName) || guaranteesRecursiveCall(n.Index, targetName)

	default:
		return false
	}
}
