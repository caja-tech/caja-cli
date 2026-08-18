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

	root.AddCommand(run)

	if err := root.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
