package compiler_test

import (
	"bufio"
	"bytes"
	"caja-cli/internal/pipeline/compiler"
	"caja-cli/internal/script"
	"fmt"
	"go/format"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
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

// TestHTTPModuleProgramRunsViaGoRun confirms a program that uses the http
// builtin module -- and therefore pulls in the http module's Go glue in
// the generated source -- runs successfully through compiler.Run's `go
// run` path, not just compiler.Compile's `go build` path exercised by
// every other http-module test below (via startHTTPSample). Both paths now
// share cajaGoEnv (compiler.go/run.go), so this guards against a
// regression that only manifests for `go run`/`caja run`/`caja listen`
// (e.g. a GOROOT or CGO_ENABLED setting `go build` tolerates but `go run`
// doesn't) slipping past a Compile-only test suite. Deliberately never
// calls http.listen (which blocks forever) -- just enough of the http
// module surface to force the same generated code into the program while
// still letting it run to completion on its own via runCajaSource.
func TestHTTPModuleProgramRunsViaGoRun(t *testing.T) {
	source := "import \"http\" as http\n" +
		"let resp = http.ok(\"hi\")\n" +
		"resp.body\n"
	out := runCajaSource(t, source)
	if !strings.Contains(out, "hi") {
		t.Errorf(`expected output containing "hi", got: %s`, out)
	}
}

// httpSampleAddr is the fixed port samples/http/http.caja listens on.
const httpSampleAddr = "127.0.0.1:8089"

// startHTTPSample compiles samples/<sampleDir>/<sampleDir>.caja to a real
// binary and starts it as a live server (unlike runCajaSource's
// run-to-completion helper — http.listen blocks forever), waiting until it
// accepts connections on addr before returning. Callers are responsible for
// stopping the process (via cmd.Process.Kill/Signal + cmd.Wait); it is not
// stopped automatically, since some callers (the graceful-shutdown test)
// need to send a specific signal and observe how the process reacts to it.
// extraEnv is appended to the started process's environment (e.g.
// CAJA_HTTP_PORT=... to exercise caja listen's port-override mechanism
// without needing to route through the CLI's compiler.Run/`go run` path).
func startHTTPSample(t *testing.T, sampleDir string, addr string, extraEnv ...string) (cmd *exec.Cmd, stderr *bytes.Buffer) {
	t.Helper()
	filePath := filepath.Join("samples", sampleDir, sampleDir+".caja")
	sourceCode, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", filePath, err)
	}

	dir := t.TempDir()
	program, _, a, err := script.ParseWithDir(string(sourceCode), filepath.Join("samples", sampleDir), filePath)
	if err != nil {
		t.Fatalf("failed to parse script: %v", err)
	}
	goCode, err := compiler.Transpile(program, a, compiler.TranspileOptions{})
	if err != nil {
		t.Fatalf("transpilation failed: %v", err)
	}

	outBin := filepath.Join(dir, "httpserver")
	if err := compiler.Compile(goCode, outBin, compiler.CompileOptions{}); err != nil {
		t.Fatalf("compilation failed: %v", err)
	}

	cmd = exec.Command(outBin)
	// Run from the sample's own directory rather than the test process's
	// (this package's source dir, since go test always sets that as the
	// working directory) -- inert for every existing sample, none of which
	// do any relative-path file I/O, but lets a sample that does (e.g.
	// router.static's dir argument) use a natural relative path like
	// "public" instead of one coupled to test internals.
	cmd.Dir = filepath.Join("samples", sampleDir)
	stderr = &bytes.Buffer{}
	cmd.Stderr = stderr
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start server binary: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		conn, dialErr := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if dialErr == nil {
			conn.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("server never accepted connections on %s within timeout; stderr:\n%s", addr, stderr.String())
		}
		time.Sleep(50 * time.Millisecond)
	}

	return cmd, stderr
}

// TestHTTPSampleServesRequests is the end-to-end check for the http builtin
// module: starts samples/http/http.caja as a live server and exercises
// routing, path params, JSON responses, and middleware-based auth over real
// HTTP against it.
func TestHTTPSampleServesRequests(t *testing.T) {
	cmd, _ := startHTTPSample(t, "http", httpSampleAddr)
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	addr := httpSampleAddr
	get := func(path string, headers map[string]string) (int, string) {
		req, err := http.NewRequest(http.MethodGet, "http://"+addr+path, nil)
		if err != nil {
			t.Fatalf("failed to build request for %s: %v", path, err)
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request to %s failed: %v", path, err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body from %s: %v", path, err)
		}
		return resp.StatusCode, string(body)
	}

	if status, body := get("/hello", nil); status != 200 || body != "Hello, world!" {
		t.Errorf("GET /hello: expected 200 %q, got %d %q", "Hello, world!", status, body)
	}

	if status, body := get("/users/42", nil); status != 200 || !strings.Contains(body, `"id":"42"`) {
		t.Errorf(`GET /users/42: expected 200 with "id":"42" in body, got %d %q`, status, body)
	}

	if status, _ := get("/admin", nil); status != 401 {
		t.Errorf("GET /admin without Authorization: expected 401, got %d", status)
	}

	if status, body := get("/admin", map[string]string{"Authorization": "x"}); status != 200 || body != "welcome, admin" {
		t.Errorf("GET /admin with Authorization: expected 200 %q, got %d %q", "welcome, admin", status, body)
	}

	postJSON := func(path string, jsonBody string) (int, string) {
		resp, err := http.Post("http://"+addr+path, "application/json", strings.NewReader(jsonBody))
		if err != nil {
			t.Fatalf("request to %s failed: %v", path, err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body from %s: %v", path, err)
		}
		return resp.StatusCode, string(body)
	}

	if status, body := postJSON("/echo", `{"name": "abner"}`); status != 200 || !strings.Contains(body, `"name":"abner"`) {
		t.Errorf(`POST /echo: expected 200 with "name":"abner" in body, got %d %q`, status, body)
	}

	// notFound/badRequest/serverError all bottom out in caja_http_text
	// (builtins.go collapsed a dedicated caja_http_status_body helper into a
	// direct caja_http_text(<code>, body) call at each of these three sites)
	// -- pin the status code and body each one actually produces over real
	// HTTP so that refactor stays covered, not just type-checked.
	if status, body := get("/missing", nil); status != 404 || body != "nothing here" {
		t.Errorf(`GET /missing: expected 404 %q, got %d %q`, "nothing here", status, body)
	}

	if status, body := get("/bad", nil); status != 400 || body != "bad input" {
		t.Errorf(`GET /bad: expected 400 %q, got %d %q`, "bad input", status, body)
	}

	if status, body := get("/boom", nil); status != 500 || body != "kaboom" {
		t.Errorf(`GET /boom: expected 500 %q, got %d %q`, "kaboom", status, body)
	}

	// /echo returns a response built via http.json(...) -- confirm
	// caja_http_json's Content-Type: application/json header (builtins.go)
	// actually reaches the real HTTP response, not just that its body happens
	// to look like JSON.
	echoResp, err := http.Post("http://"+addr+"/echo", "application/json", strings.NewReader(`{"name": "abner"}`))
	if err != nil {
		t.Fatalf("request to /echo failed: %v", err)
	}
	echoResp.Body.Close()
	if ct := echoResp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf(`POST /echo: expected Content-Type "application/json", got %q`, ct)
	}

	// /custom builds its response by directly constructing an http.Response
	// struct literal (status/headers/body fields set by hand) rather than
	// going through http.ok/text/json -- a legal, previously-unexercised
	// construction path (http.Response is just a regular module-exported
	// struct type, so `http.Response{...}` parses and type-checks the same
	// as any other qualified struct literal -- see
	// analyzer/find.go's dotted-name lookup and
	// parser.go's parseStructLiteral property-expression case). This confirms
	// that path produces a real, correctly-shaped HTTP response end to end,
	// including a header set by hand through the struct literal's `headers`
	// field rather than one of the builtin response constructors.
	customResp, err := http.Get("http://" + addr + "/custom")
	if err != nil {
		t.Fatalf("request to /custom failed: %v", err)
	}
	defer customResp.Body.Close()
	customBody, err := io.ReadAll(customResp.Body)
	if err != nil {
		t.Fatalf("failed to read /custom response body: %v", err)
	}
	if customResp.StatusCode != 201 || string(customBody) != "custom" {
		t.Errorf(`GET /custom: expected 201 "custom", got %d %q`, customResp.StatusCode, string(customBody))
	}
	if got := customResp.Header.Get("X-Custom"); got != "yes" {
		t.Errorf(`GET /custom: expected header X-Custom: "yes", got %q`, got)
	}
}

// TestHTTPSampleGracefulShutdown confirms caja_http_listen catches SIGTERM
// and shuts the server down cleanly (server.Shutdown, process exits with
// status 0) instead of Go's default behavior for an unhandled SIGTERM
// (immediate termination, reported by cmd.Wait as an ExitError carrying the
// signal rather than a clean exit) — the whole point of graceful shutdown is
// giving in-flight requests a chance to finish instead of being cut off.
func TestHTTPSampleGracefulShutdown(t *testing.T) {
	cmd, stderr := startHTTPSample(t, "http", httpSampleAddr)

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("failed to send SIGTERM: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("expected a clean exit after SIGTERM, got: %v; stderr:\n%s", err, stderr.String())
		}
	// 15s mirrors caja_http_listen's own 10s shutdown-drain timeout
	// (builtins.go's caja_http_shutdown_timeout, unexported and so not
	// referenceable from this external test package) plus slack for process
	// startup/scheduling.
	case <-time.After(15 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatalf("server did not exit within the shutdown timeout after SIGTERM; stderr:\n%s", stderr.String())
	}

	// The server must actually have stopped accepting connections, not just
	// exited for an unrelated reason.
	if _, dialErr := net.DialTimeout("tcp", httpSampleAddr, 200*time.Millisecond); dialErr == nil {
		t.Errorf("expected %s to stop accepting connections after shutdown, but it's still listening", httpSampleAddr)
	}
}

// TestHTTPSampleCajaHTTPPortEnvOverridesPort confirms caja_http_listen
// (builtins.go) honors CAJA_HTTP_PORT over the port literal the .caja source
// itself passed to http.listen(...) — the mechanism `caja listen`'s --port
// flag relies on (cmd/cli/listen.go sets this exact env var before running
// the script). samples/http/http.caja hardcodes port 8089; starting it with
// CAJA_HTTP_PORT set to a different port must make it listen there instead,
// and NOT still be reachable on its hardcoded port.
func TestHTTPSampleCajaHTTPPortEnvOverridesPort(t *testing.T) {
	const overriddenAddr = "127.0.0.1:8091"
	cmd, _ := startHTTPSample(t, "http", overriddenAddr, "CAJA_HTTP_PORT=8091")
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	resp, err := http.Get("http://" + overriddenAddr + "/hello")
	if err != nil {
		t.Fatalf("request to overridden port failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200 from the overridden port, got %d", resp.StatusCode)
	}

	if _, dialErr := net.DialTimeout("tcp", httpSampleAddr, 200*time.Millisecond); dialErr == nil {
		t.Errorf("expected the server to NOT be listening on its hardcoded port %s once CAJA_HTTP_PORT overrides it", httpSampleAddr)
	}
}

// TestHTTPSampleTrustProxyControlsIPHeaderTrust confirms req.ip ignores
// X-Forwarded-For/X-Real-IP by default (any direct client could otherwise
// spoof its IP and dodge an IP-keyed rate limiter) and only honors them once
// CAJA_TRUST_PROXY is explicitly set, matching caja_http_adapt's documented
// precedence: X-Forwarded-For's first entry, then X-Real-IP, then the raw
// TCP peer address.
func TestHTTPSampleTrustProxyControlsIPHeaderTrust(t *testing.T) {
	whoami := func(addr string, headers map[string]string) string {
		t.Helper()
		req, err := http.NewRequest(http.MethodGet, "http://"+addr+"/whoami", nil)
		if err != nil {
			t.Fatalf("failed to build request: %v", err)
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return string(body)
	}

	t.Run("untrusted by default", func(t *testing.T) {
		cmd, _ := startHTTPSample(t, "http", httpSampleAddr)
		defer func() {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}()

		ip := whoami(httpSampleAddr, map[string]string{"X-Forwarded-For": "203.0.113.5"})
		if ip != "127.0.0.1" {
			t.Errorf("expected the spoofed X-Forwarded-For to be ignored and req.ip to be the real peer 127.0.0.1, got %q", ip)
		}
	})

	t.Run("trusted once CAJA_TRUST_PROXY is set", func(t *testing.T) {
		cmd, _ := startHTTPSample(t, "http", httpSampleAddr, "CAJA_TRUST_PROXY=1")
		defer func() {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}()

		if ip := whoami(httpSampleAddr, map[string]string{"X-Forwarded-For": "203.0.113.5, 10.0.0.1"}); ip != "203.0.113.5" {
			t.Errorf("expected req.ip to be the first X-Forwarded-For entry 203.0.113.5, got %q", ip)
		}
		if ip := whoami(httpSampleAddr, map[string]string{"X-Real-IP": "198.51.100.7"}); ip != "198.51.100.7" {
			t.Errorf("expected req.ip to fall back to X-Real-IP 198.51.100.7 when X-Forwarded-For is absent, got %q", ip)
		}
		if ip := whoami(httpSampleAddr, nil); ip != "127.0.0.1" {
			t.Errorf("expected req.ip to fall back to the real peer 127.0.0.1 when neither header is present, got %q", ip)
		}
	})
}

// httpRateLimitSampleAddr is the fixed port samples/http_rate_limit listens
// on — deliberately separate from httpSampleAddr/samples/http so rapid
// rate-limit-triggering requests here can't make the base http sample's own
// assertions order-sensitive or flaky.
const httpRateLimitSampleAddr = "127.0.0.1:8090"

// TestHTTPRateLimitSampleEnforcesLimit pins both halves of the token-bucket
// contract: the first `burst` requests succeed, the next is rejected with
// 429, and after waiting long enough for the bucket to refill at the
// configured rate, a subsequent request succeeds again. Timing margins are
// generous (1.1s for a 2 req/s rate, i.e. margin for >2 tokens) to avoid CI
// flakiness.
func TestHTTPRateLimitSampleEnforcesLimit(t *testing.T) {
	cmd, stderr := startHTTPSample(t, "http_rate_limit", httpRateLimitSampleAddr)
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	get := func() int {
		resp, err := http.Get("http://" + httpRateLimitSampleAddr + "/limited")
		if err != nil {
			t.Fatalf("request failed: %v; stderr:\n%s", err, stderr.String())
		}
		defer resp.Body.Close()
		io.Copy(io.Discard, resp.Body)
		return resp.StatusCode
	}

	for i := 0; i < 2; i++ {
		if status := get(); status != 200 {
			t.Errorf("request %d: expected 200 within burst, got %d", i+1, status)
		}
	}

	if status := get(); status != 429 {
		t.Errorf("request beyond burst: expected 429, got %d", status)
	}

	time.Sleep(1100 * time.Millisecond)

	if status := get(); status != 200 {
		t.Errorf("request after refill wait: expected 200, got %d", status)
	}
}

// TestHTTPRateLimitSampleSetsRateLimitHeaders confirms X-RateLimit-Limit/
// Remaining/Reset are set on every response from a rate-limited route
// (allowed or not, so a well-behaved client can see its quota before ever
// hitting the limit), and that Retry-After is set specifically on the 429 --
// the one of the four with defined meaning outside a rate-limit context
// (RFC 7231). samples/http_rate_limit configures rps=2, burst=2.
func TestHTTPRateLimitSampleSetsRateLimitHeaders(t *testing.T) {
	cmd, stderr := startHTTPSample(t, "http_rate_limit", httpRateLimitSampleAddr)
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	get := func() *http.Response {
		resp, err := http.Get("http://" + httpRateLimitSampleAddr + "/limited")
		if err != nil {
			t.Fatalf("request failed: %v; stderr:\n%s", err, stderr.String())
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		return resp
	}

	first := get()
	if first.StatusCode != 200 {
		t.Fatalf("expected first request to be allowed (within burst), got %d", first.StatusCode)
	}
	if got := first.Header.Get("X-RateLimit-Limit"); got != "2" {
		t.Errorf("first request: expected X-RateLimit-Limit 2, got %q", got)
	}
	if got := first.Header.Get("X-RateLimit-Remaining"); got != "1" {
		t.Errorf("first request: expected X-RateLimit-Remaining 1 (2 burst minus this request), got %q", got)
	}
	if got := first.Header.Get("X-RateLimit-Reset"); got == "" {
		t.Errorf("first request: expected a non-empty X-RateLimit-Reset, got none")
	}
	if got := first.Header.Get("Retry-After"); got != "" {
		t.Errorf("first request: expected no Retry-After on an allowed request, got %q", got)
	}

	second := get()
	if got := second.Header.Get("X-RateLimit-Remaining"); got != "0" {
		t.Errorf("second request: expected X-RateLimit-Remaining 0 (burst exhausted), got %q", got)
	}

	rejected := get()
	if rejected.StatusCode != 429 {
		t.Fatalf("expected the third request beyond burst to be rejected, got %d", rejected.StatusCode)
	}
	if got := rejected.Header.Get("Retry-After"); got == "" {
		t.Errorf("expected a non-empty Retry-After on the 429 response, got none")
	}
	if got := rejected.Header.Get("X-RateLimit-Remaining"); got != "0" {
		t.Errorf("rejected request: expected X-RateLimit-Remaining 0, got %q", got)
	}
}

// TestHTTPRateLimitSamplePreservesHandlerSetHeaders confirms
// caja_http_apply_rate_limit_headers's caja_http_ensure_headers call (in
// builtins.go) only lazily initializes a nil Headers map -- it must not
// clobber headers a handler already set by hand. samples/http_rate_limit's
// /limited-custom-headers route returns an http.Response struct literal
// with a pre-populated X-Custom header (a non-nil, already-populated
// Headers map, unlike the http.ok/text-constructed empty-but-non-nil map
// every other rate-limit test exercises), wrapped by the same
// http.rateLimiter as /limited.
func TestHTTPRateLimitSamplePreservesHandlerSetHeaders(t *testing.T) {
	cmd, stderr := startHTTPSample(t, "http_rate_limit", httpRateLimitSampleAddr)
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	resp, err := http.Get("http://" + httpRateLimitSampleAddr + "/limited-custom-headers")
	if err != nil {
		t.Fatalf("request failed: %v; stderr:\n%s", err, stderr.String())
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if resp.StatusCode != 201 {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("X-Custom"); got != "yes" {
		t.Errorf("expected the handler-set X-Custom header to survive caja_http_ensure_headers, got %q", got)
	}
	if got := resp.Header.Get("X-RateLimit-Limit"); got != "2" {
		t.Errorf("expected X-RateLimit-Limit 2 alongside the handler-set header, got %q", got)
	}
	if got := resp.Header.Get("X-RateLimit-Remaining"); got == "" {
		t.Errorf("expected a non-empty X-RateLimit-Remaining alongside the handler-set header, got none")
	}
}

const httpConcurrencyLimitSampleAddr = "127.0.0.1:8099"

// TestHTTPConcurrencyLimitSampleCapsInFlightRequests is the end-to-end check
// for http.concurrencyLimiter: fires several requests at a handler that's
// deliberately slow (samples/http_concurrency_limit's busyWork -- Caja has
// no sleep/delay builtin, so a tail-call-optimized countdown is the only way
// to make a handler take a controllable, real amount of wall-clock time) at
// once, and confirms the limiter caps how many run concurrently rather than
// how many run per second. Assertions use safe bounds (never more than the
// configured max succeed; at least one is rejected) rather than an exact
// split, since precise goroutine-dispatch timing isn't guaranteed even
// though the ~500ms busy-work window makes overlap highly reliable in
// practice. A final request after the batch completes confirms slots are
// released (not leaked) once their handler returns.
func TestHTTPConcurrencyLimitSampleCapsInFlightRequests(t *testing.T) {
	cmd, stderr := startHTTPSample(t, "http_concurrency_limit", httpConcurrencyLimitSampleAddr)
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	const batchSize = 5
	const maxConcurrent = 2

	var wg sync.WaitGroup
	var mu sync.Mutex
	var successCount, rejectedCount int
	wg.Add(batchSize)
	for i := 0; i < batchSize; i++ {
		go func() {
			defer wg.Done()
			resp, err := http.Get("http://" + httpConcurrencyLimitSampleAddr + "/slow")
			if err != nil {
				t.Errorf("request failed: %v; stderr:\n%s", err, stderr.String())
				return
			}
			defer resp.Body.Close()
			io.Copy(io.Discard, resp.Body)
			mu.Lock()
			defer mu.Unlock()
			switch resp.StatusCode {
			case 200:
				successCount++
			case 429:
				rejectedCount++
			default:
				t.Errorf("unexpected status %d from a batch request", resp.StatusCode)
			}
		}()
	}
	wg.Wait()

	if successCount > maxConcurrent {
		t.Errorf("expected at most %d concurrent successes, got %d", maxConcurrent, successCount)
	}
	if successCount < 1 {
		t.Errorf("expected at least 1 request to succeed within the concurrency limit, got %d", successCount)
	}
	if rejectedCount < 1 {
		t.Errorf("expected at least 1 request to be rejected once the concurrency limit was exceeded, got %d rejected (successCount=%d)", rejectedCount, successCount)
	}

	// The batch's handlers have all returned by the time wg.Wait() unblocks,
	// so every slot should be released -- a fresh request must succeed.
	final, err := http.Get("http://" + httpConcurrencyLimitSampleAddr + "/slow")
	if err != nil {
		t.Fatalf("final request failed: %v; stderr:\n%s", err, stderr.String())
	}
	defer final.Body.Close()
	io.Copy(io.Discard, final.Body)
	if final.StatusCode != 200 {
		t.Errorf("expected a request after the batch completed to succeed (slots released, not leaked), got %d", final.StatusCode)
	}
}

// TestHTTPConcurrencyLimitSamplePreservesHandlerSetHeaders is the
// concurrency-limiter counterpart to
// TestHTTPRateLimitSamplePreservesHandlerSetHeaders: confirms
// caja_http_apply_concurrency_headers's caja_http_ensure_headers call
// (builtins.go) doesn't clobber a header the handler already set by hand
// via an http.Response struct literal (a non-nil, already-populated
// Headers map) -- covering ensure_headers's other call site.
func TestHTTPConcurrencyLimitSamplePreservesHandlerSetHeaders(t *testing.T) {
	cmd, stderr := startHTTPSample(t, "http_concurrency_limit", httpConcurrencyLimitSampleAddr)
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	resp, err := http.Get("http://" + httpConcurrencyLimitSampleAddr + "/custom-headers")
	if err != nil {
		t.Fatalf("request failed: %v; stderr:\n%s", err, stderr.String())
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if resp.StatusCode != 201 {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("X-Custom"); got != "yes" {
		t.Errorf("expected the handler-set X-Custom header to survive caja_http_ensure_headers, got %q", got)
	}
	if got := resp.Header.Get("X-Concurrency-Limit"); got != "2" {
		t.Errorf("expected X-Concurrency-Limit 2 alongside the handler-set header, got %q", got)
	}
	if got := resp.Header.Get("X-Concurrency-Remaining"); got == "" {
		t.Errorf("expected a non-empty X-Concurrency-Remaining alongside the handler-set header, got none")
	}
}

// startFakeRedisServer starts a minimal TCP server understanding just enough
// RESP framing to consume one EVAL command per call (without interpreting
// its contents) and reply with the next entry in scriptedReplies, in order.
// This repo has no CI test infrastructure at all (no go test workflow, no
// service containers, no docker-compose) and no real Redis is available in
// this environment either, so this is how the distributed rate limiter's
// actual enforcement path gets tested deterministically instead of only its
// fail-open path.
func startFakeRedisServer(t *testing.T, scriptedReplies []string) (addr string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start fake redis listener: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	var mu sync.Mutex
	next := 0
	handle := func(conn net.Conn) {
		defer conn.Close()
		r := bufio.NewReader(conn)
		for {
			if err := fakeRedisReadEvalCommand(r); err != nil {
				return
			}
			mu.Lock()
			idx := next
			next++
			mu.Unlock()
			if idx >= len(scriptedReplies) {
				return
			}
			if _, err := conn.Write([]byte(scriptedReplies[idx])); err != nil {
				return
			}
		}
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go handle(conn)
		}
	}()
	return ln.Addr().String()
}

// fakeRedisReadEvalCommand consumes exactly one RESP array-of-bulk-strings
// command from r without interpreting its contents, just enough to frame
// successive commands correctly on a reused pooled connection.
func fakeRedisReadEvalCommand(r *bufio.Reader) error {
	line, err := r.ReadString('\n')
	if err != nil {
		return err
	}
	line = strings.TrimRight(line, "\r\n")
	if len(line) == 0 || line[0] != '*' {
		return fmt.Errorf("expected array header, got %q", line)
	}
	n, err := strconv.Atoi(line[1:])
	if err != nil {
		return err
	}
	for i := 0; i < n; i++ {
		head, err := r.ReadString('\n')
		if err != nil {
			return err
		}
		head = strings.TrimRight(head, "\r\n")
		if len(head) == 0 || head[0] != '$' {
			return fmt.Errorf("expected bulk string header, got %q", head)
		}
		size, err := strconv.Atoi(head[1:])
		if err != nil {
			return err
		}
		buf := make([]byte, size+2) // +2 consumes the trailing \r\n
		if _, err := io.ReadFull(r, buf); err != nil {
			return err
		}
	}
	return nil
}

const httpDistributedRateLimitSampleAddr = "127.0.0.1:8100"

// TestHTTPDistributedRateLimitSampleEnforcesLimitAndSetsHeaders drives the
// actual enforcement path (not just fail-open) against a fake Redis server
// scripted to return specific counts, confirming the limiter correctly
// allows/rejects based on the count and fills in X-RateLimit-*/Retry-After
// from the scripted TTL. samples/http_distributed_rate_limit configures
// maxRequests=3.
func TestHTTPDistributedRateLimitSampleEnforcesLimitAndSetsHeaders(t *testing.T) {
	redisAddr := startFakeRedisServer(t, []string{
		"*2\r\n:1\r\n:60\r\n", // count=1, ttl=60 -> allowed (limit is 3)
		"*2\r\n:4\r\n:37\r\n", // count=4 > limit 3, ttl=37 -> rejected
	})

	cmd, stderr := startHTTPSample(t, "http_distributed_rate_limit", httpDistributedRateLimitSampleAddr, "CAJA_REDIS_ADDR="+redisAddr)
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	get := func() *http.Response {
		resp, err := http.Get("http://" + httpDistributedRateLimitSampleAddr + "/limited")
		if err != nil {
			t.Fatalf("request failed: %v; stderr:\n%s", err, stderr.String())
		}
		return resp
	}

	resp1 := get()
	defer resp1.Body.Close()
	if resp1.StatusCode != 200 {
		t.Errorf("expected 200 for count=1 within limit, got %d", resp1.StatusCode)
	}
	if got := resp1.Header.Get("X-RateLimit-Remaining"); got != "2" {
		t.Errorf("expected X-RateLimit-Remaining 2 (limit 3 - count 1), got %q", got)
	}

	resp2 := get()
	defer resp2.Body.Close()
	if resp2.StatusCode != 429 {
		t.Errorf("expected 429 for count=4 over limit 3, got %d", resp2.StatusCode)
	}
	if got := resp2.Header.Get("Retry-After"); got != "37" {
		t.Errorf("expected Retry-After 37 from the fake server's scripted ttl, got %q", got)
	}
}

// TestHTTPDistributedRateLimitSampleFailsOpenWhenRedisUnreachable confirms a
// request still succeeds -- and resolves quickly, not just eventually --
// when CAJA_REDIS_ADDR points at a port nothing is listening on, and that no
// X-RateLimit-* headers are set on a fail-open response (there's no real
// limiter data to report).
func TestHTTPDistributedRateLimitSampleFailsOpenWhenRedisUnreachable(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve an address: %v", err)
	}
	unreachableAddr := ln.Addr().String()
	ln.Close() // guaranteed nothing is listening here now

	cmd, stderr := startHTTPSample(t, "http_distributed_rate_limit", httpDistributedRateLimitSampleAddr, "CAJA_REDIS_ADDR="+unreachableAddr)
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	start := time.Now()
	resp, err := http.Get("http://" + httpDistributedRateLimitSampleAddr + "/limited")
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("request failed: %v; stderr:\n%s", err, stderr.String())
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected fail-open to allow the request (200), got %d", resp.StatusCode)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("expected fail-open to resolve well under a second (200ms dial timeout), took %v", elapsed)
	}
	if got := resp.Header.Get("X-RateLimit-Limit"); got != "" {
		t.Errorf("expected no X-RateLimit-* headers on a fail-open response, got %q", got)
	}
}

// TestHTTPDistributedRateLimitSampleFailsOpenWhenRedisAddrUnset confirms
// "not configured" (CAJA_REDIS_ADDR unset entirely) shares the exact same
// fail-open path as "configured but unreachable" -- no special-casing.
func TestHTTPDistributedRateLimitSampleFailsOpenWhenRedisAddrUnset(t *testing.T) {
	cmd, stderr := startHTTPSample(t, "http_distributed_rate_limit", httpDistributedRateLimitSampleAddr)
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	resp, err := http.Get("http://" + httpDistributedRateLimitSampleAddr + "/limited")
	if err != nil {
		t.Fatalf("request failed: %v; stderr:\n%s", err, stderr.String())
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected fail-open to allow the request when CAJA_REDIS_ADDR is unset, got %d", resp.StatusCode)
	}
}

// TestHTTPRateLimiterGatesAreIndependent guards the invariant documented
// alongside http.rateLimiter/http.distributedRateLimiter in builtins.go: a
// program using only the in-memory rateLimiter must not get the distributed
// (Redis-backed) limiter's Go glue injected, and vice versa. The two
// limiters share caja_http_apply_rate_limit_headers/cajaRateLimitResult
// (extracted because both need to fill in the same X-RateLimit-* headers),
// but everything else -- the token-bucket cajaRateLimiter struct on one
// side, the RESP client/cajaDistributedRateLimiter/Redis glue on the other
// -- stays behind its own usedModules["http_rate_limiter"] /
// usedModules["http_distributed_rate_limiter"] gate. A refactor that
// accidentally merged those two gates (e.g. by conditioning either
// limiter's glue on the OR of both, instead of just its own flag) would
// slip past every other http test here, since samples/http_rate_limit and
// samples/http_distributed_rate_limit each only ever call one of the two
// limiters and never assert on what's absent from the generated source.
func TestHTTPRateLimiterGatesAreIndependent(t *testing.T) {
	transpileSample := func(t *testing.T, sampleDir string) string {
		t.Helper()
		filePath := filepath.Join("samples", sampleDir, sampleDir+".caja")
		sourceCode, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("failed to read %s: %v", filePath, err)
		}
		program, _, a, err := script.ParseWithDir(string(sourceCode), filepath.Join("samples", sampleDir), filePath)
		if err != nil {
			t.Fatalf("failed to parse script: %v", err)
		}
		goCode, err := compiler.Transpile(program, a, compiler.TranspileOptions{})
		if err != nil {
			t.Fatalf("transpilation failed: %v", err)
		}
		return goCode
	}

	t.Run("in-memory rateLimiter alone emits no distributed/Redis glue", func(t *testing.T) {
		goCode := transpileSample(t, "http_rate_limit")

		// The shared header-writing glue must still be present.
		if !strings.Contains(goCode, "caja_http_apply_rate_limit_headers") || !strings.Contains(goCode, "cajaRateLimitResult") {
			t.Errorf("expected shared rate-limit result glue to be present, got:\n%s", goCode)
		}
		if !strings.Contains(goCode, "cajaRateLimiter") {
			t.Errorf("expected the in-memory token-bucket cajaRateLimiter glue to be present, got:\n%s", goCode)
		}

		forbidden := []string{"cajaDistributedRateLimiter", "cajaRedisPool", "caja_redis_bulk", "caja_redis_build_eval", "RESP", "CAJA_REDIS_ADDR"}
		for _, sym := range forbidden {
			if strings.Contains(goCode, sym) {
				t.Errorf("expected distributed-rate-limiter-only symbol %q to be absent from a program using only http.rateLimiter, but it was present:\n%s", sym, goCode)
			}
		}
	})

	t.Run("distributedRateLimiter alone emits no in-memory token-bucket glue", func(t *testing.T) {
		goCode := transpileSample(t, "http_distributed_rate_limit")

		// The shared header-writing glue must still be present.
		if !strings.Contains(goCode, "caja_http_apply_rate_limit_headers") || !strings.Contains(goCode, "cajaRateLimitResult") {
			t.Errorf("expected shared rate-limit result glue to be present, got:\n%s", goCode)
		}
		if !strings.Contains(goCode, "cajaDistributedRateLimiter") {
			t.Errorf("expected the distributed rate limiter's glue to be present, got:\n%s", goCode)
		}

		forbidden := []string{"cajaRateLimiterEntry", "caja_http_new_rate_limiter", "evictLoop"}
		for _, sym := range forbidden {
			if strings.Contains(goCode, sym) {
				t.Errorf("expected in-memory-rate-limiter-only symbol %q to be absent from a program using only http.distributedRateLimiter, but it was present:\n%s", sym, goCode)
			}
		}
		// cajaRateLimiter (the exported/lowercase struct type name) is
		// distinct from cajaRateLimiterEntry and cajaRateLimitResult, but
		// the latter's substring overlap means a plain Contains("cajaRateLimiter")
		// check would false-positive on cajaRateLimitResult -- assert the
		// struct definition itself is absent instead.
		if strings.Contains(goCode, "type cajaRateLimiter struct") {
			t.Errorf("expected the in-memory token-bucket cajaRateLimiter struct to be absent from a program using only http.distributedRateLimiter, but it was present:\n%s", goCode)
		}
	})
}

const httpJSONSampleAddr = "127.0.0.1:8095"

// TestHTTPJSONSampleParsesRequestBodies is the end-to-end check for
// http.parseJSON, including the nested-object and nested-array cases that
// exposed a real compiler bug during development: indexing into a value
// reached through an Any-typed intermediate step (any field pulled out of a
// parsed JSON body beyond the first level) used to assume it had a .Data
// field and simply failed to compile. caja_dynamic_index (transpiler.go's
// IndexExpression case + builtins.go) fixed that with a runtime type-switch;
// this test is what would have caught the bug had it existed sooner, and
// guards against regressing it.
func TestHTTPJSONSampleParsesRequestBodies(t *testing.T) {
	cmd, stderr := startHTTPSample(t, "http_json", httpJSONSampleAddr)
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	postJSON := func(path string, jsonBody string) (int, string) {
		resp, err := http.Post("http://"+httpJSONSampleAddr+path, "application/json", strings.NewReader(jsonBody))
		if err != nil {
			t.Fatalf("request to %s failed: %v; stderr:\n%s", path, err, stderr.String())
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body from %s: %v", path, err)
		}
		return resp.StatusCode, string(body)
	}

	// Flat fields of different types, all pulled out via cast.to. Lowercase
	// keys confirm struct-built responses serialize with the same casing a
	// map-literal-built response would (caja_http_to_jsonable lowercases a
	// struct field's leading letter for exactly this reason).
	if status, body := postJSON("/users", `{"name":"abner","age":30,"active":true}`); status != 200 ||
		!strings.Contains(body, `"name":"abner"`) || !strings.Contains(body, `"age":30`) || !strings.Contains(body, `"isActive":true`) {
		t.Errorf("POST /users: expected 200 with name/age/isActive echoed back, got %d %q", status, body)
	}

	// A field one level deep (parsed["address"]["city"]) -- the nested-object
	// case that needs caja_dynamic_index.
	if status, body := postJSON("/nested", `{"address":{"city":"Lisbon"}}`); status != 200 || !strings.Contains(body, `"city":"Lisbon"`) {
		t.Errorf(`POST /nested: expected 200 with "city":"Lisbon", got %d %q`, status, body)
	}

	// A numeric index into a nested array (parsed["tags"][0]) -- the other
	// branch of caja_dynamic_index.
	if status, body := postJSON("/tags", `{"tags":["go","caja"]}`); status != 200 || !strings.Contains(body, `"first":"go"`) {
		t.Errorf(`POST /tags: expected 200 with "first":"go", got %d %q`, status, body)
	}

	// A malformed (non-JSON-object) body makes caja_http_parse_json's
	// json.Unmarshal call panic; caja_http_adapt's per-request recover must
	// turn that into a 500 rather than crashing the whole server -- the next
	// request below still succeeding is what actually proves the process
	// survived, not just this one status code.
	if status, _ := postJSON("/users", `not valid json`); status != 500 {
		t.Errorf("POST /users with a malformed body: expected 500, got %d", status)
	}

	if status, body := postJSON("/users", `{"name":"still-alive","age":1,"active":false}`); status != 200 || !strings.Contains(body, `"name":"still-alive"`) {
		t.Errorf("POST /users after a prior malformed request: expected the server to still be up and serving 200s, got %d %q", status, body)
	}
}

const httpTimeoutsSampleAddr = "127.0.0.1:8096"

// TestHTTPTimeoutsSampleClosesSlowClientConnection is a slowloris-style check
// for caja_http_listen's ReadHeaderTimeout (builtins.go, 5s): a client that
// opens a connection and sends only a partial request line, then goes
// silent, must be dropped by the server on its own after roughly that
// timeout -- not held open (and its goroutine alive) indefinitely, which is
// exactly the resource-exhaustion vector an *http.Server with no timeouts
// configured is vulnerable to. This can't be expressed as ordinary Caja
// sample code (there's no way for a .caja script to act as a slow client);
// it has to be driven from the Go test via a raw socket.
func TestHTTPTimeoutsSampleClosesSlowClientConnection(t *testing.T) {
	cmd, stderr := startHTTPSample(t, "http_timeouts", httpTimeoutsSampleAddr)
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	// A normal fast request must still work fine with the timeouts in place.
	resp, err := http.Get("http://" + httpTimeoutsSampleAddr + "/ping")
	if err != nil {
		t.Fatalf("GET /ping failed: %v; stderr:\n%s", err, stderr.String())
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || string(body) != "pong" {
		t.Errorf(`GET /ping: expected 200 "pong", got %d %q`, resp.StatusCode, string(body))
	}

	conn, err := net.Dial("tcp", httpTimeoutsSampleAddr)
	if err != nil {
		t.Fatalf("failed to dial %s: %v", httpTimeoutsSampleAddr, err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("GET /ping HTTP/1.1\r\n")); err != nil {
		t.Fatalf("failed to write partial request: %v", err)
	}

	// 9s gives generous margin over the 5s ReadHeaderTimeout this mirrors
	// (builtins.go's caja_http_read_header_timeout, unexported and so not
	// referenceable from this external test package) -- if our own deadline
	// fires first, the server never closed the connection on its own.
	_ = conn.SetReadDeadline(time.Now().Add(9 * time.Second))
	buf := make([]byte, 64)
	start := time.Now()
	n, readErr := conn.Read(buf)
	elapsed := time.Since(start)

	if netErr, ok := readErr.(net.Error); ok && netErr.Timeout() {
		t.Fatalf("server never closed the stalled connection within the test's own 9s deadline; stderr:\n%s", stderr.String())
	}
	if readErr == nil {
		t.Fatalf("expected the server to close the stalled connection, got %d bytes: %q; stderr:\n%s", n, buf[:n], stderr.String())
	}
	if elapsed < 3*time.Second {
		t.Errorf("expected the connection to stay open for roughly the 5s header timeout before the server closed it, closed after only %v", elapsed)
	}
}

const httpRoutingSampleAddr = "127.0.0.1:8097"

// TestHTTPRoutingSampleMultiValueAndWildcards is the end-to-end check for
// Request.queryAll/headersAll (multi-value query params/headers, alongside
// the existing first-value-wins query/headers) and for ":name*" wildcard
// route segments (caja_http_convert_pattern's "{name...}" conversion). The
// /files/exact + /files/:path* pair also pins the literal-beats-wildcard
// precedence documented in Go's own net/http ServeMux — easy to assume
// wrongly (that registering both would conflict, or that the wildcard would
// shadow the literal), so worth a direct regression check.
func TestHTTPRoutingSampleMultiValueAndWildcards(t *testing.T) {
	cmd, stderr := startHTTPSample(t, "http_routing", httpRoutingSampleAddr)
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	get := func(path string, headers map[string]string) (int, string) {
		req, err := http.NewRequest(http.MethodGet, "http://"+httpRoutingSampleAddr+path, nil)
		if err != nil {
			t.Fatalf("failed to build request for %s: %v", path, err)
		}
		for k, v := range headers {
			req.Header.Add(k, v)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request to %s failed: %v; stderr:\n%s", path, err, stderr.String())
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body from %s: %v", path, err)
		}
		return resp.StatusCode, string(body)
	}

	if status, body := get("/tags?tag=a&tag=b", nil); status != 200 || !strings.Contains(body, `"tags":["a","b"]`) {
		t.Errorf(`GET /tags?tag=a&tag=b: expected 200 with "tags":["a","b"], got %d %q`, status, body)
	}

	// http.Header.Add (not Set) is required to actually send the same header
	// key twice -- Set would overwrite the first value.
	req, err := http.NewRequest(http.MethodGet, "http://"+httpRoutingSampleAddr+"/header-values", nil)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	req.Header.Add("X-Tag", "one")
	req.Header.Add("X-Tag", "two")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request to /header-values failed: %v; stderr:\n%s", err, stderr.String())
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(body), `"values":["one","two"]`) {
		t.Errorf(`GET /header-values: expected 200 with "values":["one","two"], got %d %q`, resp.StatusCode, string(body))
	}

	if status, body := get("/files/exact", nil); status != 200 || body != "exact match" {
		t.Errorf(`GET /files/exact: expected 200 "exact match" (literal route, not the wildcard), got %d %q`, status, body)
	}

	if status, body := get("/files/a/b/c", nil); status != 200 || body != "a/b/c" {
		t.Errorf(`GET /files/a/b/c: expected 200 "a/b/c" (wildcard capturing the rest of the path), got %d %q`, status, body)
	}
}

const httpStaticSampleAddr = "127.0.0.1:8098"

// TestHTTPStaticSampleServesFiles is the end-to-end check for router.static:
// directory-index defaulting at the mount root, an explicit file, a nested
// asset (proving the mount serves subdirectories, not just its top level),
// a 404 for a missing file within the mount, and a literal API route
// registered on the same router still working alongside the static mount.
func TestHTTPStaticSampleServesFiles(t *testing.T) {
	cmd, stderr := startHTTPSample(t, "http_static", httpStaticSampleAddr)
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	get := func(path string) (int, string) {
		resp, err := http.Get("http://" + httpStaticSampleAddr + path)
		if err != nil {
			t.Fatalf("request to %s failed: %v; stderr:\n%s", path, err, stderr.String())
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body from %s: %v", path, err)
		}
		return resp.StatusCode, string(body)
	}

	if status, body := get("/static/"); status != 200 || !strings.Contains(body, "static ok") {
		t.Errorf("GET /static/: expected 200 with index.html content, got %d %q", status, body)
	}

	if status, body := get("/static/about.html"); status != 200 || !strings.Contains(body, "about page") {
		t.Errorf("GET /static/about.html: expected 200 with about.html content, got %d %q", status, body)
	}

	if status, body := get("/static/css/style.css"); status != 200 || !strings.Contains(body, "color: black") {
		t.Errorf("GET /static/css/style.css: expected 200 with nested css content, got %d %q", status, body)
	}

	if status, _ := get("/static/nope.html"); status != 404 {
		t.Errorf("GET /static/nope.html: expected 404 for a missing file within the mount, got %d", status)
	}

	// css/ has no index.html -- must 404 rather than fall through to Go's
	// stdlib http.FileServer's auto-generated directory listing.
	if status, _ := get("/static/css/"); status != 404 {
		t.Errorf("GET /static/css/: expected 404 (no index.html, directory listing must be suppressed), got %d", status)
	}

	// .hidden-secret exists on disk but must never be servable -- dotfiles
	// are hidden regardless of stdlib http.FileServer's default (which does
	// not hide them).
	if status, _ := get("/static/.hidden-secret"); status != 404 {
		t.Errorf("GET /static/.hidden-secret: expected 404 (dotfiles must be hidden), got %d", status)
	}

	if status, body := get("/api/ping"); status != 200 || body != "pong" {
		t.Errorf(`GET /api/ping: expected 200 "pong" from a route coexisting with the static mount, got %d %q`, status, body)
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
