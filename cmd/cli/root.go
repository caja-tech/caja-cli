package main

import (
	"caja-cli/internal/compiler"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// NewRootCmd creates and returns the root command for the cajac CLI application.
func NewRootCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "cajac [token]",
		Short: "A compiler for the custom financial caja language.",
		Long:  "cajac compile custom financial algorithms from a .caja script file into a token.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// ==========================================
			// DECOMPILE MODE: A token argument was provided
			// ==========================================
			if len(args) == 1 {
				token := args[0]
				outputFile, _ := cmd.Flags().GetString("output")

				if outputFile == "" {
					return fmt.Errorf("please provide an --output file path for the decompiled script")
				}

				script, err := compiler.Decode(token)
				if err != nil {
					return fmt.Errorf("invalid token: %w", err)
				}

				err = os.WriteFile(outputFile, []byte(script.ToString()), 0644)
				if err != nil {
					return fmt.Errorf("failed to write output file: %w", err)
				}

				fmt.Printf("Successfully decompiled token to %s\n", outputFile)
				return nil
			}

			// ==========================================
			// COMPILE MODE: No token provided
			// ==========================================
			filePath, err := cmd.Flags().GetString("file")
			if err != nil {
				return fmt.Errorf("failed to retrieve 'file' flag: %w", err)
			}

			if filePath == "" {
				return fmt.Errorf("the --file flag is required to compile a script")
			}

			ext := filepath.Ext(filePath)
			if ext != ".caja" {
				return fmt.Errorf("invalid file type: expected a '.caja' file, but got '%s'", ext)
			}

			sourceCode, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("failed to read file '%s': %w", filePath, err)
			}

			script := string(sourceCode)
			token, err := compiler.Encode(script)
			if err != nil {
				return fmt.Errorf("failed to compile: %w", err)
			}

			fmt.Println(token)
			return nil
		},
	}

	cmd.Flags().StringP("file", "f", "", "File path of the script to compile")
	cmd.Flags().StringP("output", "o", "", "Path to save the decompiled script")

	return cmd, nil
}
