package script

import (
	"caja-cli/internal/pipeline/environment"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestModules(t *testing.T) {
	// Paths are relative to this test file.
	testsDir := "tests"

	testCases := []struct {
		name        string
		file        string
		expectError bool
		errorMsg    string
		expectVal   float64
	}{
		{
			name:        "Valid Module Import",
			file:        "main.caja",
			expectError: false,
			expectVal:   3,
		},
		{
			name:        "Circular Dependency",
			file:        "a.caja",
			expectError: true,
			errorMsg:    "circular import detected",
		},
		{
			name:        "Late Import",
			file:        "test_import_late.caja",
			expectError: true,
			errorMsg:    "import statements must appear at the beginning of the file",
		},
		{
			name:        "Block Import",
			file:        "test_import_block.caja",
			expectError: true,
			errorMsg:    "import statements are only allowed at the top-level of a file",
		},
		{
			name:        "Subfolder Module Import",
			file:        "main_subfolder.caja",
			expectError: false,
			expectVal:   15,
		},
		{
			name:        "Module Alias Import",
			file:        "test_import_alias.caja",
			expectError: false,
			expectVal:   15,
		},
		{
			name:        "Nullable Struct Navigation",
			file:        "test_nullable_struct_main.caja",
			expectError: false,
			expectVal:   100,
		},
		{
			name:        "Let Module Import Reassign",
			file:        "let_import_reassign.caja",
			expectError: true,
			errorMsg:    "cannot mutate property/index of constant variable 'let_module'",
		},
		{
			name:        "Type Alias Simple",
			file:        "type_alias_simple.caja",
			expectError: false,
			expectVal:   15,
		},
		{
			name:        "Const Array Mutation",
			file:        "const_array_mutation.caja",
			expectError: true,
			errorMsg:    "cannot mutate property/index of constant variable 'test_const_array'",
		},
		{
			name:        "Const Module Import Reassign",
			file:        "const_import_reassign.caja",
			expectError: true,
			errorMsg:    "semantic error: cannot assign to constant property 'val'",
		},
		{
			name:        "Transitive Module Alias Import",
			file:        "transitive_one.caja",
			expectError: false,
			expectVal:   160,
		},
		{
			name:        "Empty Module Import",
			file:        "test_import_empty.caja",
			expectError: true,
			errorMsg:    "failed to import ''",
		},
		{
			name:        "Nonexistent Module Import",
			file:        "test_import_nonexistent.caja",
			expectError: true,
			errorMsg:    "failed to import 'does_not_exist'",
		},
		{
			name:        "Struct Module Access",
			file:        "test_struct_access.caja",
			expectError: false,
			expectVal:   10,
		},
		{
			name:        "Private Struct Import",
			file:        "test_private_struct.caja",
			expectError: true,
			errorMsg:    "semantic error: undefined struct 'sm.Secret'",
		},
		{
			name:        "Const Struct Property Reassign",
			file:        "test_const_struct_prop.caja",
			expectError: true,
			errorMsg:    "semantic error: cannot assign to constant property 'id' on struct 'sm.User'",
		},
		{
			name:        "Folder Import",
			file:        "test_import_folder.caja",
			expectError: true,
			errorMsg:    "failed to import 'utils'",
		},
		{
			name:        "Math Module Tests",
			file:        "test_math.caja",
			expectError: false,
			expectVal:   48.5,
		},
		{
			name:        "Math Constants Tests",
			file:        "test_math_constants.caja",
			expectError: false,
			expectVal:   math.Pi,
		},
		{
			name:        "Log Module Tests",
			file:        "test_log.caja",
			expectError: false,
			expectVal:   1.0,
		},
		{
			name:        "Log Export Tests",
			file:        "test_log_export.caja",
			expectError: false,
			expectVal:   1.0,
		},
		{
			name:        "Valid Private Import",
			file:        "test_private_valid.caja",
			expectError: false,
			expectVal:   10.0,
		},
		{
			name:        "Invalid Private Import",
			file:        "test_private_invalid.caja",
			expectError: true,
			errorMsg:    "property 'secret' is private and cannot be accessed from outside module 'test_private_export'",
		},
		{
			name:        "Map Dictionary Test",
			file:        "test_map.caja",
			expectError: false,
			expectVal:   1.0,
		},
		{
			name:        "Map Dictionary Struct Test",
			file:        "test_map_struct.caja",
			expectError: false,
			expectVal:   100.0,
		},
		{
			name:        "Map Module Import Test",
			file:        "test_map_module_import.caja",
			expectError: false,
			expectVal:   200.0,
		},
		{
			name:        "Map Struct Closure Test",
			file:        "test_map_struct_closure.caja",
			expectError: false,
			expectVal:   10.0,
		},
		{
			name:        "Node Modules Test",
			file:        "test_node_modules.caja",
			expectError: false,
			expectVal:   220.0,
		},
		{
			name:        "Named Imports Builtin Test",
			file:        "test_named_imports_builtin.caja",
			expectError: false,
			expectVal:   20.0,
		},
		{
			name:        "Named Imports Custom Test",
			file:        "test_named_imports_custom.caja",
			expectError: false,
			expectVal:   30.0,
		},
		{
			name:        "Named Imports Node Test",
			file:        "test_named_imports_node.caja",
			expectError: false,
			expectVal:   220.0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(testsDir, tc.file)
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("failed to read test file: %v", err)
			}

			prog, env, _, err := ParseWithDir(string(content), testsDir, path)
			if err != nil {
				if !tc.expectError {
					t.Fatalf("unexpected parsing error: %v", err)
				}
				return
			}

			val, err := Run(prog, env)
			if err != nil {
				if !tc.expectError {
					t.Fatalf("unexpected runtime error: %v", err)
				}
				return
			}

			if tc.expectError {
				t.Fatalf("expected error, but got none")
			}

			// Verify return value for successful tests
			if num, ok := val.(*environment.Number); ok {
				if num.Value != tc.expectVal {
					t.Errorf("expected value %v, got %v", tc.expectVal, num.Value)
				}
			} else {
				t.Errorf("expected return value to be a Number, got %T", val)
			}
		})
	}
}
