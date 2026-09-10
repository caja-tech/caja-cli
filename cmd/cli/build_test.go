package main

import (
	"bytes"
	"caja-cli/internal/project"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestResolveOutputBin_HostBuildUnchanged confirms building for the host
// (no --os/--arch given) produces the exact same naming as before these
// flags existed — no suffix, regardless of what the host happens to be.
func TestResolveOutputBin_HostBuildUnchanged(t *testing.T) {
	outBin, resolvedOS, resolvedArch, crossCompiling, err := resolveOutputBin("/scripts/myprog.caja", "", "", "darwin", "arm64")
	if err != nil {
		t.Fatalf("resolveOutputBin failed: %v", err)
	}
	if crossCompiling {
		t.Error("expected crossCompiling to be false when neither --os nor --arch is given")
	}
	if resolvedOS != "darwin" || resolvedArch != "arm64" {
		t.Errorf("expected resolved os/arch to fall back to host (darwin/arm64), got %s/%s", resolvedOS, resolvedArch)
	}
	if outBin != "/scripts/myprog" {
		t.Errorf("expected unsuffixed host build name /scripts/myprog, got %s", outBin)
	}
}

// TestResolveOutputBin_CrossCompileSuffix confirms either flag alone
// triggers the full "-{os}-{arch}" suffix, with the untouched dimension
// resolved to the host's own value rather than left ambiguous.
func TestResolveOutputBin_CrossCompileSuffix(t *testing.T) {
	tests := []struct {
		name       string
		targetOS   string
		targetArch string
		wantBin    string
	}{
		{"os only, arch defaults to host", "linux", "", "/scripts/myprog-linux-arm64"},
		{"arch only, os defaults to host", "", "amd64", "/scripts/myprog-darwin-amd64"},
		{"both given", "linux", "amd64", "/scripts/myprog-linux-amd64"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outBin, _, _, crossCompiling, err := resolveOutputBin("/scripts/myprog.caja", tt.targetOS, tt.targetArch, "darwin", "arm64")
			if err != nil {
				t.Fatalf("resolveOutputBin failed: %v", err)
			}
			if !crossCompiling {
				t.Error("expected crossCompiling to be true when either flag is given")
			}
			if outBin != tt.wantBin {
				t.Errorf("expected %s, got %s", tt.wantBin, outBin)
			}
		})
	}
}

// TestResolveOutputBin_WindowsGetsExeSuffix confirms a windows target always
// ends in .exe, since caja can't rely on go build's implicit -o behavior for
// an explicitly-named output path (see resolveOutputBin's comment).
func TestResolveOutputBin_WindowsGetsExeSuffix(t *testing.T) {
	outBin, _, _, _, err := resolveOutputBin("/scripts/myprog.caja", "windows", "amd64", "darwin", "arm64")
	if err != nil {
		t.Fatalf("resolveOutputBin failed: %v", err)
	}
	if !strings.HasSuffix(outBin, ".exe") {
		t.Errorf("expected a windows target to end in .exe, got %s", outBin)
	}
}

// TestResolveOutputBin_JsGetsWasmSuffix confirms a js target (used for the
// browser module, which only builds under GOOS=js/GOARCH=wasm) always ends
// in .wasm, mirroring the windows/.exe case above.
func TestResolveOutputBin_JsGetsWasmSuffix(t *testing.T) {
	outBin, _, _, _, err := resolveOutputBin("/scripts/myprog.caja", "js", "wasm", "darwin", "arm64")
	if err != nil {
		t.Fatalf("resolveOutputBin failed: %v", err)
	}
	if !strings.HasSuffix(outBin, ".wasm") {
		t.Errorf("expected a js target to end in .wasm, got %s", outBin)
	}
}

// TestBuildCmd_BrowserModuleDefaultsToWasm is an end-to-end check of the
// `caja build` auto-defaulting logic: a script that imports the browser
// module, built with neither --os nor --arch given, must still succeed by
// silently targeting GOOS=js/GOARCH=wasm (the only target the module's
// generated syscall/js calls can build under) instead of failing with a
// raw "build constraints exclude all Go files" error on the host platform,
// and the resulting binary must be named with the .wasm suffix and contain
// real WebAssembly output.
func TestBuildCmd_BrowserModuleDefaultsToWasm(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "browser.caja")
	source := "import browser\nbrowser.log(\"hello\")\n"
	if err := os.WriteFile(filePath, []byte(source), 0644); err != nil {
		t.Fatalf("failed to write test script: %v", err)
	}

	cmd, _ := NewBuildCmd()
	bufOut := new(bytes.Buffer)
	cmd.SetOut(bufOut)
	cmd.SetArgs([]string{"--file", filePath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected browser-module build to succeed by auto-defaulting to js/wasm, got error: %v\noutput:\n%s", err, bufOut.String())
	}

	// crossCompiling becomes true once targetOS/targetArch are auto-filled
	// to "js"/"wasm", so resolveOutputBin also appends the "-js-wasm" name
	// suffix on top of the .wasm extension (see resolveOutputBin).
	wantBin := filepath.Join(dir, "browser-js-wasm.wasm")
	data, err := os.ReadFile(wantBin)
	if err != nil {
		t.Fatalf("expected wasm binary at %s, got error reading it: %v (build output:\n%s)", wantBin, err, bufOut.String())
	}
	t.Cleanup(func() { os.Remove(wantBin) })

	wantMagic := []byte{0x00, 'a', 's', 'm'} // WebAssembly binary magic number
	if len(data) < 4 || !bytes.Equal(data[:4], wantMagic) {
		got := data
		if len(got) > 4 {
			got = got[:4]
		}
		t.Errorf("expected %s to start with wasm magic bytes %x, got %x", wantBin, wantMagic, got)
	}

	// The auto-defaulted js/wasm build must also emit the browser test
	// harness (wasm_exec.js + HTML loader) a wasm binary needs to actually
	// run anywhere, since there's no other reason to produce a GOOS=js
	// binary from this CLI today (see compiler.WriteBrowserHarness).
	if _, err := os.Stat(filepath.Join(dir, "wasm_exec.js")); err != nil {
		t.Errorf("expected wasm_exec.js to be written alongside the binary: %v", err)
	}
	wantHTML := filepath.Join(dir, "browser-js-wasm.html")
	if _, err := os.Stat(wantHTML); err != nil {
		t.Errorf("expected harness html at %s: %v", wantHTML, err)
	}
}

// TestBuildCmd_StaticPageAutoDiscovery checks the no-"--file" project-aware
// path: with a cajaproj.yml declaring type: static-page sitting in the cwd,
// `caja build` should find main.caja on its own, compile it natively, run
// the resulting binary once, and leave the generated dist/ output behind.
func TestBuildCmd_StaticPageAutoDiscovery(t *testing.T) {
	dir := t.TempDir()
	source := "import page\npage.write(\"dist/index.html\", \"hello from static-page\")\n"
	if err := os.WriteFile(filepath.Join(dir, "main.caja"), []byte(source), 0644); err != nil {
		t.Fatalf("failed to write main.caja: %v", err)
	}
	manifest := &project.Manifest{Name: "demo", Type: project.TypeStaticPage, CajaVersion: "dev"}
	if err := project.Save(dir, manifest); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}
	chdir(t, dir)

	cmd, _ := NewBuildCmd()
	bufOut := new(bytes.Buffer)
	cmd.SetOut(bufOut)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected build to succeed via manifest auto-discovery, got: %v\noutput:\n%s", err, bufOut.String())
	}

	got, err := os.ReadFile(filepath.Join(dir, "dist", "index.html"))
	if err != nil {
		t.Fatalf("expected dist/index.html to be written: %v", err)
	}
	if string(got) != "hello from static-page" {
		t.Errorf("dist/index.html = %q, want %q", got, "hello from static-page")
	}
}

// TestBuildCmd_WebAppAutoDiscoveryWritesIndexHTML checks that a manifest-
// declared web-app project's build also writes index.html alongside the
// usual <name>.html harness, so the output directory is servable at a bare
// domain root by any static host with zero extra configuration.
func TestBuildCmd_WebAppAutoDiscoveryWritesIndexHTML(t *testing.T) {
	dir := t.TempDir()
	source := "import browser\nbrowser.log(\"hello\")\n"
	if err := os.WriteFile(filepath.Join(dir, "main.caja"), []byte(source), 0644); err != nil {
		t.Fatalf("failed to write main.caja: %v", err)
	}
	manifest := &project.Manifest{Name: "demo", Type: project.TypeWebApp, CajaVersion: "dev"}
	if err := project.Save(dir, manifest); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}
	chdir(t, dir)

	cmd, _ := NewBuildCmd()
	bufOut := new(bytes.Buffer)
	cmd.SetOut(bufOut)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected build to succeed via manifest auto-discovery, got: %v\noutput:\n%s", err, bufOut.String())
	}

	// crossCompiling becomes true once targetOS/targetArch are auto-filled
	// to "js"/"wasm" for the declared web-app type (see resolveOutputBin),
	// so the harness is named main-js-wasm.html, not main.html.
	wantHTML := filepath.Join(dir, "main-js-wasm.html")
	if _, err := os.Stat(wantHTML); err != nil {
		t.Errorf("expected harness html at %s: %v", wantHTML, err)
	}
	wantIndex := filepath.Join(dir, "index.html")
	indexData, err := os.ReadFile(wantIndex)
	if err != nil {
		t.Fatalf("expected index.html to be written alongside the harness: %v", err)
	}
	harnessData, err := os.ReadFile(wantHTML)
	if err != nil {
		t.Fatalf("failed to read harness html: %v", err)
	}
	if string(indexData) != string(harnessData) {
		t.Errorf("expected index.html to mirror the harness html content")
	}
}

// TestBuildCmd_HTTPAPIBuildsRunnableBinary is the end-to-end check for a
// manifest-declared http-api project: build.go deliberately has no
// http-api-specific branch (unlike static-page and web-app), relying on the
// http builtin module compiling like any other native program — so this
// confirms that's actually true by starting the built binary directly (not
// via `go run`, unlike the compiler package's own http-module tests) and
// hitting it over real HTTP, using the same CAJA_HTTP_PORT override
// `caja listen` relies on (see caja_http_listen in builtins.go) to bind a
// pre-reserved free port instead of the script's hardcoded 8080.
func TestBuildCmd_HTTPAPIBuildsRunnableBinary(t *testing.T) {
	dir := t.TempDir()
	source := "import \"http\" as http\nlet router = http.newRouter()\nrouter.get(\"/\", fn(req: http.Request) -> http.Response { return http.ok(\"hi from http-api\") })\nhttp.listen(router, 8080)\n"
	if err := os.WriteFile(filepath.Join(dir, "main.caja"), []byte(source), 0644); err != nil {
		t.Fatalf("failed to write main.caja: %v", err)
	}
	manifest := &project.Manifest{Name: "demo", Type: project.TypeHTTPAPI, CajaVersion: "dev"}
	if err := project.Save(dir, manifest); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}
	chdir(t, dir)

	cmd, _ := NewBuildCmd()
	bufOut := new(bytes.Buffer)
	cmd.SetOut(bufOut)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected http-api build to succeed via manifest auto-discovery, got: %v\noutput:\n%s", err, bufOut.String())
	}

	outBin := filepath.Join(dir, "main")
	if _, err := os.Stat(outBin); err != nil {
		t.Fatalf("expected a native binary at %s: %v", outBin, err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve a free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	runCmd := exec.Command(outBin)
	runCmd.Env = append(os.Environ(), fmt.Sprintf("CAJA_HTTP_PORT=%d", port))
	var stderr bytes.Buffer
	runCmd.Stderr = &stderr
	if err := runCmd.Start(); err != nil {
		t.Fatalf("failed to start the built binary: %v", err)
	}
	t.Cleanup(func() {
		_ = runCmd.Process.Kill()
		_ = runCmd.Wait()
	})

	addr := fmt.Sprintf("http://127.0.0.1:%d/", port)
	deadline := time.Now().Add(5 * time.Second)
	var resp *http.Response
	for {
		resp, err = http.Get(addr)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("built binary never accepted connections on %s within timeout; stderr:\n%s", addr, stderr.String())
		}
		time.Sleep(50 * time.Millisecond)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	if resp.StatusCode != http.StatusOK || string(body) != "hi from http-api" {
		t.Errorf("GET %s: expected 200 %q, got %d %q", addr, "hi from http-api", resp.StatusCode, body)
	}
}
