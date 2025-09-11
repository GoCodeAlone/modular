package main

import (
	"fmt"
	"os"

	"github.com/GoCodeAlone/modular/cmd/modcli/cmd"
)

func main() {
	rootCmd := cmd.NewRootCommand()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}
