package main

import (
	"fmt"
	"os"
)

var Version = "dev" // Overridden at build time

func main() {
	root, err := NewRootCmd()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	run, err := NewRunCmd()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	encode, err := NewEncodeCmd()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	decode, err := NewDecodeCmd()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	lsp, err := NewLspCmd()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	build, err := NewBuildCmd()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	root.AddCommand(run)
	root.AddCommand(encode)
	root.AddCommand(decode)
	root.AddCommand(lsp)
	root.AddCommand(build)

	if err := root.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
