package lsp

import (
	"strings"
	"testing"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

func TestTextDocumentHover(t *testing.T) {
	tests := []struct {
		name           string
		uri            string
		input          string
		queryLine      uint32
		queryChar      uint32
		expectedNil    bool
		expectedString string
	}{
		{
			name:           "Valid Variable",
			uri:            "file:///test_valid_var.caja",
			input:          "let multiplier = 5 \nmultiplier",
			queryLine:      1,
			queryChar:      3, // 'm' in multiplier
			expectedNil:    false,
			expectedString: "NUMBER",
		},
		{
			name:           "Unopened Document",
			uri:            "file:///test_unopened.caja",
			input:          "", // Won't be added to documents map
			queryLine:      0,
			queryChar:      0,
			expectedNil:    true,
		},
		{
			name:           "Whitespace",
			uri:            "file:///test_whitespace.caja",
			input:          "let x = 10\n    \nx",
			queryLine:      1,
			queryChar:      2, // Empty space
			expectedNil:    true,
		},
		{
			name:           "Complex Type (Function)",
			uri:            "file:///test_function.caja",
			input:          "let myFunc = fn(x: Number) -> Number { return x }\nmyFunc",
			queryLine:      1,
			queryChar:      2,
			expectedNil:    false,
			expectedString: "fn(NUMBER) -> NUMBER", // Function signature from analyzer
		},
		{
			name:           "Undeclared Variable",
			uri:            "file:///test_undeclared.caja",
			input:          "unknownVar",
			queryLine:      0,
			queryChar:      2,
			expectedNil:    false,
			expectedString: "ANY", // Semantic analyzer usually defaults to ANY_OBJ when it can't resolve
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.input != "" {
				documents[tt.uri] = tt.input
			}

			params := &protocol.HoverParams{
				TextDocumentPositionParams: protocol.TextDocumentPositionParams{
					TextDocument: protocol.TextDocumentIdentifier{URI: tt.uri},
					Position:     protocol.Position{Line: tt.queryLine, Character: tt.queryChar},
				},
			}

			hover, err := textDocumentHover(nil, params)
			if err != nil {
				t.Fatalf("textDocumentHover returned error: %v", err)
			}

			if tt.expectedNil {
				if hover != nil {
					t.Fatalf("Expected hover response to be nil, got %v", hover)
				}
				return
			}

			if hover == nil {
				t.Fatalf("Expected hover response, got nil")
			}

			markup, ok := hover.Contents.(protocol.MarkupContent)
			if !ok {
				t.Fatalf("Expected hover.Contents to be MarkupContent")
			}

			if !strings.Contains(markup.Value, tt.expectedString) {
				t.Errorf("Expected hover content to contain '%s', got %s", tt.expectedString, markup.Value)
			}
		})
	}
}

func TestTextDocumentDefinition(t *testing.T) {
	tests := []struct {
		name          string
		uri           string
		input         string
		queryLine     uint32
		queryChar     uint32
		expectedNil   bool
		expectedLine  uint32
	}{
		{
			name:          "Valid Variable Definition",
			uri:           "file:///test_def_valid.caja",
			input:         "let multiplier = 5 \nmultiplier",
			queryLine:     1,
			queryChar:     3,
			expectedNil:   false,
			expectedLine:  0, // The let statement is on line 0
		},
		{
			name:          "Built-in Function",
			uri:           "file:///test_def_builtin.caja",
			input:         "import math\nmath.abs(-10)",
			queryLine:     1,
			queryChar:     6, // 'abs'
			expectedNil:   true, // Built-ins have no physical definition line
		},
		{
			name:          "Function Call Definition",
			uri:           "file:///test_def_func.caja",
			input:         "let myFunc = fn() {}\nmyFunc()",
			queryLine:     1,
			queryChar:     3, // 'F' in myFunc
			expectedNil:   false,
			expectedLine:  0, // The let statement is defined on line 0
		},
		{
			name:          "Undeclared Variable",
			uri:           "file:///test_def_undeclared.caja",
			input:         "unknownVar",
			queryLine:     0,
			queryChar:     2,
			expectedNil:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.input != "" {
				documents[tt.uri] = tt.input
			}

			params := &protocol.DefinitionParams{
				TextDocumentPositionParams: protocol.TextDocumentPositionParams{
					TextDocument: protocol.TextDocumentIdentifier{URI: tt.uri},
					Position:     protocol.Position{Line: tt.queryLine, Character: tt.queryChar},
				},
			}

			res, err := textDocumentDefinition(nil, params)
			if err != nil {
				t.Fatalf("textDocumentDefinition returned error: %v", err)
			}

			if tt.expectedNil {
				if res != nil {
					t.Fatalf("Expected definition to be nil, got %v", res)
				}
				return
			}

			if res == nil {
				t.Fatalf("Expected definition response, got nil")
			}

			locations, ok := res.([]protocol.Location)
			if !ok || len(locations) == 0 {
				t.Fatalf("Expected a slice of locations with at least 1 element, got %v", res)
			}

			loc := locations[0]
			if loc.URI != tt.uri {
				t.Errorf("Expected URI %s, got %s", tt.uri, loc.URI)
			}

			if loc.Range.Start.Line != tt.expectedLine {
				t.Errorf("Expected definition on line %d, got %d", tt.expectedLine, loc.Range.Start.Line)
			}
		})
	}
}
