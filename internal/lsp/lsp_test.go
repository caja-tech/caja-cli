package lsp

import (
	"strings"
	"os"
	"path/filepath"
	"testing"
	"context"

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
			expectedString: "Number",
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
			expectedString: "add(x: Number, y: Number) -> Number",
		},
		{
			name:           "Undeclared Variable",
			uri:            "file:///test_undeclared.caja",
			input:          "let x = 10\ny",
			queryLine:      1,
			queryChar:      0,
			expectedNil:    false,
			expectedString: "Any",
		},
		{
			name:           "Array push function hover",
			uri:            "file:///test_array_push_hover.caja",
			input:          `import array
let arr = [1, 2]
array.push(arr, 3)`,
			queryLine:      2,
			queryChar:      9,
			expectedNil:    false,
			expectedString: "push(arr: [T], item: T) -> [T]",
		},
		{
			name:           "cast.to function hover",
			uri:            "file:///test_cast_to_hover.caja",
			input:          `import cast
cast.to(123, 0)`,
			queryLine:      1,
			queryChar:      6,
			expectedNil:    false,
			expectedString: "to(value: T, fallback: R) -> R",
		},
		{
			name:           "map.KeyFunc function hover",
			uri:            "file:///test_map_keyfunc_hover.caja",
			input:          `import map
type MyStruct struct { key map.KeyFunc }
let s = MyStruct{ key: fn() -> String { return "a" } }
s.key()`,
			queryLine:      3,
			queryChar:      2,
			expectedNil:    false,
			expectedString: "KeyFunc() -> String",
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
	if hover.Contents.Markup == nil || !strings.Contains(hover.Contents.Markup.Value, "add(x: Number, y: Number) -> Number") {
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

func TestSignatureHelp(t *testing.T) {
	tests := []struct {
		name          string
		text          string
		line          int
		col           int
		setupFiles    map[string]string
		expectedLabel string
		expectedParam int
		expectNil     bool
	}{
		{
			name:          "Builtin function string.substring second parameter",
			text:          `let x = string.substring("hello", `,
			line:          0,
			col:           34,
			expectedLabel: "substring(str: String, start: Number, end: Number) -> String",
			expectedParam: 1,
		},
		{
			name: "Local function first parameter",
			text: `let myFunc = fn(a: Number, b: String) -> Nil {}
myFunc(`,
			line:          1,
			col:           7,
			expectedLabel: "myFunc(a: Number, b: String) -> Any",
			expectedParam: 0,
		},
		{
			name: "Nested function calls",
			text: `let add = fn(x: Number, y: Number) -> Number { return x + y }
let mult = fn(a: Number, b: Number) -> Number { return a * b }
mult(10, add(5, `,
			line:          2,
			col:           16,
			expectedLabel: "mult(a: Number, b: Number) -> Number",
			expectedParam: 1,
		},
		{
			name: "Module function call",
			setupFiles: map[string]string{
				"math_mod.caja": `let power = fn(base: Number, exp: Number) -> Number { return base }`,
			},
			text: `import "./math_mod"
math_mod.power(2, `,
			line:          1,
			col:           18,
			expectedLabel: "power(base: Number, exp: Number) -> Number",
			expectedParam: 1,
		},
		{
			name:          "Array push function",
			text:          "import array\nlet arr = [1, 2]\narray.push(arr, ",
			line:          2,
			col:           16,
			expectedLabel: "push(arr: [T], item: T) -> [T]",
			expectedParam: 1,
		},
		{
			name:          "cast.to function",
			text:          "import cast\ncast.to(123, ",
			line:          1,
			col:           13,
			expectedLabel: "to(value: T, fallback: R) -> R",
			expectedParam: 1,
		},
		{
			name:          "map.KeyFunc function",
			text:          "import map\ntype MyStruct struct { key map.KeyFunc }\nlet s = MyStruct{ key: fn() -> String { return \"a\" } }\ns.key(",
			line:          3,
			col:           6,
			expectedLabel: "KeyFunc() -> String",
			expectedParam: 0,
		},
		{
			name:      "Outside function call",
			text:      `let x = 10`,
			line:      0,
			col:       10,
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &CajaHandler{docs: document.NewStore()}

			tempDir := t.TempDir()
			for filename, content := range tt.setupFiles {
				path := filepath.Join(tempDir, filename)
				err := os.WriteFile(path, []byte(content), 0644)
				if err != nil {
					t.Fatalf("Failed to write mock module %s: %v", filename, err)
				}
			}

			uri := "file://" + filepath.Join(tempDir, "test.caja")
			err := h.DidOpen(context.Background(), &lsp.DidOpenTextDocumentParams{
				TextDocument: lsp.TextDocumentItem{
					URI: lsp.DocumentURI(uri),
					LanguageID: "caja",
					Version: 1,
					Text: tt.text,
				},
			})
			if err != nil {
				t.Fatalf("DidOpen returned error: %v", err)
			}

			params := &lsp.SignatureHelpParams{
				TextDocumentPositionParams: lsp.TextDocumentPositionParams{
					TextDocument: lsp.TextDocumentIdentifier{URI: lsp.DocumentURI(uri)},
					Position:     lsp.Position{Line: tt.line, Character: tt.col},
				},
			}

			res, err := h.SignatureHelp(context.Background(), params)
			if err != nil {
				t.Fatalf("SignatureHelp returned error: %v", err)
			}

			if tt.expectNil {
				if res != nil {
					t.Errorf("Expected nil SignatureHelp, got %+v", res)
				}
				return
			}

			if res == nil {
				t.Fatalf("Expected SignatureHelp, got nil")
			}

			if len(res.Signatures) == 0 {
				t.Fatalf("Expected signatures, got 0")
			}

			sig := res.Signatures[0]
			if sig.Label != tt.expectedLabel {
				t.Errorf("Expected label %q, got %q", tt.expectedLabel, sig.Label)
			}

			if res.ActiveParameter != nil && *res.ActiveParameter != tt.expectedParam {
				t.Errorf("Expected active parameter %d, got %d", tt.expectedParam, *res.ActiveParameter)
			}
		})
	}
}
