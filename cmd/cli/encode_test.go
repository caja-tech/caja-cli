package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestEncodeCmd_MissingFile(t *testing.T) {
	cmd, _ := NewEncodeCmd()
	bufOut := new(bytes.Buffer)
	cmd.SetOut(bufOut)
	cmd.SetArgs([]string{}) // No file flag

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Expected error due to missing file flag")
	}

	if err.Error() != "the --file flag is required to encode a script" {
		t.Errorf("Unexpected error message: %v", err)
	}

	output := bufOut.String()
	if !strings.Contains(output, "Usage:") {
		t.Errorf("Expected help output, got: %s", output)
	}
}
