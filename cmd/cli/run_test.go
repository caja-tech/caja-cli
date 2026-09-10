package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCmd_MissingFile(t *testing.T) {
	cmd, _ := NewRunCmd()
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

func TestRunCmd_InvalidExtension(t *testing.T) {
	cmd, _ := NewRunCmd()
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

// TestRunCmd_RefusesBrowserModule confirms `caja run` rejects a script that
// imports the browser module before ever attempting `go run` on it — the
// browser module's generated syscall/js calls only build under GOOS=js/
// GOARCH=wasm, which `go run` (unlike `caja build`) can't target, so this
// must fail fast with a clear message instead of a confusing build error.
func TestRunCmd_RefusesBrowserModule(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "browser.caja")
	source := "import browser\nbrowser.log(\"hello\")\n"
	if err := os.WriteFile(filePath, []byte(source), 0644); err != nil {
		t.Fatalf("failed to write test script: %v", err)
	}

	cmd, _ := NewRunCmd()
	bufOut := new(bytes.Buffer)
	cmd.SetOut(bufOut)
	cmd.SetArgs([]string{"--file", filePath})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected an error refusing to run a browser-module script")
	}
	if !strings.Contains(err.Error(), "caja build") || !strings.Contains(err.Error(), "browser module") {
		t.Errorf("expected an error explaining the browser module needs 'caja build', got: %v", err)
	}
}
