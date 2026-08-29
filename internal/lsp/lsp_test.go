package lsp

import (
	"strings"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/owenrumney/go-lsp/document"
	"github.com/owenrumney/go-lsp/lsp"
	"github.com/owenrumney/go-lsp/servertest"
)

func TestTextDocumentHover(t *testing.T) {
	tests := []struct {
		name           string
		uri            string
		input          string
		queryLine      int
		queryChar      int
		expectedNil    bool
		expectedString string
	}{
		{
			name:           "Valid Variable",
			uri:            "file:///test_valid_var.caja",
			input:          "let multiplier = 5 \nmultiplier",
			queryLine:      1,
			queryChar:      3,
			expectedNil:    false,
			expectedString: "NUMBER",
		},
		{
			name:           "Unopened Document",
			uri:            "file:///test_unopened.caja",
			input:          "",
			queryLine:      0,
			queryChar:      0,
			expectedNil:    true,
		},
		{
			name:           "Whitespace",
			uri:            "file:///test_whitespace.caja",
			input:          "let x = 10\n    \nx",
			queryLine:      1,
			queryChar:      2,
			expectedNil:    true,
		},
		{
			name:           "Complex Type (Function)",
			uri:            "file:///test_complex_type.caja",
			input:          "let add = fn(x: Number, y: Number) -> Number { return x + y }\nadd(1, 2)",
			queryLine:      1,
			queryChar:      1,
			expectedNil:    false,
			expectedString: "fn(NUMBER, NUMBER) -> NUMBER",
		},
		{
			name:           "Undeclared Variable",
			uri:            "file:///test_undeclared.caja",
			input:          "let x = 10\ny",
			queryLine:      1,
			queryChar:      0,
			expectedNil:    false,
			expectedString: "ANY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &CajaHandler{docs: document.NewStore()}
			s := servertest.New(t, h)

			if tt.input != "" {
				s.DidOpen(lsp.DocumentURI(tt.uri), "caja", tt.input)
			}

			hover, err := s.Hover(lsp.DocumentURI(tt.uri), tt.queryLine, tt.queryChar)
			if err != nil {
				t.Fatalf("Hover returned error: %v", err)
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

			if hover.Contents.Markup == nil {
				t.Fatalf("Expected hover.Contents.Markup to be non-nil")
			}

			if !strings.Contains(hover.Contents.Markup.Value, tt.expectedString) {
				t.Errorf("Expected hover content to contain '%s', got %s", tt.expectedString, hover.Contents.Markup.Value)
			}
		})
	}
}

func TestTextDocumentDefinition(t *testing.T) {
	tests := []struct {
		name         string
		uri          string
		input        string
		queryLine    int
		queryChar    int
		expectedNil  bool
		expectedLine int
	}{
		{
			name:         "Valid Variable Definition",
			uri:          "file:///test_def_valid.caja",
			input:        "let multiplier = 5 \nmultiplier",
			queryLine:    1,
			queryChar:    3,
			expectedNil:  false,
			expectedLine: 0,
		},
		{
			name:         "Built-in Function",
			uri:          "file:///test_def_builtin.caja",
			input:        "std.math.abs(-5)",
			queryLine:    0,
			queryChar:    9,
			expectedNil:  true,
		},
		{
			name:         "Function Call Definition",
			uri:          "file:///test_def_fn.caja",
			input:        "let add = fn(x: Number) -> Number { return x }\nadd(5)",
			queryLine:    1,
			queryChar:    1,
			expectedNil:  false,
			expectedLine: 0,
		},
		{
			name:         "Undeclared Variable",
			uri:          "file:///test_def_undeclared.caja",
			input:        "undefined_var",
			queryLine:    0,
			queryChar:    0,
			expectedNil:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &CajaHandler{docs: document.NewStore()}
			s := servertest.New(t, h)

			if tt.input != "" {
				s.DidOpen(lsp.DocumentURI(tt.uri), "caja", tt.input)
			}

			locs, err := s.Definition(lsp.DocumentURI(tt.uri), tt.queryLine, tt.queryChar)
			if err != nil {
				t.Fatalf("Definition returned error: %v", err)
			}

			if tt.expectedNil {
				if locs != nil {
					t.Fatalf("Expected definition response to be nil, got %v", locs)
				}
				return
			}

			if len(locs) == 0 {
				t.Fatalf("Expected definition response, got nil")
			}

			loc := locs[0]

			if loc.URI != lsp.DocumentURI(tt.uri) {
				t.Errorf("Expected URI %s, got %s", tt.uri, loc.URI)
			}

			if loc.Range.Start.Line != tt.expectedLine {
				t.Errorf("Expected definition to start at line %d, got %d", tt.expectedLine, loc.Range.Start.Line)
			}

			if loc.Range.Start.Character != 4 {
				t.Errorf("Expected definition to start at col 4, got %d", loc.Range.Start.Character)
			}
		})
	}
}

func TestModuleLSPFeatures(t *testing.T) {
	h := &CajaHandler{docs: document.NewStore()}
	s := servertest.New(t, h)

	// Create a temporary directory for our physical workspace
	tempDir := t.TempDir()

	modulePath := filepath.Join(tempDir, "calculus.caja")
	moduleURI := "file://" + modulePath
	moduleText := `let add = fn(x: Number, y: Number) -> Number { return x + y }`
	
	// Physically write the file because modules.Load reads from disk
	err := os.WriteFile(modulePath, []byte(moduleText), 0644)
	if err != nil {
		t.Fatalf("Failed to write mock module to disk: %v", err)
	}

	// Tell LSP client we opened it
	s.DidOpen(lsp.DocumentURI(moduleURI), "caja", moduleText)

	mainPath := filepath.Join(tempDir, "main.caja")
	mainURI := "file://" + mainPath
	mainText := `import "./calculus"
calculus.add(1, 2)`
	s.DidOpen(lsp.DocumentURI(mainURI), "caja", mainText)

	// Test Hover on 'add'
	hover, err := s.Hover(lsp.DocumentURI(mainURI), 1, 10) // 'a' in 'add'
	if err != nil {
		t.Fatalf("Hover returned error: %v", err)
	}
	if hover == nil {
		t.Fatalf("Expected hover response, got nil")
	}
	if hover.Contents.Markup == nil || !strings.Contains(hover.Contents.Markup.Value, "fn(NUMBER, NUMBER) -> NUMBER") {
		t.Errorf("Expected hover content to contain 'fn(NUMBER, NUMBER) -> NUMBER', got %v", hover)
	}

	// Test Go to Definition on 'add'
	locs, err := s.Definition(lsp.DocumentURI(mainURI), 1, 10)
	if err != nil {
		t.Fatalf("Definition returned error: %v", err)
	}
	if len(locs) == 0 {
		t.Fatalf("Expected definition response, got none")
	}
	
	if string(locs[0].URI) != moduleURI {
		t.Errorf("Expected definition URI %s, got %s", moduleURI, locs[0].URI)
	}
	if locs[0].Range.Start.Line != 0 {
		t.Errorf("Expected definition line 0, got %d", locs[0].Range.Start.Line)
	}
}

func TestDiagnosticsAreClearedOnFix(t *testing.T) {
	h := &CajaHandler{docs: document.NewStore()}
	s := servertest.New(t, h)

	mainURI := lsp.DocumentURI("file:///main.caja")
	
	// 1. Open document with an error
	badText := `let x = 5 +` // Missing right operand
	s.DidOpen(mainURI, "caja", badText)
	time.Sleep(100 * time.Millisecond)

	// 2. Fetch diagnostics, there should be an error
	diags := s.Diagnostics(mainURI)
	if len(diags) == 0 {
		t.Fatalf("Expected diagnostics for missing argument, got none")
	}

	// 3. Fix the error
	goodText := `let x = 5 + 5`
	s.DidChange(mainURI, 2, goodText)
	time.Sleep(100 * time.Millisecond)

	// 4. Fetch diagnostics, it should be EMPTY
	diags = s.Diagnostics(mainURI)
	if len(diags) > 0 {
		t.Fatalf("Expected diagnostics to be cleared after fix, but got %v", diags)
	}
}

func TestMultipleStatementsOnSameLine(t *testing.T) {
	h := &CajaHandler{docs: document.NewStore()}
	s := servertest.New(t, h)

	mainURI := lsp.DocumentURI("file:///main.caja")
	
	// Open document with missing operator, which creates two statements on the same line
	badText := `let area = calculus.PI 10 * 10`
	s.DidOpen(mainURI, "caja", badText)
	time.Sleep(100 * time.Millisecond) // wait for async open

	diags := s.Diagnostics(mainURI)
	if len(diags) == 0 {
		t.Fatalf("Expected syntax error for missing operator, got none")
	}

	foundMissingNewlineErr := false
	for _, diag := range diags {
		if strings.Contains(diag.Message, "newline between statements") {
			foundMissingNewlineErr = true
			break
		}
	}

	if !foundMissingNewlineErr {
		t.Fatalf("Expected 'newline between statements' error, got %v", diags)
	}
}
