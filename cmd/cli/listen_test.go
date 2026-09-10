package main

import (
	"bytes"
	"caja-cli/internal/project"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListenCmd_MissingFile(t *testing.T) {
	cmd, _ := NewListenCmd()
	bufOut := new(bytes.Buffer)
	bufErr := new(bytes.Buffer)
	cmd.SetOut(bufOut)
	cmd.SetErr(bufErr)
	cmd.SetArgs([]string{}) // No file flag

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Expected error due to missing file flag")
	}

	if err.Error() != "the --file flag is required to run a script (or run this from a directory containing cajaproj.yml)" {
		t.Errorf("Unexpected error message: %v", err)
	}

	output := bufOut.String()
	if !strings.Contains(output, "Usage:") {
		t.Errorf("Expected help output, got: %s", output)
	}
}

func TestListenCmd_InvalidExtension(t *testing.T) {
	cmd, _ := NewListenCmd()
	bufOut := new(bytes.Buffer)
	cmd.SetOut(bufOut)
	cmd.SetArgs([]string{"--file", "test.txt"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Expected error due to invalid file extension")
	}

	if !strings.Contains(err.Error(), "invalid file type") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestListenCmd_DefaultsToStandardHTTPPort(t *testing.T) {
	cmd, _ := NewListenCmd()
	port, err := cmd.Flags().GetInt("port")
	if err != nil {
		t.Fatalf("failed to read default 'port' flag: %v", err)
	}
	if port != 80 {
		t.Errorf("expected the default port to be the standard HTTP port 80, got %d", port)
	}
}

func TestListenCmd_FileNotFound(t *testing.T) {
	cmd, _ := NewListenCmd()
	bufOut := new(bytes.Buffer)
	cmd.SetOut(bufOut)
	cmd.SetArgs([]string{"--file", "does-not-exist.caja"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected an error for a nonexistent file")
	}
	if !strings.Contains(err.Error(), "failed to read file") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestListenCmd_UsesManifestFileWhenNoFileFlag checks the no-"--file"
// project-aware path: with a cajaproj.yml declaring type: http-api sitting
// in the cwd, `caja listen` should find main.caja on its own rather than
// requiring an explicit --file, matching build/run/serve's auto-discovery.
// Only the resolution step is exercised here (via an out-of-range port,
// which fails fast before ever transpiling or binding a socket) — actually
// starting the server is covered by the http module's own compiler tests.
func TestListenCmd_UsesManifestFileWhenNoFileFlag(t *testing.T) {
	dir := t.TempDir()
	source := "import \"http\" as http\nhttp.listen(http.newRouter(), 8080)\n"
	if err := os.WriteFile(filepath.Join(dir, "main.caja"), []byte(source), 0644); err != nil {
		t.Fatalf("failed to write main.caja: %v", err)
	}
	manifest := &project.Manifest{Name: "demo", Type: project.TypeHTTPAPI, CajaVersion: "dev"}
	if err := project.Save(dir, manifest); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}
	chdir(t, dir)

	cmd, _ := NewListenCmd()
	cmd.SetArgs([]string{"--port", "0"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected an error for out-of-range port 0")
	}
	if !strings.Contains(err.Error(), "invalid port") {
		t.Errorf("expected the manifest-resolved file to reach the port check (not a missing --file error), got: %v", err)
	}
}

func TestListenCmd_RejectsOutOfRangePort(t *testing.T) {
	for _, port := range []string{"0", "-1", "65536", "999999"} {
		t.Run(port, func(t *testing.T) {
			cmd, _ := NewListenCmd()
			bufOut := new(bytes.Buffer)
			cmd.SetOut(bufOut)
			cmd.SetArgs([]string{"--file", "test.caja", "--port", port})

			err := cmd.Execute()
			if err == nil {
				t.Fatalf("expected an error for out-of-range port %s", port)
			}
			if !strings.Contains(err.Error(), "invalid port") {
				t.Errorf("unexpected error message: %v", err)
			}
		})
	}
}
