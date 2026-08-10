package main

import (
	"caja-cli/internal/environment"
	"caja-cli/internal/file"
	"caja-cli/internal/script"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// NewRunCmd creates and returns the 'run' command, which is responsible for parsing and evaluating a .caja script file.
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

			ext := filepath.Ext(filePath)
			if ext != file.EXTENSION {
				return fmt.Errorf("invalid file type: expected a %s file, but got '%s'", file.EXTENSION, ext)
			}

			sourceCode, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("failed to read file '%s': %w", filePath, err)
			}

			baseDir := filepath.Dir(filePath)

			program, globalEnv, err := script.ParseWithDir(string(sourceCode), baseDir, filePath)
			if err != nil {
				return err
			}

			eval, err := script.Run(program, globalEnv)
			if err != nil {
				return err
			}

			environment.PrintObject(eval)
			return nil
		},
	}

	cmd.Flags().StringP("file", "f", "", "File path of the script to run")
	err := cmd.MarkFlagRequired("file")
	if err != nil {
		return nil, err
	}

	return cmd, nil
}
