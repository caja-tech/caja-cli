package main

import (
	"bytes"
	"caja-cli/internal/project"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServeCmd_MissingFile(t *testing.T) {
	cmd, _ := NewServeCmd()
	bufOut := new(bytes.Buffer)
	cmd.SetOut(bufOut)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error due to missing file flag")
	}
	if err.Error() != "the --file flag is required to serve a script (or run this from a directory containing cajaproj.yml)" {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestServeCmd_ServesStaticPageDist is the end-to-end check for a
// manifest-declared static-page project: `caja serve` should build it
// natively, run the generator binary once, and serve the resulting dist/
// directory over HTTP — mirroring TestServeCmd_ServesBuiltBrowserPage's
// approach of fetching over a real, pre-reserved port.
func TestServeCmd_ServesStaticPageDist(t *testing.T) {
	dir := t.TempDir()
	source := "import doc\ndoc.write(\"dist/index.html\", \"hello from static-page\")\n"
	if err := os.WriteFile(filepath.Join(dir, "main.caja"), []byte(source), 0644); err != nil {
		t.Fatalf("failed to write main.caja: %v", err)
	}
	// Same assets/ fixture as TestBuildCmd_StaticPageAutoDiscovery: serve
	// must mirror it into dist/assets/ too (it goes through the shared
	// generateWebApp), and the mirrored file must then be reachable
	// over HTTP under the served root.
	writeProjectFile(t, dir, filepath.Join("assets", "logo.txt"), "logo")
	writeProjectFile(t, dir, filepath.Join("assets", "img", "pixel.txt"), "pixel")
	writeProjectFile(t, dir, filepath.Join("dist", "assets", "stale.txt"), "old")

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve a free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	filePath := filepath.Join(dir, "main.caja")
	server, url, err := buildWebAppServer(filePath, port)
	if err != nil {
		t.Fatalf("buildWebAppServer failed: %v", err)
	}
	t.Cleanup(func() { server.Close() })

	serveErrCh := make(chan error, 1)
	go func() { serveErrCh <- server.Serve(ln) }()

	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("failed to fetch %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 fetching %s, got %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	if string(body) != "hello from static-page" {
		t.Errorf("response body = %q, want %q", body, "hello from static-page")
	}

	assertStaticPageAssets(t, dir)

	assetResp, err := http.Get(url + "assets/logo.txt")
	if err != nil {
		t.Fatalf("failed to fetch mirrored asset: %v", err)
	}
	defer assetResp.Body.Close()
	assetBody, err := io.ReadAll(assetResp.Body)
	if err != nil {
		t.Fatalf("failed to read asset body: %v", err)
	}
	if assetResp.StatusCode != http.StatusOK || string(assetBody) != "logo" {
		t.Errorf("GET assets/logo.txt = %d %q, want 200 %q", assetResp.StatusCode, assetBody, "logo")
	}

	server.Close()
	if err := <-serveErrCh; err != nil && err != http.ErrServerClosed {
		t.Errorf("server.Serve returned an unexpected error: %v", err)
	}
}

// TestServeCmd_RejectsHTTPAPIProject confirms `caja serve` refuses to run
// against a manifest-declared http-api project with a clear "doesn't apply"
// error, rather than trying (and failing confusingly) to treat its main.caja
// as a browser or static-page script.
func TestServeCmd_RejectsHTTPAPIProject(t *testing.T) {
	dir := t.TempDir()
	source := "import \"http\" as http\nhttp.listen(http.newRouter(), 8080)\n"
	if err := os.WriteFile(filepath.Join(dir, "main.caja"), []byte(source), 0644); err != nil {
		t.Fatalf("failed to write main.caja: %v", err)
	}
	manifest := &project.Manifest{Name: "demo", Type: project.TypeHTTPAPI, CajaVersion: "dev"}
	if err := project.Save(dir, manifest); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}
	chdir(t, dir)

	cmd, _ := NewServeCmd()
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected caja serve to refuse an http-api project")
	}
	if !strings.Contains(err.Error(), "http-api project") {
		t.Errorf("expected an error mentioning the http-api project type, got: %v", err)
	}
}
