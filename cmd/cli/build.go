package main

import (
	"caja-cli/internal/file"
	"caja-cli/internal/pipeline/compiler"
	"caja-cli/internal/script"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// NewBuildCmd creates and returns the 'build' command, responsible for compiling a .caja script.
func NewBuildCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Compile a caja script to a native executable binary",
		RunE: func(cmd *cobra.Command, args []string) error {
			filePath, err := cmd.Flags().GetString("file")
			if err != nil {
				return fmt.Errorf("failed to retrieve 'file' flag: %w", err)
			}

			if filePath == "" {
				_ = cmd.Help()
				return fmt.Errorf("the --file flag is required to compile a script")
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

			// Parse the script to get the AST
			program, _, a, err := script.ParseWithDir(string(sourceCode), baseDir, filePath)
			if err != nil {
				return err
			}

			// Transpile to Go source
			goCode, err := compiler.Transpile(program, a)
			if err != nil {
				return fmt.Errorf("transpilation failed: %w", err)
			}
			
			// Format the generated Go code
			formattedCode, err := format.Source([]byte(goCode))
			if err != nil {
				// Fall back to unformatted code if formatting fails
				formattedCode = []byte(goCode)
			}
			goCode = string(formattedCode)

			// Determine output binary name
			base := filepath.Base(filePath)
			outName := strings.TrimSuffix(base, filepath.Ext(base))
			outBin, err := filepath.Abs(filepath.Join(filepath.Dir(filePath), outName))
			if err != nil {
				return err
			}

			emitGo, _ := cmd.Flags().GetBool("emit-go")
			if emitGo {
				// Write the intermediate Go code to a file so we can inspect it
				outGo := outBin + ".go"
				if err := os.WriteFile(outGo, []byte(goCode), 0644); err != nil {
					return fmt.Errorf("failed to save intermediate go file: %w", err)
				}
				fmt.Printf("Generated intermediate Go code at %s\n", outGo)
			}

			// go build automatically adds .exe if building on Windows. 
			// We can just rely on go build's default behavior.
			
			fmt.Printf("Compiling %s...\n", outBin)

			// Compile the Go code
			if err := compiler.Compile(goCode, outBin); err != nil {
				return err
			}

			fmt.Printf("Successfully built %s\n", outBin)
			return nil
		},
	}

	cmd.Flags().StringP("file", "f", "", "File path of the script to compile")
	cmd.Flags().Bool("emit-go", false, "Emit the intermediate Go source code alongside the binary")

	return cmd, nil
}
