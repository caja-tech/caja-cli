package lsp

import (
	"caja-cli/internal/pipeline/analyzer"
	"caja-cli/internal/pipeline/analyzer/symbol"
	"caja-cli/internal/pipeline/ast"
	"caja-cli/internal/pipeline/environment"
	"caja-cli/internal/pipeline/lexer"
	"caja-cli/internal/pipeline/parser"
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"sync"

	"github.com/owenrumney/go-lsp/document"
	"github.com/owenrumney/go-lsp/lsp"
	"github.com/owenrumney/go-lsp/server"
)

var (
	lsName    = "caja-lsp"
	serverVer string
)

type DocumentState struct {
	Prog     *ast.Program
	Analyzer *analyzer.Analyzer
}

type CajaHandler struct {
	docs   *document.Store
	client *server.Client

	mu       sync.RWMutex
	workers  map[lsp.DocumentURI]chan context.Context
	cancels  map[lsp.DocumentURI]context.CancelFunc
	astCache map[lsp.DocumentURI]*DocumentState
}

func NewCajaHandler() *CajaHandler {
	return &CajaHandler{
		docs:     document.NewStore(),
		workers:  make(map[lsp.DocumentURI]chan context.Context),
		cancels:  make(map[lsp.DocumentURI]context.CancelFunc),
		astCache: make(map[lsp.DocumentURI]*DocumentState),
	}
}

func Run(version string) error {
	serverVer = version

	h := NewCajaHandler()
	srv := server.NewServer(h)

	return srv.Run(context.Background(), server.RunStdio())
}

func (h *CajaHandler) Initialize(_ context.Context, _ *lsp.InitializeParams) (*lsp.InitializeResult, error) {
	return &lsp.InitializeResult{
		ServerInfo: &lsp.ServerInfo{
			Name:    lsName,
			Version: serverVer,
		},
	}, nil
}

func (h *CajaHandler) Shutdown(_ context.Context) error {
	return nil
}

func (h *CajaHandler) SetClient(client *server.Client) {
	h.client = client
}

func (h *CajaHandler) DidOpen(_ context.Context, params *lsp.DidOpenTextDocumentParams) error {
	h.mu.Lock()
	_, err := h.docs.Open(params)
	if err == nil {
		uri := params.TextDocument.URI
		if _, exists := h.workers[uri]; !exists {
			ch := make(chan context.Context, 1)
			h.workers[uri] = ch
			go h.documentWorkerLoop(uri, ch)
		}

		ch := h.workers[uri]

		if cancel, ok := h.cancels[uri]; ok {
			cancel()
		}

		ctx, cancelFunc := context.WithCancel(context.Background())
		h.cancels[uri] = cancelFunc

		select {
		case <-ch: // pop old context if full
		default:
		}
		ch <- ctx
	}
	h.mu.Unlock()
	return err
}

func (h *CajaHandler) DidChange(_ context.Context, params *lsp.DidChangeTextDocumentParams) error {
	h.mu.Lock()
	_, err := h.docs.Change(params)
	if err == nil {
		uri := params.TextDocument.URI
		if ch, ok := h.workers[uri]; ok {
			if cancel, hasCancel := h.cancels[uri]; hasCancel {
				cancel()
			}

			ctx, cancelFunc := context.WithCancel(context.Background())
			h.cancels[uri] = cancelFunc

			select {
			case <-ch:
			default:
			}
			ch <- ctx
		}
	}
	h.mu.Unlock()
	return err
}

func (h *CajaHandler) DidClose(_ context.Context, params *lsp.DidCloseTextDocumentParams) error {
	h.mu.Lock()
	uri := params.TextDocument.URI
	h.docs.Close(params)

	if cancel, ok := h.cancels[uri]; ok {
		cancel()
		delete(h.cancels, uri)
	}

	if ch, ok := h.workers[uri]; ok {
		close(ch)
		delete(h.workers, uri)
	}
	delete(h.astCache, uri)
	h.mu.Unlock()
	return nil
}

func (h *CajaHandler) documentWorkerLoop(uri lsp.DocumentURI, ch chan context.Context) {
	for ctx := range ch {
		h.validateDocument(ctx, string(uri))
	}
}

func (h *CajaHandler) validateDocument(ctx context.Context, uri string) {
	h.mu.RLock()
	text, ok := h.docs.Text(lsp.DocumentURI(uri))
	h.mu.RUnlock()

	if !ok {
		return
	}

	tknzr := lexer.New(text)
	p := parser.New(tknzr)
	prog := p.WithContext(ctx).Parse()
	if prog == nil {
		return
	}

	diagnostics := make([]lsp.Diagnostic, 0)
	for _, err := range p.DiagnosticErrors() {
		diagnostics = append(diagnostics, toLSPDiagnostic(err))
	}

	var a *analyzer.Analyzer
	if !p.HasErrors() {
		filePath := uriToPath(uri)
		baseDir := filepath.Dir(filePath)
		globalEnv := environment.NewEnvironment(baseDir, filePath, false)
		a = analyzer.New(globalEnv)
		a.WithContext(ctx).Run(prog)
		if ctx.Err() != nil {
			return
		}

		for _, err := range a.DiagnosticErrors() {
			diagnostics = append(diagnostics, toLSPDiagnostic(err))
		}
	} else {
		filePath := uriToPath(uri)
		baseDir := filepath.Dir(filePath)
		globalEnv := environment.NewEnvironment(baseDir, filePath, false)
		a = analyzer.New(globalEnv)
		a.WithContext(ctx).Run(prog)
		if ctx.Err() != nil {
			return
		}
	}

	// Safely cache the AST and Analyzer
	h.mu.Lock()
	h.astCache[lsp.DocumentURI(uri)] = &DocumentState{
		Prog:     prog,
		Analyzer: a,
	}
	h.mu.Unlock()

	if h.client != nil {
		_ = h.client.PublishDiagnostics(context.Background(), &lsp.PublishDiagnosticsParams{
			URI:         lsp.DocumentURI(uri),
			Diagnostics: diagnostics,
		})
	}
}

func toLSPDiagnostic(err ast.DiagnosticError) lsp.Diagnostic {
	line := err.Token.Line
	if line > 0 {
		line-- // LSP lines are 0-indexed
	}
	col := err.Token.Column
	if col > 0 {
		col-- // LSP cols are 0-indexed
	}

	length := len(err.Token.Literal)
	if length == 0 {
		length = 1
	}

	severity := lsp.SeverityError

	return lsp.Diagnostic{
		Range: lsp.Range{
			Start: lsp.Position{Line: line, Character: col},
			End:   lsp.Position{Line: line, Character: col + length},
		},
		Severity: &severity,
		Source:   lsName,
		Message:  err.Message,
	}
}

func (h *CajaHandler) Hover(_ context.Context, params *lsp.HoverParams) (*lsp.Hover, error) {
	h.mu.RLock()
	state, ok := h.astCache[params.TextDocument.URI]
	h.mu.RUnlock()

	if !ok || state == nil || state.Prog == nil || state.Analyzer == nil {
		return nil, nil
	}
	prog := state.Prog
	a := state.Analyzer

	node := FindNodeAtPosition(prog, params.Position.Line, params.Position.Character)
	if node == nil {
		return nil, nil
	}

	sym, ok := a.GetSymbol(node)
	if !ok {
		return nil, nil
	}

	markdown := "```caja\n" + sym.String() + "\n```"

	return &lsp.Hover{
		Contents: lsp.NewHoverContents(lsp.Markdown, markdown),
	}, nil
}

func (h *CajaHandler) Definition(_ context.Context, params *lsp.DefinitionParams) ([]lsp.Location, error) {
	h.mu.RLock()
	state, ok := h.astCache[params.TextDocument.URI]
	h.mu.RUnlock()

	if !ok || state == nil || state.Prog == nil || state.Analyzer == nil {
		return nil, nil
	}
	prog := state.Prog
	a := state.Analyzer

	filePath := uriToPath(string(params.TextDocument.URI))
	baseDir := filepath.Dir(filePath)

	node := FindNodeAtPosition(prog, params.Position.Line, params.Position.Character)
	if node == nil {
		return nil, nil
	}

	defToken, defFile, ok := a.GetDefinition(node)
	if !ok || defToken.Line == 0 {
		return nil, nil
	}

	line := defToken.Line - 1
	col := defToken.Column - 1
	length := len(defToken.Literal)
	if length == 0 {
		length = 1
	}

	targetURI := params.TextDocument.URI
	if defFile != "" {
		if !filepath.IsAbs(defFile) {
			defFile = filepath.Join(baseDir, defFile+".caja")
		}
		targetURI = lsp.DocumentURI("file://" + defFile)
	}

	loc := lsp.Location{
		URI: targetURI,
		Range: lsp.Range{
			Start: lsp.Position{Line: line, Character: col},
			End:   lsp.Position{Line: line, Character: col + length},
		},
	}

	return []lsp.Location{loc}, nil
}

func uriToPath(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return ""
	}
	return u.Path
}

// SignatureHelp provides signature information for a function call at the cursor position.
func (h *CajaHandler) SignatureHelp(_ context.Context, params *lsp.SignatureHelpParams) (*lsp.SignatureHelp, error) {
	h.mu.RLock()
	state, stateOk := h.astCache[params.TextDocument.URI]
	text, textOk := h.docs.Text(params.TextDocument.URI)
	h.mu.RUnlock()

	if !stateOk || state == nil || state.Prog == nil || state.Analyzer == nil {
		return nil, nil
	}
	if !textOk {
		return nil, fmt.Errorf("document not found: %s", params.TextDocument.URI)
	}
	prog := state.Prog
	a := state.Analyzer

	if prog == nil {
		fmt.Println("PROG IS NIL")
		return nil, nil
	}

	callExpr := FindCallExpressionAtPosition(prog, params.Position.Line, params.Position.Character)
	if callExpr == nil {
		return nil, nil
	}

	sym, _ := a.GetSymbol(callExpr.Function)

	if prop, ok := callExpr.Function.(*ast.PropertyExpression); ok {
		if objIdent, ok := prop.Object.(*ast.Identifier); ok {
			modName := objIdent.Value
			if symbols, _, ok := symbol.GetStandardModule(modName); ok {
				if symObj, exists := symbols[prop.Property.Value]; exists {
					sym = symObj
				}
			}
		}
	}

	var label string
	var paramsList []string

	if fnSym, ok := sym.(*symbol.FunctionSymbol); ok {
		label = fnSym.String()
		for i, paramType := range fnSym.ParamTypes() {
			paramName := "arg"
			if fnLit, ok := callExpr.Function.(*ast.FunctionLiteral); ok && i < len(fnLit.Parameters) {
				paramName = fnLit.Parameters[i].Name
			}
			paramsList = append(paramsList, fmt.Sprintf("%s: %s", paramName, paramType.String()))
		}
	} else if builtinSym, ok := sym.(*symbol.BuiltinSymbol); ok {
		label = builtinSym.Label
		paramsList = builtinSym.Params
	} else {
		return nil, nil
	}

	var sigParams []lsp.ParameterInformation
	for _, p := range paramsList {
		sigParams = append(sigParams, lsp.ParameterInformation{Label: p})
	}

	sigInfo := lsp.SignatureInformation{
		Label:      label,
		Parameters: sigParams,
	}

	activeParam := 0
	targetLine := params.Position.Line + 1
	targetCol := params.Position.Character + 1

	lexerInst := lexer.New(text)
	parenDepth := 0
	bracketDepth := 0
	braceDepth := 0
	targetDepth := -1

	for {
		tok := lexerInst.NextToken()
		if tok.Type == lexer.EOF {
			break
		}

		if tok.Line > targetLine || (tok.Line == targetLine && tok.Column >= targetCol) {
			break
		}

		switch tok.Type {
		case lexer.LPAREN:
			parenDepth++
			if tok.Line == callExpr.Token.Line && tok.Column == callExpr.Token.Column {
				targetDepth = parenDepth
			}
		case lexer.RPAREN:
			parenDepth--
		case lexer.LBRACKET:
			bracketDepth++
		case lexer.RBRACKET:
			bracketDepth--
		case lexer.LBRACE:
			braceDepth++
		case lexer.RBRACE:
			braceDepth--
		case lexer.COMMA:
			if targetDepth != -1 && parenDepth == targetDepth && bracketDepth == 0 && braceDepth == 0 {
				activeParam++
			}
		}
	}

	return &lsp.SignatureHelp{
		Signatures:      []lsp.SignatureInformation{sigInfo},
		ActiveSignature: intPtr(0),
		ActiveParameter: &activeParam,
	}, nil
}

func intPtr(i int) *int {
	return &i
}
