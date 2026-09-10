package main

import (
	"caja-cli/internal/file"
	"caja-cli/internal/pipeline/compiler"
	"caja-cli/internal/script"
	"fmt"
	"go/format"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// standardHTTPPort is the well-known default port for plain (non-TLS) HTTP.
// Binding to it typically requires elevated privileges on Unix-like systems
// (ports below 1024 are privileged) — that's expected OS behavior, not
// something this command works around.
const standardHTTPPort = 80

// NewListenCmd creates and returns the 'listen' command: runs a .caja script
// (via the same transpile-and-`go run` path as `caja run`) as a persistent
// HTTP server, letting the invocation itself choose the bind port instead of
// whatever numeric literal the script passed to http.listen(...). The chosen
// port is threaded through as the CAJA_HTTP_PORT environment variable, which
// caja_http_listen (compiler/builtins.go) checks and prefers over its own
// port argument when set — so the deployment/run environment controls the
// actual bind port, the same way PORT env vars commonly do in other
// ecosystems, without needing any new Caja-language syntax.
func NewListenCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "listen",
		Short: "Run a caja http server script locally",
		Long:  "Run a caja script that uses the http builtin module as a live local server, optionally choosing the port it listens on.",
		RunE: func(cmd *cobra.Command, args []string) error {
			filePath, err := cmd.Flags().GetString("file")
			if err != nil {
				_ = cmd.Help()
				return fmt.Errorf("failed to retrieve 'file' flag: %w", err)
			}

			if filePath == "" {
				_ = cmd.Help()
				return fmt.Errorf("the --file flag is required to run a script")
			}

			ext := filepath.Ext(filePath)
			if ext != file.EXTENSION {
				return fmt.Errorf("invalid file type: expected a %s file, but got '%s'", file.EXTENSION, ext)
			}

			port, err := cmd.Flags().GetInt("port")
			if err != nil {
				return fmt.Errorf("failed to retrieve 'port' flag: %w", err)
			}
			if port < 1 || port > 65535 {
				return fmt.Errorf("invalid port %d: must be between 1 and 65535", port)
			}

			sourceCode, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("failed to read file '%s': %w", filePath, err)
			}

			baseDir := filepath.Dir(filePath)

			program, _, a, err := script.ParseWithDir(string(sourceCode), baseDir, filePath)
			if err != nil {
				return err
			}

			goCode, err := compiler.Transpile(program, a, compiler.TranspileOptions{})
			if err != nil {
				return fmt.Errorf("transpilation failed: %w", err)
			}

			formattedCode, err := format.Source([]byte(goCode))
			if err != nil {
				// Fall back to unformatted code if formatting fails
				formattedCode = []byte(goCode)
			}
			goCode = string(formattedCode)

			fmt.Printf("Listening on port %d...\n", port)

			extraEnv := []string{fmt.Sprintf("CAJA_HTTP_PORT=%d", port)}
			return compiler.Run(goCode, extraEnv, os.Stdin, os.Stdout, os.Stderr)
		},
	}

	cmd.Flags().StringP("file", "f", "", "File path of the http server script to run")
	cmd.Flags().IntP("port", "p", standardHTTPPort, "Port for the server to listen on")

	return cmd, nil
}
