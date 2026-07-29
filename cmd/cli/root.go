package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	var filePath string

	cmd := &cobra.Command{
		Use:   "cajac",
		Short: "A command line tool for a custom financial DSL evaluator",
		Long:  "cajac parses and evaluates custom financial algorithms from a .caja script file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if filePath == "" {
				fmt.Println("Error: You must provide a file path using --file or -f")
				if err := cmd.Help(); err != nil {
					return fmt.Errorf("a problem occurred while trying to get help message: %w", err)
				}
			}

			ext := filepath.Ext(filePath)
			if ext != ".caja" {
				return fmt.Errorf("invalid file type: expected a '.caja' file, but got '%s'", ext)
			}

			sourceCode, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("failed to read file '%s': %w", filePath, err)
			}

			fmt.Println(string(sourceCode))
			return nil
		},
	}

	cmd.Flags().StringVarP(&filePath, "file", "f", "", "File path to evaluate")

	return cmd
}
