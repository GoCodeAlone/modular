package cmd

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/spf13/cobra"
)

var (
	// Version information (set during build)
	Version string = "dev"
	Commit  string = "none"
	Date    string = "unknown"
	// OsExit allows us to mock os.Exit for testing
	OsExit = os.Exit
)

func init() {
	// Try to read build info embedded by the Go toolchain (Go 1.18+)
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		// Build info not available. The defaults or ldflags-set values will be used.
		return
	}

	if Version != "dev" {
		// Version was set by ldflags, no need to override it
		return
	}

	// --- Populate variables ONLY if they still hold the default values ---
	// This ensures that ldflags values take precedence if they were provided during build.

	// Populate Version from build info if it wasn't set by ldflags
	// Use the main module's version. Avoid "(devel)" which is less specific than "dev".
	if bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		Version = bi.Main.Version
	}

	// Extract commit and date from VCS settings in the build info
	var foundCommit, foundDate string
	for _, setting := range bi.Settings {
		switch setting.Key {
		case "vcs.revision":
			foundCommit = setting.Value
		case "vcs.time":
			foundDate = setting.Value // This is the commit timestamp
		}
		// Optimization: stop searching once both are found (if both exist)
		if foundCommit != "" && foundDate != "" {
			break
		}
	}

	// Populate Commit from build info if it wasn't set by ldflags
	if foundCommit != "" {
		Commit = foundCommit
	}

	// Populate Date from build info (commit date) if it wasn't set by ldflags
	// Note: This uses the commit date, not the build date. If you specifically
	// need build date via ldflags, that's fine, this just provides a fallback.
	if foundDate != "" {
		// Optional: Parse and reformat the date string if desired
		// Example: parsedTime, err := time.Parse(time.RFC3339Nano, foundDate) ...
		Date = foundDate
	}
}

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
			_ = cmd.Help()
		},
	}

	// Add version flag
	cmd.Flags().BoolP("version", "v", false, "Print version information")
	cmd.Version = Version

	// Add subcommands
	cmd.AddCommand(NewGenerateCommand())
	cmd.AddCommand(NewDebugCommand())

	return cmd
}

// NewGenerateCommand creates the generate command
func NewGenerateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate various components",
		Long:  `Generate modules, configurations, and other components for the Modular framework.`,
		Run: func(cmd *cobra.Command, args []string) {
			_ = cmd.Help()
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
