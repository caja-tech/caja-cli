package encoder

import (
	"bytes"
	"caja-cli/internal/file"
	"compress/zlib"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTempFile(t *testing.T, dir, name, content string) string {
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	return path
}

func TestEncodeDecodeValidScript(t *testing.T) {
	dir := t.TempDir()
	input := "let amount = 100\nreturn amount"
	writeTempFile(t, dir, file.MAIN_FILE, input)

	token, err := Encode(file.MAIN_FILE, dir)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	bundle, err := Decode(token)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if bundle.Entrypoint != file.MAIN_FILE {
		t.Errorf("Expected entrypoint %s, got %s", file.MAIN_FILE, bundle.Entrypoint)
	}
	if !strings.Contains(bundle.Modules[file.MAIN_FILE], "amount") {
		t.Errorf("Expected module content to contain 'amount'")
	}
}

func TestEncodeDecodeWithImports(t *testing.T) {
	dir := t.TempDir()
	writeTempFile(t, dir, "main.caja", "import second\nreturn 1")
	writeTempFile(t, dir, "second.caja", "return 2")

	token, err := Encode("main.caja", dir)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	bundle, err := Decode(token)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if len(bundle.Modules) != 2 {
		t.Errorf("Expected 2 modules, got %d", len(bundle.Modules))
	}
	if _, ok := bundle.Modules["second.caja"]; !ok {
		t.Errorf("Expected second.caja to be bundled")
	}
}

func TestEncodeDecodeEmptyScript(t *testing.T) {
	dir := t.TempDir()
	writeTempFile(t, dir, "main.caja", "")

	token, err := Encode("main.caja", dir)
	if err != nil {
		t.Fatalf("Encode failed for empty script: %v", err)
	}

	bundle, err := Decode(token)
	if err != nil {
		t.Fatalf("Decode failed for empty script: %v", err)
	}

	if bundle == nil {
		t.Fatal("Expected non-nil bundle for empty script")
	}
}

func TestEncodeInvalidSyntax(t *testing.T) {
	dir := t.TempDir()
	writeTempFile(t, dir, "main.caja", "let 123 invalid syntax; @")

	_, err := Encode("main.caja", dir)
	if err == nil {
		t.Fatal("Expected Encode to fail with syntax errors")
	}
}

func TestDecodeInvalidBase64(t *testing.T) {
	invalidToken := "not-a-base64-token!!##"
	_, err := Decode(invalidToken)
	if err == nil {
		t.Fatal("Expected Decode to fail on invalid base64")
	}
}

func TestDecodeCorruptedZlib(t *testing.T) {
	badZlibToken := base64.RawURLEncoding.EncodeToString([]byte("just some random text"))
	_, err := Decode(badZlibToken)
	if err == nil {
		t.Fatal("Expected Decode to fail on corrupted zlib data")
	}
}

func TestDecodeValidZlibInvalidScript(t *testing.T) {
	invalidScript := "let let = @#$%"
	var b bytes.Buffer
	w, _ := zlib.NewWriterLevel(&b, zlib.BestCompression)
	_, _ = w.Write([]byte(invalidScript))
	_ = w.Close()

	token := base64.RawURLEncoding.EncodeToString(b.Bytes())

	bundle, err := Decode(token)
	if err != nil {
		t.Fatalf("Decode shouldn't fail on bad script during bundle fallback, but it did: %v", err)
	}
	if bundle.Modules["main.caja"] != invalidScript {
		t.Errorf("Fallback mismatch")
	}
}

func TestEncodeLargeScript(t *testing.T) {
	dir := t.TempDir()
	var sb strings.Builder
	sb.WriteString("let x = 0\n")
	for i := 1; i < 10000; i++ {
		sentence := fmt.Sprintf("x = %d\n", i)
		sb.WriteString(sentence)
	}
	sb.WriteString("return x")

	writeTempFile(t, dir, "main.caja", sb.String())

	token, err := Encode("main.caja", dir)
	if err != nil {
		t.Fatalf("Encode failed for large script: %v", err)
	}

	bundle, err := Decode(token)
	if err != nil {
		t.Fatalf("Decode failed for large script: %v", err)
	}

	if bundle == nil {
		t.Fatal("Expected non-nil bundle for large script")
	}
}
