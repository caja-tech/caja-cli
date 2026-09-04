package parser

import (
	"caja-cli/internal/pipeline/ast"
	"caja-cli/internal/pipeline/lexer"
	"context"

	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// prefixParseFunc is a parsing function invoked when a token appears in prefix
// position (i.e. at the start of an expression). It returns the parsed
// Expression node.
type prefixParseFunc func() ast.Expression

// infixParseFunc is a parsing function invoked when a token appears between two
// expressions (infix position). It receives the already-parsed left-hand side
// and returns the combined Expression node.
type infixParseFunc func(ast.Expression) ast.Expression

// Parser is a Pratt (top-down operator precedence) parser that transforms a
// stream of tokens produced by a Tokenizer into an abstract syntax tree. It
// maintains a current and peek token for single-token lookahead and dispatches
// to registered prefix and infix parse functions based on token type.

type Parser struct {
	tknzr *lexer.Lexer
	ctx   context.Context

	currToken lexer.Token
	peekToken lexer.Token

	prefixParseFuncs map[lexer.TokenType]prefixParseFunc
	infixParseFuncs  map[lexer.TokenType]infixParseFunc

	diagnosticErrors []ast.DiagnosticError
}

// New creates a Parser for the given Tokenizer, registers the built-in prefix
// and infix parse functions for identifiers, numbers, grouped expressions, and
// arithmetic operators, and primes the two-token lookahead by reading twice.
func New(t *lexer.Lexer) *Parser {
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
	p.prefixParseFuncs[lexer.NIL] = p.parseNilLiteral
	p.prefixParseFuncs[lexer.FN] = p.parseFunctionLiteral
	p.prefixParseFuncs[lexer.LBRACKET] = p.parseArrayLiteral
	p.prefixParseFuncs[lexer.LBRACE] = p.parseMapLiteral

	p.prefixParseFuncs[lexer.BANG] = p.parsePrefixExpression
	p.prefixParseFuncs[lexer.MINUS] = p.parsePrefixExpression
	p.prefixParseFuncs[lexer.MOVE] = p.parsePrefixExpression

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
	p.infixParseFuncs[lexer.DOUBLE_COLON] = p.parseTurbofishExpression
	p.infixParseFuncs[lexer.LBRACE] = p.parseStructLiteral
	p.infixParseFuncs[lexer.LBRACKET] = p.parseIndexExpression
	p.infixParseFuncs[lexer.DOT] = p.parsePropertyExpression
	p.infixParseFuncs[lexer.QUESTIONDOT] = p.parsePropertyExpression
	p.infixParseFuncs[lexer.AND] = p.parseInfixExpression
	p.infixParseFuncs[lexer.OR] = p.parseInfixExpression
	p.infixParseFuncs[lexer.XOR] = p.parseInfixExpression
	p.infixParseFuncs[lexer.PIPE] = p.parsePipeExpression
	p.infixParseFuncs[lexer.SAFE_PIPE] = p.parseSafePipeExpression

	p.nextToken()
	p.nextToken()

	return p
}

// Parse consumes all tokens from the Tokenizer and returns a Program AST.
// Each iteration parses one statement; if parsing fails, synchronize is called
// to skip ahead to the next likely statement boundary before continuing.
func (p *Parser) Parse() *ast.Program {
	program := &ast.Program{}
	program.Statements = []ast.Statement{}

	for p.currToken.Type != lexer.EOF {
		if p.ctx != nil && p.ctx.Err() != nil {
			return nil
		}

		if p.ctx != nil && p.ctx.Err() != nil {
			return nil
		}
		statement := p.parseStatement()
		if statement != nil {
			program.Statements = append(program.Statements, statement)
			if p.peekToken.Type != lexer.EOF && p.peekToken.Line == p.currToken.Line {
				p.reportError(p.peekToken, fmt.Sprintf("syntax error: unexpected token '%s'. Expected a newline between statements", p.peekToken.Literal))
				p.synchronize()
			}
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
	for _, err := range p.diagnosticErrors {
		allErrors = append(allErrors, err.String())
	}
	return allErrors
}

// DiagnosticErrors returns the structured diagnostic errors.
func (p *Parser) DiagnosticErrors() []ast.DiagnosticError {
	return p.diagnosticErrors
}

// HasErrors returns true if the parser or tokenizer encountered any errors.
func (p *Parser) HasErrors() bool {
	return len(p.Errors()) > 0
}

// parseNilLiteral returns a NilLiteral expression node for the current token.
func (p *Parser) parseNilLiteral() ast.Expression {
	return &ast.NilLiteral{Token: p.currToken}
}

// parseNumberLiteral converts the current token's literal into a float64 and
// returns a NumberLiteral expression node. If the literal cannot be parsed as
// a float, an error is recorded and nil is returned.
func (p *Parser) parseNumberLiteral() ast.Expression {
	literal := &ast.NumberLiteral{Token: p.currToken}

	value, err := strconv.ParseFloat(p.currToken.Literal, 64)
	if err != nil {
		p.reportError(p.currToken, fmt.Sprintf("could not parse %q as float", p.currToken.Literal))
		return nil
	}

	literal.Value = value
	return literal
}

// parseStringLiteral returns a StringLiteral expression node for the current token.
func (p *Parser) parseStringLiteral() ast.Expression {
	return &ast.StringLiteral{
		Token: p.currToken,
		Value: p.currToken.Literal,
	}
}

// parseBooleanLiteral returns a BooleanLiteral expression node for the current token.
func (p *Parser) parseBooleanLiteral() ast.Expression {
	return &ast.BooleanLiteral{
		Token: p.currToken,
		Value: p.currToken.Type == lexer.TRUE,
	}
}

// parseDateLiteral parses a date literal string, validates its format
// as 'YYYY-MM-DD', and returns a DateLiteral expression node.
func (p *Parser) parseDateLiteral() ast.Expression {
	lit := &ast.DateLiteral{
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

// parseArrayLiteral consumes the opening bracket, parses a comma-separated list
// of expressions, and returns an ArrayLiteral expression node.
func (p *Parser) parseArrayLiteral() ast.Expression {
	array := &ast.ArrayLiteral{Token: p.currToken}
	array.Elements = p.parseExpressionList(lexer.RBRACKET)
	return array
}

// parseFunctionLiteral parses a function definition, including its parameters,
// optional return type, and body block.
func (p *Parser) parseFunctionLiteral() ast.Expression {
	lit := &ast.FunctionLiteral{Token: p.currToken}

	if p.peekToken.Type == lexer.LT {
		p.nextToken() // move to <
		lit.TypeParameters = p.parseTypeParameters()
	}

	if !p.expectPeek(lexer.LPAREN) {
		p.reportError(p.peekToken, fmt.Sprintf("expected '(', got %s", p.currToken.Type))
		return nil
	}

	lit.Parameters = p.parseFunctionParameters()

	if p.peekToken.Type == lexer.LBRACE {
		lit.ReturnType = "Nothing"
		p.nextToken() // move to LBRACE
	} else {
		if !p.expectPeek(lexer.ARROW) {
			p.reportError(p.peekToken, fmt.Sprintf("expected '->' or '{', got %s", p.peekToken.Type))
			return nil
		}
		lit.ReturnType = p.parseTypeSignature()
		if !p.expectPeek(lexer.LBRACE) {
			p.reportError(p.peekToken, fmt.Sprintf("expected '{', got %s", p.peekToken.Type))
			return nil
		}
	}

	lit.Body = p.parseBlockStatement()

	return lit
}

// parseTypeParameters parses the comma-separated list of generic type variables
// enclosed in angle brackets. Assumes the opening '<' has been consumed.
func (p *Parser) parseTypeParameters() []string {
	var typeParams []string

	if p.peekToken.Type == lexer.GT {
		p.reportError(p.peekToken, "expected at least one generic type parameter")
		p.nextToken() // consume '>'
		return typeParams
	}

	p.nextToken()
	if p.currToken.Type != lexer.IDENT {
		p.reportError(p.currToken, fmt.Sprintf("expected identifier for generic type parameter, got %s", p.currToken.Type))
		return nil
	}
	typeParams = append(typeParams, p.currToken.Literal)

	for p.peekToken.Type == lexer.COMMA {
		p.nextToken() // consume ','
		p.nextToken() // move to next identifier

		if p.currToken.Type != lexer.IDENT {
			p.reportError(p.currToken, fmt.Sprintf("expected identifier for generic type parameter, got %s", p.currToken.Type))
			return nil
		}
		typeParams = append(typeParams, p.currToken.Literal)
	}

	if !p.expectPeek(lexer.GT) {
		p.reportError(p.peekToken, fmt.Sprintf("expected '>', got %s", p.peekToken.Type))
		return nil
	}

	return typeParams
}

// parseFunctionParameters parses the comma-separated list of typed parameters
// within a function declaration.
func (p *Parser) parseFunctionParameters() []*ast.Parameter {
	var parameters []*ast.Parameter

	if p.peekToken.Type == lexer.RPAREN {
		p.nextToken()
		return parameters
	}

	p.nextToken()
	parseSingleParam := func() *ast.Parameter {
		if lexer.IsKeyword(p.currToken.Type) {
			p.reportError(p.currToken, fmt.Sprintf("syntax error: cannot use keyword '%s' as a parameter name", p.currToken.Literal))
			return nil
		}
		param := &ast.Parameter{Name: p.currToken.Literal}

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

// parseStructLiteral parses a struct instantiation of the form `MyStruct { a: 1, b: 2 }` or `MyStruct::<Type> { a: 1 }`.
func (p *Parser) parseStructLiteral(left ast.Expression) ast.Expression {
	var structName string
	var typeArgs []string
	if ident, ok := left.(*ast.Identifier); ok {
		structName = ident.Value
	} else if genIdent, ok := left.(*ast.GenericIdentifier); ok {
		structName = genIdent.Identifier.Value
		typeArgs = genIdent.TypeArguments
	} else if prop, ok := left.(*ast.PropertyExpression); ok {
		// e.g. "sm.User" -> "sm.User"
		if modId, ok := prop.Object.(*ast.Identifier); ok {
			structName = modId.Value + "." + prop.Property.Value
		} else {
			p.reportError(p.currToken, "invalid property expression for struct literal")
			return nil
		}
	} else {
		p.reportError(p.currToken, "expected identifier or property expression before struct literal")
		return nil
	}

	literal := &ast.StructLiteral{
		Token:         p.currToken, // The '{' token
		StructName:    structName,
		TypeArguments: typeArgs,
		Fields:        make(map[string]ast.Expression),
	}

	if p.peekToken.Type == lexer.RBRACE {
		p.nextToken()
		return literal
	}

	for p.peekToken.Type != lexer.RBRACE && p.peekToken.Type != lexer.EOF {
		p.nextToken()
		if p.currToken.Type != lexer.IDENT {
			p.reportError(p.currToken, fmt.Sprintf("expected identifier as struct field, got %s", p.currToken.Type))
			return nil
		}

		fieldName := p.currToken.Literal
		if !p.expectPeek(lexer.COLON) {
			p.reportError(p.peekToken, fmt.Sprintf("expected colon after struct field, got %s", p.currToken.Type))
			return nil
		}

		p.nextToken()
		val := p.parseExpression(lexer.LOWEST_PRECEDENCE)
		if val == nil {
			return nil
		}

		literal.Fields[fieldName] = val

		if p.peekToken.Type == lexer.COMMA {
			p.nextToken()
		} else if p.peekToken.Type != lexer.RBRACE {
			p.reportError(p.peekToken, fmt.Sprintf("expected comma or rbrace, got %s", p.peekToken.Type))
			return nil
		}
	}

	if !p.expectPeek(lexer.RBRACE) {
		p.reportError(p.peekToken, fmt.Sprintf("expected rbrace, got %s", p.currToken.Type))
		return nil
	}

	return literal
}

// parseStatement determines the kind of statement to parse by peeking at the
// next token. If the next token is an ASSIGN_PRECEDENCE operator, an AssignStatement is
// parsed; otherwise the current tokens are treated as an ExpressionStatement.
func (p *Parser) parseStatement() ast.Statement {
	if p.currToken.Type == lexer.RETURN {
		stmt := p.parseReturnStatement()
		if stmt == nil {
			return nil
		}
		return stmt
	}

	isPrivate := false
	if p.currToken.Type == lexer.PRIVATE {
		isPrivate = true
		p.nextToken()
		if p.currToken.Type != lexer.LET && p.currToken.Type != lexer.TYPE && p.currToken.Type != lexer.CONST && p.currToken.Type != lexer.DEFINE {
			p.reportError(p.currToken, "syntax error: 'private' modifier must be followed by 'let', 'const', 'type', or 'define'")
			return nil
		}
	}

	if p.currToken.Type == lexer.LET {
		stmt := p.parseLetStatement()
		if stmt == nil {
			return nil
		}
		stmt.IsPrivate = isPrivate
		return stmt
	}

	if p.currToken.Type == lexer.CONST {
		stmt := p.parseConstStatement()
		if stmt == nil {
			return nil
		}
		stmt.IsPrivate = isPrivate
		return stmt
	}

	if p.currToken.Type == lexer.IMPORT {
		if isPrivate {
			p.reportError(p.currToken, "syntax error: 'private' modifier cannot be applied to imports")
			return nil
		}
		stmt := p.parseImportStatement()
		if stmt == nil {
			return nil
		}
		return stmt
	}

	if p.currToken.Type == lexer.TYPE {
		stmt := p.parseTypeAliasStatement()
		if stmt == nil {
			return nil
		}
		stmt.IsPrivate = isPrivate
		return stmt
	}

	if p.currToken.Type == lexer.DEFINE {
		stmt := p.parseTypeConstraintStatement()
		if stmt == nil {
			return nil
		}
		// Assuming we don't need IsPrivate for DEFINE for now, or add it if necessary.
		return stmt
	}

	if p.peekToken.Type == lexer.ASSIGN && lexer.IsKeyword(p.currToken.Type) {
		p.reportError(p.currToken, fmt.Sprintf("syntax error: cannot use keyword '%s' as a variable name", p.currToken.Literal))
		return nil
	}

	exprStmt := p.parseExpressionStatement()
	if p.peekToken.Type == lexer.ASSIGN {
		p.nextToken() // move to '='
		assignToken := p.currToken
		p.nextToken() // move to RHS
		rhs := p.parseExpression(lexer.LOWEST_PRECEDENCE)

		if ident, ok := exprStmt.Expression.(*ast.Identifier); ok {
			if lexer.IsKeyword(ident.Token.Type) {
				p.reportError(ident.Token, fmt.Sprintf("syntax error: cannot use keyword '%s' as a variable name", ident.Token.Literal))
				return nil
			}
			return &ast.AssignStatement{Token: assignToken, Name: ident, Value: rhs}
		}
		if idxExpr, ok := exprStmt.Expression.(*ast.IndexExpression); ok {
			return &ast.IndexAssignmentStatement{Token: assignToken, Left: idxExpr.Left, Index: idxExpr.Index, Value: rhs}
		}
		if propExpr, ok := exprStmt.Expression.(*ast.PropertyExpression); ok {
			return &ast.PropertyAssignmentStatement{
				Token:    assignToken,
				Object:   propExpr.Object,
				Property: propExpr.Property,
				Value:    rhs,
				Safe:     propExpr.Safe,
			}
		}

		p.reportError(assignToken, "syntax error: invalid assignment target")
		return nil
	}
	return exprStmt
}

// parseImportStatement parses an import statement of the form "import module" or "import \"path/to/module\"".
func (p *Parser) parseImportStatement() *ast.ImportStatement {
	statement := &ast.ImportStatement{Token: p.currToken}

	if p.peekToken.Type == lexer.LBRACE {
		p.nextToken() // move to '{'
		
		for p.peekToken.Type != lexer.RBRACE && p.peekToken.Type != lexer.EOF {
			p.nextToken()
			if p.currToken.Type == lexer.COMMA {
				continue
			}
			if p.currToken.Type != lexer.IDENT && !lexer.IsKeyword(p.currToken.Type) {
				p.reportError(p.currToken, fmt.Sprintf("expected identifier in named import, got %s", p.currToken.Type))
				return nil
			}
			statement.NamedImports = append(statement.NamedImports, &ast.Identifier{Token: p.currToken, Value: p.currToken.Literal})
		}
		
		if !p.expectPeek(lexer.RBRACE) {
			return nil
		}
		
		if !p.expectPeek(lexer.IDENT) || p.currToken.Literal != "from" {
			p.reportError(p.currToken, "expected 'from' after named imports")
			return nil
		}
	}

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
		statement.Name = &ast.Identifier{Token: p.currToken, Value: basename}
	} else if p.peekToken.Type == lexer.IDENT {
		p.nextToken()
		if lexer.IsKeyword(p.currToken.Type) {
			p.reportError(p.currToken, fmt.Sprintf("syntax error: cannot use keyword '%s' as a module name", p.currToken.Literal))
			return nil
		}
		statement.Path = p.currToken.Literal
		statement.Name = &ast.Identifier{Token: p.currToken, Value: p.currToken.Literal}
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

		statement.Name = &ast.Identifier{Token: p.currToken, Value: p.currToken.Literal}
	}

	return statement
}

// parseLetStatement parses a variable declaration of the form "let ident = expr".
// It captures the "let" keyword token, ensures the next token is an identifier,
// expects an assignment operator, and then parses the initialization expression.
func (p *Parser) parseLetStatement() *ast.LetStatement {
	statement := &ast.LetStatement{Token: p.currToken}

	if lexer.IsKeyword(p.peekToken.Type) {
		p.reportError(p.peekToken, fmt.Sprintf("syntax error: cannot use keyword '%s' as a variable name", p.peekToken.Literal))
		return nil
	}

	if !p.expectPeek(lexer.IDENT) {
		p.reportError(p.peekToken, fmt.Sprintf("expected identifier, got %s", p.currToken.Type))
		return nil
	}
	statement.Name = &ast.Identifier{Token: p.currToken, Value: p.currToken.Literal}

	if p.peekToken.Type == lexer.COLON {
		p.nextToken() // move to colon
		statement.ValueType = p.parseTypeSignature()
	}

	if !p.expectPeek(lexer.ASSIGN) {
		p.reportError(p.peekToken, fmt.Sprintf("expected assignment, got %s", p.currToken.Type))
		return nil
	}

	p.nextToken()
	statement.Value = p.parseExpression(lexer.LOWEST_PRECEDENCE)

	return statement
}

// parseConstStatement parses a constant variable declaration of the form "const ident = expr".
func (p *Parser) parseConstStatement() *ast.ConstStatement {
	statement := &ast.ConstStatement{Token: p.currToken}

	if lexer.IsKeyword(p.peekToken.Type) {
		p.reportError(p.peekToken, fmt.Sprintf("syntax error: cannot use keyword '%s' as a variable name", p.peekToken.Literal))
		return nil
	}

	if !p.expectPeek(lexer.IDENT) {
		p.reportError(p.peekToken, fmt.Sprintf("expected identifier, got %s", p.currToken.Type))
		return nil
	}
	statement.Name = &ast.Identifier{Token: p.currToken, Value: p.currToken.Literal}

	if p.peekToken.Type == lexer.COLON {
		p.nextToken() // move to colon
		statement.ValueType = p.parseTypeSignature()
	}

	if !p.expectPeek(lexer.ASSIGN) {
		p.reportError(p.peekToken, fmt.Sprintf("expected assignment, got %s", p.currToken.Type))
		return nil
	}

	p.nextToken()
	statement.Value = p.parseExpression(lexer.LOWEST_PRECEDENCE)

	return statement
}

// parseBlockStatement parses a block of statements enclosed in curly braces.
// It consumes tokens and parses statements until a closing brace or EOF is encountered.
func (p *Parser) parseBlockStatement() *ast.BlockStatement {
	block := &ast.BlockStatement{Token: p.currToken}
	block.Statements = []ast.Statement{}

	p.nextToken()

	for p.currToken.Type != lexer.RBRACE && p.currToken.Type != lexer.EOF {
		statement := p.parseStatement()
		if statement != nil {
			block.Statements = append(block.Statements, statement)
			if p.peekToken.Type != lexer.RBRACE && p.peekToken.Type != lexer.EOF && p.peekToken.Line == p.currToken.Line {
				p.reportError(p.peekToken, fmt.Sprintf("syntax error: unexpected token '%s'. Expected a newline between statements", p.peekToken.Literal))
				p.synchronize()
			}
		}
		p.nextToken()
	}

	return block
}

// parseTypeAliasStatement parses a type alias declaration of the form "type Name fn(...): ReturnType".
// It captures the "type" keyword token implicitly, ensures the next token is an identifier,
// expects the "fn" keyword, and then parses the function signature.
func (p *Parser) parseTypeConstraintStatement() *ast.TypeConstraintStatement {
	stmt := &ast.TypeConstraintStatement{Token: p.currToken}

	if !p.expectPeek(lexer.IDENT) {
		return nil
	}
	stmt.Name = &ast.Identifier{Token: p.currToken, Value: p.currToken.Literal}

	if !p.expectPeek(lexer.CONSTRAINTS) {
		return nil
	}

	if !p.expectPeek(lexer.IDENT) {
		return nil
	}
	stmt.BaseType = &ast.Identifier{Token: p.currToken, Value: p.currToken.Literal}

	if !p.expectPeek(lexer.WITH) {
		return nil
	}

	if !p.expectPeek(lexer.COLON) {
		return nil
	}

	p.nextToken() // move past COLON to start of expression
	stmt.Predicate = p.parseExpression(lexer.LOWEST_PRECEDENCE)

	return stmt
}

func (p *Parser) parseTypeAliasStatement() *ast.TypeAliasStatement {
	statement := &ast.TypeAliasStatement{Token: p.currToken}

	if !p.expectPeek(lexer.IDENT) {
		p.reportError(p.peekToken, fmt.Sprintf("expected identifier, got %s", p.currToken.Type))
		return nil
	}

	if len(p.currToken.Literal) > 0 {
		firstRune, _ := utf8.DecodeRuneInString(p.currToken.Literal)
		if !unicode.IsUpper(firstRune) {
			p.reportError(p.currToken, fmt.Sprintf("type name '%s' must start with a capital letter", p.currToken.Literal))
			return nil
		}
	}

	statement.Name = &ast.Identifier{Token: p.currToken, Value: p.currToken.Literal}

	if p.peekToken.Type == lexer.LT {
		p.nextToken() // move to <
		statement.TypeParameters = p.parseTypeParameters()
	}

	if p.peekToken.Type == lexer.FN {
		p.nextToken() // move to fn

		statement.Signature = &ast.FunctionSignature{}
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

		if p.peekToken.Type == lexer.ARROW {
			p.nextToken() // consume '->'
			statement.Signature.ReturnType = p.parseTypeSignature()
		} else {
			statement.Signature.ReturnType = "Nothing"
		}
	} else if p.peekToken.Type == lexer.STRUCT {
		p.nextToken() // move to struct

		statement.StructDefinition = &ast.StructDefinition{Token: p.currToken}
		if !p.expectPeek(lexer.LBRACE) {
			p.reportError(p.peekToken, fmt.Sprintf("expected lbrace, got %s", p.currToken.Type))
			return nil
		}

		// parse fields
		for p.peekToken.Type != lexer.RBRACE && p.peekToken.Type != lexer.EOF {
			p.nextToken() // move to first token of field (const or IDENT)

			isConstant := false
			if p.currToken.Type == lexer.CONST {
				isConstant = true
				if !p.expectPeek(lexer.IDENT) {
					p.reportError(p.peekToken, fmt.Sprintf("expected identifier after const, got %s", p.currToken.Type))
					return nil
				}
			}

			if p.currToken.Type != lexer.IDENT {
				p.reportError(p.currToken, fmt.Sprintf("expected identifier, got %s", p.currToken.Type))
				return nil
			}

			field := ast.StructField{
				Name:       &ast.Identifier{Token: p.currToken, Value: p.currToken.Literal},
				IsConstant: isConstant,
			}

			field.Type = p.parseTypeSignature()
			if field.Type == "" {
				p.reportError(p.currToken, "expected type signature")
				return nil
			}

			statement.StructDefinition.Fields = append(statement.StructDefinition.Fields, field)
		}

		if !p.expectPeek(lexer.RBRACE) {
			p.reportError(p.peekToken, fmt.Sprintf("expected rbrace, got %s", p.currToken.Type))
			return nil
		}
	} else {
		statement.TargetType = p.parseTypeSignature()
	}

	return statement
}

// parseTypeSignature parses a type identifier or array type like [Number] or [[Number]].
func (p *Parser) parseTypeSignature() string {
	if p.peekToken.Type == lexer.FN {
		p.nextToken() // move to fn
		if !p.expectPeek(lexer.LPAREN) {
			return ""
		}

		var params []string
		if p.peekToken.Type != lexer.RPAREN {
			paramType := p.parseTypeSignature()
			if paramType != "" {
				params = append(params, paramType)
			}
			for p.peekToken.Type == lexer.COMMA {
				p.nextToken() // move to comma
				paramType := p.parseTypeSignature()
				if paramType != "" {
					params = append(params, paramType)
				}
			}
		}

		if !p.expectPeek(lexer.RPAREN) {
			return ""
		}

		returnType := "Nothing"
		if p.peekToken.Type == lexer.ARROW {
			p.nextToken() // move to ->
			returnType = p.parseTypeSignature()
		}

		typeName := "fn(" + strings.Join(params, ", ") + ") -> " + returnType
		if p.peekToken.Type == lexer.QUESTION {
			p.nextToken() // move to ?
			typeName += "?"
		}
		return typeName
	}

	if p.peekToken.Type == lexer.LBRACKET {
		p.nextToken() // move to [
		innerType := p.parseTypeSignature()
		if !p.expectPeek(lexer.RBRACKET) {
			return ""
		}

		typeName := "[" + innerType + "]"
		if p.peekToken.Type == lexer.QUESTION {
			p.nextToken() // move to ?
			typeName += "?"
		}
		return typeName
	}

	if p.expectPeek(lexer.IDENT) {
		typeName := p.currToken.Literal

		if p.peekToken.Type == lexer.LT {
			p.nextToken() // move to <
			var typeArgs []string
			if p.peekToken.Type != lexer.GT {
				for {
					typeArg := p.parseTypeSignature()
					if typeArg != "" {
						typeArgs = append(typeArgs, typeArg)
					}
					if p.peekToken.Type == lexer.COMMA {
						p.nextToken() // move to comma
					} else {
						break
					}
				}
			}
			if !p.expectPeek(lexer.GT) {
				return ""
			}
			typeName += "<" + strings.Join(typeArgs, ", ") + ">"
		}

		if typeName == "map" && p.peekToken.Type == lexer.LBRACKET {
			p.nextToken() // move to [
			keyType := p.parseTypeSignature()
			if !p.expectPeek(lexer.RBRACKET) {
				return ""
			}
			valueType := p.parseTypeSignature()
			return "map[" + keyType + "]" + valueType
		}

		if p.peekToken.Type == lexer.DOT {
			p.nextToken() // move to .
			if p.expectPeek(lexer.IDENT) {
				typeName += "." + p.currToken.Literal
			} else {
				return ""
			}
		}

		if p.peekToken.Type == lexer.QUESTION {
			if typeName == "Number" || typeName == "String" || typeName == "Boolean" || typeName == "Date" {
				p.reportError(p.peekToken, fmt.Sprintf("syntax error: primitive type '%s' cannot be nullable", typeName))
				p.nextToken() // consume ?
				return typeName
			}
			p.nextToken() // move to ?
			typeName += "?"
		}
		return typeName
	}
	p.reportError(p.peekToken, fmt.Sprintf("expected type identifier, got %s", p.peekToken.Type))
	return ""
}

// parseMapLiteral parses a map/dictionary definition, e.g., {"key": "value"}.
func (p *Parser) parseMapLiteral() ast.Expression {
	mapLiteral := &ast.MapLiteral{
		Token: p.currToken,
		Pairs: make(map[ast.Expression]ast.Expression),
	}

	for p.peekToken.Type != lexer.RBRACE {
		p.nextToken()
		key := p.parseExpression(lexer.LOWEST_PRECEDENCE)

		if !p.expectPeek(lexer.COLON) {
			p.reportError(p.peekToken, fmt.Sprintf("expected colon in map literal, got %s", p.currToken.Type))
			return nil
		}

		p.nextToken()
		value := p.parseExpression(lexer.LOWEST_PRECEDENCE)

		mapLiteral.Pairs[key] = value

		if p.peekToken.Type != lexer.RBRACE && !p.expectPeek(lexer.COMMA) {
			p.reportError(p.peekToken, fmt.Sprintf("expected comma or rbrace in map literal, got %s", p.currToken.Type))
			return nil
		}
	}

	if !p.expectPeek(lexer.RBRACE) {
		p.reportError(p.peekToken, fmt.Sprintf("expected rbrace in map literal, got %s", p.currToken.Type))
		return nil
	}

	return mapLiteral
}

// parseReturnStatement parses a return statement of the form "return expr".
// It captures the "return" keyword token, advances past it, and parses the
// return value expression with the lowest precedence.
func (p *Parser) parseReturnStatement() *ast.ReturnStatement {
	statement := &ast.ReturnStatement{Token: p.currToken}
	if p.peekToken.Type == lexer.RBRACE || p.peekToken.Type == lexer.EOF {
		return statement
	}

	p.nextToken()
	statement.ReturnValue = p.parseExpression(lexer.LOWEST_PRECEDENCE)

	return statement
}

// parseExpressionStatement wraps a standalone expression (one that is not part
// of an assignment) into an ExpressionStatement node.
func (p *Parser) parseExpressionStatement() *ast.ExpressionStatement {
	statement := &ast.ExpressionStatement{Token: p.currToken}
	statement.Expression = p.parseExpression(lexer.LOWEST_PRECEDENCE)
	return statement
}

// parseExpressionList parses a comma-separated list of expressions until it
// encounters the specified end token (e.g., closing parenthesis or bracket).
func (p *Parser) parseExpressionList(end lexer.TokenType) []ast.Expression {
	var list []ast.Expression

	if p.peekToken.Type == end {
		p.nextToken()
		return list
	}

	p.nextToken()
	list = append(list, p.parseExpression(lexer.LOWEST_PRECEDENCE))
	for p.peekToken.Type == lexer.COMMA {
		p.nextToken() // Move to the comma
		p.nextToken() // Move past the comma to the next expression

		list = append(list, p.parseExpression(lexer.LOWEST_PRECEDENCE))
	}

	if !p.expectPeek(end) {
		p.reportError(p.peekToken, fmt.Sprintf("expected '%s', got %s", end, p.currToken.Type))
		return nil
	}

	return list
}

// parseExpression is the core of the Pratt parser. It looks up a prefix parse
// function for the current token, then repeatedly applies infix parse functions
// as long as the next token's precedence exceeds the given precedence level,
// building a left-recursive expression tree.
func (p *Parser) parseExpression(precedence int) ast.Expression {
	prefix := p.prefixParseFuncs[p.currToken.Type]
	if prefix == nil {
		p.reportError(p.currToken, fmt.Sprintf("unknown prefix type %q", p.currToken.Type))
		return nil
	}

	leftExpression := prefix()
	for p.peekToken.Type != lexer.EOF && precedence < lexer.VerifyPrecedenceLevel(p.peekToken.Type) {
		infix := p.infixParseFuncs[p.peekToken.Type]
		if infix == nil {
			return leftExpression
		}

		p.nextToken()
		leftExpression = infix(leftExpression)
	}

	return leftExpression
}

// parseIdentifier returns an Identifier expression node for the current token.
func (p *Parser) parseIdentifier() ast.Expression {
	return &ast.Identifier{Token: p.currToken, Value: p.currToken.Literal}
}

// parsePrefixExpression parses a prefix operator expression, such as -5 or !true.
func (p *Parser) parsePrefixExpression() ast.Expression {
	expression := &ast.PrefixExpression{
		Token:    p.currToken,
		Operator: p.currToken.Literal,
	}
	p.nextToken()
	expression.Right = p.parseExpression(lexer.PREFIX_PRECEDENCE)
	return expression
}

// parseInfixExpression builds an InfixExpression node using the already-parsed
// left operand, the current operator token, and a recursively parsed right
// operand whose binding power is determined by the current token's precedence.
func (p *Parser) parseInfixExpression(left ast.Expression) ast.Expression {
	expression := &ast.InfixExpression{
		Token:    p.currToken,
		Operator: p.currToken.Literal,
		Left:     left,
	}

	precedence := lexer.VerifyPrecedenceLevel(p.currToken.Type)
	p.nextToken()

	expression.Right = p.parseExpression(precedence)

	return expression
}

// parsePropertyExpression parses an object property access, capturing the left-hand
// expression (the object) and parsing the identifier following the dot.
func (p *Parser) parsePropertyExpression(left ast.Expression) ast.Expression {
	exp := &ast.PropertyExpression{
		Token:  p.currToken,
		Object: left,
		Safe:   p.currToken.Type == lexer.QUESTIONDOT,
	}

	if p.peekToken.Type == lexer.IDENT || lexer.IsKeyword(p.peekToken.Type) {
		p.nextToken()
	} else {
		p.reportError(p.peekToken, fmt.Sprintf("expected property name, got %s", p.peekToken.Type))
		return nil
	}

	exp.Property = &ast.Identifier{Token: p.currToken, Value: p.currToken.Literal}
	return exp
}

// parseIfExpression parses an 'if' expression, expecting an opening parenthesis,
// a condition, a closing parenthesis, and a block statement for the consequence.
// It also parses an optional 'else' block if the 'else' keyword is present.
func (p *Parser) parseIfExpression() ast.Expression {
	expression := &ast.IfExpression{Token: p.currToken}

	if !p.expectPeek(lexer.LPAREN) {
		p.reportError(p.peekToken, fmt.Sprintf("expected '(', got %s", p.currToken.Type))
		return nil
	}
	p.nextToken()

	expression.Condition = p.parseExpression(lexer.LOWEST_PRECEDENCE)

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

// parseIndexExpression parses an array index operation, capturing the left-hand
// expression (the array) and parsing the expression inside the brackets as the index.
func (p *Parser) parseIndexExpression(left ast.Expression) ast.Expression {
	exp := &ast.IndexExpression{Token: p.currToken, Left: left}
	p.nextToken()
	exp.Index = p.parseExpression(lexer.LOWEST_PRECEDENCE)
	if !p.expectPeek(lexer.RBRACKET) {
		return nil
	}
	return exp
}

// parseGroupedExpression handles parenthesized sub-expressions. It consumes
// the opening '(', recursively parses the inner expression with the lowest
// precedence, and expects a closing ')' — recording an error if it is missing.
func (p *Parser) parseGroupedExpression() ast.Expression {
	p.nextToken()

	groupedExpression := p.parseExpression(lexer.LOWEST_PRECEDENCE)
	if !p.expectPeek(lexer.RPAREN) {
		p.reportError(p.peekToken, fmt.Sprintf("expected ')' after grouped expression, got %s", p.currToken.Type))
		return nil
	}

	return groupedExpression
}

// parsePipeExpression rewrites A |> f(B) to f(A, B)
func (p *Parser) parsePipeExpression(left ast.Expression) ast.Expression {
	p.nextToken() // Move past '|>'

	right := p.parseExpression(lexer.PIPE_PRECEDENCE)

	if callExp, ok := right.(*ast.CallExpression); ok {
		callExp.Arguments = append([]ast.Expression{left}, callExp.Arguments...)
		return callExp
	}

	return &ast.CallExpression{
		Function:  right,
		Arguments: []ast.Expression{left},
	}
}

// parseFunctionCallExpression parses a function call, capturing the function
// expression and its parsed arguments.
func (p *Parser) parseFunctionCallExpression(function ast.Expression) ast.Expression {
	exp := &ast.CallExpression{Token: p.currToken, Function: function}
	exp.Arguments = p.parseExpressionList(lexer.RPAREN)
	exp.RParenToken = p.currToken
	return exp
}

// parseTurbofishExpression parses a turbofish operator `::` followed by `<Type>`.
// It returns either a GenericIdentifier (if not followed by `(`) or a CallExpression.
func (p *Parser) parseTurbofishExpression(left ast.Expression) ast.Expression {
	tok := p.currToken // The '::' token

	if !p.expectPeek(lexer.LT) {
		return nil
	}

	typeArgs := p.parseTypeParameters()

	// If the next token is `(`, it's a generic function call
	if p.peekToken.Type == lexer.LPAREN {
		p.nextToken() // move to '('
		exp := &ast.CallExpression{Token: tok, Function: left, TypeArguments: typeArgs}
		exp.Arguments = p.parseExpressionList(lexer.RPAREN)
		exp.RParenToken = p.currToken
		return exp
	}

	// Otherwise, it's a generic identifier (e.g. for struct instantiation)
	if ident, ok := left.(*ast.Identifier); ok {
		return &ast.GenericIdentifier{
			Token:         tok,
			Identifier:    ident,
			TypeArguments: typeArgs,
		}
	}

	p.reportError(p.currToken, "invalid generic expression")
	return nil
}

// reportError formats and appends a syntax error with the given token's line and column.
func (p *Parser) reportError(token lexer.Token, msg string) {
	p.diagnosticErrors = append(p.diagnosticErrors, ast.DiagnosticError{Token: token, Message: msg})
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

func (p *Parser) WithContext(ctx context.Context) *Parser {
	p.ctx = ctx
	return p
}

// parseSafePipeExpression rewrites A ?> f(B) into a SafePipeExpression with Left=A, Call=f(A, B)
func (p *Parser) parseSafePipeExpression(left ast.Expression) ast.Expression {
	token := p.currToken // The '?>' token
	p.nextToken() // Move past '?>'

	right := p.parseExpression(lexer.PIPE_PRECEDENCE)

	var callExp *ast.CallExpression
	if ce, ok := right.(*ast.CallExpression); ok {
		ce.Arguments = append([]ast.Expression{left}, ce.Arguments...)
		callExp = ce
	} else {
		callExp = &ast.CallExpression{
			Function:  right,
			Arguments: []ast.Expression{left},
		}
	}

	return &ast.SafePipeExpression{
		Token: token,
		Left:  left,
		Call:  callExp,
	}
}
