package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewRootCommand creates the root command for the modcli application
func NewRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "modcli",
		Short: "Modular CLI - Tools for working with the Modular Go Framework",
		Long: `Modular CLI provides tools for working with the Modular Go Framework.
It helps with generating modules, configurations, and other common tasks.`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	// Add subcommands
	cmd.AddCommand(NewGenerateCommand())

	return cmd
}

// NewGenerateCommand creates the generate command
func NewGenerateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate various components",
		Long:  `Generate modules, configurations, and other components for the Modular framework.`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	// Add subcommands for generation
	cmd.AddCommand(NewGenerateModuleCommand())
	cmd.AddCommand(NewGenerateConfigCommand())

	return cmd
}

// Version information
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// PrintVersion prints version information
func PrintVersion() string {
	return fmt.Sprintf("Modular CLI v%s (commit: %s, built on: %s)", Version, Commit, Date)
}
