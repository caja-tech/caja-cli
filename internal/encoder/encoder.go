package encoder

import (
	"bytes"
	"caja-cli/internal/script"
	"caja-cli/internal/syntax"
	"compress/zlib"
	"encoding/base64"
	"fmt"
	"io"
)

// Encode compresses the script and returns a URL-safe Base64 token
func Encode(input string) (token string, err error) {
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

	program, parseErr := script.Parse(input)
	if parseErr != nil {
		return "", parseErr
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
func Decode(token string) (program *syntax.Program, err error) {
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

	p, parseErr := script.Parse(b.String())
	if parseErr != nil {
		return nil, parseErr
	}

	return p, nil
}
