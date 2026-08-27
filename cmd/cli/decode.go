package main

import (
	"caja-cli/internal/encoder"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// NewDecodeCmd creates and returns the 'decode' command.
func NewDecodeCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "decode [token]",
		Short: "decode a token",
		Long:  "Decode a token string back into its original .caja script modules and save them to a directory.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				_ = cmd.Help()
				return fmt.Errorf("a single token argument is required")
			}
			token := args[0]
			outputDir, _ := cmd.Flags().GetString("output")

			if outputDir == "" {
				_ = cmd.Help()
				return fmt.Errorf("please provide an --output dir for the decoded scripts (e.g., --output .)")
			}

			bundle, err := encoder.Decode(token)
			if err != nil {
				return fmt.Errorf("invalid token: %w", err)
			}

			err = os.MkdirAll(outputDir, 0755)
			if err != nil {
				return fmt.Errorf("failed to create output directory: %w", err)
			}

			for name, content := range bundle.Modules {
				outPath := filepath.Join(outputDir, name)
				if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
					return fmt.Errorf("failed to write module %s: %w", name, err)
				}
			}

			fmt.Printf("Successfully decoded token to directory %s\n", outputDir)
			return nil
		},
	}

	cmd.Flags().StringP("output", "o", "", "Directory path to save the decoded script and its modules (use --output . for current directory)")
	return cmd, nil
}
