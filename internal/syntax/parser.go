package syntax

import (
	"caja-cli/internal/lexer"
	"fmt"
	"strconv"
	"time"
)

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

// newParser creates a Parser for the given Tokenizer, registers the built-in prefix
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
	p.prefixParseFuncs[lexer.DATE] = p.parseDateLiteral
	p.prefixParseFuncs[lexer.LPAREN] = p.parseGroupedExpression
	p.prefixParseFuncs[lexer.IF] = p.parseIfExpression
	p.prefixParseFuncs[lexer.TRUE] = p.parseBooleanLiteral
	p.prefixParseFuncs[lexer.FALSE] = p.parseBooleanLiteral
	p.prefixParseFuncs[lexer.FN] = p.parseFunctionLiteral
	p.prefixParseFuncs[lexer.LBRACKET] = p.parseArrayLiteral

	p.prefixParseFuncs[lexer.BANG] = p.parsePrefixExpression
	p.prefixParseFuncs[lexer.MINUS] = p.parsePrefixExpression

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
	p.infixParseFuncs[lexer.LBRACKET] = p.parseIndexExpression
	p.infixParseFuncs[lexer.DOT] = p.parsePropertyExpression
	p.infixParseFuncs[lexer.AND] = p.parseInfixExpression
	p.infixParseFuncs[lexer.OR] = p.parseInfixExpression
	p.infixParseFuncs[lexer.XOR] = p.parseInfixExpression

	p.nextToken()
	p.nextToken()

	return p
}

// parse consumes all tokens from the Tokenizer and returns a Program AST.
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

// PrintErrors prints all encountered parser errors to standard output.
func (p *Parser) PrintErrors() {
	if p.HasErrors() {
		fmt.Println("Parser errors found:")
		for _, msg := range p.Errors() {
			fmt.Printf("\t- %s\n", msg)
		}
	}
}

// Errors returns the combined list of errors from both the tokenizer and the
// parser, in the order they were encountered.
func (p *Parser) Errors() []string {
	allErrors := append([]string{}, p.tknzr.Errors...)
	allErrors = append(allErrors, p.errors...)

	return allErrors
}

// HasErrors returns true if the parser or tokenizer encountered any errors.
func (p *Parser) HasErrors() bool {
	return len(p.Errors()) > 0
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

	if p.currToken.Type == lexer.IMPORT {
		return p.parseImportStatement()
	}

	if p.currToken.Type == lexer.TYPE {
		return p.parseTypeAliasStatement()
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

// parseImportStatement parses an import statement of the form "import module" or "import \"path/to/module\"".
func (p *Parser) parseImportStatement() *ImportStatement {
	statement := &ImportStatement{Token: p.currToken}

	if p.peekToken.Type == lexer.STRING {
		p.nextToken()
		statement.Path = p.currToken.Literal
		// Extract basename from path for the identifier
		// Using standard path.Base to handle forward slashes
		basename := p.currToken.Literal
		for i := len(basename) - 1; i >= 0; i-- {
			if basename[i] == '/' {
				basename = basename[i+1:]
				break
			}
		}
		statement.Name = &Identifier{Token: p.currToken, Value: basename}
	} else if p.peekToken.Type == lexer.IDENT {
		p.nextToken()
		if lexer.IsKeyword(p.currToken.Type) {
			p.reportError(p.currToken, fmt.Sprintf("syntax error: cannot use keyword '%s' as a module name", p.currToken.Literal))
			return nil
		}
		statement.Path = p.currToken.Literal
		statement.Name = &Identifier{Token: p.currToken, Value: p.currToken.Literal}
	} else {
		p.reportError(p.peekToken, fmt.Sprintf("expected identifier or string for module name, got %s", p.peekToken.Type))
		return nil
	}

	if p.peekToken.Type == lexer.AS {
		p.nextToken() // move to 'as'
		if !p.expectPeek(lexer.IDENT) {
			return nil
		}

		if lexer.IsKeyword(p.currToken.Type) {
			p.reportError(p.currToken, fmt.Sprintf("syntax error: cannot use keyword '%s' as a module alias", p.currToken.Literal))
			return nil
		}

		statement.Name = &Identifier{Token: p.currToken, Value: p.currToken.Literal}
	}

	return statement
}

// parseLetStatement parses a variable declaration of the form "let ident = expr".
// It captures the "let" keyword token, ensures the next token is an identifier,
// expects an assignment operator, and then parses the initialization expression.
func (p *Parser) parseLetStatement() *LetStatement {
	statement := &LetStatement{Token: p.currToken}

	if lexer.IsKeyword(p.peekToken.Type) {
		p.reportError(p.peekToken, fmt.Sprintf("syntax error: cannot use keyword '%s' as a variable name", p.peekToken.Literal))
		return nil
	}

	if !p.expectPeek(lexer.IDENT) {
		p.reportError(p.peekToken, fmt.Sprintf("expected identifier, got %s", p.currToken.Type))
		return nil
	}
	statement.Name = &Identifier{Token: p.currToken, Value: p.currToken.Literal}

	if !p.expectPeek(lexer.ASSIGN) {
		p.reportError(p.peekToken, fmt.Sprintf("expected assignment, got %s", p.currToken.Type))
		return nil
	}

	p.nextToken()
	statement.Value = p.parseExpression(LOWEST)

	return statement
}

// parseTypeAliasStatement parses a type alias declaration of the form "type Name fn(...): ReturnType".
// It captures the "type" keyword token implicitly, ensures the next token is an identifier,
// expects the "fn" keyword, and then parses the function signature.
func (p *Parser) parseTypeAliasStatement() *TypeAliasStatement {
	statement := &TypeAliasStatement{Token: p.currToken}

	if !p.expectPeek(lexer.IDENT) {
		p.reportError(p.peekToken, fmt.Sprintf("expected identifier, got %s", p.currToken.Type))
		return nil
	}
	statement.Name = &Identifier{Token: p.currToken, Value: p.currToken.Literal}

	if !p.expectPeek(lexer.FN) {
		p.reportError(p.peekToken, fmt.Sprintf("expected function name, got %s", p.currToken.Type))
		return nil
	}

	statement.Signature = &FunctionSignature{}
	if !p.expectPeek(lexer.LPAREN) {
		p.reportError(p.peekToken, fmt.Sprintf("expected lparen, got %s", p.currToken.Type))
		return nil
	}

	if p.peekToken.Type != lexer.RPAREN {
		paramType := p.parseTypeSignature()
		if paramType != "" {
			statement.Signature.ParamTypes = append(statement.Signature.ParamTypes, paramType)
		}
		for p.peekToken.Type == lexer.COMMA {
			p.nextToken() // move to comma
			paramType := p.parseTypeSignature()
			if paramType != "" {
				statement.Signature.ParamTypes = append(statement.Signature.ParamTypes, paramType)
			}
		}
	}

	if !p.expectPeek(lexer.RPAREN) {
		p.reportError(p.peekToken, fmt.Sprintf("expected rparen, got %s", p.currToken.Type))
		return nil
	}

	if !p.expectPeek(lexer.COLON) {
		p.reportError(p.peekToken, fmt.Sprintf("expected colon got %s", p.currToken.Type))
		return nil
	}

	statement.Signature.ReturnType = p.parseTypeSignature()

	return statement
}

// parseAssignStatement parses an assignment of the form "identifier = expr".
// It captures the left-hand identifier, consumes the '=' token, and parses
// the right-hand expression with the lowest precedence.
func (p *Parser) parseAssignStatement() *AssignStatement {
	if lexer.IsKeyword(p.currToken.Type) {
		p.reportError(p.currToken, fmt.Sprintf("syntax error: cannot use keyword '%s' as a variable name", p.currToken.Literal))
		return nil
	}

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

// parseIdentifier returns an Identifier expression node for the current token.
func (p *Parser) parseIdentifier() Expression {
	return &Identifier{Token: p.currToken, Value: p.currToken.Literal}
}

// parsePrefixExpression parses a prefix operator expression, such as -5 or !true.
func (p *Parser) parsePrefixExpression() Expression {
	expression := &PrefixExpression{
		Token:    p.currToken,
		Operator: p.currToken.Literal,
	}
	p.nextToken()
	expression.Right = p.parseExpression(PREFIX)
	return expression
}

// parseNumberLiteral converts the current token's literal into a float64 and
// returns a NumberLiteral expression node. If the literal cannot be parsed as
// a float, an error is recorded and nil is returned.
func (p *Parser) parseNumberLiteral() Expression {
	literal := &NumberLiteral{Token: p.currToken}

	value, err := strconv.ParseFloat(p.currToken.Literal, 64)
	if err != nil {
		p.reportError(p.currToken, fmt.Sprintf("could not parse %q as float", p.currToken.Literal))
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

// parseDateLiteral parses a date literal string, validates its format
// as 'YYYY-MM-DD', and returns a DateLiteral expression node.
func (p *Parser) parseDateLiteral() Expression {
	lit := &DateLiteral{
		Token: p.currToken,
		Value: p.currToken.Literal,
	}

	_, err := time.Parse("2006-01-02", lit.Value)
	if err != nil {
		msg := fmt.Sprintf("syntax error: invalid date format '%s'. Must use 'YYYY-MM-DD'", lit.Value)
		p.reportError(p.currToken, msg)
		return nil
	}

	return lit
}

// parseBooleanLiteral returns a BooleanLiteral expression node for the current token.
func (p *Parser) parseBooleanLiteral() Expression {
	return &BooleanLiteral{
		Token: p.currToken,
		Value: p.currToken.Type == lexer.TRUE,
	}
}

// parseArrayLiteral consumes the opening bracket, parses a comma-separated list
// of expressions, and returns an ArrayLiteral expression node.
func (p *Parser) parseArrayLiteral() Expression {
	array := &ArrayLiteral{Token: p.currToken}
	array.Elements = p.parseExpressionList(lexer.RBRACKET)
	return array
}

// parseIndexExpression parses an array index operation, capturing the left-hand
// expression (the array) and parsing the expression inside the brackets as the index.
func (p *Parser) parseIndexExpression(left Expression) Expression {
	exp := &IndexExpression{Token: p.currToken, Left: left}
	p.nextToken()
	exp.Index = p.parseExpression(LOWEST)
	if !p.expectPeek(lexer.RBRACKET) {
		return nil
	}
	return exp
}

// parsePropertyExpression parses an object property access, capturing the left-hand
// expression (the object) and parsing the identifier following the dot.
func (p *Parser) parsePropertyExpression(left Expression) Expression {
	exp := &PropertyExpression{Token: p.currToken, Object: left}

	if !p.expectPeek(lexer.IDENT) {
		p.reportError(p.peekToken, fmt.Sprintf("expected property name, got %s", p.currToken.Type))
		return nil
	}

	exp.Property = &Identifier{Token: p.currToken, Value: p.currToken.Literal}
	return exp
}

// parseExpression is the core of the Pratt parser. It looks up a prefix parse
// function for the current token, then repeatedly applies infix parse functions
// as long as the next token's precedence exceeds the given precedence level,
// building a left-recursive expression tree.
func (p *Parser) parseExpression(precedence int) Expression {
	prefix := p.prefixParseFuncs[p.currToken.Type]
	if prefix == nil {
		p.reportError(p.currToken, fmt.Sprintf("unknown prefix type %q", p.currToken.Type))
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
		p.reportError(p.peekToken, fmt.Sprintf("expected ')' after grouped expression, got %s", p.currToken.Type))
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
		p.reportError(p.peekToken, fmt.Sprintf("expected '(', got %s", p.currToken.Type))
		return nil
	}
	p.nextToken()

	expression.Condition = p.parseExpression(LOWEST)

	if !p.expectPeek(lexer.RPAREN) {
		p.reportError(p.peekToken, fmt.Sprintf("expected ')' after condition, got %s", p.currToken.Type))
		return nil
	}
	if !p.expectPeek(lexer.LBRACE) {
		p.reportError(p.peekToken, fmt.Sprintf("expected '{', got %s", p.currToken.Type))
		return nil
	}

	expression.Consequence = p.parseBlockStatement()

	if p.peekToken.Type == lexer.ELSE {
		p.nextToken()

		if !p.expectPeek(lexer.LBRACE) {
			p.reportError(p.peekToken, fmt.Sprintf("expected '{', got %s", p.currToken.Type))
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
		p.reportError(p.peekToken, fmt.Sprintf("expected '(', got %s", p.currToken.Type))
		return nil
	}

	lit.Parameters = p.parseFunctionParameters()

	if !p.expectPeek(lexer.COLON) {
		p.reportError(p.peekToken, fmt.Sprintf("expected ':', got %s", p.currToken.Type))
		return nil
	}

	lit.ReturnType = p.parseTypeSignature()

	if !p.expectPeek(lexer.LBRACE) {
		p.reportError(p.peekToken, fmt.Sprintf("expected '{', got %s", p.currToken.Type))
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
		if lexer.IsKeyword(p.currToken.Type) {
			p.reportError(p.currToken, fmt.Sprintf("syntax error: cannot use keyword '%s' as a parameter name", p.currToken.Literal))
			return nil
		}
		param := &Parameter{Name: p.currToken.Literal}

		if !p.expectPeek(lexer.COLON) {
			p.reportError(p.peekToken, fmt.Sprintf("expected ':', got '%s'", param.Type))
			return nil
		}

		param.Type = p.parseTypeSignature()
		if param.Type == "" {
			return nil
		}

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
	exp.Arguments = p.parseExpressionList(lexer.RPAREN)
	return exp
}

// parseExpressionList parses a comma-separated list of expressions until it
// encounters the specified end token (e.g., closing parenthesis or bracket).
func (p *Parser) parseExpressionList(end lexer.TokenType) []Expression {
	var list []Expression

	if p.peekToken.Type == end {
		p.nextToken()
		return list
	}

	p.nextToken()
	list = append(list, p.parseExpression(LOWEST))
	for p.peekToken.Type == lexer.COMMA {
		p.nextToken() // Move to the comma
		p.nextToken() // Move past the comma to the next expression

		list = append(list, p.parseExpression(LOWEST))
	}

	if !p.expectPeek(end) {
		p.reportError(p.peekToken, fmt.Sprintf("expected '%s', got %s", end, p.currToken.Type))
		return nil
	}

	return list
}

// parseTypeSignature parses a type identifier or array type like [Number] or [[Number]].
func (p *Parser) parseTypeSignature() string {
	if p.peekToken.Type == lexer.LBRACKET {
		p.nextToken() // move to [
		innerType := p.parseTypeSignature()
		if !p.expectPeek(lexer.RBRACKET) {
			return ""
		}
		return "[" + innerType + "]"
	}

	if p.expectPeek(lexer.IDENT) {
		return p.currToken.Literal
	}

	p.reportError(p.peekToken, fmt.Sprintf("expected type identifier, got %s", p.peekToken.Type))
	return ""
}

// reportError formats and appends a syntax error with the given token's line and column.
func (p *Parser) reportError(token lexer.Token, msg string) {
	p.errors = append(p.errors, fmt.Sprintf("[Line %d, Column %d] %s", token.Line, token.Column, msg))
}

// nextToken advances the parser's two-token window by shifting peekToken into
// currToken and reading the next token from the Tokenizer into peekToken.
func (p *Parser) nextToken() {
	p.currToken = p.peekToken
	p.peekToken = p.tknzr.NextToken()
}

// expectPeek checks whether the peek token matches the expected type. If it
// does, the parser advances and returns true; otherwise it records a peek error
// and returns false without advancing.
func (p *Parser) expectPeek(tokenType lexer.TokenType) bool {
	if p.peekToken.Type == tokenType {
		p.nextToken()
		return true
	}

	p.reportError(p.peekToken, fmt.Sprintf("expected next token to be %s, got %s instead", tokenType, p.peekToken.Type))
	return false
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
