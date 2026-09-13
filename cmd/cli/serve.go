package main

import (
	"caja-cli/internal/pipeline/compiler"
	"caja-cli/internal/project"
	"fmt"
	"mime"
	"net/http"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
)

// buildWebAppServer builds filePath as a static-page project: a native
// compile (no GOOS/GOARCH override, unlike buildBrowserPageServer), then
// runs the resulting generator binary once so its doc.write calls populate
// the project's dist/ directory, then returns a plain *http.Server rooted
// there. A static-page
// project's main.caja is never expected to import browser.
func init() {
	// Go's mime table has no entry for .webmanifest, so a manifest would go
	// out as text/plain. Browsers mostly tolerate that, but Chrome logs a
	// warning and Lighthouse counts it against installability — and the
	// whole point of this project type is to produce something installable.
	_ = mime.AddExtensionType(".webmanifest", "application/manifest+json")
}

func buildWebAppServer(filePath string, port int) (server *http.Server, url string, err error) {
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
	if err := generateWebApp(outBin, projectDir); err != nil {
		return nil, "", err
	}

	distDir := filepath.Join(projectDir, project.OutputDir)
	server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: http.FileServer(http.Dir(distDir)),
	}
	url = fmt.Sprintf("http://localhost:%d/", port)
	return server, url, nil
}

// NewServeCmd creates and returns the 'serve' command: builds a project and
// serves its generated output over HTTP on the given port, replacing the
// manual "serve this directory yourself" step `caja build` otherwise leaves
// the user with. Serving over HTTP rather than opening the files via file://
// is not a convenience: a service worker will not register under file://,
// and neither will a web app manifest resolve, so a PWA is simply not
// testable that way.
func NewServeCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Build a caja project and serve its output over HTTP",
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
			case manifest != nil && manifest.Type == project.TypeHTTPAPI:
				err = fmt.Errorf("%q is an http-api project — 'caja serve' doesn't apply to it; use 'caja listen' instead", filePath)
			default:
				// Every other project builds a directory of static files, so
				// there is one serving path. A standalone script with no
				// manifest lands here too and is handled by the same builder.
				server, url, err = buildWebAppServer(filePath, port)
			}
			if err != nil {
				return err
			}

			fmt.Printf("Serving %s (press Ctrl+C to stop)\n", url)
			return server.ListenAndServe()
		},
	}

	cmd.Flags().StringP("file", "f", "", "File path of the script to build and serve")
	cmd.Flags().IntP("port", "p", 8080, "Port to serve the built page on")

	return cmd, nil
}
