package parser

import "caja-cli/internal/lexer"

type Node interface {
	TokenLiteral() string
}

type Statement interface {
	Node
	statementNode()
}

type Expression interface {
	Node
	expressionNode()
}

type Program struct {
	Statements []Statement
}

func (p *Program) TokenLiteral() string {
	if len(p.Statements) > 0 {
		return p.Statements[0].TokenLiteral()
	}

	return ""
}

type Identifier struct {
	Token lexer.Token
	Value string
}

func (i *Identifier) expressionNode() {}
func (i *Identifier) TokenLiteral() string {
	return i.Token.Literal
}

type NumberLiteral struct {
	Token lexer.Token
	Value float64
}

func (n *NumberLiteral) expressionNode() {}
func (n *NumberLiteral) TokenLiteral() string {
	return n.Token.Literal
}

type InfixExpression struct {
	Token    lexer.Token
	Left     Expression
	Operator string
	Right    Expression
}

func (i *InfixExpression) expressionNode() {}
func (i *InfixExpression) TokenLiteral() string {
	return i.Token.Literal
}

type AssignStatement struct {
	Token lexer.Token
	Name  *Identifier
	Value Expression
}

func (a *AssignStatement) statementNode() {}
func (a *AssignStatement) TokenLiteral() string {
	return a.Token.Literal
}

type ExpressionStatement struct {
	Token      lexer.Token
	Expression Expression
}

func (e *ExpressionStatement) statementNode() {}
func (e *ExpressionStatement) TokenLiteral() string {
	return e.Token.Literal
}
