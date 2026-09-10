package main

import (
	"bytes"
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

	if err.Error() != "the --file flag is required to run a script" {
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
