package main

import (
	"github.com/spf13/cobra"
)

// NewRootCmd creates and returns the root command for the cajac CLI application.
func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cajac",
		Short: "A command line tool for the caja language, a custom financial DSL evaluator.",
		Long:  "cajac compile, parse and evaluate custom financial algorithms from a .caja script file.",
	}

	return cmd
}
