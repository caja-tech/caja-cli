package main

import (
	"caja-cli/internal/encoder"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// NewRootCmd creates and returns the root command for the cajac CLI application.
func NewRootCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:           "cajac [token]",
		Short:         "A encoder for the custom financial caja language.",
		Long:          "cajac encode custom financial algorithms from a .caja script file into a token.",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// ==========================================
			// DECODE MODE: A token argument was provided
			// ==========================================
			if len(args) == 1 {
				token := args[0]
				outputFile, _ := cmd.Flags().GetString("output")

				if outputFile == "" {
					_ = cmd.Help()
					return fmt.Errorf("please provide an --output file path for the decoded script")
				}

				script, err := encoder.Decode(token)
				if err != nil {
					return fmt.Errorf("invalid token: %w", err)
				}

				err = os.WriteFile(outputFile, []byte(script.String()), 0644)
				if err != nil {
					return fmt.Errorf("failed to write output file: %w", err)
				}

				fmt.Printf("Successfully decoded token to %s\n", outputFile)
				return nil
			}

			// ==========================================
			// ENCODE MODE: No token provided
			// ==========================================
			filePath, err := cmd.Flags().GetString("file")
			if err != nil {
				_ = cmd.Help()
				return fmt.Errorf("failed to retrieve 'file' flag: %w", err)
			}

			if filePath == "" {
				_ = cmd.Help()
				return fmt.Errorf("the --file flag is required to encode a script")
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
			token, err := encoder.Encode(script)
			if err != nil {
				return fmt.Errorf("failed to encode token: %w", err)
			}

			fmt.Println(token)
			return nil
		},
	}

	cmd.Flags().StringP("file", "f", "", "File path of the script to encode")
	cmd.Flags().StringP("output", "o", "", "Path to save the decoded script")

	return cmd, nil
}
