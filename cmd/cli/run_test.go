package main

import (
	"bytes"
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

	if err.Error() != "the --file flag is required to run a script" {
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
