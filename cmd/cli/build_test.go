package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
