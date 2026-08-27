package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestDecodeCmd_MissingToken(t *testing.T) {
	cmd, _ := NewDecodeCmd()
	bufOut := new(bytes.Buffer)
	cmd.SetOut(bufOut)
	cmd.SetArgs([]string{}) // No token argument

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Expected error due to missing token argument")
	}

	if err.Error() != "a single token argument is required" {
		t.Errorf("Unexpected error message: %v", err)
	}

	output := bufOut.String()
	if !strings.Contains(output, "Usage:") {
		t.Errorf("Expected help output, got: %s", output)
	}
}

func TestDecodeCmd_MissingOutputDir(t *testing.T) {
	cmd, _ := NewDecodeCmd()
	bufOut := new(bytes.Buffer)
	cmd.SetOut(bufOut)
	cmd.SetArgs([]string{"dummy_token"}) // Token present, but no --output flag

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Expected error due to missing output dir")
	}

	if err.Error() != "please provide an --output dir for the decoded scripts (e.g., --output .)" {
		t.Errorf("Unexpected error message: %v", err)
	}

	output := bufOut.String()
	if !strings.Contains(output, "Usage:") {
		t.Errorf("Expected help output, got: %s", output)
	}
}
