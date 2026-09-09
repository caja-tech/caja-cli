package compiler_test

import (
	"bytes"
	"caja-cli/internal/pipeline/compiler"
	"caja-cli/internal/script"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSamplesCompilation(t *testing.T) {
	samplesDir := "samples"
	
	entries, err := os.ReadDir(samplesDir)
	if err != nil {
		t.Fatalf("failed to read samples directory: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirName := entry.Name()
		t.Run(dirName, func(t *testing.T) {
			filePath := filepath.Join(samplesDir, dirName, dirName+".caja")
			
			sourceCode, err := os.ReadFile(filePath)
			if err != nil {
				t.Fatalf("failed to read file '%s': %v", filePath, err)
			}

			baseDir, _ := filepath.Abs(filepath.Dir(filePath))

			// Parse the script to get the AST
			program, _, a, err := script.ParseWithDir(string(sourceCode), baseDir, filePath)
			if err != nil {
				t.Fatalf("failed to parse script: %v", err)
			}

			// Transpile to Go source
			goCode, err := compiler.Transpile(program, a, compiler.TranspileOptions{})
			if err != nil {
				t.Fatalf("transpilation failed: %v", err)
			}

			outBin := filepath.Join(baseDir, dirName)
			
			// Test compilation
			err = compiler.Compile(goCode, outBin, compiler.CompileOptions{})
			if err != nil {
				t.Fatalf("compilation failed: %v", err)
			}
			
			// Verify binary exists
			if _, err := os.Stat(outBin); os.IsNotExist(err) {
				t.Fatalf("expected binary %s to be generated, but it was not", outBin)
			}
			
			// Clean up binary
			os.Remove(outBin)
		})
	}
}

// TestRunReportsCajaSourceLocation is an end-to-end check that a runtime
// panic in a `caja run`-executed program is reported against the original
// .caja file/line (via the `//line` directives Transpile emits, honored by
// the Go compiler even through `go run`), instead of a temp Go file deleted
// before the user could ever see it.
func TestRunReportsCajaSourceLocation(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "panic.caja")
	source := "let arr = [1, 2, 3]\narr[10]\n"
	if err := os.WriteFile(filePath, []byte(source), 0644); err != nil {
		t.Fatalf("failed to write test script: %v", err)
	}

	program, _, a, err := script.ParseWithDir(source, dir, filePath)
	if err != nil {
		t.Fatalf("failed to parse script: %v", err)
	}

	goCode, err := compiler.Transpile(program, a, compiler.TranspileOptions{PrintResult: true})
	if err != nil {
		t.Fatalf("transpilation failed: %v", err)
	}
	if formatted, err := format.Source([]byte(goCode)); err == nil {
		goCode = string(formatted)
	}

	var stderr bytes.Buffer
	err = compiler.Run(goCode, nil, nil, &bytes.Buffer{}, &stderr)
	if err == nil {
		t.Fatalf("expected the out-of-range access to fail, but Run succeeded; stderr:\n%s", stderr.String())
	}

	wantLocation := fmt.Sprintf("%s:2", filePath)
	if !bytes.Contains(stderr.Bytes(), []byte(wantLocation)) {
		t.Errorf("expected stderr to report the panic at %q, got:\n%s", wantLocation, stderr.String())
	}
}

// assertCleanAsyncPanicReport is the shared assertion for the two tests
// below: a panic inside a spawned goroutine (stream-pipe stage, join call,
// or async task) must produce caja run's normal clean "error: ..." message,
// never Go's raw "panic: ...\ngoroutine N [running]:" crash dump — which is
// what escaping goroutines produced before caja_report_async_panic/
// caja_check_async_panic existed, since Go's panic/recover only propagates
// within the same goroutine and main()'s own recover can't see it.
func assertCleanAsyncPanicReport(t *testing.T, stderr string) {
	t.Helper()
	if !strings.Contains(stderr, "error:") {
		t.Errorf("expected a clean \"error: ...\" message, got:\n%s", stderr)
	}
	if strings.Contains(stderr, "goroutine") || strings.Contains(stderr, "panic:") {
		t.Errorf("expected no raw Go panic/goroutine crash dump, got:\n%s", stderr)
	}
}

// TestRunRecoversPanicInStreamPipeStage checks that a panic inside a `|>>`
// stage's spawned goroutine is caught by that goroutine's own recover
// (caja_report_async_panic) and surfaces as caja run's normal clean error
// message via caja_check_async_panic after the pipe's collection loop,
// instead of crashing the whole process with a raw goroutine panic dump.
func TestRunRecoversPanicInStreamPipeStage(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "pipe_panic.caja")
	source := "let boom = fn(x: Number) -> Number {\n" +
		"\tlet arr = [1, 2, 3]\n" +
		"\treturn arr[10]\n" +
		"}\n" +
		"let items = [1, 2, 3]\n" +
		"let result = items |>> boom\n" +
		"return 0\n"
	if err := os.WriteFile(filePath, []byte(source), 0644); err != nil {
		t.Fatalf("failed to write test script: %v", err)
	}

	program, _, a, err := script.ParseWithDir(source, dir, filePath)
	if err != nil {
		t.Fatalf("failed to parse script: %v", err)
	}

	goCode, err := compiler.Transpile(program, a, compiler.TranspileOptions{})
	if err != nil {
		t.Fatalf("transpilation failed: %v", err)
	}
	if formatted, err := format.Source([]byte(goCode)); err == nil {
		goCode = string(formatted)
	}

	var stderr bytes.Buffer
	err = compiler.Run(goCode, nil, nil, &bytes.Buffer{}, &stderr)
	if err == nil {
		t.Fatalf("expected the out-of-range access inside the pipe stage to fail, but Run succeeded; stderr:\n%s", stderr.String())
	}
	assertCleanAsyncPanicReport(t, stderr.String())
}

// TestRunRecoversPanicInAsyncTask is the async/unwrap counterpart: a panic
// inside an `async` task's goroutine must be caught there, surfaced cleanly
// by caja_check_async_panic right after `unwrap`'s <-task.done wait — and
// must report the *original* panic, not a masked "interface conversion"
// panic from type-asserting task.val while it's still its zero value.
func TestRunRecoversPanicInAsyncTask(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "async_panic.caja")
	source := "let boom = fn(x: Number) -> Number {\n" +
		"\tlet arr = [1, 2, 3]\n" +
		"\treturn arr[10]\n" +
		"}\n" +
		"let pending = async boom(1)\n" +
		"let val = unwrap pending\n" +
		"return 0\n"
	if err := os.WriteFile(filePath, []byte(source), 0644); err != nil {
		t.Fatalf("failed to write test script: %v", err)
	}

	program, _, a, err := script.ParseWithDir(source, dir, filePath)
	if err != nil {
		t.Fatalf("failed to parse script: %v", err)
	}

	goCode, err := compiler.Transpile(program, a, compiler.TranspileOptions{})
	if err != nil {
		t.Fatalf("transpilation failed: %v", err)
	}
	if formatted, err := format.Source([]byte(goCode)); err == nil {
		goCode = string(formatted)
	}

	var stderr bytes.Buffer
	err = compiler.Run(goCode, nil, nil, &bytes.Buffer{}, &stderr)
	if err == nil {
		t.Fatalf("expected the out-of-range access inside the async task to fail, but Run succeeded; stderr:\n%s", stderr.String())
	}
	assertCleanAsyncPanicReport(t, stderr.String())

	if strings.Contains(stderr.String(), "interface conversion") {
		t.Errorf("expected the original panic to be reported, not a masked type-assertion failure from a zero-value task.val; got:\n%s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "index out of range") {
		t.Errorf("expected the original index-out-of-range panic message to be reported, got:\n%s", stderr.String())
	}
}

// TestCompileCrossCompiles picks a target OS that's never the test-running
// host, compiles a trivial script for it via CompileOptions, and checks the
// resulting file's magic bytes match that target's executable format. This
// is a stronger check than just asserting the file exists — that alone
// wouldn't catch GOOS/GOARCH being silently ignored and a host binary
// produced instead.
func TestCompileCrossCompiles(t *testing.T) {
	targetOS := "linux"
	if runtime.GOOS == "linux" {
		targetOS = "darwin"
	}
	targetArch := "arm64"

	dir := t.TempDir()
	filePath := filepath.Join(dir, "trivial.caja")
	source := "let x = 1\n"
	if err := os.WriteFile(filePath, []byte(source), 0644); err != nil {
		t.Fatalf("failed to write test script: %v", err)
	}

	program, _, a, err := script.ParseWithDir(source, dir, filePath)
	if err != nil {
		t.Fatalf("failed to parse script: %v", err)
	}
	goCode, err := compiler.Transpile(program, a, compiler.TranspileOptions{})
	if err != nil {
		t.Fatalf("transpilation failed: %v", err)
	}

	outBin := filepath.Join(dir, "trivial-"+targetOS+"-"+targetArch)
	if err := compiler.Compile(goCode, outBin, compiler.CompileOptions{GOOS: targetOS, GOARCH: targetArch}); err != nil {
		t.Fatalf("cross-compilation to %s/%s failed: %v", targetOS, targetArch, err)
	}

	data, err := os.ReadFile(outBin)
	if err != nil {
		t.Fatalf("failed to read compiled binary: %v", err)
	}
	if len(data) < 4 {
		t.Fatalf("compiled binary is too short to inspect: %d bytes", len(data))
	}

	var wantMagic []byte
	switch targetOS {
	case "linux":
		wantMagic = []byte{0x7f, 'E', 'L', 'F'} // ELF
	case "darwin":
		wantMagic = []byte{0xcf, 0xfa, 0xed, 0xfe} // Mach-O 64-bit (MH_MAGIC_64, little-endian)
	}
	if !bytes.Equal(data[:4], wantMagic) {
		t.Errorf("expected %s/%s binary to start with magic bytes %x, got %x — GOOS/GOARCH may have been ignored", targetOS, targetArch, wantMagic, data[:4])
	}
}

// TestUsesBrowserModule checks the substring-based detection
// compiler.UsesBrowserModule uses to let cmd/cli auto-select GOOS=js/
// GOARCH=wasm — it must key off the actual generated `"syscall/js"` import
// line, not merely the presence of the word "browser" somewhere in the
// source (e.g. in a comment or an unrelated string literal).
func TestUsesBrowserModule(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   bool
	}{
		{
			name:   "generated syscall/js import is detected",
			source: "package main\n\nimport \"syscall/js\"\n\nfunc main() {\n\t_ = js.Global()\n}\n",
			want:   true,
		},
		{
			name:   "plain program without syscall/js is not flagged",
			source: "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n",
			want:   false,
		},
		{
			name:   "mentioning 'browser' in a comment or string is not enough",
			source: "package main\n\n// this talks about the browser module\nfunc main() {\n\t_ = \"browser\"\n}\n",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := compiler.UsesBrowserModule(tt.source); got != tt.want {
				t.Errorf("UsesBrowserModule(%q) = %v, want %v", tt.source, got, tt.want)
			}
		})
	}
}

// TestCompileBrowserModuleForWasm is the end-to-end check that a program
// using the browser module doesn't just transpile to text that *looks*
// right (transpiler_test.go's "Browser builtins" case), but actually
// compiles for real under GOOS=js/GOARCH=wasm — the one target the
// generated `syscall/js` calls (js.Global, .Get, .Call, .Set) can build
// under at all. This is what cmd/cli's browser-import auto-defaulting
// (targetOS/targetArch -> "js"/"wasm") exists to make possible.
func TestCompileBrowserModuleForWasm(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "browser.caja")
	source := "import browser\n" +
		"let el = browser.getElementById(\"app\")\n" +
		"browser.setText(el, \"hi\")\n" +
		"browser.setHTML(el, \"<b>hi</b>\")\n" +
		"browser.log(\"hello\")\n" +
		"browser.alert(\"hi\")\n"
	if err := os.WriteFile(filePath, []byte(source), 0644); err != nil {
		t.Fatalf("failed to write test script: %v", err)
	}

	program, _, a, err := script.ParseWithDir(source, dir, filePath)
	if err != nil {
		t.Fatalf("failed to parse script: %v", err)
	}
	goCode, err := compiler.Transpile(program, a, compiler.TranspileOptions{})
	if err != nil {
		t.Fatalf("transpilation failed: %v", err)
	}
	if formatted, err := format.Source([]byte(goCode)); err == nil {
		goCode = string(formatted)
	} else {
		t.Fatalf("generated Go source failed to format (likely invalid): %v\n%s", err, goCode)
	}

	if !compiler.UsesBrowserModule(goCode) {
		t.Fatalf("expected UsesBrowserModule to detect the generated syscall/js import in:\n%s", goCode)
	}

	outBin := filepath.Join(dir, "browser.wasm")
	if err := compiler.Compile(goCode, outBin, compiler.CompileOptions{GOOS: "js", GOARCH: "wasm"}); err != nil {
		t.Fatalf("compiling the browser module for js/wasm failed: %v", err)
	}

	data, err := os.ReadFile(outBin)
	if err != nil {
		t.Fatalf("failed to read compiled wasm binary: %v", err)
	}
	wantMagic := []byte{0x00, 'a', 's', 'm'} // WebAssembly binary magic number
	if len(data) < 4 || !bytes.Equal(data[:4], wantMagic) {
		got := data
		if len(got) > 4 {
			got = got[:4]
		}
		t.Errorf("expected a wasm binary starting with magic bytes %x, got %x", wantMagic, got)
	}
}

// runCajaSource is a small end-to-end helper shared by the copy-on-write
// tests below: parses, transpiles with PrintResult (so the last expression
// statement auto-prints), formats, and runs the source, returning stdout.
func runCajaSource(t *testing.T, source string) string {
	t.Helper()
	dir := t.TempDir()
	filePath := filepath.Join(dir, "cow.caja")
	if err := os.WriteFile(filePath, []byte(source), 0644); err != nil {
		t.Fatalf("failed to write test script: %v", err)
	}

	program, _, a, err := script.ParseWithDir(source, dir, filePath)
	if err != nil {
		t.Fatalf("failed to parse script: %v", err)
	}
	goCode, err := compiler.Transpile(program, a, compiler.TranspileOptions{PrintResult: true})
	if err != nil {
		t.Fatalf("transpilation failed: %v", err)
	}
	if formatted, err := format.Source([]byte(goCode)); err == nil {
		goCode = string(formatted)
	} else {
		t.Fatalf("generated Go source failed to format (likely invalid): %v\n%s", err, goCode)
	}

	var stdout, stderr bytes.Buffer
	if err := compiler.Run(goCode, nil, nil, &stdout, &stderr); err != nil {
		t.Fatalf("run failed: %v\nstderr:\n%s", err, stderr.String())
	}
	return stdout.String()
}

// TestCOWStructAssignmentDoesNotAlias is the core behavior this whole change
// exists for: `let b = a` used to alias the same struct pointer, so
// mutating a.x through the original binding was also visible through b.
// With copy-on-write, b must see the value as it was at the time of
// assignment, unaffected by a's later mutation.
func TestCOWStructAssignmentDoesNotAlias(t *testing.T) {
	source := "type Point struct {\n\tx Number\n\ty Number\n}\n" +
		"let a = Point{x: 1, y: 2}\n" +
		"let b = a\n" +
		"a.x = 99\n" +
		"let result = [a.x, b.x]\n" +
		"result\n"
	out := runCajaSource(t, source)
	if !strings.Contains(out, "[99, 1]") {
		t.Errorf("expected [99, 1] (a mutated, b unaffected), got: %s", out)
	}
}

// TestCOWStructFunctionArgumentDoesNotAlias exercises a second aliasing
// site: passing an existing struct as a function argument. Function
// parameters are constant in caja (can't be mutated directly), so the
// function rebinds it via a local `let` first — mutating that local must
// not be visible to the caller's original binding, cascading the shared
// bit through two aliasing hops (call argument, then let) before the
// property assignment that finally triggers the copy.
func TestCOWStructFunctionArgumentDoesNotAlias(t *testing.T) {
	source := "type Point struct {\n\tx Number\n\ty Number\n}\n" +
		"let mutateIt = fn(p: Point) -> Number {\n\tlet local = p\n\tlocal.x = 100\n\treturn local.x\n}\n" +
		"let a = Point{x: 1, y: 2}\n" +
		"let r = mutateIt(a)\n" +
		"let result = [a.x, r]\n" +
		"result\n"
	out := runCajaSource(t, source)
	if !strings.Contains(out, "[1, 100]") {
		t.Errorf("expected [1, 100] (caller's a untouched, function saw the mutation), got: %s", out)
	}
}

// TestCOWStructMoveStillWorks confirms `move` — now meaningful under
// copy-on-write, since isOwned treats a moved expression as already
// uniquely owned — still produces a normal, usable value. Note: the
// analyzer already statically forbids reusing a moved-from variable (`use
// of moved variable` — internal/pipeline/analyzer/analyzer.go:1087-1089),
// so unlike a hand-rolled convention, there's no legal caja program left
// that could observe a is still shared after `let b = move a` to compare
// against; this just checks the move path itself still works.
func TestCOWStructMoveStillWorks(t *testing.T) {
	source := "type Point struct {\n\tx Number\n\ty Number\n}\n" +
		"let a = Point{x: 1, y: 2}\n" +
		"let b = move a\n" +
		"b.x = 99\n" +
		"b.x\n"
	out := runCajaSource(t, source)
	if !strings.Contains(out, "99") {
		t.Errorf("expected 99, got: %s", out)
	}
}

// TestCOWHiddenFieldNeverLeaksIntoOutput confirms cajaShared (copy-on-
// write's hidden bookkeeping field) never appears in a struct's printed
// representation.
func TestCOWHiddenFieldNeverLeaksIntoOutput(t *testing.T) {
	source := "type Point struct {\n\tx Number\n\ty Number\n}\n" +
		"Point{x: 1, y: 2}\n"
	out := runCajaSource(t, source)
	if strings.Contains(out, "cajaShared") {
		t.Errorf("cajaShared leaked into printed output: %s", out)
	}
	if !strings.Contains(out, "X: 1") || !strings.Contains(out, "Y: 2") {
		t.Errorf("expected the struct's real fields to be printed, got: %s", out)
	}
}

// TestRunLastStatementVoidBuiltinCallDoesNotCrash is a regression test for a
// bug found while verifying copy-on-write against the real move_semantics
// sample: log.info/warn/error/export were analyzer-typed as STRING_OBJ/
// ANY_OBJ instead of NULL_OBJ (contradicting their own "-> Nil" label in
// symbol.go and their real void behavior in the compiler), so `caja run`'s
// print-last-statement feature wrongly wrapped a trailing void builtin call
// as if it returned a value, generating Go that failed to compile ("...
// (no value) used as value"). Fixed in
// internal/pipeline/analyzer/builtin.go's analyzeLogFunction/
// analyzeLogExportFunction.
func TestRunLastStatementVoidBuiltinCallDoesNotCrash(t *testing.T) {
	for _, source := range []string{
		"import \"log\" as log\nlog.info(\"msg\", 1)\n",
		"import \"log\" as log\nlog.warn(\"msg\", 1)\n",
		"import \"log\" as log\nlog.error(\"msg\", 1)\n",
		"import \"log\" as log\nlog.export(1)\n",
	} {
		runCajaSource(t, source)
	}
}

// TestNothingReturnTypeCompiles is a regression test for a bug where a
// function explicitly declared "-> Nothing" (Caja's void return type, a
// synthetic zero-field struct the analyzer injects globally — see
// isNothingReturnType in transpiler.go) failed to compile: mapSymbolToGoType
// treated it like any other struct and emitted "*Nothing", but no `type
// Nothing struct{}` is ever generated (Nothing is analyzer-only, not a real
// user-declared struct), producing "undefined: Nothing"; separately, Go
// still required a return value for that bogus non-void signature, producing
// "missing return" for a function with no explicit return statement. Covers
// all three ways a "-> Nothing" function's body can end: implicit fallthrough,
// a bare `return`, and an explicit `return Nothing {}`.
func TestNothingReturnTypeCompiles(t *testing.T) {
	for name, source := range map[string]string{
		"implicit return": "import \"log\" as log\n" +
			"let f = fn(msg: String) -> Nothing {\n\tlog.info(msg, 1)\n}\n" +
			"f(\"hi\")\n",
		"bare return": "let f = fn() -> Nothing {\n\treturn\n}\n" +
			"f()\n",
		"explicit return Nothing {}": "let f = fn() -> Nothing {\n\treturn Nothing {}\n}\n" +
			"f()\n",
	} {
		t.Run(name, func(t *testing.T) {
			runCajaSource(t, source)
		})
	}
}

// TestCOWMemoizedFunctionCacheNotCorrupted is a regression test for a real
// bug found while hardening copy-on-write: transpileMemoBinding's cache
// wrapper is hand-generated Go text (transpiler.go's `result := impl(args);
// cache.Store(key, result); return result`) that bypasses the normal
// transpileStatement dispatch maybeShareStruct hooks into at the five
// designed aliasing sites. A memoized function returning a fresh struct
// literal directly (trusted as "owned", so not marked shared by its own
// return statement) used to store that same unshared pointer in the cache
// AND hand it to the caller — mutating the caller's copy corrupted the
// cached entry for every future call with the same key.
func TestCOWMemoizedFunctionCacheNotCorrupted(t *testing.T) {
	source := "type Point struct {\n\tx Number\n\ty Number\n}\n" +
		"let makePoint = memo fn(seed: Number) -> Point {\n\treturn Point{x: seed, y: seed}\n}\n" +
		"let a = makePoint(1)\n" +
		"a.x = 999\n" +
		"let b = makePoint(1)\n" +
		"b.x\n"
	out := runCajaSource(t, source)
	if !strings.Contains(out, "1") || strings.Contains(out, "999") {
		t.Errorf("expected the second identical memoized call to return the original cached value 1 (unaffected by mutating a), got: %s", out)
	}
}

// TestCOWStructArgumentToBuiltinCall (point 4 from the survey of remaining
// gaps): a struct passed as an argument to a builtin module call
// (log.info's second argument here) goes through transpileBuiltinCall's own
// argument loop, entirely bypassing maybeShareStruct. This confirms that
// doesn't interfere with — or substitute for — the sharing already
// established by an earlier aliasing site (the plain `let b = a` here):
// mutating the original afterward must still leave the earlier alias
// unaffected.
func TestCOWStructArgumentToBuiltinCall(t *testing.T) {
	source := "import \"log\" as log\n" +
		"type Point struct {\n\tx Number\n\ty Number\n}\n" +
		"let a = Point{x: 1, y: 2}\n" +
		"let b = a\n" +
		"log.info(\"point\", b)\n" +
		"a.x = 99\n" +
		"b.x\n"
	out := runCajaSource(t, source)
	if !strings.Contains(out, "1") {
		t.Errorf("expected b.x to remain 1 (unaffected by mutating a after the builtin call), got: %s", out)
	}
}

// TestCOWStructThroughGenericFunction (point 5 from the survey): a struct
// passed through a generic function parameter. maybeShareStruct checks the
// argument expression's own resolved type (Point, from `a`'s declaration),
// never the callee's declared — possibly generic — parameter type, so this
// is expected to behave identically to the non-generic function-argument
// case regardless of genericity.
func TestCOWStructThroughGenericFunction(t *testing.T) {
	source := "type Point struct {\n\tx Number\n\ty Number\n}\n" +
		"let identity = fn<T>(x: T) -> T {\n\treturn x\n}\n" +
		"let a = Point{x: 1, y: 2}\n" +
		"let b = identity(a)\n" +
		"a.x = 99\n" +
		"b.x\n"
	out := runCajaSource(t, source)
	if !strings.Contains(out, "1") {
		t.Errorf("expected b.x to remain 1 (unaffected by mutating a after passing through a generic function), got: %s", out)
	}
}

// TestCOWStructArrayFieldIsProtected (point 6 from the original struct-COW
// survey, now closed by arrays/maps COW + the unified ensureUnshared
// chain-cascading mechanism): mutating an array-typed struct field
// (lib.books[0] = x) now correctly clones the array — and, if the struct
// itself is shared, cascades through it too — so a second alias of the
// struct no longer observes the mutation. This used to be a documented,
// accepted limitation (mutating an array/map-typed struct field was a plain
// index-assignment that never triggered any clone); it's real, correct
// behavior now.
func TestCOWStructArrayFieldIsProtected(t *testing.T) {
	source := "type Library struct {\n\tbooks [String]\n}\n" +
		"let lib = Library{books: [\"a\"]}\n" +
		"let lib2 = lib\n" +
		"lib.books[0] = \"b\"\n" +
		"lib2.books[0]\n"
	out := runCajaSource(t, source)
	if !strings.Contains(out, "a") {
		t.Errorf("expected lib2.books[0] to still read \"a\" (unaffected by mutating lib), got: %s", out)
	}
	if strings.Contains(out, "b") {
		t.Errorf("expected lib2.books[0] to be unaffected by lib.books[0] = \"b\", got: %s", out)
	}
}

// TestCOWArrayIndexAssignmentDoesNotAlias is the array counterpart of
// TestCOWStructAssignmentDoesNotAlias: `let b = a` used to alias the same
// backing slice, so mutating a[0] through the original binding was also
// visible through b. Arrays now get the same copy-on-write treatment as
// structs, via the *cajaArray[T] wrapper and the same ensureUnshared check
// IndexAssignmentStatement now runs.
func TestCOWArrayIndexAssignmentDoesNotAlias(t *testing.T) {
	source := "let a = [1, 2, 3]\n" +
		"let b = a\n" +
		"a[0] = 99\n" +
		"let result = [a[0], b[0]]\n" +
		"result\n"
	out := runCajaSource(t, source)
	if !strings.Contains(out, "[99, 1]") {
		t.Errorf("expected [99, 1] (a mutated, b unaffected), got: %s", out)
	}
}

// TestCOWMapIndexAssignmentDoesNotAlias is the map counterpart.
func TestCOWMapIndexAssignmentDoesNotAlias(t *testing.T) {
	source := "let a = {\"x\": 1}\n" +
		"let b = a\n" +
		"a[\"x\"] = 99\n" +
		"let result = [a[\"x\"], b[\"x\"]]\n" +
		"result\n"
	out := runCajaSource(t, source)
	if !strings.Contains(out, "[99, 1]") {
		t.Errorf("expected [99, 1] (a mutated, b unaffected), got: %s", out)
	}
}

// TestCOWChainedPropertyAssignmentCascades confirms copy-on-write now
// cascades through an arbitrarily deep chain (a.b.field = x), not just a
// single-level p.field = x — the scope boundary the original struct-COW
// plan deliberately deferred, closed here by the same ensureUnshared
// mechanism arrays/maps needed anyway.
func TestCOWChainedPropertyAssignmentCascades(t *testing.T) {
	source := "type Inner struct {\n\tval Number\n}\n" +
		"type Outer struct {\n\tinner Inner\n}\n" +
		"let i1 = Inner{val: 1}\n" +
		"let o1 = Outer{inner: i1}\n" +
		"let o2 = o1\n" +
		"o1.inner.val = 99\n" +
		"let result = [o1.inner.val, o2.inner.val, i1.val]\n" +
		"result\n"
	out := runCajaSource(t, source)
	if !strings.Contains(out, "[99, 1, 1]") {
		t.Errorf("expected [99, 1, 1] (o1 mutated, o2 and the original i1 unaffected), got: %s", out)
	}
}

// TestCOWMatrixIndexAssignmentDoesNotAlias confirms nested-array (matrix)
// copy-on-write composes correctly: mutating one row through one alias must
// not be visible through another alias of the whole matrix.
func TestCOWMatrixIndexAssignmentDoesNotAlias(t *testing.T) {
	source := "let m = [[1, 2], [3, 4]]\n" +
		"let m2 = m\n" +
		"m[0][0] = 99\n" +
		"let result = [m[0][0], m2[0][0]]\n" +
		"result\n"
	out := runCajaSource(t, source)
	if !strings.Contains(out, "[99, 1]") {
		t.Errorf("expected [99, 1] (m mutated, m2 unaffected), got: %s", out)
	}
}

// TestCOWMoveArrayStillWorks is the array counterpart of
// TestCOWStructMoveStillWorks: move skips the shared-mark for arrays too
// (isOwned already treated a `move`d expression this way generally), so a
// moved array can still be mutated/extended normally.
func TestCOWMoveArrayStillWorks(t *testing.T) {
	source := "import \"array\" as array\n" +
		"let a = [1, 2, 3]\n" +
		"let b = move a\n" +
		"let c = array.push(move b, 4)\n" +
		"c\n"
	out := runCajaSource(t, source)
	if !strings.Contains(out, "[1, 2, 3, 4]") {
		t.Errorf("expected [1, 2, 3, 4], got: %s", out)
	}
}

// TestCOWPipelineBuiltinChainStillZeroCopy confirms copy-on-write didn't
// reintroduce a full-array copy at each stage of a builtin-chained pipeline.
// `|>` has no dedicated codegen case at all — it desugars into a plain
// CallExpression at parse time, and a piped call into a *builtin* module
// function (array.push here) is intercepted by transpileBuiltinCall before
// the general call-argument loop maybeShareValue hooks into, so isOwned's
// existing owned/non-owned split (caja_array_push_owned mutating .Data in
// place vs. caja_array_push copying) is completely unaffected by COW — it
// just now operates through the wrapper's .Data field instead of a bare
// slice. This only re-checks end-to-end correctness of that chain; the
// actual "still the owned fast path" claim is pinned precisely at the
// codegen level by TestTranspile's "Move in pipeline" and "Pipeline
// temporary values are implicitly moved" cases (they assert
// caja_array_push_owned appears, not caja_array_push).
func TestCOWPipelineBuiltinChainStillZeroCopy(t *testing.T) {
	source := "import \"array\" as array\n" +
		"let numbers = [10, 20, 30]\n" +
		"let result = numbers |> array.push(40) |> array.push(50) |> array.push(60)\n" +
		"let original = numbers[0]\n" +
		"let combined = [original, result[0], result[1], result[2], result[3], result[4], result[5]]\n" +
		"combined\n"
	out := runCajaSource(t, source)
	if !strings.Contains(out, "[10, 10, 20, 30, 40, 50, 60]") {
		t.Errorf("expected [10, 10, 20, 30, 40, 50, 60] (numbers untouched, result correctly extended), got: %s", out)
	}
}

// TestCOWPipedIntoUserFunctionProtectsCaller confirms a real, new
// correctness guarantee: `a |> mutateIt` desugars into the exact same
// CallExpression shape as `mutateIt(a)` (see the package comment on
// TestCOWPipelineBuiltinChainStillZeroCopy), so it goes through
// maybeShareValue like any other function call — meaning a function that
// mutates its array parameter (or a local rebinding of it) no longer
// corrupts the caller's array, whether it's called directly or through a
// pipe. This was never part of the pipeline zero-copy guarantee (that one
// is specifically about builtin-to-builtin temporaries); it closes a
// pre-existing aliasing bug for the pipe-syntax call form too.
func TestCOWPipedIntoUserFunctionProtectsCaller(t *testing.T) {
	source := "let mutateIt = fn(arr: [Number]) -> Number {\n" +
		"\tlet local = arr\n" +
		"\tlocal[0] = 999\n" +
		"\treturn local[0]\n" +
		"}\n" +
		"let a = [1, 2, 3]\n" +
		"let r = a |> mutateIt\n" +
		"let result = [a[0], r]\n" +
		"result\n"
	out := runCajaSource(t, source)
	if !strings.Contains(out, "[1, 999]") {
		t.Errorf("expected [1, 999] (a untouched, mutateIt saw the mutation), got: %s", out)
	}
}

// TestCOWMemoizedFunctionArrayCacheNotCorrupted is the array counterpart of
// TestCOWMemoizedFunctionCacheNotCorrupted: the memo-cache-store fix
// (isSharableSymbol(fnSym.ReturnType()) in transpileMemoBinding) generalized
// automatically to arrays/maps when isStructSymbol was renamed to
// isSharableSymbol for the arrays/maps work, but that was never confirmed
// with its own test — this is exactly the class of bug that was real for
// structs, so it's worth pinning down explicitly rather than assuming the
// rename alone was sufficient.
func TestCOWMemoizedFunctionArrayCacheNotCorrupted(t *testing.T) {
	source := "let makeArr = memo fn(seed: Number) -> [Number] {\n\treturn [seed, seed]\n}\n" +
		"let a = makeArr(1)\n" +
		"a[0] = 999\n" +
		"let b = makeArr(1)\n" +
		"b[0]\n"
	out := runCajaSource(t, source)
	if !strings.Contains(out, "1") || strings.Contains(out, "999") {
		t.Errorf("expected the second identical memoized call to return the original cached value 1 (unaffected by mutating a), got: %s", out)
	}
}

// TestCOWArrayReturnedFromFunctionDoesNotAlias mirrors the struct version
// (aliasing on return) for arrays: two separate calls to a function
// returning a freshly-built array must not alias each other.
func TestCOWArrayReturnedFromFunctionDoesNotAlias(t *testing.T) {
	source := "let makeArr = fn() -> [Number] {\n\tlet data = [1, 2, 3]\n\treturn data\n}\n" +
		"let a = makeArr()\n" +
		"let b = makeArr()\n" +
		"a[0] = 99\n" +
		"let result = [a[0], b[0]]\n" +
		"result\n"
	out := runCajaSource(t, source)
	if !strings.Contains(out, "[99, 1]") {
		t.Errorf("expected [99, 1] (a mutated, b from a separate call unaffected), got: %s", out)
	}
}

// TestCOWArrayAsStructLiteralFieldDoesNotAlias confirms the same array
// value placed into two different struct literals' fields is correctly
// marked shared at each placement, so mutating through one struct's field
// doesn't affect the other struct's field or the original array binding.
func TestCOWArrayAsStructLiteralFieldDoesNotAlias(t *testing.T) {
	source := "type Box struct {\n\titems [Number]\n}\n" +
		"let shared = [1, 2, 3]\n" +
		"let box1 = Box{items: shared}\n" +
		"let box2 = Box{items: shared}\n" +
		"box1.items[0] = 99\n" +
		"let result = [box1.items[0], box2.items[0], shared[0]]\n" +
		"result\n"
	out := runCajaSource(t, source)
	if !strings.Contains(out, "[99, 1, 1]") {
		t.Errorf("expected [99, 1, 1] (box1 mutated, box2 and the original shared array unaffected), got: %s", out)
	}
}

// TestCOWDeadRebindStaysProtectedUnderTailCallRecursion is the regression
// guard for maybeShareValue's dead-name elision inside a self-tail-recursive
// function (see functionBodyHasSelfTailCall / ctx.inLoop). "p" is referenced
// by name exactly once in build's entire body — the recursive call threads
// the accumulated value forward under a DIFFERENT name ("local", already
// rebound from p), so p itself has no later textual occurrence anywhere in
// the function. A naive whole-function "is this name read again" check would
// therefore wrongly conclude it's dead and skip marking it shared — but the
// same `let local = p` statement re-executes on every simulated "call" once
// the whole body is wrapped in tail-call optimization's `for{}`, and p is
// reassigned (via the TCO rewrite) to the previous iteration's own `local`
// object each time. Without ctx.inLoop forcing the share (and therefore the
// clone at the mutation site) on every iteration regardless, iteration N's
// in-place mutation would keep mutating the very same object already pushed
// into acc by iteration N-1, so every previously pushed entry would end up
// reflecting the final iteration's cumulative value instead of its own.
func TestCOWDeadRebindStaysProtectedUnderTailCallRecursion(t *testing.T) {
	source := "import \"array\" as array\n" +
		"type Portfolio struct {\n\tvalue Number\n}\n" +
		"let build = fn(n: Number, p: Portfolio, acc: [Portfolio]) -> [Portfolio] {\n" +
		"\tif (n == 0) {\n" +
		"\t\treturn acc\n" +
		"\t}\n" +
		"\tlet local = p\n" +
		"\tlocal.value = local.value + n\n" +
		"\tlet newAcc = array.push(acc, local)\n" +
		"\treturn build(n - 1, local, newAcc)\n" +
		"}\n" +
		"let template = Portfolio{value: 0}\n" +
		"let seed = [Portfolio{value: -1}]\n" + // avoids an empty array literal, which infers [Any] and fails to typecheck against [Portfolio]
		"let acc = build(3, template, seed)\n" +
		"let result = [acc[1].value, acc[2].value, acc[3].value]\n" +
		"result\n"
	out := runCajaSource(t, source)
	if !strings.Contains(out, "[3, 5, 6]") {
		t.Errorf("expected [3, 5, 6] (each pushed element keeps its own iteration's cumulative value, not all aliased to the final one), got: %s", out)
	}
}

// TestCOWDeadRebindStaysProtectedAcrossClosureCapture is the regression
// guard for the closure-capture hazard in identifierReadAfter: a closure
// defined BEFORE a parameter rebind, capturing that parameter by reference,
// can still be invoked at an arbitrary later time — even one textually after
// the rebind — so any occurrence of the name inside a nested function
// literal must count as "still reachable" regardless of its own position.
// Getting this wrong would let `let local = p` skip marking p shared (since
// its only other occurrence, inside getX, is textually BEFORE the rebind,
// so a naive position-only comparison would see no LATER occurrence), and
// the later call to getX() would then observe the mutated value instead of
// the original. `move a` at the call site is deliberate: it's what makes p
// enter f with cajaShared already false, so the only thing that can still
// force a clone at `local.x = 999` is identifierReadAfter correctly treating
// getX's capture as live — without `move` the call-site aliasing check
// would independently mark p shared regardless, masking this specific bug.
func TestCOWDeadRebindStaysProtectedAcrossClosureCapture(t *testing.T) {
	source := "type Point struct {\n\tx Number\n}\n" +
		"let f = fn(p: Point) -> Number {\n" +
		"\tlet getX = fn() -> Number { return p.x }\n" +
		"\tlet local = p\n" +
		"\tlocal.x = 999\n" +
		"\treturn getX()\n" +
		"}\n" +
		"let a = Point{x: 1}\n" +
		"f(move a)\n"
	out := runCajaSource(t, source)
	if !strings.Contains(out, "1") || strings.Contains(out, "999") {
		t.Errorf("expected 1 (getX's captured p observes the pre-mutation value), got: %s", out)
	}
}
