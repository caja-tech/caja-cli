package parser

import (
	"caja-cli/internal/tokenizer"
	"fmt"
	"strconv"
)

const (
	_ int = iota
	LOWEST
	ASSIGN
	SUM
	PRODUCT
)

var precedences = map[tokenizer.TokenType]int{
	tokenizer.ASSIGN:   ASSIGN,
	tokenizer.PLUS:     SUM,
	tokenizer.MINUS:    SUM,
	tokenizer.ASTERISK: PRODUCT,
	tokenizer.SLASH:    PRODUCT,
}

// verifyPrecedenceLevel returns the precedence level associated with the given
// token type. If the token type has no registered precedence, LOWEST is returned.
func verifyPrecedenceLevel(t tokenizer.TokenType) int {
	if p, ok := precedences[t]; ok {
		return p
	}

	return LOWEST
}

// prefixParseFunc is a parsing function invoked when a token appears in prefix
// position (i.e. at the start of an expression). It returns the parsed
// Expression node.
type prefixParseFunc func() Expression

// infixParseFunc is a parsing function invoked when a token appears between two
// expressions (infix position). It receives the already-parsed left-hand side
// and returns the combined Expression node.
type infixParseFunc func(Expression) Expression

// Parser is a Pratt (top-down operator precedence) parser that transforms a
// stream of tokens produced by a Tokenizer into an abstract syntax tree. It
// maintains a current and peek token for single-token lookahead and dispatches
// to registered prefix and infix parse functions based on token type.
type Parser struct {
	tknzr *tokenizer.Tokenizer

	currToken tokenizer.Token
	peekToken tokenizer.Token

	prefixParseFuncs map[tokenizer.TokenType]prefixParseFunc
	infixParseFuncs  map[tokenizer.TokenType]infixParseFunc

	errors []string
}

// New creates a Parser for the given Tokenizer, registers the built-in prefix
// and infix parse functions for identifiers, numbers, grouped expressions, and
// arithmetic operators, and primes the two-token lookahead by reading twice.
func New(t *tokenizer.Tokenizer) *Parser {
	p := &Parser{
		tknzr: t,
	}

	p.prefixParseFuncs = make(map[tokenizer.TokenType]prefixParseFunc)
	p.prefixParseFuncs[tokenizer.IDENT] = p.parseIdentifier
	p.prefixParseFuncs[tokenizer.NUMBER] = p.parseNumberLiteral
	p.prefixParseFuncs[tokenizer.LPAREN] = p.parseGroupedExpression

	p.infixParseFuncs = make(map[tokenizer.TokenType]infixParseFunc)
	p.infixParseFuncs[tokenizer.PLUS] = p.parseInfixExpression
	p.infixParseFuncs[tokenizer.MINUS] = p.parseInfixExpression
	p.infixParseFuncs[tokenizer.ASTERISK] = p.parseInfixExpression
	p.infixParseFuncs[tokenizer.SLASH] = p.parseInfixExpression

	p.nextToken()
	p.nextToken()

	return p
}

// Parse consumes all tokens from the Tokenizer and returns a Program AST.
// Each iteration parses one statement; if parsing fails, synchronize is called
// to skip ahead to the next likely statement boundary before continuing.
func (p *Parser) Parse() *Program {
	program := &Program{}
	program.Statements = []Statement{}

	for p.currToken.Type != tokenizer.EOF {
		statement := p.parseStatement()
		if statement != nil {
			program.Statements = append(program.Statements, statement)
		} else {
			p.synchronize()
		}
		p.nextToken()
	}

	return program
}

// Errors returns the combined list of errors from both the tokenizer and the
// parser, in the order they were encountered.
func (p *Parser) Errors() []string {
	allErrors := append([]string{}, p.tknzr.Errors...)
	allErrors = append(allErrors, p.errors...)

	return allErrors
}

// parseStatement determines the kind of statement to parse by peeking at the
// next token. If the next token is an ASSIGN operator, an AssignStatement is
// parsed; otherwise the current tokens are treated as an ExpressionStatement.
func (p *Parser) parseStatement() Statement {
	if p.peekToken.Type == tokenizer.ASSIGN {
		return p.parseAssignStatement()
	}

	return p.parseExpressionStatement()
}

// parseAssignStatement parses an assignment of the form "identifier = expr".
// It captures the left-hand identifier, consumes the '=' token, and parses
// the right-hand expression with the lowest precedence.
func (p *Parser) parseAssignStatement() *AssignStatement {
	statement := &AssignStatement{
		Name: &Identifier{Token: p.currToken, Value: p.currToken.Literal},
	}

	p.nextToken()
	statement.Token = p.currToken

	p.nextToken()
	statement.Value = p.parseExpression(LOWEST)

	return statement
}

// parseExpressionStatement wraps a standalone expression (one that is not part
// of an assignment) into an ExpressionStatement node.
func (p *Parser) parseExpressionStatement() *ExpressionStatement {
	statement := &ExpressionStatement{Token: p.currToken}
	statement.Expression = p.parseExpression(LOWEST)
	return statement
}

// nextToken advances the parser's two-token window by shifting peekToken into
// currToken and reading the next token from the Tokenizer into peekToken.
func (p *Parser) nextToken() {
	p.currToken = p.peekToken
	p.peekToken = p.tknzr.NextToken()
}

// parseIdentifier returns an Identifier expression node for the current token.
func (p *Parser) parseIdentifier() Expression {
	return &Identifier{Token: p.currToken, Value: p.currToken.Literal}
}

// parseNumberLiteral converts the current token's literal into a float64 and
// returns a NumberLiteral expression node. If the literal cannot be parsed as
// a float, an error is recorded and nil is returned.
func (p *Parser) parseNumberLiteral() Expression {
	literal := &NumberLiteral{Token: p.currToken}

	value, err := strconv.ParseFloat(p.currToken.Literal, 64)
	if err != nil {
		p.errors = append(p.errors, fmt.Sprintf("could not parse %q as float", p.currToken.Literal))
		return nil
	}

	literal.Value = value
	return literal
}

// parseExpression is the core of the Pratt parser. It looks up a prefix parse
// function for the current token, then repeatedly applies infix parse functions
// as long as the next token's precedence exceeds the given precedence level,
// building a left-recursive expression tree.
func (p *Parser) parseExpression(precedence int) Expression {
	prefix := p.prefixParseFuncs[p.currToken.Type]
	if prefix == nil {
		p.errors = append(p.errors, fmt.Sprintf("unknown prefix type %q", p.currToken.Type))
		return nil
	}

	leftExpression := prefix()
	for p.peekToken.Type != tokenizer.EOF && precedence < verifyPrecedenceLevel(p.peekToken.Type) {
		infix := p.infixParseFuncs[p.peekToken.Type]
		if infix == nil {
			return leftExpression
		}

		p.nextToken()
		leftExpression = infix(leftExpression)
	}

	return leftExpression
}

// parseInfixExpression builds an InfixExpression node using the already-parsed
// left operand, the current operator token, and a recursively parsed right
// operand whose binding power is determined by the current token's precedence.
func (p *Parser) parseInfixExpression(left Expression) Expression {
	expression := &InfixExpression{
		Token:    p.currToken,
		Operator: p.currToken.Literal,
		Left:     left,
	}

	precedence := verifyPrecedenceLevel(p.currToken.Type)
	p.nextToken()

	expression.Right = p.parseExpression(precedence)

	return expression
}

// parseGroupedExpression handles parenthesized sub-expressions. It consumes
// the opening '(', recursively parses the inner expression with the lowest
// precedence, and expects a closing ')' — recording an error if it is missing.
func (p *Parser) parseGroupedExpression() Expression {
	p.nextToken()

	groupedExpression := p.parseExpression(LOWEST)
	if !p.expectPeek(tokenizer.RPAREN) {
		return nil
	}

	return groupedExpression
}

// expectPeek checks whether the peek token matches the expected type. If it
// does, the parser advances and returns true; otherwise it records a peek error
// and returns false without advancing.
func (p *Parser) expectPeek(tokenType tokenizer.TokenType) bool {
	if p.peekToken.Type == tokenType {
		p.nextToken()
		return true
	}

	p.peekError(tokenType)
	return false
}

// peekError appends a formatted syntax-error message indicating that the peek
// token did not match the expected type.
func (p *Parser) peekError(t tokenizer.TokenType) {
	msg := fmt.Sprintf("Syntax Error: expected next token to be %s, got %s instead", t, p.peekToken.Type)
	p.errors = append(p.errors, msg)
}

// synchronize performs panic-mode error recovery by discarding tokens until it
// finds one that could plausibly begin a new statement (an IDENT or NUMBER) or
// until EOF is reached.
func (p *Parser) synchronize() {
	for p.peekToken.Type != tokenizer.EOF {
		if p.peekToken.Type == tokenizer.IDENT || p.peekToken.Type == tokenizer.NUMBER {
			return
		}
		p.nextToken()
	}
}
