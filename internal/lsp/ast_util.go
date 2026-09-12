package lsp

import (
	"caja-cli/internal/lsp/posmap"
	"caja-cli/internal/pipeline/ast"
	"caja-cli/internal/pipeline/lexer"
)

// PathAt returns the chain of nodes enclosing a position, outermost first and innermost
// last. Every position-based feature is a question about this chain: hover and
// go-to-definition want its last element, signature help wants the innermost enclosing
// call, and selection range is the chain itself.
//
// line is 0-indexed and byteCol is a 0-based byte column — convert an editor position
// with posmap.LineIndex.ByteColumn before calling, never pass an LSP character offset.
func PathAt(root ast.Node, line, byteCol int) []ast.Node {
	// The lexer reports 1-based line and column.
	return appendPath(nil, root, line+1, byteCol+1)
}

func appendPath(path []ast.Node, node ast.Node, line, col int) []ast.Node {
	if node == nil {
		return path
	}

	for _, child := range ast.Children(node) {
		if !spansPosition(child, line, col) {
			continue
		}
		return appendPath(append(path, node), child, line, col)
	}
	return append(path, node)
}

// spansPosition reports whether a node covers a position. Container nodes are measured by
// their full span; a node that has no position at all (the parser could not place it)
// covers nothing.
func spansPosition(node ast.Node, line, col int) bool {
	start, end := ast.Span(node)
	if start.Line == 0 {
		return false
	}

	endCol := end.Column + posmap.TokenByteLen(end)
	switch {
	case line < start.Line || line > end.Line:
		return false
	case start.Line == end.Line:
		return col >= start.Column && col < endCol
	case line == start.Line:
		return col >= start.Column
	case line == end.Line:
		return col < endCol
	default:
		return true
	}
}

// FindNodeAtPosition returns the innermost node enclosing the given 0-indexed line and
// byte column, or nil when the position falls outside the tree.
func FindNodeAtPosition(node ast.Node, line, col int) ast.Node {
	path := PathAt(node, line, col)
	if len(path) == 0 {
		return nil
	}

	innermost := path[len(path)-1]
	// The root covers the whole file, so reporting it means "nothing specific here"
	// rather than a real hit — hovering blank space should answer nothing.
	if _, isRoot := innermost.(*ast.Program); isRoot {
		return nil
	}
	return innermost
}

// FindCallExpressionAtPosition returns the innermost call whose parentheses contain the
// position, which is the region where signature help applies.
//
// This deliberately searches the whole tree rather than reusing PathAt. Signature help is
// most wanted precisely when the call is still being typed — `substring("hello", ` with
// no closing paren yet — and there the cursor sits beyond every node that has been
// parsed, so a containment-based descent never reaches the call. Matching on the call's
// own parentheses instead still finds it.
func FindCallExpressionAtPosition(node ast.Node, line, col int) *ast.CallExpression {
	return findCallExpression(node, line+1, col+1)
}

func findCallExpression(node ast.Node, line, col int) *ast.CallExpression {
	if node == nil {
		return nil
	}

	// Children before self, so the innermost enclosing call wins.
	for _, child := range ast.Children(node) {
		if found := findCallExpression(child, line, col); found != nil {
			return found
		}
	}

	if call, ok := node.(*ast.CallExpression); ok && callSpansPosition(call, line, col) {
		return call
	}
	return nil
}

// callSpansPosition reports whether a 1-based line/column falls between a call's
// parentheses. Token is the opening paren and RParenToken the closing one; either may be
// absent in a partially-typed buffer, in which case the call has no usable span.
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
	default: // strictly between the boundary lines
		return true
	}
}

// GetNodeToken returns the token a node starts at.
func GetNodeToken(node ast.Node) lexer.Token {
	return ast.StartToken(node)
}
