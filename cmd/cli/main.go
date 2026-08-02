package main

import (
	"fmt"
	"os"
)

func main() {
	root := NewRootCmd()

	run, err := NewRunCmd()
	if err != nil {
		panic(err)
	}

	root.AddCommand(run)

	if err := root.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
