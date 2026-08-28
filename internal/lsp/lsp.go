package lsp

import (
	"caja-cli/internal/pipeline/analyzer"
	"caja-cli/internal/pipeline/ast"
	"caja-cli/internal/pipeline/environment"
	"caja-cli/internal/pipeline/lexer"
	"caja-cli/internal/pipeline/parser"
	"net/url"
	"path/filepath"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
	"github.com/tliron/glsp/server"
	"github.com/tliron/commonlog"
)

var (
	lsName    = "caja-lsp"
	documents = make(map[string]string)
	serverVer string // Stores the injected version
)

func Run(version string) error {
	serverVer = version
	commonlog.Configure(1, nil)
	
	handler := protocol.Handler{
		Initialize:             initialize,
		Initialized:            initialized,
		TextDocumentDidOpen:    textDocumentDidOpen,
		TextDocumentDidChange:  textDocumentDidChange,
		TextDocumentDidClose:   textDocumentDidClose,
		TextDocumentDidSave:    textDocumentDidSave,
		TextDocumentHover:      textDocumentHover,
		TextDocumentDefinition: textDocumentDefinition,
	}

	srv := server.NewServer(&handler, lsName, false)
	return srv.RunStdio()
}

func initialize(context *glsp.Context, params *protocol.InitializeParams) (any, error) {
	capabilities := protocol.ServerCapabilities{
		TextDocumentSync:   protocol.TextDocumentSyncKindFull,
		HoverProvider:      true,
		DefinitionProvider: true,
	}
	
	return protocol.InitializeResult{
		Capabilities: capabilities,
		ServerInfo: &protocol.InitializeResultServerInfo{
			Name:    lsName,
			Version: &serverVer,
		},
	}, nil
}

func initialized(context *glsp.Context, params *protocol.InitializedParams) error {
	return nil
}

func textDocumentDidOpen(context *glsp.Context, params *protocol.DidOpenTextDocumentParams) error {
	documents[params.TextDocument.URI] = params.TextDocument.Text
	validateDocument(context, params.TextDocument.URI, params.TextDocument.Text)
	return nil
}

func textDocumentDidChange(context *glsp.Context, params *protocol.DidChangeTextDocumentParams) error {
	text := params.ContentChanges[0].(protocol.TextDocumentContentChangeEvent).Text
	documents[params.TextDocument.URI] = text
	validateDocument(context, params.TextDocument.URI, text)
	return nil
}

func textDocumentDidSave(context *glsp.Context, params *protocol.DidSaveTextDocumentParams) error {
	return nil
}

func textDocumentDidClose(context *glsp.Context, params *protocol.DidCloseTextDocumentParams) error {
	delete(documents, params.TextDocument.URI)
	return nil
}

func validateDocument(context *glsp.Context, uri string, text string) {
	tknzr := lexer.New(text)
	p := parser.New(tknzr)
	prog := p.Parse()

	var diagnostics []protocol.Diagnostic

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

	go context.Notify(protocol.ServerTextDocumentPublishDiagnostics, protocol.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: diagnostics,
	})
}

func toLSPDiagnostic(err ast.DiagnosticError) protocol.Diagnostic {
	line := protocol.UInteger(err.Token.Line)
	if line > 0 {
		line-- // LSP lines are 0-indexed
	}
	col := protocol.UInteger(err.Token.Column)
	if col > 0 {
		col-- // LSP cols are 0-indexed
	}

	length := protocol.UInteger(len(err.Token.Literal))
	if length == 0 {
		length = 1
	}

	severity := protocol.DiagnosticSeverityError

	return protocol.Diagnostic{
		Range: protocol.Range{
			Start: protocol.Position{Line: line, Character: col},
			End:   protocol.Position{Line: line, Character: col + length},
		},
		Severity: &severity,
		Source:   &lsName,
		Message:  err.Message,
	}
}

func textDocumentHover(context *glsp.Context, params *protocol.HoverParams) (*protocol.Hover, error) {
	text, ok := documents[params.TextDocument.URI]
	if !ok {
		return nil, nil
	}

	tknzr := lexer.New(text)
	p := parser.New(tknzr)
	prog := p.Parse()

	filePath := uriToPath(params.TextDocument.URI)
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
	
	return &protocol.Hover{
		Contents: protocol.MarkupContent{
			Kind:  protocol.MarkupKindMarkdown,
			Value: markdown,
		},
	}, nil
}

func textDocumentDefinition(context *glsp.Context, params *protocol.DefinitionParams) (any, error) {
	text, ok := documents[params.TextDocument.URI]
	if !ok {
		return nil, nil
	}

	tknzr := lexer.New(text)
	p := parser.New(tknzr)
	prog := p.Parse()

	filePath := uriToPath(params.TextDocument.URI)
	baseDir := filepath.Dir(filePath)
	globalEnv := environment.NewEnvironment(baseDir, filePath, false)
	a := analyzer.New(globalEnv)
	a.Run(prog)

	node := FindNodeAtPosition(prog, params.Position.Line, params.Position.Character)
	if node == nil {
		return nil, nil
	}

	defToken, ok := a.GetDefinition(node)
	if !ok || defToken.Line == 0 {
		return nil, nil
	}

	line := protocol.UInteger(defToken.Line - 1)
	col := protocol.UInteger(defToken.Column)
	length := protocol.UInteger(len(defToken.Literal))
	if length == 0 {
		length = 1
	}

	return []protocol.Location{
		{
			URI: params.TextDocument.URI,
			Range: protocol.Range{
				Start: protocol.Position{Line: line, Character: col},
				End:   protocol.Position{Line: line, Character: col + length},
			},
		},
	}, nil
}

func uriToPath(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return ""
	}
	return u.Path
}
