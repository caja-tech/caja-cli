package lsp

import (
	"caja-cli/internal/pipeline/lexer"
	"caja-cli/internal/pipeline/parser"
	"testing"
)

func TestFindNodeAtPosition(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		targetLine   int // 0-indexed (LSP)
		targetCol    int // 0-indexed (LSP)
		expectedType string // string representation of the expected node
	}{
		{
			name:         "Variable Declaration Name",
			input:        "let myVar = 100",
			targetLine:   0,
			targetCol:    5, // Points to 'm' in myVar
			expectedType: "myVar",
		},
		{
			name:         "Variable Usage",
			input:        "let myVar = 100\nmyVar + 50",
			targetLine:   1,
			targetCol:    2, // Points to 'V' in myVar on line 2
			expectedType: "myVar",
		},
		{
			name: "Struct Field Usage",
			input: `let user = {
	name: "Alice",
	age: 30
}`,
			targetLine:   1,
			targetCol:    3, // Points to 'm' in name
			expectedType: "name",
		},
		{
			name: "Struct Value Usage",
			input: `let user = {
	name: "Alice",
	age: 30
}`,
			targetLine:   1,
			targetCol:    9, // Points to 'A' in "Alice"
			expectedType: `"Alice"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tknzr := lexer.New(tt.input)
			p := parser.New(tknzr)
			prog := p.Parse()

			if len(p.Errors()) > 0 {
				t.Fatalf("Failed to parse test input: %v", p.Errors())
			}

			node := FindNodeAtPosition(prog, tt.targetLine, tt.targetCol)
			if node == nil {
				t.Fatalf("Expected to find a node at [%d:%d], got nil", tt.targetLine, tt.targetCol)
			}

			if node.String() != tt.expectedType {
				t.Errorf("Expected node '%s', got '%s'", tt.expectedType, node.String())
			}
		})
	}
}
