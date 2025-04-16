package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version information (set during build)
var (
	Version string = "dev"
	Commit  string = "none"
	Date    string = "unknown"
)

// OsExit allows us to mock os.Exit for testing
var OsExit = os.Exit

// NewRootCommand creates the root command for the modcli application
func NewRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "modcli",
		Short: "Modular CLI - Tools for working with the Modular Go Framework",
		Long: `Modular CLI provides tools for working with the Modular Go Framework.
It helps with generating modules, configurations, and other common tasks.`,
		Run: func(cmd *cobra.Command, args []string) {
			// Check if version flag is set
			versionFlag, _ := cmd.Flags().GetBool("version")
			if versionFlag {
				fmt.Println(PrintVersion())
				OsExit(0)
				return
			}
			cmd.Help()
		},
	}

	// Add version flag
	cmd.Flags().BoolP("version", "v", false, "Print version information")
	cmd.Version = Version

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

// PrintVersion prints version information
func PrintVersion() string {
	return fmt.Sprintf("Modular CLI v%s (commit: %s, built on: %s)", Version, Commit, Date)
}
