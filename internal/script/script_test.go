package script

import (
	"bytes"
	"caja-cli/internal/pipeline/compiler"
	"go/format"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
		{
			// Regression test: reexport_index.caja named-imports
			// registerRoutes from reexport_router.caja without ever calling
			// it itself, purely to re-export it — a facade shape that used
			// to type-check but fail `go build` with
			// "undefined: reexport_index_registerRoutes", since nothing
			// declared that Go symbol (only the true origin,
			// reexport_router_registerRoutes, was ever emitted).
			name:        "Named Imports Re-export Chain Test",
			file:        "test_named_imports_reexport.caja",
			expectError: false,
			expectVal:   30.0,
		},
		{
			name:        "Named Imports Type/Union Test",
			file:        "test_named_imports_type.caja",
			expectError: false,
			expectVal:   7.0,
		},
		{
			name:        "Named Imports Private Type Rejected Test",
			file:        "test_named_imports_type_private.caja",
			expectError: true,
			errorMsg:    "semantic error: module 'second_types' has no exported member 'Hidden'",
		},
		{
			name:        "Wildcard Imports Builtin Test",
			file:        "test_wildcard_imports_builtin.caja",
			expectError: false,
			expectVal:   30.0,
		},
		{
			name:        "Wildcard Imports Custom Module Test",
			file:        "test_wildcard_imports_custom.caja",
			expectError: false,
			expectVal:   15.0,
		},
		{
			name:        "Wildcard Imports Types Test",
			file:        "test_wildcard_imports_types.caja",
			expectError: false,
			expectVal:   13.0,
		},
		{
			// Two wildcards sharing a name is legal; only reaching for the
			// colliding name bare is an error.
			name:        "Wildcard Ambiguous Name Used",
			file:        "test_wildcard_ambiguous.caja",
			expectError: true,
			errorMsg:    "ambiguous reference to 'len'",
		},
		{
			name:        "Wildcard Ambiguous Name Never Used",
			file:        "test_wildcard_ambiguous_unused.caja",
			expectError: false,
			expectVal:   3.0,
		},
		{
			// Wildcard-imported names are not re-exported, so a module that
			// only wildcard-imported 'add' cannot hand it on.
			name:        "Wildcard Imports Are Not Re-exported",
			file:        "test_wildcard_no_reexport.caja",
			expectError: true,
			errorMsg:    "semantic error: module 'wildcard_facade' has no exported member 'add'",
		},
		{
			name:        "Wildcard Import After A Declaration",
			file:        "test_wildcard_late.caja",
			expectError: true,
			errorMsg:    "import statements must appear at the beginning of the file",
		},
		{
			// The type side mirrors the value side: two wildcards exporting
			// the same type name only error where the bare name is used.
			name:        "Wildcard Ambiguous Type Used",
			file:        "test_wildcard_ambiguous_type.caja",
			expectError: true,
			errorMsg:    "ambiguous type 'Shape'",
		},
		{
			name:        "Wildcard Ambiguous Type Never Used",
			file:        "test_wildcard_ambiguous_type_unused.caja",
			expectError: false,
			expectVal:   33.0,
		},
		{
			// Confirms both a String-typed and a Number-typed "${...}"
			// segment interpolate correctly end to end (built, compiled,
			// and actually run) — and specifically that a whole number
			// formats as "5", not "5.0", via an exact string comparison
			// rather than just checking the code compiles.
			name:        "String Interpolation Test",
			file:        "test_string_interpolation.caja",
			expectError: false,
			expectVal:   1.0,
		},
		{
			// Confirms escapes, string.format, and their interaction with
			// interpolation (a nested string literal inside "${...}", and
			// an escaped "\${" alongside a real interpolation in the same
			// string) all produce the exact expected runtime values, not
			// just "it compiles" — 5 independent checks summed to 5.0.
			name:        "String Escapes And Format Test",
			file:        "test_string_format.caja",
			expectError: false,
			expectVal:   5.0,
		},
		{
			name:        "Trailing Block Call Syntax",
			file:        "test_trailing_block.caja",
			expectError: false,
			expectVal:   2.0,
		},
		{
			name:        "Custom DSL Rule Engine With Evaluator",
			file:        "test_dsl_rule_engine.caja",
			expectError: false,
			expectVal:   2.0,
		},
		{
			name:        "Custom DSL Workflow With Evaluator",
			file:        "test_dsl_workflow_evaluator.caja",
			expectError: false,
			expectVal:   3.0,
		},
		{
			name:        "Custom DSL Constrained Subtype Narrowing",
			file:        "test_dsl_constrained_subtype.caja",
			expectError: false,
			expectVal:   10.0,
		},
		{
			name:        "Custom DSL Constrained Subtype Non-Nullable Binding Rejected",
			file:        "test_dsl_constrained_subtype_errors.caja",
			expectError: true,
			errorMsg:    "type error: cannot assign State to NonEmptyState",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(testsDir, tc.file)
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("failed to read test file: %v", err)
			}

			prog, _, a, err := ParseWithDir(string(content), testsDir, path)
			if err != nil {
				if !tc.expectError {
					t.Fatalf("unexpected parsing error: %v", err)
				}
				return
			}

			goCode, err := compiler.Transpile(prog, a, compiler.TranspileOptions{PrintResult: true})
			if err != nil {
				if !tc.expectError {
					t.Fatalf("unexpected transpile error: %v", err)
				}
				return
			}
			if formatted, err := format.Source([]byte(goCode)); err == nil {
				goCode = string(formatted)
			} else {
				t.Fatalf("generated Go source failed to format (likely invalid): %v\n%s", err, goCode)
			}

			var stdout, stderr bytes.Buffer
			if err := compiler.Run(goCode, nil, nil, &stdout, &stderr); err != nil {
				if !tc.expectError {
					t.Fatalf("unexpected runtime error: %v\nstderr:\n%s", err, stderr.String())
				}
				return
			}

			if tc.expectError {
				t.Fatalf("expected error, but got none")
			}

			// Verify the printed return value for successful tests. Some
			// scripts print other output (log.info/warn/error, log.export)
			// before their final result, so only the LAST line is the
			// result — matching caja_print_result's own fmt.Println.
			lines := strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n")
			lastLine := lines[len(lines)-1]
			got, err := strconv.ParseFloat(strings.TrimSpace(lastLine), 64)
			if err != nil {
				t.Fatalf("expected numeric stdout on the last line, got %q: %v", stdout.String(), err)
			}
			if got != tc.expectVal {
				t.Errorf("expected value %v, got %v", tc.expectVal, got)
			}
		})
	}
}
