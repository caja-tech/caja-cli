package encoder

import (
	"bytes"
	"caja-cli/internal/file"
	"caja-cli/internal/lexer"
	"caja-cli/internal/syntax"
	"compress/zlib"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type Bundle struct {
	Entrypoint string            `json:"entrypoint"`
	Modules    map[string]string `json:"modules"`
}

// Encode compresses a script and all its dependencies into a URL-safe Base64 token
func Encode(entryFile string, baseDir string) (token string, err error) {
	bundle := Bundle{
		Entrypoint: filepath.Base(entryFile),
		Modules:    make(map[string]string),
	}

	var collect func(filename string) error
	collect = func(filename string) error {
		if _, exists := bundle.Modules[filename]; exists {
			return nil // already collected
		}

		path := filepath.Join(baseDir, filename)
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("failed to read %s: %w", path, readErr)
		}

		source := string(content)
		bundle.Modules[filename] = source

		tknzr := lexer.New(source)
		parser := syntax.New(tknzr)
		prog := parser.Parse()
		if parser.HasErrors() {
			return fmt.Errorf("failed to parse %s", filename)
		}

		for _, stmt := range prog.Statements {
			if importStmt, ok := stmt.(*syntax.ImportStatement); ok {
				collectErr := collect(filepath.FromSlash(importStmt.Path) + file.EXTENSION)
				if collectErr != nil {
					return collectErr
				}
			}
		}

		return nil
	}

	if err := collect(bundle.Entrypoint); err != nil {
		return "", err
	}

	jsonData, err := json.Marshal(bundle)
	if err != nil {
		return "", err
	}

	var b bytes.Buffer
	w, zlibErr := zlib.NewWriterLevel(&b, zlib.BestCompression)
	if zlibErr != nil {
		return "", zlibErr
	}

	_, writeErr := w.Write(jsonData)
	if writeErr != nil {
		_ = w.Close()
		return "", writeErr
	}

	if closeErr := w.Close(); closeErr != nil {
		return "", fmt.Errorf("failed to close writer: %w", closeErr)
	}

	return base64.RawURLEncoding.EncodeToString(b.Bytes()), nil
}

// Decode reverses the token back into a Bundle
func Decode(token string) (bundle *Bundle, err error) {
	compressedData, decodeErr := base64.RawURLEncoding.DecodeString(token)
	if decodeErr != nil {
		return nil, decodeErr
	}

	reader, zlibErr := zlib.NewReader(bytes.NewReader(compressedData))
	if zlibErr != nil {
		return nil, zlibErr
	}
	defer func(r io.ReadCloser) {
		closeErr := r.Close()
		if closeErr != nil {
			err = closeErr
		}
	}(reader)

	var buff bytes.Buffer
	_, copyErr := io.Copy(&buff, reader)
	if copyErr != nil {
		return nil, copyErr
	}

	if jsonErr := json.Unmarshal(buff.Bytes(), &bundle); jsonErr != nil {
		// Fallback for old single-script tokens
		innerBundle := Bundle{
			Entrypoint: file.MAIN_FILE,
			Modules: map[string]string{
				file.MAIN_FILE: buff.String(),
			},
		}
		return &innerBundle, err
	}

	return bundle, err
}
