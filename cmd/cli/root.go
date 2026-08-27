package main

import (
	"caja-cli/internal/file"
	"fmt"

	"github.com/spf13/cobra"
)

// NewRootCmd creates and returns the root command for the cajac CLI application.
func NewRootCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:           "caja",
		Short:         "A cli for the caja language.",
		Long:          fmt.Sprintf("caja can be used to run '%s' script files, also encode/decode them into a token.", file.EXTENSION),
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = cmd.Help()
			return nil
		},
	}

	return cmd, nil
}
