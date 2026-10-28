package main

import (
	"fmt"
	"os"

	"workbench/internal/app"
)

func main() {
	exitCode, err := app.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		if exitCode == 0 {
			exitCode = 1
		}
	}

	os.Exit(exitCode)
}
