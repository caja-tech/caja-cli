package lsp

import (
	"caja-cli/internal/pipeline/ast"
	"caja-cli/internal/pipeline/lexer"
)

// FindNodeAtPosition traverses the AST and returns the innermost Node
// that encapsulates the given line and column (0-indexed).
func FindNodeAtPosition(node ast.Node, line, col uint32) ast.Node {
	if node == nil {
		return nil
	}

	// 1-indexed conversion
	targetLine := int(line + 1)
	targetCol := int(col + 1)

	return findTightestNode(node, targetLine, targetCol)
}

func findTightestNode(node ast.Node, line, col int) ast.Node {
	if node == nil {
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
	var t lexer.Token
	// Since node interface only has TokenLiteral(), we need to use type assertion or reflection
	// to get the actual token line and column.
	switch n := node.(type) {
	case *ast.Identifier:
		t = n.Token
	case *ast.NumberLiteral:
		t = n.Token
	case *ast.StringLiteral:
		t = n.Token
	case *ast.BooleanLiteral:
		t = n.Token
	case *ast.NilLiteral:
		t = n.Token
	case *ast.GenericIdentifier:
		t = n.Token
	case *ast.CallExpression:
		t = n.Token
	default:
		return false // Many nodes span multiple lines. For exact hover we only care about terminal tokens usually.
	}

	length := len(t.Literal)
	if length == 0 {
		length = 1
	}

	if t.Line == line && col >= t.Column && col < t.Column+length {
		return true
	}
	return false
}
