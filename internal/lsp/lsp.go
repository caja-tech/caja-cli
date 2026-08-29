package lsp

import (
	"caja-cli/internal/pipeline/analyzer"
	"caja-cli/internal/pipeline/ast"
	"caja-cli/internal/pipeline/environment"
	"caja-cli/internal/pipeline/lexer"
	"caja-cli/internal/pipeline/parser"
	"context"
	"net/url"
	"path/filepath"

	"github.com/owenrumney/go-lsp/document"
	"github.com/owenrumney/go-lsp/lsp"
	"github.com/owenrumney/go-lsp/server"
)

var (
	lsName    = "caja-lsp"
	serverVer string
)

type CajaHandler struct {
	docs   *document.Store
	client *server.Client
}

func Run(version string) error {
	serverVer = version

	h := &CajaHandler{docs: document.NewStore()}
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

func (h *CajaHandler) DidOpen(ctx context.Context, params *lsp.DidOpenTextDocumentParams) error {
	_, err := h.docs.Open(params)
	if err == nil {
		h.validateDocument(ctx, string(params.TextDocument.URI))
	}
	return err
}

func (h *CajaHandler) DidChange(ctx context.Context, params *lsp.DidChangeTextDocumentParams) error {
	_, err := h.docs.Change(params)
	if err == nil {
		h.validateDocument(ctx, string(params.TextDocument.URI))
	}
	return err
}

func (h *CajaHandler) DidClose(_ context.Context, params *lsp.DidCloseTextDocumentParams) error {
	h.docs.Close(params)
	return nil
}

func (h *CajaHandler) validateDocument(ctx context.Context, uri string) {
	text, ok := h.docs.Text(lsp.DocumentURI(uri))
	if !ok {
		return
	}

	tknzr := lexer.New(text)
	p := parser.New(tknzr)
	prog := p.Parse()

	diagnostics := make([]lsp.Diagnostic, 0)

	for _, err := range p.DiagnosticErrors() {
		diagnostics = append(diagnostics, toLSPDiagnostic(err))
	}

	if !p.HasErrors() {
		filePath := uriToPath(uri)
		baseDir := filepath.Dir(filePath)
		globalEnv := environment.NewEnvironment(baseDir, filePath, false)
		a := analyzer.New(globalEnv)
		a.Run(prog)

		for _, err := range a.DiagnosticErrors() {
			diagnostics = append(diagnostics, toLSPDiagnostic(err))
		}
	}

	if h.client != nil {
		_ = h.client.PublishDiagnostics(ctx, &lsp.PublishDiagnosticsParams{
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
	text, ok := h.docs.Text(params.TextDocument.URI)
	if !ok {
		return nil, nil
	}

	tknzr := lexer.New(text)
	p := parser.New(tknzr)
	prog := p.Parse()

	filePath := uriToPath(string(params.TextDocument.URI))
	baseDir := filepath.Dir(filePath)
	globalEnv := environment.NewEnvironment(baseDir, filePath, false)
	a := analyzer.New(globalEnv)
	a.Run(prog) // we run it even if there are syntax errors to try to get symbol info

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
	text, ok := h.docs.Text(params.TextDocument.URI)
	if !ok {
		return nil, nil
	}

	tknzr := lexer.New(text)
	p := parser.New(tknzr)
	prog := p.Parse()

	filePath := uriToPath(string(params.TextDocument.URI))
	baseDir := filepath.Dir(filePath)
	globalEnv := environment.NewEnvironment(baseDir, filePath, false)
	a := analyzer.New(globalEnv)
	a.Run(prog)

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
