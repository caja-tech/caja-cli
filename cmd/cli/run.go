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

// NewRunCmd creates and returns the 'run' command, which transpiles a .caja
// script to Go and runs it via `go run` — giving interpreter-like ergonomics
// (no binary left behind) while executing through the same compiler backend
// `caja build` uses.
func NewRunCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "run a caja language script file.",
		Long:  "parse and evaluate a caja script file in order to run it.",
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

			sourceCode, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("failed to read file '%s': %w", filePath, err)
			}

			baseDir := filepath.Dir(filePath)

			program, _, a, err := script.ParseWithDir(string(sourceCode), baseDir, filePath)
			if err != nil {
				return err
			}

			goCode, err := compiler.Transpile(program, a, compiler.TranspileOptions{PrintResult: true})
			if err != nil {
				return fmt.Errorf("transpilation failed: %w", err)
			}

			formattedCode, err := format.Source([]byte(goCode))
			if err != nil {
				// Fall back to unformatted code if formatting fails
				formattedCode = []byte(goCode)
			}
			goCode = string(formattedCode)

			if compiler.UsesBrowserModule(goCode) {
				return fmt.Errorf("the browser module requires 'caja build' (it targets GOOS=js/GOARCH=wasm, which 'go run' can't execute); it can't be used with 'caja run'")
			}

			exportPath, err := cmd.Flags().GetString("export")
			if err != nil {
				return fmt.Errorf("failed to retrieve 'export' flag: %w", err)
			}
			var extraEnv []string
			if exportPath != "" {
				extraEnv = append(extraEnv, "CAJA_EXPORT_PATH="+exportPath)
			}

			return compiler.Run(goCode, extraEnv, os.Stdin, os.Stdout, os.Stderr)
		},
	}

	cmd.Flags().StringP("file", "f", "", "File path of the script to run")
	cmd.Flags().StringP("export", "e", "", "File name to export log values to (e.g. data.csv)")

	return cmd, nil
}
