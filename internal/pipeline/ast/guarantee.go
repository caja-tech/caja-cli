package ast

// guaranteesReturn checks if a given AST node is guaranteed to execute a return
// statement on all of its code paths.
func GuaranteesReturn(node Node) bool {
	switch n := node.(type) {
	case *ReturnStatement:
		return true

	case *BlockStatement:
		if len(n.Statements) == 0 {
			return false
		}
		lastStatement := n.Statements[len(n.Statements)-1]
		return GuaranteesReturn(lastStatement)

	case *ExpressionStatement:
		return GuaranteesReturn(n.Expression)

	case *IfExpression:
		if n.Alternative != nil {
			return GuaranteesReturn(n.Consequence) && GuaranteesReturn(n.Alternative)
		}
		return false

	default:
		return false
	}
}

// guaranteesRecursiveCall determines if evaluating the given AST node guarantees
// that the function named targetName will be recursively called unconditionally
// on all of its code execution paths.
func GuaranteesRecursiveCall(node Node, targetName string) bool {
	if node == nil {
		return false
	}

	switch n := node.(type) {
	case *BlockStatement:
		for _, stmt := range n.Statements {
			if GuaranteesRecursiveCall(stmt, targetName) {
				return true
			}
		}
		return false

	case *CallExpression:
		if ident, ok := n.Function.(*Identifier); ok && ident.Value == targetName {
			return true
		}

		for _, arg := range n.Arguments {
			if GuaranteesRecursiveCall(arg, targetName) {
				return true
			}
		}
		return GuaranteesRecursiveCall(n.Function, targetName)

	case *IfExpression:
		if GuaranteesRecursiveCall(n.Condition, targetName) {
			return true
		}
		if n.Alternative == nil {
			return false
		}
		return GuaranteesRecursiveCall(n.Consequence, targetName) &&
			GuaranteesRecursiveCall(n.Alternative, targetName)

	case *ReturnStatement:
		return GuaranteesRecursiveCall(n.ReturnValue, targetName)

	case *LetStatement:
		return GuaranteesRecursiveCall(n.Value, targetName)

	case *ExpressionStatement:
		return GuaranteesRecursiveCall(n.Expression, targetName)

	case *InfixExpression:
		return GuaranteesRecursiveCall(n.Left, targetName) || GuaranteesRecursiveCall(n.Right, targetName)

	case *FunctionLiteral:
		return false

	case *ArrayLiteral:
		for _, el := range n.Elements {
			if GuaranteesRecursiveCall(el, targetName) {
				return true
			}
		}
		return false

	case *IndexExpression:
		return GuaranteesRecursiveCall(n.Left, targetName) || GuaranteesRecursiveCall(n.Index, targetName)

	default:
		return false
	}
}
