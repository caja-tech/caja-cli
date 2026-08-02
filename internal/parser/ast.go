package parser

import (
	"caja-cli/internal/tokenizer"
)

// Node is the base interface for every element in the abstract syntax tree.
// Every node can report the literal text of its originating token and produce
// a human-readable string representation of itself.
type Node interface {
	TokenLiteral() string
	ToString() string
}

// Statement represents an AST node that performs an action (e.g. an
// assignment). It embeds Node and adds the unexported statementNode marker
// method to distinguish statements from expressions at compile time.
type Statement interface {
	Node
	statementNode()
}

// Expression represents an AST node that produces a value (e.g. a number
// literal or an infix operation). It embeds Node and adds the unexported
// expressionNode marker method to distinguish expressions from statements.
type Expression interface {
	Node
	expressionNode()
}

// Program is the root node of every parsed AST. It holds the ordered list of
// top-level statements that make up the source program.
type Program struct {
	Statements []Statement
}

func (p *Program) TokenLiteral() string {
	if len(p.Statements) > 0 {
		return p.Statements[0].TokenLiteral()
	}

	return ""
}

func (p *Program) ToString() string {
	output := ""
	for _, statement := range p.Statements {
		output += statement.ToString()
	}
	return output
}

// Identifier is an expression node that represents a named variable reference.
// Token carries the original tokenizer.Token and Value holds the identifier's
// name as a plain string.
type Identifier struct {
	Token tokenizer.Token
	Value string
}

func (i *Identifier) expressionNode() {}
func (i *Identifier) TokenLiteral() string {
	return i.Token.Literal
}
func (i *Identifier) ToString() string { return i.Value }

// ReturnStatement is a statement node that represents an explicit return from
// the script (e.g. "return rate * 2"). Token holds the "return" keyword token
// and ReturnValue is the expression whose result becomes the script's output.
type ReturnStatement struct {
	Token       tokenizer.Token
	ReturnValue Expression
}

func (r *ReturnStatement) statementNode()       {}
func (r *ReturnStatement) TokenLiteral() string { return r.Token.Literal }
func (r *ReturnStatement) ToString() string {
	if r.ReturnValue != nil {
		return r.TokenLiteral() + " " + r.ReturnValue.ToString()
	}
	return r.TokenLiteral()
}

// NumberLiteral is an expression node that represents a numeric constant.
// Token carries the original tokenizer.Token, and Value holds the parsed
// float64 representation of the literal.
type NumberLiteral struct {
	Token tokenizer.Token
	Value float64
}

func (n *NumberLiteral) expressionNode() {}
func (n *NumberLiteral) TokenLiteral() string {
	return n.Token.Literal
}
func (n *NumberLiteral) ToString() string { return n.Token.Literal }

// InfixExpression is an expression node for binary operations such as
// addition, subtraction, multiplication, and division. It stores the operator
// token, the operator symbol as a string, and the left and right operands.
type InfixExpression struct {
	Token    tokenizer.Token
	Left     Expression
	Operator string
	Right    Expression
}

func (i *InfixExpression) expressionNode() {}
func (i *InfixExpression) TokenLiteral() string {
	return i.Token.Literal
}
func (i *InfixExpression) ToString() string {
	return "(" + i.Left.ToString() + " " + i.Operator + " " + i.Right.ToString() + ")"
}

// AssignStatement is a statement node that binds the result of an expression
// to an identifier (e.g. "rate = 100 / 2"). Token holds the '=' token, Name
// is the target identifier, and Value is the right-hand-side expression.
type AssignStatement struct {
	Token tokenizer.Token
	Name  *Identifier
	Value Expression
}

func (a *AssignStatement) statementNode() {}
func (a *AssignStatement) TokenLiteral() string {
	return a.Token.Literal
}
func (a *AssignStatement) ToString() string {
	return a.Name.ToString() + " = " + a.Value.ToString()
}

// ExpressionStatement wraps a standalone expression that appears as a
// top-level statement (e.g. "rate + 50"). Token is the first token of the
// expression, and Expression holds the parsed expression tree.
type ExpressionStatement struct {
	Token      tokenizer.Token
	Expression Expression
}

func (e *ExpressionStatement) statementNode() {}
func (e *ExpressionStatement) TokenLiteral() string {
	return e.Token.Literal
}
func (e *ExpressionStatement) ToString() string {
	return e.Expression.ToString()
}
