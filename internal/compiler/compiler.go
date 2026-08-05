package compiler

import (
	"bytes"
	"caja-cli/internal/lexer"
	"caja-cli/internal/semantic"
	"caja-cli/internal/syntax"
	"compress/zlib"
	"encoding/base64"
	"fmt"
	"io"
)

// Encode compresses the script and returns a URL-safe Base64 token
func Encode(script string) (token string, err error) {
	var b bytes.Buffer

	w, zlibErr := zlib.NewWriterLevel(&b, zlib.BestCompression)
	if zlibErr != nil {
		return "", zlibErr
	}

	defer func(w *zlib.Writer) {
		closeErr := w.Close()
		if closeErr != nil {
			if err == nil {
				err = fmt.Errorf("failed to close writer: %w", closeErr)
			}
		}
	}(w)

	tknzr := lexer.New(script)
	p := syntax.New(tknzr)
	program := p.Parse()

	hasSyntaticOrSemanticErrors := false
	if len(p.Errors()) > 0 {
		fmt.Println("Cannot compile: script contains syntax errors:")
		for _, msg := range p.Errors() {
			fmt.Printf("\t- %s\n", msg)
		}
		hasSyntaticOrSemanticErrors = true
	} else {
		analyzer := semantic.New()
		analyzer.Analyze(program)
		if len(analyzer.Errors()) > 0 {
			fmt.Println("Semantic errors found:")
			for _, msg := range analyzer.Errors() {
				fmt.Printf("\t- %s\n", msg)
			}
			hasSyntaticOrSemanticErrors = true
		}
	}

	if hasSyntaticOrSemanticErrors {
		return "", fmt.Errorf("failed to parse the script")
	}

	_, writeErr := w.Write([]byte(program.String()))
	if writeErr != nil {
		return "", writeErr
	}

	if closeErr := w.Close(); closeErr != nil {
		return "", fmt.Errorf("failed to close writer: %w", closeErr)
	}

	return base64.RawURLEncoding.EncodeToString(b.Bytes()), nil
}

// Decode reverses the token back into the original script string
func Decode(token string) (script *syntax.Program, err error) {
	compressedData, decodeErr := base64.RawURLEncoding.DecodeString(token)
	if decodeErr != nil {
		return nil, decodeErr
	}

	r, zlibErr := zlib.NewReader(bytes.NewReader(compressedData))
	if zlibErr != nil {
		return nil, zlibErr
	}

	defer func(r io.ReadCloser) {
		closeErr := r.Close()
		if closeErr != nil {
			if err == nil {
				err = fmt.Errorf("failed to close reader: %w", closeErr)
			}
		}
	}(r)

	var b bytes.Buffer
	_, copyErr := io.Copy(&b, r)
	if copyErr != nil {
		return nil, copyErr
	}

	decodedScript := b.String()
	tknzr := lexer.New(decodedScript)
	p := syntax.New(tknzr)
	program := p.Parse()

	if len(p.Errors()) > 0 {
		fmt.Println("Cannot decompile: script contains syntax errors:")
		for _, msg := range p.Errors() {
			fmt.Printf("\t- %s\n", msg)
		}
		return nil, fmt.Errorf("decompilation aborted")
	}

	return program, nil
}
