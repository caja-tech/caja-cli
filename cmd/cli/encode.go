package main

import (
	"caja-cli/internal/encoder"
	"caja-cli/internal/file"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
)

// NewEncodeCmd creates and returns the 'encode' command.
func NewEncodeCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "encode",
		Short: "encode a script",
		Long:  "Encode a .caja script file and its dependencies into a single base64-like token string.",
		RunE: func(cmd *cobra.Command, args []string) error {
			filePath, err := cmd.Flags().GetString("file")
			if err != nil {
				return fmt.Errorf("failed to retrieve 'file' flag: %w", err)
			}

			if filePath == "" {
				_ = cmd.Help()
				return fmt.Errorf("the --file flag is required to encode a script")
			}

			ext := filepath.Ext(filePath)
			if ext != file.EXTENSION {
				return fmt.Errorf("invalid file type: expected a '%s' file, but got '%s'", file.EXTENSION, ext)
			}

			baseDir := filepath.Dir(filePath)
			token, err := encoder.Encode(filepath.Base(filePath), baseDir)
			if err != nil {
				return fmt.Errorf("failed to encode token: %w", err)
			}

			fmt.Println(token)
			return nil
		},
	}

	cmd.Flags().StringP("file", "f", "", "File path of the script to encode")
	return cmd, nil
}
