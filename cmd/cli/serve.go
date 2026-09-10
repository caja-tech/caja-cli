package main

import (
	"caja-cli/internal/pipeline/compiler"
	"caja-cli/internal/project"
	"fmt"
	"net/http"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
)

// buildBrowserPageServer builds filePath for the browser (GOOS=js/GOARCH=wasm
// — the only target its generated syscall/js calls can build under, same
// auto-detection cmd/cli/build.go uses but required here rather than
// optional, since there's nothing to serve for a non-browser script), writes
// its test harness (wasm_exec.js + HTML loader) alongside the binary via
// compiler.WriteBrowserHarness, and returns an *http.Server (constructed,
// not yet listening) rooted at that directory plus the URL of the harness
// page to open. Split out from NewServeCmd's RunE so tests can drive the
// server's lifecycle directly (start it on a known port, hit it, shut it
// down) instead of going through Cobra or blocking forever on
// ListenAndServe.
func buildBrowserPageServer(filePath string, port int) (server *http.Server, url string, err error) {
	goCode, err := transpileCajaFile(filePath)
	if err != nil {
		return nil, "", err
	}

	if !compiler.UsesBrowserModule(goCode) {
		return nil, "", fmt.Errorf("'%s' doesn't use the browser module, so there's nothing to serve — use 'caja run' or 'caja build' instead", filePath)
	}

	outBin, _, _, _, err := resolveOutputBin(filePath, "js", "wasm", runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return nil, "", err
	}

	if err := compiler.Compile(goCode, outBin, compiler.CompileOptions{GOOS: "js", GOARCH: "wasm"}); err != nil {
		return nil, "", err
	}

	htmlPath, err := compiler.WriteBrowserHarness(outBin)
	if err != nil {
		return nil, "", fmt.Errorf("failed to write browser test harness: %w", err)
	}

	dir := filepath.Dir(htmlPath)
	server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: http.FileServer(http.Dir(dir)),
	}
	url = fmt.Sprintf("http://localhost:%d/%s", port, filepath.Base(htmlPath))
	return server, url, nil
}

// buildStaticPageServer builds filePath as a static-page project: a native
// compile (no GOOS/GOARCH override, unlike buildBrowserPageServer), then
// runs the resulting generator binary once so its page.write calls populate
// the project's dist/ directory, then returns a plain *http.Server rooted
// there — bypassing the UsesBrowserModule gate entirely, since a static-page
// project's main.caja is never expected to import browser.
func buildStaticPageServer(filePath string, port int) (server *http.Server, url string, err error) {
	goCode, err := transpileCajaFile(filePath)
	if err != nil {
		return nil, "", err
	}

	outBin, _, _, _, err := resolveOutputBin(filePath, "", "", runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return nil, "", err
	}

	if err := compiler.Compile(goCode, outBin, compiler.CompileOptions{}); err != nil {
		return nil, "", err
	}

	projectDir := filepath.Dir(filePath)
	if err := runBuiltBinaryOnce(outBin, projectDir); err != nil {
		return nil, "", err
	}

	distDir := filepath.Join(projectDir, project.StaticPageOutputDir)
	server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: http.FileServer(http.Dir(distDir)),
	}
	url = fmt.Sprintf("http://localhost:%d/", port)
	return server, url, nil
}

// NewServeCmd creates and returns the 'serve' command: builds a .caja
// browser script and serves the resulting page over HTTP on the given port,
// replacing the manual "serve this directory yourself" step `caja build`
// otherwise leaves the user with — opening the compiled page directly via
// file:// doesn't work, since Chrome blocks the wasm binary's fetch() under
// that scheme's CORS rules.
func NewServeCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Build a caja browser script and serve it over HTTP",
		RunE: func(cmd *cobra.Command, args []string) error {
			filePath, err := cmd.Flags().GetString("file")
			if err != nil {
				return fmt.Errorf("failed to retrieve 'file' flag: %w", err)
			}

			filePath, manifest, err := resolveProjectContext(filePath)
			if err != nil {
				return err
			}
			if filePath == "" {
				_ = cmd.Help()
				return fmt.Errorf("the --file flag is required to serve a script (or run this from a directory containing %s)", project.ManifestFile)
			}

			port, err := cmd.Flags().GetInt("port")
			if err != nil {
				return fmt.Errorf("failed to retrieve 'port' flag: %w", err)
			}

			var server *http.Server
			var url string
			switch {
			case manifest != nil && manifest.Type == project.TypeStaticPage:
				server, url, err = buildStaticPageServer(filePath, port)
			case manifest != nil && manifest.Type == project.TypeHTTPAPI:
				err = fmt.Errorf("%q is an http-api project — 'caja serve' doesn't apply to it; use 'caja listen' instead", filePath)
			default:
				server, url, err = buildBrowserPageServer(filePath, port)
			}
			if err != nil {
				return err
			}

			fmt.Printf("Serving %s (press Ctrl+C to stop)\n", url)
			return server.ListenAndServe()
		},
	}

	cmd.Flags().StringP("file", "f", "", "File path of the browser script to build and serve")
	cmd.Flags().IntP("port", "p", 8080, "Port to serve the built page on")

	return cmd, nil
}
