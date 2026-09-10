package main

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestServeCmd_RefusesNonBrowserScript confirms `caja serve` rejects a
// script that doesn't import the browser module before ever binding a port
// — there's no page to serve for it, so this should fail the same way
// `caja run` fails fast for a browser-module script it can't execute (see
// TestRunCmd_RefusesBrowserModule), rather than silently starting a server
// over an empty directory.
func TestServeCmd_RefusesNonBrowserScript(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "plain.caja")
	if err := os.WriteFile(filePath, []byte("let x = 1\n"), 0644); err != nil {
		t.Fatalf("failed to write test script: %v", err)
	}

	_, _, err := buildBrowserPageServer(filePath, 0)
	if err == nil {
		t.Fatal("expected an error for a non-browser script, got nil")
	}
	if !strings.Contains(err.Error(), "browser module") {
		t.Errorf("expected an error mentioning the browser module, got: %v", err)
	}
}

func TestServeCmd_MissingFile(t *testing.T) {
	cmd, _ := NewServeCmd()
	bufOut := new(bytes.Buffer)
	cmd.SetOut(bufOut)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error due to missing file flag")
	}
	if err.Error() != "the --file flag is required to serve a script" {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestServeCmd_ServesBuiltBrowserPage is the end-to-end check that `caja
// serve` actually serves a working page, not just that it compiles one: it
// builds a real browser-module script, starts the resulting *http.Server on
// a pre-reserved free port (rather than ListenAndServe's default of picking
// and hiding its own port, which a test can't discover), then fetches both
// the HTML harness and the compiled wasm binary over real HTTP and checks
// their content — including the wasm magic bytes, confirming the server is
// handing out a genuine WebAssembly binary and not, say, a 404 page with a
// 200 status.
func TestServeCmd_ServesBuiltBrowserPage(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "browser.caja")
	source := "import browser\n" +
		"browser.setHTML(browser.getElementById(\"app\"), \"<h1>hi</h1>\")\n"
	if err := os.WriteFile(filePath, []byte(source), 0644); err != nil {
		t.Fatalf("failed to write test script: %v", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve a free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	server, url, err := buildBrowserPageServer(filePath, port)
	if err != nil {
		t.Fatalf("buildBrowserPageServer failed: %v", err)
	}
	t.Cleanup(func() { server.Close() })

	serveErrCh := make(chan error, 1)
	go func() { serveErrCh <- server.Serve(ln) }()

	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("failed to fetch harness page at %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 fetching %s, got %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read harness page body: %v", err)
	}
	if !strings.Contains(string(body), `src="wasm_exec.js"`) {
		t.Errorf("expected harness page to reference wasm_exec.js, got:\n%s", body)
	}

	wasmURL := strings.TrimSuffix(url, filepath.Ext(url)) + ".wasm"
	wasmResp, err := http.Get(wasmURL)
	if err != nil {
		t.Fatalf("failed to fetch wasm binary at %s: %v", wasmURL, err)
	}
	defer wasmResp.Body.Close()
	wasmBytes, err := io.ReadAll(wasmResp.Body)
	if err != nil {
		t.Fatalf("failed to read wasm binary body: %v", err)
	}
	wantMagic := []byte{0x00, 'a', 's', 'm'}
	if len(wasmBytes) < 4 || !bytes.Equal(wasmBytes[:4], wantMagic) {
		got := wasmBytes
		if len(got) > 4 {
			got = got[:4]
		}
		t.Errorf("expected %s to start with wasm magic bytes %x, got %x", wasmURL, wantMagic, got)
	}

	server.Close()
	if err := <-serveErrCh; err != nil && err != http.ErrServerClosed {
		t.Errorf("server.Serve returned an unexpected error: %v", err)
	}
}
