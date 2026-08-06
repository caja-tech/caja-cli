package syntax

import (
	"caja-cli/internal/lexer"
	"fmt"
	"strconv"
)

const (
	_ int = iota
	LOWEST
	ASSIGN
	COMPARISON
	SUM
	PRODUCT
	EXPONENT
	CALL
)

var precedences = map[lexer.TokenType]int{
	lexer.ASSIGN:   ASSIGN,
	lexer.LT:       COMPARISON,
	lexer.GT:       COMPARISON,
	lexer.LTEQ:     COMPARISON,
	lexer.GTEQ:     COMPARISON,
	lexer.EQ:       COMPARISON,
	lexer.NEQ:      COMPARISON,
	lexer.PLUS:     SUM,
	lexer.MINUS:    SUM,
	lexer.ASTERISK: PRODUCT,
	lexer.SLASH:    PRODUCT,
	lexer.MODULO:   PRODUCT,
	lexer.POWER:    EXPONENT,
	lexer.LPAREN:   CALL,
}

// verifyPrecedenceLevel returns the precedence level associated with the given
// token type. If the token type has no registered precedence, LOWEST is returned.
func verifyPrecedenceLevel(t lexer.TokenType) int {
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
	tknzr *lexer.Tokenizer

	currToken lexer.Token
	peekToken lexer.Token

	prefixParseFuncs map[lexer.TokenType]prefixParseFunc
	infixParseFuncs  map[lexer.TokenType]infixParseFunc

	errors []string
}

// New creates a Parser for the given Tokenizer, registers the built-in prefix
// and infix parse functions for identifiers, numbers, grouped expressions, and
// arithmetic operators, and primes the two-token lookahead by reading twice.
func New(t *lexer.Tokenizer) *Parser {
	p := &Parser{
		tknzr: t,
	}

	p.prefixParseFuncs = make(map[lexer.TokenType]prefixParseFunc)
	p.prefixParseFuncs[lexer.IDENT] = p.parseIdentifier
	p.prefixParseFuncs[lexer.NUMBER] = p.parseNumberLiteral
	p.prefixParseFuncs[lexer.STRING] = p.parseStringLiteral
	p.prefixParseFuncs[lexer.LPAREN] = p.parseGroupedExpression
	p.prefixParseFuncs[lexer.IF] = p.parseIfExpression
	p.prefixParseFuncs[lexer.TRUE] = p.parseBoolean
	p.prefixParseFuncs[lexer.FALSE] = p.parseBoolean
	p.prefixParseFuncs[lexer.FN] = p.parseFunctionLiteral

	p.infixParseFuncs = make(map[lexer.TokenType]infixParseFunc)
	p.infixParseFuncs[lexer.PLUS] = p.parseInfixExpression
	p.infixParseFuncs[lexer.MINUS] = p.parseInfixExpression
	p.infixParseFuncs[lexer.ASTERISK] = p.parseInfixExpression
	p.infixParseFuncs[lexer.SLASH] = p.parseInfixExpression
	p.infixParseFuncs[lexer.POWER] = p.parseInfixExpression
	p.infixParseFuncs[lexer.MODULO] = p.parseInfixExpression
	p.infixParseFuncs[lexer.LT] = p.parseInfixExpression
	p.infixParseFuncs[lexer.GT] = p.parseInfixExpression
	p.infixParseFuncs[lexer.LTEQ] = p.parseInfixExpression
	p.infixParseFuncs[lexer.GTEQ] = p.parseInfixExpression
	p.infixParseFuncs[lexer.EQ] = p.parseInfixExpression
	p.infixParseFuncs[lexer.NEQ] = p.parseInfixExpression
	p.infixParseFuncs[lexer.LPAREN] = p.parseFunctionCallExpression

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

	for p.currToken.Type != lexer.EOF {
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
	if p.currToken.Type == lexer.RETURN {
		return p.parseReturnStatement()
	}

	if p.currToken.Type == lexer.LET {
		return p.parseLetStatement()
	}

	if p.peekToken.Type == lexer.ASSIGN {
		return p.parseAssignStatement()
	}

	return p.parseExpressionStatement()
}

// parseReturnStatement parses a return statement of the form "return expr".
// It captures the "return" keyword token, advances past it, and parses the
// return value expression with the lowest precedence.
func (p *Parser) parseReturnStatement() *ReturnStatement {
	statement := &ReturnStatement{Token: p.currToken}
	p.nextToken()
	statement.ReturnValue = p.parseExpression(LOWEST)

	return statement
}

// parseLetStatement parses a variable declaration of the form "let ident = expr".
// It captures the "let" keyword token, ensures the next token is an identifier,
// expects an assignment operator, and then parses the initialization expression.
func (p *Parser) parseLetStatement() *LetStatement {
	statement := &LetStatement{Token: p.currToken}

	if !p.expectPeek(lexer.IDENT) {
		p.errors = append(p.errors, fmt.Sprintf("expected identifier, got %s", p.currToken.Type))
		return nil
	}
	statement.Name = &Identifier{Token: p.currToken, Value: p.currToken.Literal}

	if !p.expectPeek(lexer.ASSIGN) {
		p.errors = append(p.errors, fmt.Sprintf("expected assignment, got %s", p.currToken.Type))
		return nil
	}

	p.nextToken()
	statement.Value = p.parseExpression(LOWEST)

	return statement
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

// parseStringLiteral returns a StringLiteral expression node for the current token.
func (p *Parser) parseStringLiteral() Expression {
	return &StringLiteral{
		Token: p.currToken,
		Value: p.currToken.Literal,
	}
}

// parseBoolean returns a Boolean expression node for the current token.
func (p *Parser) parseBoolean() Expression {
	return &Boolean{
		Token: p.currToken,
		Value: p.currToken.Type == lexer.TRUE,
	}
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
	for p.peekToken.Type != lexer.EOF && precedence < verifyPrecedenceLevel(p.peekToken.Type) {
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
	if !p.expectPeek(lexer.RPAREN) {
		return nil
	}

	return groupedExpression
}

// parseIfExpression parses an 'if' expression, expecting an opening parenthesis,
// a condition, a closing parenthesis, and a block statement for the consequence.
// It also parses an optional 'else' block if the 'else' keyword is present.
func (p *Parser) parseIfExpression() Expression {
	expression := &IfExpression{Token: p.currToken}

	if !p.expectPeek(lexer.LPAREN) {
		return nil
	}
	p.nextToken()

	expression.Condition = p.parseExpression(LOWEST)

	if !p.expectPeek(lexer.RPAREN) {
		return nil
	}
	if !p.expectPeek(lexer.LBRACE) {
		return nil
	}

	expression.Consequence = p.parseBlockStatement()

	if p.peekToken.Type == lexer.ELSE {
		p.nextToken()

		if !p.expectPeek(lexer.LBRACE) {
			return nil
		}
		expression.Alternative = p.parseBlockStatement()
	}

	return expression
}

// parseBlockStatement parses a block of statements enclosed in curly braces.
// It consumes tokens and parses statements until a closing brace or EOF is encountered.
func (p *Parser) parseBlockStatement() *BlockStatement {
	block := &BlockStatement{Token: p.currToken}
	block.Statements = []Statement{}

	p.nextToken()

	for p.currToken.Type != lexer.RBRACE && p.currToken.Type != lexer.EOF {
		statement := p.parseStatement()
		if statement != nil {
			block.Statements = append(block.Statements, statement)
		}
		p.nextToken()
	}

	return block
}

// parseFunctionLiteral parses a function definition, including its parameters,
// optional return type, and body block.
func (p *Parser) parseFunctionLiteral() Expression {
	lit := &FunctionLiteral{Token: p.currToken}

	if !p.expectPeek(lexer.LPAREN) {
		return nil
	}

	lit.Parameters = p.parseFunctionParameters()

	if p.peekToken.Type == lexer.COLON {
		p.nextToken() // Move to the ':'

		if !p.expectPeek(lexer.IDENT) {
			return nil
		} // Expect the type (e.g., String)
		lit.ReturnType = p.currToken.Literal
	}

	if !p.expectPeek(lexer.LBRACE) {
		return nil
	}
	lit.Body = p.parseBlockStatement()

	return lit
}

// parseFunctionParameters parses the comma-separated list of typed parameters
// within a function declaration.
func (p *Parser) parseFunctionParameters() []*Parameter {
	var parameters []*Parameter

	if p.peekToken.Type == lexer.RPAREN {
		p.nextToken()
		return parameters
	}

	p.nextToken()
	parseSingleParam := func() *Parameter {
		param := &Parameter{Name: p.currToken.Literal}

		if !p.expectPeek(lexer.COLON) {
			return nil
		}

		if !p.expectPeek(lexer.IDENT) {
			return nil
		}
		param.Type = p.currToken.Literal

		return param
	}

	if param := parseSingleParam(); param != nil {
		parameters = append(parameters, param)
	}

	for p.peekToken.Type == lexer.COMMA {
		p.nextToken() // Move to comma
		p.nextToken() // Move to next parameter name

		if param := parseSingleParam(); param != nil {
			parameters = append(parameters, param)
		}
	}

	if !p.expectPeek(lexer.RPAREN) {
		return nil
	}
	return parameters
}

// parseFunctionCallExpression parses a function call, capturing the function
// expression and its parsed arguments.
func (p *Parser) parseFunctionCallExpression(function Expression) Expression {
	exp := &CallExpression{Token: p.currToken, Function: function}
	exp.Arguments = p.parseFunctionCallArguments()
	return exp
}

// parseFunctionCallArguments parses the comma-separated list of expressions
// provided as arguments to a function call.
func (p *Parser) parseFunctionCallArguments() []Expression {
	var args []Expression

	if p.peekToken.Type == lexer.RPAREN {
		p.nextToken()
		return args
	}

	p.nextToken()
	args = append(args, p.parseExpression(LOWEST))
	for p.peekToken.Type == lexer.COMMA {
		p.nextToken()
		p.nextToken()

		args = append(args, p.parseExpression(LOWEST))
	}

	if !p.expectPeek(lexer.RPAREN) {
		return nil
	}
	return args
}

// expectPeek checks whether the peek token matches the expected type. If it
// does, the parser advances and returns true; otherwise it records a peek error
// and returns false without advancing.
func (p *Parser) expectPeek(tokenType lexer.TokenType) bool {
	if p.peekToken.Type == tokenType {
		p.nextToken()
		return true
	}

	p.peekError(tokenType)
	return false
}

// peekError appends a formatted syntax-error message indicating that the peek
// token did not match the expected type.
func (p *Parser) peekError(t lexer.TokenType) {
	msg := fmt.Sprintf("Syntax Error: expected next token to be %s, got %s instead", t, p.peekToken.Type)
	p.errors = append(p.errors, msg)
}

// synchronize performs panic-mode error recovery by discarding tokens until it
// finds one that could plausibly begin a new statement (an IDENT or NUMBER) or
// until EOF is reached.
func (p *Parser) synchronize() {
	for p.peekToken.Type != lexer.EOF {
		if p.peekToken.Type == lexer.IDENT || p.peekToken.Type == lexer.NUMBER {
			return
		}
		p.nextToken()
	}
}
