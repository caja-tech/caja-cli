package main

import (
	"caja-cli/internal/lsp"

	"github.com/spf13/cobra"
)

func NewLspCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "lsp",
		Short: "Start the Caja Language Server",
		Long:  "Starts the language server for the Caja language, communicating over standard input/output.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return lsp.Run(Version)
		},
	}
	return cmd, nil
}
