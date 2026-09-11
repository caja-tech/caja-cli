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
	p.prefixParseFuncs[lexer.IDENT] = p.parseIdentifierOrAnonymousFunction
	p.prefixParseFuncs[lexer.NUMBER] = p.parseNumberLiteral
	p.prefixParseFuncs[lexer.STRING] = p.parseStringLiteral
	p.prefixParseFuncs[lexer.DATE] = p.parseDateLiteral
	p.prefixParseFuncs[lexer.LPAREN] = p.parseGroupedExpressionOrAnonymousFunction
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
	p.prefixParseFuncs[lexer.REACT] = p.parsePrefixExpression
	p.prefixParseFuncs[lexer.MEMO] = p.parseMemoExpression
	p.prefixParseFuncs[lexer.ASYNC] = p.parseAsyncExpression
	p.prefixParseFuncs[lexer.UNWRAP] = p.parseUnwrapExpression

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
	p.infixParseFuncs[lexer.IS] = p.parseIsExpression
	p.infixParseFuncs[lexer.LPAREN] = p.parseFunctionCallExpression
	p.infixParseFuncs[lexer.DOUBLE_COLON] = p.parseTurbofishExpression
	p.infixParseFuncs[lexer.LBRACE] = p.parseBraceInfixExpression
	p.infixParseFuncs[lexer.LBRACKET] = p.parseIndexExpression
	p.infixParseFuncs[lexer.DOT] = p.parsePropertyExpression
	p.infixParseFuncs[lexer.QUESTIONDOT] = p.parsePropertyExpression
	p.infixParseFuncs[lexer.AND] = p.parseInfixExpression
	p.infixParseFuncs[lexer.OR] = p.parseInfixExpression
	p.infixParseFuncs[lexer.XOR] = p.parseInfixExpression
	p.infixParseFuncs[lexer.PIPE] = p.parsePipeExpression
	p.infixParseFuncs[lexer.SAFE_PIPE] = p.parseSafePipeExpression
	p.infixParseFuncs[lexer.STREAM_PIPE] = p.parseStreamPipeExpression
	p.infixParseFuncs[lexer.SAFE_STREAM_PIPE] = p.parseSafeStreamPipeExpression

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

// parseStringLiteral returns a StringLiteral expression node for the current
// token, or — if the raw literal contains a backslash escape or "${"
// (Kotlin-style string interpolation) — runs the full escape+interpolation
// scan and returns either a decoded StringLiteral (no embedded expressions
// found) or an InterpolatedStringLiteral. A plain literal with neither
// anywhere in it parses exactly as before, byte for byte — this is a cheap
// fast path, not just an optimization: the overwhelming majority of string
// literals in any program have no escapes or interpolation at all.
func (p *Parser) parseStringLiteral() ast.Expression {
	token := p.currToken
	raw := token.Literal

	if !strings.Contains(raw, "\\") && !strings.Contains(raw, "${") {
		return &ast.StringLiteral{Token: token, Value: raw}
	}

	segments, hasExpr, ok := p.scanStringLiteralContent(token)
	if !ok {
		return nil
	}
	if !hasExpr {
		value := ""
		if len(segments) > 0 {
			value = segments[0].Text
		}
		return &ast.StringLiteral{Token: token, Value: value}
	}
	return &ast.InterpolatedStringLiteral{Token: token, Segments: segments}
}

// decodeEscape decodes a single escape sequence at the start of s (which
// must begin with '\'), returning the decoded text, how many raw bytes it
// consumed (including the backslash), and whether it was recognized.
// Matches Kotlin's own escape set exactly: \t \b \n \r \' \" \\ \$, plus
// \uXXXX (exactly 4 hex digits) for an arbitrary Unicode code point. An
// unrecognized escape returns ok=false — Caja is strict here, matching
// Kotlin/Go's own compile-time rejection of unknown escapes rather than
// silently passing an unrecognized "\x" through as two literal characters.
func decodeEscape(s string) (decoded string, consumed int, ok bool) {
	if len(s) < 2 {
		return "", len(s), false
	}
	switch s[1] {
	case 't':
		return "\t", 2, true
	case 'b':
		return "\b", 2, true
	case 'n':
		return "\n", 2, true
	case 'r':
		return "\r", 2, true
	case '\'':
		return "'", 2, true
	case '"':
		return "\"", 2, true
	case '\\':
		return "\\", 2, true
	case '$':
		return "$", 2, true
	case 'u':
		if len(s) < 6 {
			return "", len(s), false
		}
		code, err := strconv.ParseInt(s[2:6], 16, 32)
		if err != nil {
			return "", 6, false
		}
		return string(rune(code)), 6, true
	default:
		return "", 2, false
	}
}

// scanStringLiteralContent walks a STRING token's raw literal text once,
// decoding escape sequences and splitting out "${...}" interpolations at
// the same time — order matters here, for two reasons:
//
//   - An escaped "\$" must not be mistaken for the start of a real
//     interpolation. Decoding escapes as a separate, EARLIER pass (e.g.
//     inside lexer.readString itself) would collapse "\${x}" down to "${x}"
//     before this scan ever ran, defeating the escape's whole purpose — so
//     escape-decoding and "${"-detection happen together, in one left-to-
//     right pass, exactly in source order.
//   - Interpolation boundary-finding must stay anchored to RAW source byte
//     positions for advancePosition/lexer.NewAt's seeding to stay correct —
//     decoding a literal-text segment can only shrink it (e.g. "\n", two
//     source bytes, becomes one real newline byte), so position tracking is
//     always computed against the untouched raw text, never the decoded
//     output.
//
// Boundary-finding itself (matching "${" to its own "}", skipping over any
// nested string/date literal so braces or quotes INSIDE one don't miscount)
// is done by plain text scanning (findInterpolationEnd), not by tokenizing —
// Lexer exposes no byte-offset accessor a token-based scan could use to know
// where to resume. Each extracted expression's text is then parsed with a
// fresh lexer+parser, seeded via lexer.NewAt at its true position in the
// original file, so a syntax error inside "${...}" is still reported at the
// right line/column, not relative to the extracted substring.
//
// Returns the segment list, whether at least one embedded expression was
// found (false means the string is just a decoded plain string, packaged by
// the caller into a StringLiteral instead), and ok=false if a fatal error
// (e.g. an unterminated interpolation) was already reported and parsing
// should abort for this literal.
func (p *Parser) scanStringLiteralContent(token lexer.Token) (segments []ast.InterpolatedStringSegment, hasExpr bool, ok bool) {
	raw := token.Literal
	var textBuf strings.Builder
	line, col := token.Line, token.Column+1 // position of raw[0]
	i := 0

	flushText := func() {
		if textBuf.Len() > 0 {
			segments = append(segments, ast.InterpolatedStringSegment{Text: textBuf.String()})
			textBuf.Reset()
		}
	}

	for i < len(raw) {
		if raw[i] == '\\' {
			decoded, consumed, escOk := decodeEscape(raw[i:])
			if !escOk {
				end := min(i+consumed, len(raw))
				p.reportError(token, fmt.Sprintf("syntax error: unrecognized escape sequence '%s' in string literal", raw[i:end]))
				textBuf.WriteByte('\\')
				line, col = advancePosition(line, col, raw[i:i+1])
				i++
				continue
			}
			textBuf.WriteString(decoded)
			line, col = advancePosition(line, col, raw[i:i+consumed])
			i += consumed
			continue
		}

		if raw[i] == '$' && i+1 < len(raw) && raw[i+1] == '{' {
			flushText()
			hasExpr = true

			// Position of the expression's first byte, i.e. right after "${".
			exprLine, exprCol := advancePosition(line, col, raw[i:i+2])

			end := findInterpolationEnd(raw[i+2:])
			if end == -1 {
				p.reportError(token, "syntax error: unterminated string interpolation, missing '}'")
				return nil, false, false
			}
			exprText := raw[i+2 : i+2+end]

			// NewAt's seed column is "one before" the first character it will
			// report (its own priming readChar() call unconditionally advances
			// Column once before any real token is produced — the same
			// convention New(input) itself relies on for column 1 to be a
			// file's first real character), hence exprCol-1 here.
			exprLexer := lexer.NewAt(exprText, exprLine, exprCol-1)
			exprParser := New(exprLexer)
			expr := exprParser.parseExpression(lexer.LOWEST_PRECEDENCE)
			if exprParser.peekToken.Type != lexer.EOF {
				exprParser.reportError(exprParser.peekToken, fmt.Sprintf("syntax error: unexpected token '%s' after string interpolation expression", exprParser.peekToken.Literal))
			}
			p.diagnosticErrors = append(p.diagnosticErrors, exprParser.diagnosticErrors...)
			p.tknzr.Errors = append(p.tknzr.Errors, exprLexer.Errors...)

			segments = append(segments, ast.InterpolatedStringSegment{Expr: expr})

			consumedThrough := i + 2 + end + 1 // everything through the closing '}'
			line, col = advancePosition(line, col, raw[i:consumedThrough])
			i = consumedThrough
			continue
		}

		textBuf.WriteByte(raw[i])
		line, col = advancePosition(line, col, raw[i:i+1])
		i++
	}
	flushText()
	return segments, hasExpr, true
}

// findInterpolationEnd finds the byte offset (relative to the start of s) of
// the "}" that closes an interpolation opened by "${", given s is everything
// after that "${". Braces and quotes preceded by "\" are skipped without
// inspection (mirroring decodeEscape's own escape recognition, so an escaped
// brace/quote inside the expression text can't desync the count or the
// nested-literal scan below), and braces inside a nested string ("...") or
// date ('...') literal don't count either, mirroring readString/readDate's
// own scan-to-the-next-unescaped-delimiter rule. Returns -1 if no matching
// "}" is found.
func findInterpolationEnd(s string) int {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++ // skip the escaped character without inspecting it
		case '"', '\'':
			quote := s[i]
			i++
			for i < len(s) && s[i] != quote {
				if s[i] == '\\' {
					i++
				}
				i++
			}
		case '{':
			depth++
		case '}':
			if depth == 0 {
				return i
			}
			depth--
		}
	}
	return -1
}

// advancePosition returns the (line, column) of the character immediately
// following text, given text's own first character sits at (line, col) —
// mirroring Lexer.readChar's newline-counting rule (a lone '\r', or '\n',
// starts a new line; "\r\n" together counts as one newline, matching
// isCurrentCharANewLine).
func advancePosition(line, col int, text string) (int, int) {
	for i := 0; i < len(text); i++ {
		ch := text[i]
		isNewline := ch == '\n' || (ch == '\r' && (i+1 >= len(text) || text[i+1] != '\n'))
		if isNewline {
			line++
			col = 1
		} else {
			col++
		}
	}
	return line, col
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
		param := &ast.Parameter{Token: p.currToken, Name: p.currToken.Literal}

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

// parseBraceInfixExpression is the sole entry point registered for '{' in
// infix position. If left is a call expression, '{' introduces a trailing
// block (Kotlin-style DSL sugar: call(args) { x y } -> call(args, [x, y])).
// Otherwise it's a struct literal (existing behavior), including the
// existing error for any other left type.
func (p *Parser) parseBraceInfixExpression(left ast.Expression) ast.Expression {
	if call, ok := left.(*ast.CallExpression); ok {
		return p.parseTrailingBlockCall(call)
	}
	return p.parseStructLiteral(left)
}

// parseTrailingBlockCall implements Kotlin-style trailing-block sugar:
// call(args) { stmt1 stmt2 ... } desugars into call(args, [stmt1, stmt2, ...]).
// Every line inside the block must be a bare expression statement; the
// collected expressions become the elements of a new ArrayLiteral appended
// as the call's final positional argument. p.currToken is the '{' token on
// entry, per the infixParseFunc convention.
func (p *Parser) parseTrailingBlockCall(call *ast.CallExpression) ast.Expression {
	openBrace := p.currToken
	block := p.parseBlockStatement() // leaves p.currToken on '}' (or EOF)

	elements := make([]ast.Expression, 0, len(block.Statements))
	for _, stmt := range block.Statements {
		exprStmt, ok := stmt.(*ast.ExpressionStatement)
		if !ok {
			p.reportError(statementToken(stmt, openBrace), fmt.Sprintf("syntax error: only expressions are allowed inside a trailing block, got '%s'", stmt.TokenLiteral()))
			continue
		}
		elements = append(elements, exprStmt.Expression)
	}

	call.Arguments = append(call.Arguments, &ast.ArrayLiteral{
		Token:    openBrace,
		Elements: elements,
	})
	call.RParenToken = p.currToken // extend the call's span to the block's closing '}'

	return call
}

// statementToken returns the token best representing where a statement
// begins, used for precise diagnostics. Falls back to fallback for any
// statement kind not explicitly recognized.
func statementToken(s ast.Statement, fallback lexer.Token) lexer.Token {
	switch st := s.(type) {
	case *ast.LetStatement:
		return st.Token
	case *ast.ConstStatement:
		return st.Token
	case *ast.ReturnStatement:
		return st.Token
	case *ast.ImportStatement:
		return st.Token
	case *ast.TypeAliasStatement:
		return st.Token
	case *ast.TypeConstraintStatement:
		return st.Token
	case *ast.AwaitStatement:
		return st.Token
	case *ast.AssignStatement:
		return st.Token
	case *ast.IndexAssignmentStatement:
		return st.Token
	case *ast.PropertyAssignmentStatement:
		return st.Token
	default:
		return fallback
	}
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
		if p.currToken.Type != lexer.LET && p.currToken.Type != lexer.TYPE && p.currToken.Type != lexer.CONST && p.currToken.Type != lexer.DEFINE && p.currToken.Type != lexer.UNION {
			p.reportError(p.currToken, "syntax error: 'private' modifier must be followed by 'let', 'const', 'type', 'define', or 'union'")
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

	if p.currToken.Type == lexer.UNION {
		stmt := p.parseUnionStatement()
		if stmt == nil {
			return nil
		}
		stmt.IsPrivate = isPrivate
		return stmt
	}

	if p.currToken.Type == lexer.AWAIT {
		return p.parseAwaitStatement()
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

	if p.peekToken.Type == lexer.ASTERISK {
		// `import * from mod` — '*' and '{...}' are alternatives, never combined.
		p.nextToken() // move to '*'
		statement.IsWildcard = true

		// 'from' is not a keyword or token type of its own; it is matched as a
		// literal IDENT, the same way the named-import branch below does.
		if !p.expectPeek(lexer.IDENT) || p.currToken.Literal != "from" {
			p.reportError(p.currToken, "expected 'from' after wildcard import")
			return nil
		}
	} else if p.peekToken.Type == lexer.LBRACE {
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

	if p.peekToken.Type == lexer.ACTIVE {
		p.nextToken() // consume 'active'
		statement.IsActive = true
	}

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

// parseUnionStatement parses a union type declaration of the form
// "union Name = Variant1 | Variant2 | ... | VariantN".
func (p *Parser) parseUnionStatement() *ast.UnionStatement {
	stmt := &ast.UnionStatement{Token: p.currToken}

	if !p.expectPeek(lexer.IDENT) {
		p.reportError(p.peekToken, fmt.Sprintf("expected identifier, got %s", p.currToken.Type))
		return nil
	}
	stmt.Name = &ast.Identifier{Token: p.currToken, Value: p.currToken.Literal}

	if !p.expectPeek(lexer.ASSIGN) {
		p.reportError(p.peekToken, fmt.Sprintf("expected '=', got %s", p.currToken.Type))
		return nil
	}

	if !p.expectPeek(lexer.IDENT) {
		p.reportError(p.peekToken, fmt.Sprintf("expected variant identifier, got %s", p.currToken.Type))
		return nil
	}
	stmt.Variants = append(stmt.Variants, &ast.Identifier{Token: p.currToken, Value: p.currToken.Literal})

	for p.peekToken.Type == lexer.BAR {
		p.nextToken() // move to '|'
		if !p.expectPeek(lexer.IDENT) {
			p.reportError(p.peekToken, fmt.Sprintf("expected variant identifier, got %s", p.currToken.Type))
			return nil
		}
		stmt.Variants = append(stmt.Variants, &ast.Identifier{Token: p.currToken, Value: p.currToken.Literal})
	}

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

// parseCallArgumentList is parseExpressionList's counterpart for call sites
// that also accept named arguments (name: value), e.g. route(method: "GET").
// Positional arguments must come before named ones; a positional argument
// appearing after a named one is a syntax error, keeping the resolution
// rule simple ("named args fill whatever positionals left open").
func (p *Parser) parseCallArgumentList(end lexer.TokenType) ([]ast.Expression, []*ast.NamedArgument) {
	var positional []ast.Expression
	var named []*ast.NamedArgument

	if p.peekToken.Type == end {
		p.nextToken()
		return positional, named
	}

	p.nextToken()
	if !p.parseCallArgument(&positional, &named) {
		return nil, nil
	}

	for p.peekToken.Type == lexer.COMMA {
		p.nextToken() // Move to the comma
		p.nextToken() // Move past the comma to the next argument

		if !p.parseCallArgument(&positional, &named) {
			return nil, nil
		}
	}

	if !p.expectPeek(end) {
		p.reportError(p.peekToken, fmt.Sprintf("expected '%s', got %s", end, p.currToken.Type))
		return nil, nil
	}

	return positional, named
}

// parseCallArgument parses a single call argument (with p.currToken on its
// first token) as either positional or named ("name: value"), appending it
// to the corresponding slice. Returns false (having already reported an
// error) if a positional argument follows a named one, or if the argument
// expression itself fails to parse.
func (p *Parser) parseCallArgument(positional *[]ast.Expression, named *[]*ast.NamedArgument) bool {
	if p.currToken.Type == lexer.IDENT && p.peekToken.Type == lexer.COLON {
		nameToken := p.currToken
		nameIdent := &ast.Identifier{Token: nameToken, Value: nameToken.Literal}
		p.nextToken() // move onto ':'
		p.nextToken() // move past ':' onto the value's first token
		val := p.parseExpression(lexer.LOWEST_PRECEDENCE)
		if val == nil {
			return false
		}
		*named = append(*named, &ast.NamedArgument{Token: nameToken, Name: nameIdent, Value: val})
		return true
	}

	if len(*named) > 0 {
		p.reportError(p.currToken, "syntax error: positional arguments must come before named arguments")
		return false
	}

	val := p.parseExpression(lexer.LOWEST_PRECEDENCE)
	if val == nil {
		return false
	}
	*positional = append(*positional, val)
	return true
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

	// Kotlin-style trailing lambda: "call(args) paramName => { ... }" appends
	// a bare FunctionLiteral (not an ArrayLiteral, unlike the trailing-BLOCK
	// sugar above which triggers on '{' via infixParseFuncs[LBRACE]) as the
	// call's last argument. Deliberately NOT implemented as a registered
	// infixParseFuncs[IDENT] entry: IDENT is the single most common prefix
	// token in the grammar, so giving it a real infix precedence would make
	// the loop above attempt this check after every identifier-ending
	// expression, everywhere — a much larger blast radius than this
	// call-expression-only, two-token-confirmed pattern needs. The
	// "precedence == LOWEST_PRECEDENCE" guard restricts this to where an
	// expression is entered fresh (a statement, a let/const RHS, a call
	// argument) rather than mid-operator, which is every context the actual
	// feature needs and no more.
	if precedence == lexer.LOWEST_PRECEDENCE {
		if call, ok := leftExpression.(*ast.CallExpression); ok && p.peekToken.Type == lexer.IDENT {
			// One-token-ahead-of-peek check via a cloned lexer, mirroring
			// isAnonymousFunctionLookahead's established pattern: p.tknzr's
			// cursor already sits just past peekToken, so cloning it and
			// pulling one token tells us what follows the identifier
			// without consuming anything from the real parser state.
			if p.tknzr.Clone().NextToken().Type == lexer.FAT_ARROW {
				paramTok := p.peekToken
				p.nextToken() // consume the parameter identifier
				p.nextToken() // consume FAT_ARROW
				fn := &ast.FunctionLiteral{Token: paramTok}
				fn.Parameters = []*ast.Parameter{{Token: paramTok, Name: paramTok.Literal, Type: ""}}
				fn.Body = p.parseArrowFunctionBody()
				call.Arguments = append(call.Arguments, fn)
				call.RParenToken = p.currToken // extend the call's span, mirroring parseTrailingBlockCall
			}
		}
	}

	return leftExpression
}

// parseIdentifier returns an Identifier expression node for the current token.
func (p *Parser) parseIdentifier() ast.Expression {
	return &ast.Identifier{Token: p.currToken, Value: p.currToken.Literal}
}

// parseIdentifierOrAnonymousFunction parses an identifier, but if it is followed
// by a FAT_ARROW (=>), it parses it as a single-parameter anonymous function.
func (p *Parser) parseIdentifierOrAnonymousFunction() ast.Expression {
	if p.peekToken.Type == lexer.FAT_ARROW {
		lit := &ast.FunctionLiteral{Token: p.currToken}
		param := &ast.Parameter{Token: p.currToken, Name: p.currToken.Literal, Type: ""}
		lit.Parameters = []*ast.Parameter{param}

		p.nextToken() // move to FAT_ARROW
		lit.Body = p.parseArrowFunctionBody()
		return lit
	}
	return p.parseIdentifier()
}

// parseArrowFunctionBody parses the body of an arrow function once its
// parameter list is built and p.currToken sits on the FAT_ARROW ('=>')
// token: a full block if '{' follows, otherwise a single expression
// implicitly wrapped in a return statement (e.g. "x => x + 1"). Shared by
// every "=>"-based arrow-function form — single-param
// (parseIdentifierOrAnonymousFunction), multi-param (parseAnonymousFunction),
// and the trailing-lambda sugar (parseExpression) — so the three forms'
// bodies parse identically rather than drifting apart.
func (p *Parser) parseArrowFunctionBody() *ast.BlockStatement {
	if p.peekToken.Type == lexer.LBRACE {
		p.nextToken() // move to LBRACE
		return p.parseBlockStatement()
	}
	p.nextToken() // move to start of expression
	expr := p.parseExpression(lexer.LOWEST_PRECEDENCE)
	return &ast.BlockStatement{
		Token: p.currToken,
		Statements: []ast.Statement{
			&ast.ReturnStatement{
				Token:       p.currToken,
				ReturnValue: expr,
			},
		},
	}
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

// parseMemoExpression parses the 'memo' modifier keyword, which must be
// immediately followed by a function literal. It sets IsMemo on the parsed
// FunctionLiteral and returns it directly, rather than introducing a
// separate wrapper AST node.
func (p *Parser) parseMemoExpression() ast.Expression {
	memoToken := p.currToken
	p.nextToken()
	expr := p.parseExpression(lexer.PREFIX_PRECEDENCE)

	fnLit, ok := expr.(*ast.FunctionLiteral)
	if !ok {
		p.reportError(memoToken, "syntax error: 'memo' modifier must be applied to a function literal")
		return nil
	}

	fnLit.IsMemo = true
	return fnLit
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

// parseIsExpression parses a union-narrowing check of the form
// "left is TypeName". Unlike parseInfixExpression, the right-hand side is a
// bare type name, not a parsed expression, so it is registered separately
// and reads the identifier literal directly (mirroring how "as" in
// parseImportStatement and type-signature parsing treat type names as raw
// strings rather than expressions).
func (p *Parser) parseIsExpression(left ast.Expression) ast.Expression {
	tok := p.currToken

	if !p.expectPeek(lexer.IDENT) {
		p.reportError(p.peekToken, fmt.Sprintf("expected type name after 'is', got %s", p.peekToken.Type))
		return nil
	}
	typeName := p.currToken.Literal

	// Allow a module-qualified type name (e.g. "animal is animals.Cat"),
	// mirroring how parseTypeSignature reads dotted type names elsewhere.
	if p.peekToken.Type == lexer.DOT {
		p.nextToken() // move to '.'
		if !p.expectPeek(lexer.IDENT) {
			p.reportError(p.peekToken, fmt.Sprintf("expected type name after '.', got %s", p.peekToken.Type))
			return nil
		}
		typeName += "." + p.currToken.Literal
	}

	return &ast.IsExpression{Token: tok, Left: left, TypeName: typeName}
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

// isAnonymousFunctionLookahead checks if the parens enclose a parameter list followed by FAT_ARROW
func (p *Parser) isAnonymousFunctionLookahead() bool {
	tmpLexer := p.tknzr.Clone()
	
	depth := 1 // currently at '('
	
	for {
		tok := tmpLexer.NextToken()
		if tok.Type == lexer.EOF {
			return false
		}
		if tok.Type == lexer.LPAREN {
			depth++
		} else if tok.Type == lexer.RPAREN {
			depth--
			if depth == 0 {
				nextTok := tmpLexer.NextToken()
				return nextTok.Type == lexer.FAT_ARROW
			}
		}
	}
}

// parseAsyncExpression parses `async <expr>`. The operand is parsed at
// LOWEST_PRECEDENCE (not PREFIX_PRECEDENCE, unlike parsePrefixExpression)
// so it greedily captures a full trailing pipe chain — PIPE_PRECEDENCE sits
// below PREFIX_PRECEDENCE, so a tighter binding would stop before |>/|>>.
// The analyzer, not the parser, restricts where this node may legally
// appear (see Analyzer.analyzeTopLevelValue).
func (p *Parser) parseAsyncExpression() ast.Expression {
	token := p.currToken // the 'async' token
	p.nextToken()
	right := p.parseExpression(lexer.LOWEST_PRECEDENCE)
	return &ast.AsyncExpression{Token: token, Right: right}
}

// parseUnwrapExpression parses `unwrap <expr>`, used to extract the value
// out of an async handle — the only construct that does so, since `await`
// is a pure synchronization barrier that never produces a value (see
// parseAwaitStatement and ast.AwaitStatement).
func (p *Parser) parseUnwrapExpression() ast.Expression {
	token := p.currToken // the 'unwrap' token
	p.nextToken()
	right := p.parseExpression(lexer.LOWEST_PRECEDENCE)
	return &ast.UnwrapExpression{Token: token, Right: right}
}

// parseAwaitStatement parses a bare 'await' statement: the WaitGroup-style
// synchronization barrier `await p1 & p2 & ... & pn` (one or more '&'-joined
// operands, ast.AwaitStatement) — reusing the same unparenthesized
// '&'-lookahead pattern already proven for parallel join groups
// (parseGroupedExpressionOrAnonymousFunction), which works because AMP has
// no registered precedence/infix function and so
// parseExpression(LOWEST_PRECEDENCE) naturally stops right before it. Await
// never produces a value — even a single-operand `await p` is a statement,
// not an expression; use `unwrap p` to extract a value afterward.
func (p *Parser) parseAwaitStatement() ast.Statement {
	token := p.currToken // the 'await' token
	p.nextToken()
	first := p.parseExpression(lexer.LOWEST_PRECEDENCE)

	pipelines := []ast.Expression{first}
	for p.peekToken.Type == lexer.AMP {
		p.nextToken() // consume '&'
		p.nextToken() // move to start of next operand
		next := p.parseExpression(lexer.LOWEST_PRECEDENCE)
		pipelines = append(pipelines, next)
	}
	return &ast.AwaitStatement{Token: token, Pipelines: pipelines}
}

// parseGroupedExpressionOrAnonymousFunction handles parenthesized sub-expressions,
// anonymous functions, or parallel join groups (f1 & f2 & ... & fn) — a
// fixed-size set of independent function calls meant to be invoked
// concurrently as a single |>>/?>> stage's input (see JoinGroupExpression).
func (p *Parser) parseGroupedExpressionOrAnonymousFunction() ast.Expression {
	if p.isAnonymousFunctionLookahead() {
		return p.parseAnonymousFunction()
	}

	token := p.currToken // the '(' token
	p.nextToken()

	first := p.parseExpression(lexer.LOWEST_PRECEDENCE)

	if p.peekToken.Type == lexer.AMP {
		calls := []*ast.CallExpression{asJoinCall(first)}
		for p.peekToken.Type == lexer.AMP {
			p.nextToken() // consume '&'
			p.nextToken() // move to start of next call
			next := p.parseExpression(lexer.LOWEST_PRECEDENCE)
			calls = append(calls, asJoinCall(next))
		}
		if !p.expectPeek(lexer.RPAREN) {
			p.reportError(p.peekToken, fmt.Sprintf("expected ')' after join group, got %s", p.currToken.Type))
			return nil
		}
		return &ast.JoinGroupExpression{Token: token, Calls: calls}
	}

	if !p.expectPeek(lexer.RPAREN) {
		p.reportError(p.peekToken, fmt.Sprintf("expected ')' after grouped expression, got %s", p.currToken.Type))
		return nil
	}

	return first
}

// asJoinCall normalizes one member of a parallel join group into a
// *ast.CallExpression, wrapping a bare function reference (e.g. an
// identifier or property expression with no explicit call) the same way
// parseStreamPipeExpressionCommon does for a normal, non-join stage.
func asJoinCall(expr ast.Expression) *ast.CallExpression {
	if ce, ok := expr.(*ast.CallExpression); ok {
		return ce
	}
	return &ast.CallExpression{Function: expr}
}

func (p *Parser) parseAnonymousFunction() ast.Expression {
	lit := &ast.FunctionLiteral{Token: p.currToken} // Token is '('
	
	lit.Parameters = p.parseAnonymousFunctionParameters()

	if !p.expectPeek(lexer.FAT_ARROW) {
		return nil
	}

	lit.Body = p.parseArrowFunctionBody()
	return lit
}

func (p *Parser) parseAnonymousFunctionParameters() []*ast.Parameter {
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
		param := &ast.Parameter{Token: p.currToken, Name: p.currToken.Literal}

		if p.peekToken.Type == lexer.COLON {
			p.nextToken() // move to colon
			param.Type = p.parseTypeSignature()
		} else {
			param.Type = "" // inferred
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

// parsePipeExpression rewrites A |> f(B) to f(A, B)
func (p *Parser) parsePipeExpression(left ast.Expression) ast.Expression {
	token := p.currToken // The '|>' token
	p.nextToken()         // Move past '|>'

	if _, ok := left.(*ast.StreamPipeExpression); ok {
		p.reportError(token, "syntax error: a stream pipe chain (|>>/?>>) cannot feed into a regular pipe (|>) — chain further stream stages or bind the result to a variable first")
	}

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
	exp.Arguments, exp.NamedArguments = p.parseCallArgumentList(lexer.RPAREN)
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
		exp.Arguments, exp.NamedArguments = p.parseCallArgumentList(lexer.RPAREN)
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

	if _, ok := left.(*ast.StreamPipeExpression); ok {
		p.reportError(token, "syntax error: a stream pipe chain (|>>/?>>) cannot feed into a regular pipe (?>) — chain further stream stages or bind the result to a variable first")
	}

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

// parseStreamPipeExpression rewrites A |>> f(B) into a StreamPipeExpression
// with Left=A, Call=f(A, B), Safe=false.
func (p *Parser) parseStreamPipeExpression(left ast.Expression) ast.Expression {
	return p.parseStreamPipeExpressionCommon(left, false)
}

// parseSafeStreamPipeExpression rewrites A ?>> f(B) into a StreamPipeExpression
// with Left=A, Call=f(A, B), Safe=true.
func (p *Parser) parseSafeStreamPipeExpression(left ast.Expression) ast.Expression {
	return p.parseStreamPipeExpressionCommon(left, true)
}

// parseStreamPipeExpressionCommon implements the shared rewrite logic for both
// stream pipe variants (|>> and ?>>). It also handles parallel join groups
// (see JoinGroupExpression): if the parsed right-hand side is a join group,
// this stage becomes a dangling join stage (Join set, Call nil) that must be
// consumed by a following stream stage; if `left` is itself such a dangling
// join stage, this stage instead fuses with it — the join's N results become
// this stage's Call's first N (synthesized) positional arguments, and the
// join's own upstream/Safe are carried through as this stage's Left/effective
// nullability boundary.
func (p *Parser) parseStreamPipeExpressionCommon(left ast.Expression, safe bool) ast.Expression {
	token := p.currToken // The '|>>' or '?>>' token
	p.nextToken()         // Move past the operator

	right := p.parseExpression(lexer.PIPE_PRECEDENCE)

	if joinGroup, ok := right.(*ast.JoinGroupExpression); ok {
		for _, call := range joinGroup.Calls {
			call.Arguments = append([]ast.Expression{left}, call.Arguments...)
		}
		return &ast.StreamPipeExpression{
			Token: token,
			Left:  left,
			Join:  joinGroup,
			Safe:  safe,
		}
	}

	var callExp *ast.CallExpression
	if ce, ok := right.(*ast.CallExpression); ok {
		callExp = ce
	} else {
		callExp = &ast.CallExpression{Function: right}
	}

	if prevJoinStage, ok := left.(*ast.StreamPipeExpression); ok && prevJoinStage.Join != nil && prevJoinStage.Call == nil {
		// Fuse: this call consumes the pending join's N results as its own
		// leading arguments. Those N slots are left nil here — analysis and
		// codegen each fill them in independently with their own synthesized
		// placeholders — followed by whatever extra curried arguments were
		// written explicitly at this call site.
		placeholders := make([]ast.Expression, len(prevJoinStage.Join.Calls))
		callExp.Arguments = append(placeholders, callExp.Arguments...)
		return &ast.StreamPipeExpression{
			Token: token,
			Left:  prevJoinStage.Left,
			Join:  prevJoinStage.Join,
			Call:  callExp,
			Safe:  prevJoinStage.Safe,
		}
	}

	callExp.Arguments = append([]ast.Expression{left}, callExp.Arguments...)
	return &ast.StreamPipeExpression{
		Token: token,
		Left:  left,
		Call:  callExp,
		Safe:  safe,
	}
}
