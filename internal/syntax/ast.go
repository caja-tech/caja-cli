package syntax

import (
	"caja-cli/internal/lexer"
)

// Node is the base interface for every element in the abstract syntax tree.
// Every node can report the literal text of its originating token and produce
// a human-readable string representation of itself.
type Node interface {
	TokenLiteral() string
	String() string
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

func (p *Program) String() string {
	output := ""
	for _, statement := range p.Statements {
		output += statement.String()
	}
	return output
}

// Identifier is an expression node that represents a named variable reference.
// Token carries the original tokenizer.Token and Value holds the identifier's
// name as a plain string.
type Identifier struct {
	Token lexer.Token
	Value string
}

func (i *Identifier) expressionNode() {}
func (i *Identifier) TokenLiteral() string {
	return i.Token.Literal
}
func (i *Identifier) String() string { return i.Value }

// ReturnStatement is a statement node that represents an explicit return from
// the script (e.g. "return rate * 2"). Token holds the "return" keyword token
// and ReturnValue is the expression whose result becomes the script's output.
type ReturnStatement struct {
	Token       lexer.Token
	ReturnValue Expression
}

func (r *ReturnStatement) statementNode()       {}
func (r *ReturnStatement) TokenLiteral() string { return r.Token.Literal }
func (r *ReturnStatement) String() string {
	if r.ReturnValue != nil {
		return r.TokenLiteral() + " " + r.ReturnValue.String()
	}
	return r.TokenLiteral()
}

// BlockStatement is a statement node that represents a sequence of statements
// grouped together, typically enclosed in curly braces. Token holds the '{' token,
// and Statements contains the ordered list of statements within the block.
type BlockStatement struct {
	Token      lexer.Token
	Statements []Statement
}

func (bs *BlockStatement) statementNode()       {}
func (bs *BlockStatement) TokenLiteral() string { return bs.Token.Literal }
func (bs *BlockStatement) String() string {
	var out string
	for _, s := range bs.Statements {
		out += s.String()
	}
	return out
}

// NumberLiteral is an expression node that represents a numeric constant.
// Token carries the original tokenizer.Token, and Value holds the parsed
// float64 representation of the literal.
type NumberLiteral struct {
	Token lexer.Token
	Value float64
}

func (n *NumberLiteral) expressionNode() {}
func (n *NumberLiteral) TokenLiteral() string {
	return n.Token.Literal
}
func (n *NumberLiteral) String() string { return n.Token.Literal }

// InfixExpression is an expression node for binary operations such as
// addition, subtraction, multiplication, and division. It stores the operator
// token, the operator symbol as a string, and the left and right operands.
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
func (i *InfixExpression) String() string {
	return "(" + i.Left.String() + " " + i.Operator + " " + i.Right.String() + ")"
}

// IfExpression is an expression node that represents a conditional branching
// construct. Token holds the 'if' token, Condition is the expression to be
// evaluated, Consequence is the block of statements to execute if the condition
// is true, and Alternative is the optional block of statements to execute if
// the condition is false.
type IfExpression struct {
	Token       lexer.Token
	Condition   Expression
	Consequence *BlockStatement
	Alternative *BlockStatement
}

func (ie *IfExpression) expressionNode()      {}
func (ie *IfExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *IfExpression) String() string {
	out := "if " + ie.Condition.String() + " " + ie.Consequence.String()
	if ie.Alternative != nil {
		out += " else " + ie.Alternative.String()
	}
	return out
}

// LetStatement is a statement node that represents a variable declaration
// and initialization (e.g. "let rate = 100"). Token holds the "let" keyword token,
// Name is the target identifier, and Value is the right-hand-side expression.
type LetStatement struct {
	Token lexer.Token
	Name  *Identifier
	Value Expression
}

func (ls *LetStatement) statementNode()       {}
func (ls *LetStatement) TokenLiteral() string { return ls.Token.Literal }
func (ls *LetStatement) String() string {
	var out string
	out += ls.TokenLiteral() + " " + ls.Name.String() + " = "
	if ls.Value != nil {
		out += ls.Value.String()
	}
	return out
}

// AssignStatement is a statement node that binds the result of an expression
// to an identifier (e.g. "rate = 100 / 2"). Token holds the '=' token, Name
// is the target identifier, and Value is the right-hand-side expression.
type AssignStatement struct {
	Token lexer.Token
	Name  *Identifier
	Value Expression
}

func (a *AssignStatement) statementNode() {}
func (a *AssignStatement) TokenLiteral() string {
	return a.Token.Literal
}
func (a *AssignStatement) String() string {
	return a.Name.String() + " = " + a.Value.String()
}

// ExpressionStatement wraps a standalone expression that appears as a
// top-level statement (e.g. "rate + 50"). Token is the first token of the
// expression, and Expression holds the parsed expression tree.
type ExpressionStatement struct {
	Token      lexer.Token
	Expression Expression
}

func (e *ExpressionStatement) statementNode() {}
func (e *ExpressionStatement) TokenLiteral() string {
	return e.Token.Literal
}
func (e *ExpressionStatement) String() string {
	return e.Expression.String()
}
