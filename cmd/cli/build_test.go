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

// TestBuildCmd_WebAppAutoDiscovery checks the no-"--file" project-aware
// path: with a cajaproj.yml declaring type: web-app sitting in the cwd,
// `caja build` should find main.caja on its own, compile it natively, run
// the resulting binary once, and leave the generated dist/ output behind.
func TestBuildCmd_WebAppAutoDiscovery(t *testing.T) {
	dir := t.TempDir()
	source := "import doc\ndoc.write(\"dist/index.html\", \"hello from web-app\")\n"
	if err := os.WriteFile(filepath.Join(dir, "main.caja"), []byte(source), 0644); err != nil {
		t.Fatalf("failed to write main.caja: %v", err)
	}
	manifest := &project.Manifest{Name: "demo", Type: project.TypeWebApp, CajaVersion: "dev"}
	if err := project.Save(dir, manifest); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}
	// assets/ (including a nested folder) must be mirrored into dist/assets/,
	// and a stale file already sitting in dist/assets/ from a previous build
	// must NOT survive — dist/assets is recreated from scratch each time.
	writeProjectFile(t, dir, filepath.Join("assets", "logo.txt"), "logo")
	writeProjectFile(t, dir, filepath.Join("assets", "img", "pixel.txt"), "pixel")
	writeProjectFile(t, dir, filepath.Join("dist", "assets", "stale.txt"), "old")
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
	if string(got) != "hello from web-app" {
		t.Errorf("dist/index.html = %q, want %q", got, "hello from web-app")
	}

	assertStaticPageAssets(t, dir)
}

// writeProjectFile writes content to rel inside dir, creating parents.
func writeProjectFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatalf("failed to create %s: %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write %s: %v", full, err)
	}
}

// assertStaticPageAssets checks the assets/ → dist/assets/ mirror that
// generateWebApp performs, for a project prepared with writeProjectFile
// the way TestBuildCmd_WebAppAutoDiscovery does. Shared with the serve
// test so both commands are held to the same contract.
func assertStaticPageAssets(t *testing.T, dir string) {
	t.Helper()
	for rel, want := range map[string]string{
		filepath.Join("dist", "assets", "logo.txt"):         "logo",
		filepath.Join("dist", "assets", "img", "pixel.txt"): "pixel",
	} {
		got, err := os.ReadFile(filepath.Join(dir, rel))
		if err != nil {
			t.Errorf("expected %s to be copied from assets/: %v", rel, err)
			continue
		}
		if string(got) != want {
			t.Errorf("%s = %q, want %q", rel, got, want)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "dist", "assets", "stale.txt")); !os.IsNotExist(err) {
		t.Errorf("expected stale dist/assets/stale.txt to be removed by the rebuild, stat err = %v", err)
	}
}

// TestBuildCmd_HTTPAPIBuildsRunnableBinary is the end-to-end check for a
// manifest-declared http-api project: build.go deliberately has no
// http-api-specific branch (unlike web-app and web-app), relying on the
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
