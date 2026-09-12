package script

import (
	"bytes"
	"caja-cli/internal/pipeline/analyzer"
	"caja-cli/internal/pipeline/ast"
	"caja-cli/internal/pipeline/compiler"
	"caja-cli/internal/pipeline/environment"
	"caja-cli/internal/pipeline/lexer"
	"caja-cli/internal/pipeline/parser"
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
		// An import is a local binding, never a re-export, so a module's public
		// surface never grows to the transitive closure of what it imports. The
		// REJECTION half of that rule lives in TestImportIsNotReexported, which
		// asserts each exact diagnostic; a case here could only assert "some
		// error", since ParseWithDir returns one generic error for all of them.
		// What only this table can cover is the escape hatch — a facade that
		// *declares* the name — surviving transpile + `go build` + run, not
		// just analysis.
		{
			// Value side: `let registerRoutes = router.registerRoutes` is a
			// real declaration of the facade's own, so it IS exported.
			name:        "Explicit Re-declaration Is Exported Test",
			file:        "test_explicit_reexport.caja",
			expectError: false,
			expectVal:   30.0,
		},
		{
			// The type-side escape hatch, end to end: alias_type_facade only
			// named-imports 'Point' (so Point itself is not re-exported), but
			// `type MyPoint Point` is a declaration of its own and therefore
			// IS exported - and must survive transpilation, not just analysis.
			name:        "Type Alias Of An Imported Type Is Exported Test",
			file:        "test_type_alias_reexport.caja",
			expectError: false,
			expectVal:   2.0,
		},
		{
			// Cutting alias re-export must not break the legitimate way
			// through a chain: chain_top -> chain_mid -> chain_leaf, each hop
			// calling only what the previous module actually declared.
			name:        "Declared Calls Still Traverse A Deep Chain",
			file:        "test_chain_declared_calls.caja",
			expectError: false,
			expectVal:   7.0,
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

// analyzeFixture runs lexer→parser→analyzer over the fixture at
// testsDir/file and returns the analyzer's diagnostics. It deliberately drives
// the stages directly instead of going through ParseWithDir, which only prints
// diagnostics as a side effect and returns a generic error — the tests below
// assert the exact message.
//
// A parser error is a fatal setup failure: these are semantic-diagnostic
// tests, so a syntactically broken fixture must never look like a passing
// negative case.
func analyzeFixture(t *testing.T, testsDir, file string) []ast.DiagnosticError {
	t.Helper()

	path := filepath.Join(testsDir, file)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read test file: %v", err)
	}

	p := parser.New(lexer.New(string(content)))
	program := p.Parse()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	a := analyzer.New(environment.NewEnvironment(testsDir, path, false))
	a.Run(program)

	return a.DiagnosticErrors()
}

// assertFirstDiagnostic checks diags against wantError: an empty wantError
// demands a clean analysis, otherwise the FIRST diagnostic must match exactly.
// Only the first is pinned because most rejections legitimately cascade a
// recovery-time follow-on error.
func assertFirstDiagnostic(t *testing.T, diags []ast.DiagnosticError, wantError string) {
	t.Helper()

	if wantError == "" {
		if len(diags) > 0 {
			t.Fatalf("expected no errors, got: %v", diags)
		}
		return
	}
	if len(diags) == 0 {
		t.Fatalf("expected error %q, got none", wantError)
	}
	if diags[0].Message != wantError {
		t.Errorf("expected error %q, got %q", wantError, diags[0].Message)
	}
}

// TestImportIsNotReexported pins the exact diagnostic behind each way an
// import can fail to be re-exported. TestModules cannot do this: ParseWithDir
// prints diagnostics as a side effect and returns a nil analyzer plus a generic
// "errors found while performing semantical analysis on input", so its own
// errorMsg column is never asserted and every negative case there passes on
// *any* error - including an unrelated typo in the fixture. Running the
// lexer/parser/analyzer directly is what makes these cases actually load-bearing.
//
// The cases deliberately cover every binding shape an import can take, because
// each is detected by a different mechanism: named imports and wildcard members
// via ScopeEntry.IsImport, the module alias via its *symbol.ModuleSymbol type
// (it goes through declare(), not declareImport(), so IsImport is false for
// it), and imported types via importedTypes (named) / wildcardTypes (wildcard),
// the two halves of isImportedType.
func TestImportIsNotReexported(t *testing.T) {
	testsDir := "tests"

	testCases := []struct {
		name string
		file string
		// wantError is the exact first diagnostic; empty means the file must
		// analyze cleanly.
		wantError string
		// wantErrorCount, when non-zero, additionally pins the TOTAL number of
		// diagnostics. Only set it where the absence of a follow-on error is
		// itself the point; most rejections legitimately cascade one
		// recovery-time error (e.g. "undeclared variable 'x'") after the
		// import failure, and pinning that everywhere would just make the
		// table brittle to unrelated changes in error recovery.
		wantErrorCount int
	}{
		{
			// Named import: reexport_index imports registerRoutes without
			// declaring it, so a consumer cannot pull it back out.
			name:      "named import is not re-exported",
			file:      "test_named_imports_reexport.caja",
			wantError: "semantic error: module 'reexport_index' has no exported member 'registerRoutes'",
		},
		{
			// Wildcard member: wildcard_facade wildcard-imported 'add'.
			name:      "wildcard member is not re-exported",
			file:      "test_wildcard_no_reexport.caja",
			wantError: "semantic error: module 'wildcard_facade' has no exported member 'add'",
		},
		{
			// A wildcard binds the exporter's export set, so a wildcard of a
			// facade cannot launder what that facade merely imported either.
			name:      "wildcard of a facade does not see the facade's imports",
			file:      "test_wildcard_of_facade_no_reexport.caja",
			wantError: "semantic error: undeclared variable 'registerRoutes'. Use 'let' to declare it.",
		},
		{
			// Module alias: transitive_two binds transitive_three as 't'.
			name:      "module alias is not re-exported",
			file:      "test_transitive_alias_no_reexport.caja",
			wantError: "semantic error: property 't' not found on module",
		},
		{
			// Named-imported type: type_facade imports Point.
			name:      "named imported type is not re-exported",
			file:      "test_type_no_reexport.caja",
			wantError: "semantic error: module 'type_facade' has no exported member 'Point'",
		},
		{
			// The wildcard half of isImportedType, which the named case above
			// does not reach: wildcard_type_facade got 'Point' via
			// `import * from "second_types"`, so the type lands in
			// wildcardTypes rather than importedTypes and is excluded by the
			// other branch.
			name:      "wildcard imported type is not re-exported",
			file:      "test_wildcard_type_no_reexport.caja",
			wantError: "semantic error: module 'wildcard_type_facade' has no exported member 'Point'",
		},
		{
			// One hop past chain_top, which aliases chain_mid as 'mid'. The
			// blocked alias sits in CALL position here, unlike the pure
			// property chain below, so this is the one case that reaches
			// analyzeCallExpression with a callee whose receiver failed to
			// resolve. wantErrorCount pins what that path must do: the
			// non-function branch is guarded on ANY_OBJ, so recovery stays
			// silent and no "cannot call a non-function" is piled on top of
			// the real diagnostic. Without the count this row would be a pure
			// duplicate of the two-hop one - same single message, same
			// position - since the error fires before the call is considered.
			name:           "blocked module alias in call position does not cascade",
			file:           "test_chain_alias_blocked.caja",
			wantError:      "semantic error: property 'mid' not found on module",
			wantErrorCount: 1,
		},
		{
			// Two hops. The diagnostic must still name 'mid', not 'leaf':
			// the chain is cut at the first hop, so the deeper reach never
			// even gets a chance to resolve.
			name:      "module alias chain is cut at the first hop, not the last",
			file:      "test_chain_alias_two_hops.caja",
			wantError: "semantic error: property 'mid' not found on module",
		},
		{
			// The importedTypes marker makes a named-imported type non-
			// exported, but it does NOT make the name re-declarable: unlike a
			// wildcard-imported type, an explicitly named-imported one is
			// still an explicit binding and collides. (This is the observable
			// consequence of checkNameAvailable consulting only wildcardTypes.)
			name:      "named imported type still collides with a local declaration",
			file:      "test_type_import_then_redeclare.caja",
			wantError: "semantic error: 'Point' is already declared as a type",
		},
		{
			// Escape hatch, value side: a facade that *declares* the name.
			name: "explicitly re-declared value is exported",
			file: "test_explicit_reexport.caja",
		},
		{
			// Escape hatch, type side: `type MyPoint Point` is a declaration
			// of the facade's own, so it is exported even though Point is not.
			name: "type alias of an imported type is exported",
			file: "test_type_alias_reexport.caja",
		},
		{
			// The rule must not break the legitimate path through a chain:
			// every hop calls what the previous module actually declared.
			name: "declared calls still traverse a deep chain",
			file: "test_chain_declared_calls.caja",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			diags := analyzeFixture(t, testsDir, tc.file)
			assertFirstDiagnostic(t, diags, tc.wantError)
			if tc.wantErrorCount != 0 && len(diags) != tc.wantErrorCount {
				t.Errorf("expected exactly %d diagnostic(s), got %d: %v", tc.wantErrorCount, len(diags), diags)
			}
		})
	}
}

// TestUFCSAcrossRealModules covers the parts of UFCS resolution that only a
// real, separately-analyzed .caja module on disk can exercise — the analyzer's
// own table-driven tests are single-file and in-memory, so they can only reach
// the builtin-module and same-file branches of ufcsCandidates.
//
// What is module-specific here: a module's *private* members must be invisible
// to UFCS exactly as they are to a qualified `mod.name` access (the visibility
// rule cannot be sidestepped by using the sugar), and two real modules can
// collide the same way a module and a local function already do.
func TestUFCSAcrossRealModules(t *testing.T) {
	testsDir := "tests"

	testCases := []struct {
		name string
		file string
		// wantError is the exact first diagnostic; empty means the file must
		// analyze cleanly.
		wantError string
	}{
		{
			// Positive control for the two rejections below: without it, a
			// broken fixture (wrong path, wrong type) would make them pass for
			// the wrong reason, since both expect the same generic fallthrough
			// error that an unresolvable receiver produces.
			name: "an exported module function resolves via UFCS",
			file: "test_ufcs_module_public.caja",
		},
		{
			// ufcsCandidates consults ModuleSymbol.IsPrivate before looking the
			// name up, so a private member is never a candidate and the call
			// falls through to the generic property-access error — the same
			// outcome as if the function did not exist at all, which is the
			// point: the sugar leaks no more than qualified access does.
			name:      "a private module function is not a UFCS candidate",
			file:      "test_ufcs_module_private.caja",
			wantError: "type error: property access not supported for Number",
		},
		{
			// Two real modules exporting the same name with the same first
			// parameter type. Both aliases are named in the message, ordered by
			// scope name, and the suggestion is the explicit qualified form for
			// each.
			name:      "two modules exporting the same function make the call ambiguous",
			file:      "test_ufcs_module_ambiguous.caja",
			wantError: "type error: ambiguous method call 'triple': matches 'ufcs_helpers' and 'ufcs_helpers_alt'. Suggestion: call it explicitly (ufcs_helpers.triple(...) or ufcs_helpers_alt.triple(...))",
		},
		{
			// The struct-receiver contest with MORE THAN ONE surviving UFCS
			// candidate, which no single-file test can build: two functions of
			// the same name need two files. The struct's own 'at' field drops
			// out on argument type, and the two remaining candidates are then
			// judged against each other — so the message must be the plain
			// two-function one, NOT the struct-specific "matches the property
			// 'at' on struct 'Tag'..." form, and the field must not appear in
			// it at all now that it has lost.
			name:      "two functions surviving a struct field contest report the plain ambiguity",
			file:      "test_ufcs_struct_ambiguous.caja",
			wantError: "type error: ambiguous method call 'at': matches the directly-callable function 'at' and 'ufcs_helpers'. Suggestion: call it explicitly (at(...) or ufcs_helpers.at(...))",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assertFirstDiagnostic(t, analyzeFixture(t, testsDir, tc.file), tc.wantError)
		})
	}
}
