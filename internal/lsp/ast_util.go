package lsp

import (
	"caja-cli/internal/pipeline/ast"
	"caja-cli/internal/pipeline/lexer"
)

// FindNodeAtPosition traverses the AST and returns the innermost Node
// that encapsulates the given line and column (0-indexed).
func FindNodeAtPosition(node ast.Node, line, col int) ast.Node {
	if node == nil {
		return nil
	}

	// 1-indexed conversion
	targetLine := line + 1
	targetCol := col + 1

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
	case *ast.TypeConstraintStatement:
		if child := findTightestNode(n.Name, line, col); child != nil {
			bestChild = child
		} else if child := findTightestNode(n.BaseType, line, col); child != nil {
			bestChild = child
		} else if child := findTightestNode(n.Predicate, line, col); child != nil {
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

// FindCallExpressionAtPosition recursively searches the AST for the tightest CallExpression
// that encapsulates the given line and column, including the area between the '(' and ')'.
func FindCallExpressionAtPosition(node ast.Node, line, col int) *ast.CallExpression {
	if node == nil {
		return nil
	}
	targetLine := line + 1
	targetCol := col + 1

	return findCallExpression(node, targetLine, targetCol)
}

func findCallExpression(node ast.Node, line, col int) *ast.CallExpression {
	if node == nil {
		return nil
	}

	var bestCall *ast.CallExpression

	switch n := node.(type) {
	case *ast.Program:
		for _, s := range n.Statements {
			if child := findCallExpression(s, line, col); child != nil {
				bestCall = child
			}
		}
	case *ast.BlockStatement:
		for _, s := range n.Statements {
			if child := findCallExpression(s, line, col); child != nil {
				bestCall = child
			}
		}
	case *ast.ExpressionStatement:
		if child := findCallExpression(n.Expression, line, col); child != nil {
			bestCall = child
		}
	case *ast.SafePipeExpression:
		if child := findCallExpression(n.Left, line, col); child != nil {
			bestCall = child
		} else if child := findCallExpression(n.Call, line, col); child != nil {
			bestCall = child
		}
	case *ast.LetStatement:
		if child := findCallExpression(n.Value, line, col); child != nil {
			bestCall = child
		}
	case *ast.AssignStatement:
		if child := findCallExpression(n.Value, line, col); child != nil {
			bestCall = child
		}
	case *ast.ReturnStatement:
		if child := findCallExpression(n.ReturnValue, line, col); child != nil {
			bestCall = child
		}
	case *ast.CallExpression:
		// Check arguments first
		for _, arg := range n.Arguments {
			if child := findCallExpression(arg, line, col); child != nil {
				return child
			}
		}
		
		// If not inside arguments, check if we are inside this call's parentheses
		startTok := n.Token
		endTok := n.RParenToken
		
		if startTok.Line == 0 || endTok.Line == 0 {
			break
		}
		
		// Multi-line call
		if line > startTok.Line && line < endTok.Line {
			return n
		}
		// Single-line call
		if startTok.Line == endTok.Line && line == startTok.Line {
			if col >= startTok.Column && col <= endTok.Column {
				return n
			}
		}
		// Multi-line start boundary
		if line == startTok.Line && line < endTok.Line {
			if col >= startTok.Column {
				return n
			}
		}
		// Multi-line end boundary
		if line > startTok.Line && line == endTok.Line {
			if col <= endTok.Column {
				return n
			}
		}
	case *ast.InfixExpression:
		if child := findCallExpression(n.Left, line, col); child != nil {
			bestCall = child
		} else if child := findCallExpression(n.Right, line, col); child != nil {
			bestCall = child
		}
	case *ast.IfExpression:
		if child := findCallExpression(n.Condition, line, col); child != nil {
			bestCall = child
		} else if child := findCallExpression(n.Consequence, line, col); child != nil {
			bestCall = child
		} else if child := findCallExpression(n.Alternative, line, col); child != nil {
			bestCall = child
		}
	}

	return bestCall
}

// GetNodeToken extracts the starting token of an AST node.
func GetNodeToken(node ast.Node) lexer.Token {
	var t lexer.Token
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
	case *ast.PrefixExpression:
		t = n.Token
	case *ast.ArrayLiteral:
		t = n.Token
	case *ast.MapLiteral:
		t = n.Token
	case *ast.FunctionLiteral:
		t = n.Token
	}
	return t
}
