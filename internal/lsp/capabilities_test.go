package lsp

import (
	"testing"

	"github.com/owenrumney/go-lsp/lsp"
	"github.com/owenrumney/go-lsp/server"
)

// The server derives its advertised capabilities from the interfaces this handler
// satisfies. That is convenient but silent: renaming a handler method, or drifting its
// signature, un-registers the feature with no compile error and no failing test anywhere
// else — an editor would simply stop asking for it.
//
// These assertions make that failure loud and immediate. They are compile-time rather
// than a runtime test on purpose: the build breaks the moment a method stops matching,
// which is the earliest anything can notice.
var (
	_ server.LifecycleHandler        = (*CajaHandler)(nil)
	_ server.TextDocumentSyncHandler = (*CajaHandler)(nil)

	_ server.CompletionHandler    = (*CajaHandler)(nil)
	_ server.HoverHandler         = (*CajaHandler)(nil)
	_ server.SignatureHelpHandler = (*CajaHandler)(nil)
	_ server.DefinitionHandler    = (*CajaHandler)(nil)

	_ server.ReferencesHandler        = (*CajaHandler)(nil)
	_ server.DocumentHighlightHandler = (*CajaHandler)(nil)
	_ server.RenameHandler            = (*CajaHandler)(nil)
	_ server.PrepareRenameHandler     = (*CajaHandler)(nil)

	_ server.DocumentSymbolHandler = (*CajaHandler)(nil)
	_ server.FoldingRangeHandler   = (*CajaHandler)(nil)
	_ server.SelectionRangeHandler = (*CajaHandler)(nil)

	_ server.CodeActionHandler         = (*CajaHandler)(nil)
	_ server.SemanticTokensFullHandler = (*CajaHandler)(nil)

	_ server.TextDocumentSaveHandler      = (*CajaHandler)(nil)
	_ server.DidChangeWatchedFilesHandler = (*CajaHandler)(nil)
	_ server.WorkspaceSymbolHandler       = (*CajaHandler)(nil)
)

// TestInitializeDeclaresWhatCannotBeDerived covers the two capabilities the server has to
// state for itself, because no interface can express them.
func TestInitializeDeclaresWhatCannotBeDerived(t *testing.T) {
	h := NewCajaHandler()

	result, err := h.Initialize(t.Context(), &lsp.InitializeParams{})
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	// The semantic token legend: only the server knows which token types it emits, and
	// without the legend a client cannot decode a single token.
	provider := result.Capabilities.SemanticTokensProvider
	if provider == nil {
		t.Fatal("no semantic tokens capability advertised")
	}
	if len(provider.Legend.TokenTypes) != len(semanticTokenTypes) {
		t.Errorf("the legend advertises %d token types but the server emits %d",
			len(provider.Legend.TokenTypes), len(semanticTokenTypes))
	}
	if len(provider.Legend.TokenModifiers) != len(semanticTokenModifiers) {
		t.Errorf("the legend advertises %d modifiers but the server emits %d",
			len(provider.Legend.TokenModifiers), len(semanticTokenModifiers))
	}

	// Completion trigger characters.
	if result.Capabilities.CompletionProvider == nil {
		t.Fatal("no completion capability advertised")
	}
	if len(result.Capabilities.CompletionProvider.TriggerCharacters) == 0 {
		t.Error("completion advertises no trigger characters, so it would fire only on " +
			"explicit invocation")
	}

	if result.ServerInfo == nil || result.ServerInfo.Name == "" {
		t.Error("the server does not identify itself, which editors show in their logs")
	}
}

// TestSemanticLegendIndicesAreStable guards the wire format. Token types and modifiers
// travel as indices into the legend, so reordering either table silently reassigns every
// colour in every open file. Entries may be appended; they may not move.
func TestSemanticLegendIndicesAreStable(t *testing.T) {
	expectedTypes := map[int]string{
		tokNamespace:     "namespace",
		tokType:          "type",
		tokStruct:        "struct",
		tokEnum:          "enum",
		tokEnumMember:    "enumMember",
		tokInterface:     "interface",
		tokTypeParameter: "typeParameter",
		tokParameter:     "parameter",
		tokVariable:      "variable",
		tokProperty:      "property",
		tokFunction:      "function",
		tokNumber:        "number",
		tokString:        "string",
	}

	for index, want := range expectedTypes {
		if index >= len(semanticTokenTypes) {
			t.Errorf("token constant %d (%s) is past the end of the legend", index, want)
			continue
		}
		if got := semanticTokenTypes[index]; got != want {
			t.Errorf("legend index %d is %q, want %q — the constants and the legend have "+
				"drifted apart, so every token of this type is mis-coloured", index, got, want)
		}
	}

	expectedModifiers := map[int]string{
		0: "declaration",
		1: "readonly",
		2: "defaultLibrary",
		3: "async",
	}
	for index, want := range expectedModifiers {
		if got := semanticTokenModifiers[index]; got != want {
			t.Errorf("modifier bit %d is %q, want %q", index, got, want)
		}
	}
}
