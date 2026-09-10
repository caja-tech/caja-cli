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
			opts := compiler.CompileOptions{}
			// A browser-module sample only builds under GOOS=js/GOARCH=wasm
			// (it compiles to syscall/js calls) — same auto-detection
			// cmd/cli/build.go uses, so this generic loop doesn't need a
			// hardcoded list of which sample directories are "the browser
			// one(s)".
			if compiler.UsesBrowserModule(goCode) {
				opts = compiler.CompileOptions{GOOS: "js", GOARCH: "wasm"}
				outBin += ".wasm"
			}

			// Test compilation
			err = compiler.Compile(goCode, outBin, opts)
			if err != nil {
				t.Fatalf("compilation failed: %v", err)
			}

			// Verify binary exists
			data, err := os.ReadFile(outBin)
			if err != nil {
				t.Fatalf("expected binary %s to be generated, but it was not: %v", outBin, err)
			}
			if opts.GOOS == "js" {
				wantMagic := []byte{0x00, 'a', 's', 'm'} // WebAssembly binary magic number
				if len(data) < 4 || !bytes.Equal(data[:4], wantMagic) {
					got := data
					if len(got) > 4 {
						got = got[:4]
					}
					t.Errorf("expected %s to start with wasm magic bytes %x, got %x", outBin, wantMagic, got)
				}
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
		"browser.alert(\"hi\")\n" +
		"browser.setValue(el, \"typed value\")\n" +
		"let v = browser.getValue(el)\n" +
		"browser.log(v)\n"
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

// TestCompileBrowserOnForWasm is the same kind of real-compile check as
// TestCompileBrowserModuleForWasm, but for browser.on specifically: it
// exercises the one piece of codegen distinct from the other browser
// builtins — js.FuncOf wrapping a Caja closure passed by value, and the
// time.Sleep keep-alive loop emitted at the end of main() (see transpiler.go
// for why it's a sleep loop and not a bare select{}) to keep the wasm
// instance's Go runtime scheduling so the registered listener can still fire
// after main would otherwise have returned. Manually verified once (outside
// this test suite, via headless Chrome) that a real DOM click actually
// reaches the Go closure and updates the page; this test only re-confirms
// the generated program still compiles for js/wasm.
func TestCompileBrowserOnForWasm(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "on.caja")
	source := "import browser\n" +
		"let el = browser.getElementById(\"btn\")\n" +
		"let handleClick = fn() -> Nothing {\n" +
		"\tbrowser.log(\"clicked\")\n" +
		"}\n" +
		"browser.on(\"click\", el, handleClick)\n"
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

	if !strings.Contains(goCode, "time.Sleep") {
		t.Errorf("expected \"on\" usage to emit a time.Sleep keep-alive loop at the end of main so the listener keeps working, got:\n%s", goCode)
	}

	outBin := filepath.Join(dir, "on.wasm")
	if err := compiler.Compile(goCode, outBin, compiler.CompileOptions{GOOS: "js", GOARCH: "wasm"}); err != nil {
		t.Fatalf("compiling browser.on for js/wasm failed: %v", err)
	}

	data, err := os.ReadFile(outBin)
	if err != nil {
		t.Fatalf("failed to read compiled wasm binary: %v", err)
	}
	wantMagic := []byte{0x00, 'a', 's', 'm'}
	if len(data) < 4 || !bytes.Equal(data[:4], wantMagic) {
		got := data
		if len(got) > 4 {
			got = got[:4]
		}
		t.Errorf("expected a wasm binary starting with magic bytes %x, got %x", wantMagic, got)
	}
}

// TestCompileBrowserFetchForWasm is the same kind of real-compile check as
// TestCompileBrowserModuleForWasm/TestCompileBrowserOnClickForWasm, for
// browser.fetch: it exercises the injected caja_browser_fetch helper (the
// Promise-to-channel bridge — see injectBuiltinDependencies) both called
// directly and wrapped in async/unwrap, confirming both compile for js/wasm.
// Manually verified once (outside this suite, via headless Chrome) that a
// real fetch — synchronous, concurrent via async/await/unwrap, and a network
// failure — actually works end to end; this test only re-confirms the
// generated program still compiles.
func TestCompileBrowserFetchForWasm(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "fetch.caja")
	source := "import browser\n" +
		"let body = browser.fetch(\"/data\")\n" +
		"browser.log(body)\n" +
		"let t = async browser.fetch(\"/data2\")\n" +
		"let body2 = unwrap t\n" +
		"browser.log(body2)\n"
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

	if !strings.Contains(goCode, "func caja_browser_fetch(url string) string {") {
		t.Errorf("expected the caja_browser_fetch helper to be injected, got:\n%s", goCode)
	}

	outBin := filepath.Join(dir, "fetch.wasm")
	if err := compiler.Compile(goCode, outBin, compiler.CompileOptions{GOOS: "js", GOARCH: "wasm"}); err != nil {
		t.Fatalf("compiling browser.fetch for js/wasm failed: %v", err)
	}

	data, err := os.ReadFile(outBin)
	if err != nil {
		t.Fatalf("failed to read compiled wasm binary: %v", err)
	}
	wantMagic := []byte{0x00, 'a', 's', 'm'}
	if len(data) < 4 || !bytes.Equal(data[:4], wantMagic) {
		got := data
		if len(got) > 4 {
			got = got[:4]
		}
		t.Errorf("expected a wasm binary starting with magic bytes %x, got %x", wantMagic, got)
	}
}

// TestCompileBrowserFetchThenForWasm is the same kind of real-compile check
// as the other browser wasm tests, for browser.fetchThen specifically — the
// safe, callback-driven alternative to plain fetch for use inside a
// browser.on handler (see the deadlock this replaces: a synchronous
// browser.fetch call, even wrapped in async+unwrap, permanently freezes the
// whole page when called from inside a handler, because syscall/js.handleEvent
// — Go's dispatcher for every js.FuncOf callback — is not async-aware the way
// wasm_exec.js's top-level run() is, so a goroutine blocking while nested
// under it can never resume). Manually verified once (outside this suite,
// via a headless-Chrome CDP session) that clicking a real button whose
// handler calls fetchThen actually completes and updates the DOM, repeatedly,
// with the page staying fully responsive throughout — this test only
// re-confirms the generated program still compiles for js/wasm.
func TestCompileBrowserFetchThenForWasm(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "fetchthen.caja")
	source := "import browser\n" +
		"let el = browser.getElementById(\"btn\")\n" +
		"let handleBody = fn(body: String) -> Nothing {\n" +
		"\tbrowser.log(body)\n" +
		"}\n" +
		"let handleClick = fn() -> Nothing {\n" +
		"\tbrowser.fetchThen(\"/data\", handleBody)\n" +
		"}\n" +
		"browser.on(\"click\", el, handleClick)\n"
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

	if !strings.Contains(goCode, "func caja_browser_fetch_then(url string, onSuccess func(string)) {") {
		t.Errorf("expected the caja_browser_fetch_then helper to be injected, got:\n%s", goCode)
	}
	if !strings.Contains(goCode, "func caja_wrap_callback(fn func()) {") {
		t.Errorf("expected the caja_wrap_callback helper to be injected, got:\n%s", goCode)
	}

	outBin := filepath.Join(dir, "fetchthen.wasm")
	if err := compiler.Compile(goCode, outBin, compiler.CompileOptions{GOOS: "js", GOARCH: "wasm"}); err != nil {
		t.Fatalf("compiling browser.fetchThen for js/wasm failed: %v", err)
	}

	data, err := os.ReadFile(outBin)
	if err != nil {
		t.Fatalf("failed to read compiled wasm binary: %v", err)
	}
	wantMagic := []byte{0x00, 'a', 's', 'm'}
	if len(data) < 4 || !bytes.Equal(data[:4], wantMagic) {
		got := data
		if len(got) > 4 {
			got = got[:4]
		}
		t.Errorf("expected a wasm binary starting with magic bytes %x, got %x", wantMagic, got)
	}
}

// TestCompileBrowserQuerySelectorForWasm is the same kind of real-compile
// check as the other browser wasm tests, for browser.querySelector (nullable
// Element?, the first builtin to return one — see its comment in
// symbol.GetStandardModule) and browser.querySelectorAll (Array<Element>).
// Also exercises the correct way to consume an Element? result: Caja has no
// if-narrowing, so `if (el != nil) { ... }` alone does NOT change el's
// static type inside the block — passing el directly to a non-nullable
// Element parameter there is a compile error (see
// checkBrowserArgNotNullable in builtin.go); cast.to(el, fallback) is what
// actually unwraps it, using a same-type fallback that's provably
// unreachable once already inside the null check. Also covers array.len
// over the returned Array<Element> — unmodified existing machinery, not
// anything new. Manually verified once (outside this suite, via a
// headless-Chrome CDP session) that a real querySelector finds the right
// element, a miss narrows correctly to nil, and querySelectorAll returns the
// right count; this test only re-confirms the generated program still
// compiles for js/wasm.
func TestCompileBrowserQuerySelectorForWasm(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "queryselector.caja")
	source := "import browser\n" +
		"import array\n" +
		"import cast\n" +
		"let first = browser.querySelector(\".item\")\n" +
		"if (first != nil) {\n" +
		"\tbrowser.setText(cast.to(first, browser.getElementById(\"app\")), \"found\")\n" +
		"}\n" +
		"let items = browser.querySelectorAll(\".item\")\n" +
		"let count = array.len(items)\n" +
		"browser.log(\"done\")\n"
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

	if !strings.Contains(goCode, "func caja_browser_query_selector(selector string) *js.Value {") {
		t.Errorf("expected the caja_browser_query_selector helper to be injected, got:\n%s", goCode)
	}
	if !strings.Contains(goCode, "func caja_browser_query_selector_all(selector string) *cajaArray[js.Value] {") {
		t.Errorf("expected the caja_browser_query_selector_all helper to be injected, got:\n%s", goCode)
	}

	outBin := filepath.Join(dir, "queryselector.wasm")
	if err := compiler.Compile(goCode, outBin, compiler.CompileOptions{GOOS: "js", GOARCH: "wasm"}); err != nil {
		t.Fatalf("compiling browser.querySelector/querySelectorAll for js/wasm failed: %v", err)
	}

	data, err := os.ReadFile(outBin)
	if err != nil {
		t.Fatalf("failed to read compiled wasm binary: %v", err)
	}
	wantMagic := []byte{0x00, 'a', 's', 'm'}
	if len(data) < 4 || !bytes.Equal(data[:4], wantMagic) {
		got := data
		if len(got) > 4 {
			got = got[:4]
		}
		t.Errorf("expected a wasm binary starting with magic bytes %x, got %x", wantMagic, got)
	}
}

// TestCompileBrowserAttributesAndClassesForWasm is the same kind of
// real-compile check as the other browser wasm tests, for setAttribute,
// getAttribute (Nullable String, like querySelector's Element?), addClass,
// and removeClass — plus the cast.to fix that makes getAttribute's result
// actually usable: cast.to(nullableValue, fallback) now correctly
// dereferences with a nil-safe fallback instead of forwarding the raw
// pointer (a real bug, confirmed via headless Chrome pre-fix: calling
// browser.setText with a raw Nullable String panicked with "ValueOf: invalid
// value"; cast.to is the fix's target since it's the only general unwrap
// mechanism Caja has — there's no if-narrowing and no other safe way to pull
// a plain value out of a Nullable). Manually verified once (outside this
// suite, via headless Chrome) that setAttribute/getAttribute/addClass/
// removeClass and the cast.to-based consumption pattern all work correctly
// against a real DOM element; this test only re-confirms the generated
// program still compiles for js/wasm.
func TestCompileBrowserAttributesAndClassesForWasm(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "attrs.caja")
	source := "import browser\n" +
		"import cast\n" +
		"let el = browser.getElementById(\"box\")\n" +
		"browser.setAttribute(el, \"data-role\", \"widget\")\n" +
		"let role = browser.getAttribute(el, \"data-role\")\n" +
		"browser.setText(el, cast.to(role, \"not set\"))\n" +
		"browser.addClass(el, \"highlight\")\n" +
		"browser.removeClass(el, \"a\")\n"
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

	if !strings.Contains(goCode, "func caja_browser_get_attribute(el js.Value, name string) *string {") {
		t.Errorf("expected the caja_browser_get_attribute helper to be injected, got:\n%s", goCode)
	}

	outBin := filepath.Join(dir, "attrs.wasm")
	if err := compiler.Compile(goCode, outBin, compiler.CompileOptions{GOOS: "js", GOARCH: "wasm"}); err != nil {
		t.Fatalf("compiling browser attribute/class builtins for js/wasm failed: %v", err)
	}

	data, err := os.ReadFile(outBin)
	if err != nil {
		t.Fatalf("failed to read compiled wasm binary: %v", err)
	}
	wantMagic := []byte{0x00, 'a', 's', 'm'}
	if len(data) < 4 || !bytes.Equal(data[:4], wantMagic) {
		got := data
		if len(got) > 4 {
			got = got[:4]
		}
		t.Errorf("expected a wasm binary starting with magic bytes %x, got %x", wantMagic, got)
	}
}

func TestCompileBrowserLocalStorageForWasm(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "localstorage.caja")
	source := "import browser\n" +
		"import cast\n" +
		"browser.localStorageSet(\"theme\", \"dark\")\n" +
		"let theme = browser.localStorageGet(\"theme\")\n" +
		"browser.log(cast.to(theme, \"light\"))\n" +
		"browser.localStorageRemove(\"theme\")\n"
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

	if !strings.Contains(goCode, "func caja_browser_local_storage_get(key string) *string {") {
		t.Errorf("expected the caja_browser_local_storage_get helper to be injected, got:\n%s", goCode)
	}

	outBin := filepath.Join(dir, "localstorage.wasm")
	if err := compiler.Compile(goCode, outBin, compiler.CompileOptions{GOOS: "js", GOARCH: "wasm"}); err != nil {
		t.Fatalf("compiling browser localStorage builtins for js/wasm failed: %v", err)
	}

	data, err := os.ReadFile(outBin)
	if err != nil {
		t.Fatalf("failed to read compiled wasm binary: %v", err)
	}
	wantMagic := []byte{0x00, 'a', 's', 'm'}
	if len(data) < 4 || !bytes.Equal(data[:4], wantMagic) {
		got := data
		if len(got) > 4 {
			got = got[:4]
		}
		t.Errorf("expected a wasm binary starting with magic bytes %x, got %x", wantMagic, got)
	}
}

func TestCompileBrowserElementCreationAndStyleForWasm(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "create.caja")
	source := "import browser\n" +
		"let list = browser.getElementById(\"list\")\n" +
		"let item = browser.createElement(\"li\")\n" +
		"browser.setText(item, \"new item\")\n" +
		"browser.setStyle(item, \"color\", \"blue\")\n" +
		"browser.appendChild(list, item)\n" +
		"browser.removeElement(item)\n"
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

	if !strings.Contains(goCode, `Call("createElement", "li")`) {
		t.Errorf("expected createElement's codegen to call document.createElement, got:\n%s", goCode)
	}

	outBin := filepath.Join(dir, "create.wasm")
	if err := compiler.Compile(goCode, outBin, compiler.CompileOptions{GOOS: "js", GOARCH: "wasm"}); err != nil {
		t.Fatalf("compiling browser element-creation/style builtins for js/wasm failed: %v", err)
	}

	data, err := os.ReadFile(outBin)
	if err != nil {
		t.Fatalf("failed to read compiled wasm binary: %v", err)
	}
	wantMagic := []byte{0x00, 'a', 's', 'm'}
	if len(data) < 4 || !bytes.Equal(data[:4], wantMagic) {
		got := data
		if len(got) > 4 {
			got = got[:4]
		}
		t.Errorf("expected a wasm binary starting with magic bytes %x, got %x", wantMagic, got)
	}
}

func TestCompileBrowserInteractionPrimitivesForWasm(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "interact.caja")
	source := "import browser\n" +
		"let list = browser.getElementById(\"list\")\n" +
		"let ref = browser.getElementById(\"a-item\")\n" +
		"let newItem = browser.createElement(\"li\")\n" +
		"browser.insertBefore(list, newItem, ref)\n" +
		"browser.toggleClass(newItem, \"open\")\n" +
		"let isOpen = browser.hasClass(newItem, \"open\")\n" +
		"browser.removeAttribute(newItem, \"data-open\")\n" +
		"let name = browser.getElementById(\"name\")\n" +
		"browser.focus(name)\n" +
		"browser.blur(name)\n" +
		"let box = browser.getElementById(\"remember\")\n" +
		"browser.setChecked(box, true)\n" +
		"let checked = browser.getChecked(box)\n" +
		"let revert = fn() -> Nothing { browser.log(\"reverted\") }\n" +
		"let timerId = browser.setTimeout(2000, revert)\n" +
		"browser.clearTimeout(timerId)\n"
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

	if !strings.Contains(goCode, `Call("setTimeout"`) {
		t.Errorf("expected setTimeout's codegen to call js's setTimeout, got:\n%s", goCode)
	}

	outBin := filepath.Join(dir, "interact.wasm")
	if err := compiler.Compile(goCode, outBin, compiler.CompileOptions{GOOS: "js", GOARCH: "wasm"}); err != nil {
		t.Fatalf("compiling browser interaction-primitive builtins for js/wasm failed: %v", err)
	}

	data, err := os.ReadFile(outBin)
	if err != nil {
		t.Fatalf("failed to read compiled wasm binary: %v", err)
	}
	wantMagic := []byte{0x00, 'a', 's', 'm'}
	if len(data) < 4 || !bytes.Equal(data[:4], wantMagic) {
		got := data
		if len(got) > 4 {
			got = got[:4]
		}
		t.Errorf("expected a wasm binary starting with magic bytes %x, got %x", wantMagic, got)
	}
}

// TestWriteBrowserHarness confirms the wasm_exec.js + HTML loader pair
// WriteBrowserHarness produces next to a compiled browser-module binary are
// both present, correctly named relative to the binary, and reference each
// other correctly — the harness a browser actually needs to run the wasm
// binary TestCompileBrowserModuleForWasm proves compiles.
func TestWriteBrowserHarness(t *testing.T) {
	dir := t.TempDir()
	outBin := filepath.Join(dir, "demo-js-wasm.wasm")
	if err := os.WriteFile(outBin, []byte("fake wasm bytes"), 0644); err != nil {
		t.Fatalf("failed to write fake wasm binary: %v", err)
	}

	htmlPath, err := compiler.WriteBrowserHarness(outBin)
	if err != nil {
		t.Fatalf("WriteBrowserHarness failed: %v", err)
	}

	wantHTMLPath := filepath.Join(dir, "demo-js-wasm.html")
	if htmlPath != wantHTMLPath {
		t.Errorf("expected html path %s, got %s", wantHTMLPath, htmlPath)
	}

	html, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatalf("failed to read generated html: %v", err)
	}
	if !strings.Contains(string(html), `src="wasm_exec.js"`) {
		t.Errorf("expected html to reference wasm_exec.js, got:\n%s", html)
	}
	if !strings.Contains(string(html), `fetch("demo-js-wasm.wasm")`) {
		t.Errorf("expected html to fetch the wasm binary by its own basename, got:\n%s", html)
	}

	wasmExecPath := filepath.Join(dir, "wasm_exec.js")
	info, err := os.Stat(wasmExecPath)
	if err != nil {
		t.Fatalf("expected wasm_exec.js to be copied alongside the binary: %v", err)
	}
	if info.Size() == 0 {
		t.Errorf("expected wasm_exec.js to be non-empty")
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

// TestActiveVariableMutatesInPlace is Milestone 1 of the active/react
// feature verified end-to-end: reassigning an active variable actually
// changes the value a later read observes — this is a core language
// feature, not a browser-module one, so a native run (no wasm, no headless
// Chrome) is the right verification, matching the plan's confirmed
// synchronous, event-loop-free propagation model.
func TestActiveVariableMutatesInPlace(t *testing.T) {
	source := "let active counter = 0\n" +
		"counter = counter + 1\n" +
		"counter = counter + 1\n" +
		"return counter\n"
	out := runCajaSource(t, source)
	if !strings.Contains(out, "2") {
		t.Errorf("expected 2, got: %s", out)
	}
}

// TestActivePrivateModifierComposeInEitherOrder confirms 'private let active
// n = ...' works exactly like 'let active n = ...' from within the same
// module (privacy only affects cross-module visibility, verified separately
// in the analyzer package) — the two modifiers need zero interaction code,
// per the corrected design in the plan.
func TestActivePrivateModifierComposeInEitherOrder(t *testing.T) {
	source := "private let active n = \"name\"\n" +
		"n = \"renamed\"\n" +
		"return n\n"
	out := runCajaSource(t, source)
	if !strings.Contains(out, "renamed") {
		t.Errorf("expected renamed, got: %s", out)
	}
}

// TestReactiveCallUpdatesAutomatically is Milestone 2 of the active/react
// feature verified end-to-end: the motivating example from the plan —
// reactFn(react counter)'s bound result changes when counter is reassigned,
// with no explicit re-call written anywhere.
func TestReactiveCallUpdatesAutomatically(t *testing.T) {
	source := "import cast\nimport log\n" +
		"let active counter = 0\n" +
		"let reactFn = fn(c: Number) -> String { return \"count is \" + cast.to(c, \"\") }\n" +
		"let active result = reactFn(react counter)\n" +
		"log.info(result, \"\")\n" +
		"counter = counter + 1\n" +
		"log.info(result, \"\")\n"
	out := runCajaSource(t, source)
	if !strings.Contains(out, "count is 0") {
		t.Errorf("expected the initial reactive value 'count is 0' in output, got: %s", out)
	}
	if !strings.Contains(out, "count is 1") {
		t.Errorf("expected the updated reactive value 'count is 1' after counter changed, got: %s", out)
	}
}

// TestReactiveCallComposesWithMemoizedCallee guarantees a memoized function
// works correctly as a reactive callee, and that the memo cache is actually
// exercised by the reactive re-invocation (not just by direct calls): when
// counter cycles back to a value already seen, recompute must hit the memo
// cache rather than recomputing.
func TestReactiveCallComposesWithMemoizedCallee(t *testing.T) {
	source := "import cast\nimport log\n" +
		"let reactFn = memo fn(c: Number) -> String {\n" +
		"\tlog.info(\"recomputing\", \"\")\n" +
		"\treturn \"count is \" + cast.to(c, \"\")\n" +
		"}\n" +
		"let active counter = 0\n" +
		"let active result = reactFn(react counter)\n" +
		"log.info(result, \"\")\n" +
		"counter = counter + 1\n" +
		"log.info(result, \"\")\n" +
		"counter = counter - 1\n" +
		"log.info(result, \"\")\n"
	out := runCajaSource(t, source)
	recomputeCount := strings.Count(out, "recomputing")
	if recomputeCount != 2 {
		t.Errorf("expected exactly 2 recomputations (c=0, then c=1 — the third call with c=0 again should hit the memo cache), got %d. Output:\n%s", recomputeCount, out)
	}
	if !strings.Contains(out, "count is 0") || !strings.Contains(out, "count is 1") {
		t.Errorf("expected both 'count is 0' and 'count is 1' in output, got:\n%s", out)
	}
}

// TestReactiveDiamondRecomputesSharedConsumerExactlyOnce is the regression
// guard for a real bug found and reproduced manually before this test was
// written: with b = f(react a), c = g(react a), and d = h(react b, react
// c), changing 'a' used to cascade each edge (a->b, a->c) immediately and
// independently, so d recomputed twice — once right after b updated (while
// c was still stale, observing a torn "2,0" pairing instead of the correct
// "2,3") and once after c updated. cajaPropagate's topological batch fixes
// this: d must recompute exactly once per change to 'a', only after both b
// and c have already settled, and must never observe a torn pairing.
func TestReactiveDiamondRecomputesSharedConsumerExactlyOnce(t *testing.T) {
	source := "import cast\nimport log\n" +
		"let active a = 0\n" +
		"let double = fn(x: Number) -> Number { return x * 2 }\n" +
		"let triple = fn(x: Number) -> Number { return x * 3 }\n" +
		"let active b = double(react a)\n" +
		"let active c = triple(react a)\n" +
		"let sumBC = fn(x: Number, y: Number) -> String {\n" +
		"\tlog.info(\"sumBC called with\", cast.to(x, \"\") + \",\" + cast.to(y, \"\"))\n" +
		"\treturn cast.to(x, \"\") + \"+\" + cast.to(y, \"\")\n" +
		"}\n" +
		"let active d = sumBC(react b, react c)\n" +
		"a = a + 1\n" +
		"log.info(\"final:\", cast.to(d, \"\"))\n"
	out := runCajaSource(t, source)

	if strings.Contains(out, "2,0") {
		t.Errorf("observed the torn 'b updated, c still stale' glitch (sumBC called with 2,0) — cajaPropagate should never let this happen:\n%s", out)
	}
	callCount := strings.Count(out, "sumBC called with")
	if callCount != 2 {
		t.Errorf("expected exactly 2 sumBC calls (initial value, then after a=1), got %d:\n%s", callCount, out)
	}
	if !strings.Contains(out, "sumBC called with 2,3") {
		t.Errorf("expected the post-change recomputation to see the correct settled pairing '2,3', got:\n%s", out)
	}
	if !strings.Contains(out, "final: 2+3") {
		t.Errorf("expected the final value to be '2+3', got:\n%s", out)
	}
}

// TestReactiveChainPropagatesThroughDerivedActive guards the already-working
// case (verified manually before cajaPropagate existed): a derived active
// variable used as its own react source (b = f(react a), c = g(react b))
// must keep propagating correctly under the new topological scheduler, not
// just under the old immediate-cascade design it replaces.
func TestReactiveChainPropagatesThroughDerivedActive(t *testing.T) {
	source := "import cast\nimport log\n" +
		"let active a = 0\n" +
		"let f = fn(x: Number) -> Number { return x * 2 }\n" +
		"let active b = f(react a)\n" +
		"let g = fn(y: Number) -> Number { return y + 100 }\n" +
		"let active c = g(react b)\n" +
		"log.info(cast.to(c, \"\"), \"\")\n" +
		"a = a + 1\n" +
		"log.info(cast.to(c, \"\"), \"\")\n"
	out := runCajaSource(t, source)
	if !strings.Contains(out, "100") {
		t.Errorf("expected the initial chained value 100, got:\n%s", out)
	}
	if !strings.Contains(out, "102") {
		t.Errorf("expected the updated chained value 102 after a=1, got:\n%s", out)
	}
}
