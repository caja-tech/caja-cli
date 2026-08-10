package script

import (
	"caja-cli/internal/environment"
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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(testsDir, tc.file)
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("failed to read test file: %v", err)
			}

			prog, env, err := ParseWithDir(string(content), testsDir, path)
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
