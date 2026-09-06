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
