package main

import (
	"caja-cli/internal/encoder"
	"caja-cli/internal/file"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// NewRootCmd creates and returns the root command for the cajac CLI application.
func NewRootCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:           "caja [token]",
		Short:         "A cli for the caja language.",
		Long:          fmt.Sprintf("caja can be used to run '%s' script files, also encode/decode them into a token.", file.EXTENSION),
		Version:       Version,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				return decodeMode(cmd, args[0])
			}

			return encodeMode(cmd)
		},
	}

	cmd.Flags().StringP("file", "f", "", "File path of the script to encode")
	cmd.Flags().StringP("output", "o", "", "Directory path to save the decoded script and its modules (use --output . for current directory)")

	return cmd, nil
}

// encodeMode handles encoding a .caja script file into a single token string.
func encodeMode(cmd *cobra.Command) error {
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
}

// decodeMode handles decoding a token string back into its original .caja script modules, writing them to the specified output directory.
func decodeMode(cmd *cobra.Command, token string) error {
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
}
