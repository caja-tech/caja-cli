package main

import (
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
