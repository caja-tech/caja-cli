package encoder

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
)

// TestEncodeDecodeValidScript verifies a basic end-to-end compilation and decompilation
func TestEncodeDecodeValidScript(t *testing.T) {
	script := "let amount = 100\nreturn amount"
	token, err := Encode(script)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if token == "" {
		t.Fatal("Expected a token, got empty string")
	}

	program, err := Decode(token)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if !strings.Contains(program.String(), "amount") {
		t.Errorf("Expected decoded program to contain 'amount', got: %s", program.String())
	}
}

// TestEncodeDecodeEmptyScript verifies that empty strings are handled correctly
func TestEncodeDecodeEmptyScript(t *testing.T) {
	script := ""
	token, err := Encode(script)
	if err != nil {
		t.Fatalf("Encode failed for empty script: %v", err)
	}

	program, err := Decode(token)
	if err != nil {
		t.Fatalf("Decode failed for empty script: %v", err)
	}

	if program == nil {
		t.Fatal("Expected non-nil program for empty script")
	}
}

// TestEncodeInvalidSyntax verifies compilation aborts on syntax errors
func TestEncodeInvalidSyntax(t *testing.T) {
	script := "let 123 invalid syntax; @"
	_, err := Encode(script)
	if err == nil {
		t.Fatal("Expected Encode to fail with syntax errors, but it succeeded")
	}
	if !strings.Contains(err.Error(), "failed to parse") {
		t.Errorf("Expected failed to parse error, got: %v", err)
	}
}

// TestDecodeInvalidBase64 verifies decode handles bad base64 properly
func TestDecodeInvalidBase64(t *testing.T) {
	invalidToken := "not-a-base64-token!!##"
	_, err := Decode(invalidToken)
	if err == nil {
		t.Fatal("Expected Decode to fail on invalid base64")
	}
}

// TestDecodeCorruptedZlib verifies decode handles valid base64 but bad zlib data
func TestDecodeCorruptedZlib(t *testing.T) {
	badZlibToken := base64.RawURLEncoding.EncodeToString([]byte("just some random text"))
	_, err := Decode(badZlibToken)
	if err == nil {
		t.Fatal("Expected Decode to fail on corrupted zlib data")
	}
}

// TestDecodeValidZlibInvalidScript verifies decode handles syntactically invalid decompressed scripts
func TestDecodeValidZlibInvalidScript(t *testing.T) {
	invalidScript := "let let = @#$%"
	var b bytes.Buffer
	w, _ := zlib.NewWriterLevel(&b, zlib.BestCompression)
	_, err := w.Write([]byte(invalidScript))
	if err != nil {
		t.Errorf("Failed writing invalid script: %v", err)
	}
	err = w.Close()
	if err != nil {
		t.Errorf("Failed closing writer: %v", err)
	}

	token := base64.RawURLEncoding.EncodeToString(b.Bytes())

	_, err = Decode(token)
	if err == nil {
		t.Fatal("Expected Decode to fail due to script syntax errors")
	}
	if !strings.Contains(err.Error(), "failed to parse the script") {
		t.Errorf("Expected failed to parse error, got: %v", err)
	}
}

// TestEncodeLargeScript verifies the compiler can handle very large scripts
func TestEncodeLargeScript(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("let x = 0\n")
	for i := 1; i < 10000; i++ {
		sentence := fmt.Sprintf("x = %d\n", i)
		sb.WriteString(sentence)
	}
	sb.WriteString("return x")
	script := sb.String()

	token, err := Encode(script)
	if err != nil {
		t.Fatalf("Encode failed for large script: %v", err)
	}

	program, err := Decode(token)
	if err != nil {
		t.Fatalf("Decode failed for large script: %v", err)
	}

	if program == nil {
		t.Fatal("Expected non-nil program for large script")
	}
}
