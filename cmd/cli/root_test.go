package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootCmdHelp(t *testing.T) {
	cmd, err := NewRootCmd()
	if err != nil {
		t.Fatalf("Failed to create root command: %v", err)
	}

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{}) // No args should print help

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("Root command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Usage:") {
		t.Errorf("Expected help output, got: %s", output)
	}
}
