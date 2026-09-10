package compiler

import (
	"caja-cli/internal/toolchain"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// browserHarnessHTML is a minimal, hand-written page that loads a compiled
// Caja wasm binary — boilerplate written once here, not generated per
// project (every language's wasm-on-web story needs an equivalent shell; see
// Blazor/Rust-Yew). It fetches the wasm bytes directly and calls
// WebAssembly.instantiate rather than instantiateStreaming, since the latter
// requires the serving static file server to send a correct
// "application/wasm" Content-Type header — not guaranteed from an ad hoc
// dev server (e.g. an old `python3 -m http.server`), whereas the byte-array
// path works regardless of what Content-Type was sent.
const browserHarnessHTML = `<!doctype html>
<html>
<head><meta charset="utf-8"><title>%s</title></head>
<body>
<div id="app"></div>
<script src="wasm_exec.js"></script>
<script>
const go = new Go();
fetch("%s")
	.then((resp) => resp.arrayBuffer())
	.then((bytes) => WebAssembly.instantiate(bytes, go.importObject))
	.then((result) => go.run(result.instance))
	.catch((err) => console.error("failed to load wasm module:", err));
</script>
</body>
</html>
`

// WriteBrowserHarness writes the two files a compiled browser-module wasm
// binary needs to actually run in a browser, alongside outputBin:
// wasm_exec.js (Go's own JS glue code, copied out of the toolchain rather
// than hand-maintained, so it always matches the Go version that produced
// the binary) and a matching <name>.html loader page. Callers still need to
// serve the directory over HTTP themselves (opening the HTML file directly
// via file:// blocks the wasm fetch under Chrome's CORS rules) — that's
// deliberately left to the caller rather than baked in here, since this
// package has no dev-server concept yet.
func WriteBrowserHarness(outputBin string) (htmlPath string, err error) {
	goBin, err := toolchain.EnsureToolchain()
	if err != nil {
		return "", fmt.Errorf("failed to get toolchain: %w", err)
	}
	goroot := filepath.Dir(filepath.Dir(goBin))

	wasmExecSrc := filepath.Join(goroot, "misc", "wasm", "wasm_exec.js")
	if _, err := os.Stat(wasmExecSrc); err != nil {
		// Go 1.24+ moved wasm_exec.js from misc/wasm to lib/wasm; check both
		// locations rather than assuming the toolchain.GoVersion pin never
		// moves forward.
		altSrc := filepath.Join(goroot, "lib", "wasm", "wasm_exec.js")
		if _, altErr := os.Stat(altSrc); altErr != nil {
			return "", fmt.Errorf("could not find wasm_exec.js in toolchain at %s or %s: %w", wasmExecSrc, altSrc, err)
		}
		wasmExecSrc = altSrc
	}

	dir := filepath.Dir(outputBin)
	if err := copyFile(wasmExecSrc, filepath.Join(dir, "wasm_exec.js")); err != nil {
		return "", fmt.Errorf("failed to copy wasm_exec.js: %w", err)
	}

	wasmName := filepath.Base(outputBin)
	htmlName := strings.TrimSuffix(wasmName, filepath.Ext(wasmName)) + ".html"
	htmlPath = filepath.Join(dir, htmlName)
	html := fmt.Sprintf(browserHarnessHTML, htmlName, wasmName)
	if err := os.WriteFile(htmlPath, []byte(html), 0644); err != nil {
		return "", fmt.Errorf("failed to write harness html: %w", err)
	}

	return htmlPath, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
