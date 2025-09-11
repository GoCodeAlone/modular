package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/CrisisTextLine/modular/cmd/modcli/internal/contract"
	"github.com/CrisisTextLine/modular/cmd/modcli/internal/git"
	"github.com/spf13/cobra"
)

// Define static errors
var (
	ErrUnsupportedFormat = errors.New("unsupported output format")
)

// NewContractCommand creates the contract command
func NewContractCommand() *cobra.Command {
	// Local flag variables to avoid global state issues in tests
	var (
		outputFile      string
		includePrivate  bool
		includeTests    bool
		includeInternal bool
		outputFormat    string
		ignorePositions bool
		ignoreComments  bool
		verbose         bool
		baseline        string
		versionPattern  string
	)

	contractCmd := &cobra.Command{
		Use:   "contract",
		Short: "API contract management for Go packages",
		Long: `The contract command provides functionality to extract, compare, and manage
API contracts for Go packages. This helps detect breaking changes and track
API evolution over time.

Available subcommands:
  extract  - Extract API contract from a Go package
  compare  - Compare two API contracts and show differences
  diff     - Alias for compare command`,
	}

	// Create extract command with local flag variables
	extractCmd := &cobra.Command{
		Use:   "extract [package]",
		Short: "Extract API contract from a Go package",
		Long: `Extract the public API contract from a Go package or directory.
The contract includes exported interfaces, types, functions, variables, and constants.

Examples:
  modcli contract extract .                     # Current directory
  modcli contract extract ./modules/auth       # Specific directory
  modcli contract extract github.com/user/pkg  # Remote package
  modcli contract extract -o contract.json .   # Save to file`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExtractContractWithFlags(cmd, args, outputFile, includePrivate, includeTests, includeInternal, verbose)
		},
	}

	// Create compare command with local flag variables
	compareCmd := &cobra.Command{
		Use:   "compare <old-contract> <new-contract>",
		Short: "Compare two API contracts",
		Long: `Compare two API contract files and show the differences.
This command identifies breaking changes, additions, and modifications.

Examples:
  modcli contract compare old.json new.json
  modcli contract compare old.json new.json -o diff.json
  modcli contract compare old.json new.json --format=markdown`,
		Args:    cobra.ExactArgs(2),
		Aliases: []string{"diff"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCompareContractWithFlags(cmd, args, outputFile, outputFormat, ignorePositions, ignoreComments, verbose)
		},
	}

	// Setup extract command flags
	extractCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file (default: stdout)")
	extractCmd.Flags().BoolVar(&includePrivate, "include-private", false, "Include unexported items")
	extractCmd.Flags().BoolVar(&includeTests, "include-tests", false, "Include test files")
	extractCmd.Flags().BoolVar(&includeInternal, "include-internal", false, "Include internal packages")
	extractCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")

	// Setup compare command flags
	compareCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file (default: stdout)")
	compareCmd.Flags().StringVar(&outputFormat, "format", "json", "Output format: json, markdown, text")
	compareCmd.Flags().BoolVar(&ignorePositions, "ignore-positions", true, "Ignore source position changes")
	compareCmd.Flags().BoolVar(&ignoreComments, "ignore-comments", false, "Ignore documentation comment changes")
	compareCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")

	// Create git-diff command for comparing git refs
	gitDiffCmd := &cobra.Command{
		Use:   "git-diff [old-ref] [new-ref] [package-path]",
		Short: "Compare API contracts between git references",
		Long: `Compare API contracts between two git references (branches, tags, commits).
This command extracts contracts from both references and shows the differences.

Examples:
  modcli contract git-diff v1.0.0 main .           # Compare tag v1.0.0 with main branch
  modcli contract git-diff HEAD~1 HEAD ./module    # Compare last commit with current
  modcli contract git-diff --baseline v1.1.0 .     # Compare v1.1.0 with current working directory`,
		Args: cobra.RangeArgs(0, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGitDiffContractWithFlags(cmd, args, outputFile, outputFormat, ignorePositions, ignoreComments, verbose, baseline, versionPattern)
		},
	}

	// Create tags command for listing version tags
	tagsCmd := &cobra.Command{
		Use:   "tags [package-path]",
		Short: "List available version tags for contract comparison",
		Long: `List all version tags in the repository that can be used for contract comparison.
By default, shows tags matching semantic versioning pattern (v1.2.3).

Examples:
  modcli contract tags .                           # List version tags in current directory
  modcli contract tags --pattern "^release-.*"    # List tags matching custom pattern`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runListTagsWithFlags(cmd, args, versionPattern, verbose)
		},
	}

	// Setup git-diff command flags
	gitDiffCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file (default: stdout)")
	gitDiffCmd.Flags().StringVar(&outputFormat, "format", "markdown", "Output format: json, markdown, text")
	gitDiffCmd.Flags().BoolVar(&ignorePositions, "ignore-positions", true, "Ignore source position changes")
	gitDiffCmd.Flags().BoolVar(&ignoreComments, "ignore-comments", false, "Ignore documentation comment changes")
	gitDiffCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	gitDiffCmd.Flags().StringVar(&baseline, "baseline", "", "Baseline reference (if only one ref is provided, compares with working directory)")
	gitDiffCmd.Flags().StringVar(&versionPattern, "version-pattern", `^v\d+\.\d+\.\d+.*$`, "Pattern for identifying version tags")

	// Setup tags command flags
	tagsCmd.Flags().StringVar(&versionPattern, "pattern", `^v\d+\.\d+\.\d+.*$`, "Pattern for matching version tags")
	tagsCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")

	contractCmd.AddCommand(extractCmd)
	contractCmd.AddCommand(compareCmd)
	contractCmd.AddCommand(gitDiffCmd)
	contractCmd.AddCommand(tagsCmd)
	return contractCmd
}

func runExtractContractWithFlags(cmd *cobra.Command, args []string, outputFile string, includePrivate bool, includeTests bool, includeInternal bool, verbose bool) error {
	packagePath := args[0]

	if verbose {
		fmt.Fprintf(os.Stderr, "Extracting API contract from: %s\n", packagePath)
	}

	extractor := contract.NewExtractor()
	extractor.IncludePrivate = includePrivate
	extractor.IncludeTests = includeTests
	extractor.IncludeInternal = includeInternal

	var apiContract *contract.Contract
	var err error

	// Check if it's a local directory
	if strings.HasPrefix(packagePath, ".") || strings.HasPrefix(packagePath, "/") {
		// Resolve relative paths
		if absPath, err := filepath.Abs(packagePath); err == nil {
			packagePath = absPath
		}
		apiContract, err = extractor.ExtractFromDirectory(packagePath)
	} else {
		// Treat as a package path
		apiContract, err = extractor.ExtractFromPackage(packagePath)
	}

	if err != nil {
		return fmt.Errorf("failed to extract contract: %w", err)
	}

	// Output the contract
	if outputFile != "" {
		if verbose {
			fmt.Fprintf(os.Stderr, "Saving contract to: %s\n", outputFile)
		}

		if err := apiContract.SaveToFile(outputFile); err != nil {
			return fmt.Errorf("failed to save contract: %w", err)
		}

		fmt.Printf("API contract extracted and saved to %s\n", outputFile)
	} else {
		// Output to stdout as pretty JSON
		data, err := json.MarshalIndent(apiContract, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal contract: %w", err)
		}
		fmt.Println(string(data))
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "Contract extracted successfully:\n")
		fmt.Fprintf(os.Stderr, "  - Package: %s\n", apiContract.PackageName)
		fmt.Fprintf(os.Stderr, "  - Interfaces: %d\n", len(apiContract.Interfaces))
		fmt.Fprintf(os.Stderr, "  - Types: %d\n", len(apiContract.Types))
		fmt.Fprintf(os.Stderr, "  - Functions: %d\n", len(apiContract.Functions))
		fmt.Fprintf(os.Stderr, "  - Variables: %d\n", len(apiContract.Variables))
		fmt.Fprintf(os.Stderr, "  - Constants: %d\n", len(apiContract.Constants))
	}

	return nil
}

func runCompareContractWithFlags(cmd *cobra.Command, args []string, outputFile string, outputFormat string, ignorePositions bool, ignoreComments bool, verbose bool) error {
	oldFile := args[0]
	newFile := args[1]

	if verbose {
		fmt.Fprintf(os.Stderr, "Comparing contracts: %s -> %s\n", oldFile, newFile)
	}

	// Load contracts
	oldContract, err := contract.LoadFromFile(oldFile)
	if err != nil {
		return fmt.Errorf("failed to load old contract: %w", err)
	}

	newContract, err := contract.LoadFromFile(newFile)
	if err != nil {
		return fmt.Errorf("failed to load new contract: %w", err)
	}

	// Compare contracts
	differ := contract.NewDiffer()
	differ.IgnorePositions = ignorePositions
	differ.IgnoreComments = ignoreComments

	diff, err := differ.Compare(oldContract, newContract)
	if err != nil {
		return fmt.Errorf("failed to compare contracts: %w", err)
	}

	// Format and output the diff
	var output string
	switch strings.ToLower(outputFormat) {
	case "json":
		output, err = formatDiffAsJSON(diff)
	case "markdown", "md":
		output, err = formatDiffAsMarkdown(diff)
	case "text", "txt":
		output, err = formatDiffAsText(diff)
	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedFormat, outputFormat)
	}

	if err != nil {
		return fmt.Errorf("failed to format diff: %w", err)
	}

	// Output the diff
	if outputFile != "" {
		if verbose {
			fmt.Fprintf(os.Stderr, "Saving diff to: %s\n", outputFile)
		}

		if err := os.WriteFile(outputFile, []byte(output), 0600); err != nil {
			return fmt.Errorf("failed to save diff: %w", err)
		}

		fmt.Printf("Contract diff saved to %s\n", outputFile)
	} else {
		fmt.Print(output)
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "Comparison completed:\n")
		fmt.Fprintf(os.Stderr, "  - Breaking changes: %d\n", diff.Summary.TotalBreakingChanges)
		fmt.Fprintf(os.Stderr, "  - Additions: %d\n", diff.Summary.TotalAdditions)
		fmt.Fprintf(os.Stderr, "  - Modifications: %d\n", diff.Summary.TotalModifications)
	}

	// Exit with error code if there are breaking changes
	if diff.Summary.HasBreakingChanges {
		fmt.Fprintf(os.Stderr, "WARNING: Breaking changes detected!\n")
		os.Exit(1)
	}

	return nil
}

func formatDiffAsJSON(diff *contract.ContractDiff) (string, error) {
	data, err := json.MarshalIndent(diff, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal diff as JSON: %w", err)
	}
	return string(data), nil
}

func formatDiffAsMarkdown(diff *contract.ContractDiff) (string, error) {
	var md strings.Builder

	md.WriteString(fmt.Sprintf("# API Contract Diff: %s\n\n", diff.PackageName))

	if diff.OldVersion != "" || diff.NewVersion != "" {
		md.WriteString("## Version Information\n")
		if diff.OldVersion != "" {
			md.WriteString(fmt.Sprintf("- **Old Version**: %s\n", diff.OldVersion))
		}
		if diff.NewVersion != "" {
			md.WriteString(fmt.Sprintf("- **New Version**: %s\n", diff.NewVersion))
		}
		md.WriteString("\n")
	}

	// Summary
	md.WriteString("## Summary\n\n")
	md.WriteString(fmt.Sprintf("- **Breaking Changes**: %d\n", diff.Summary.TotalBreakingChanges))
	md.WriteString(fmt.Sprintf("- **Additions**: %d\n", diff.Summary.TotalAdditions))
	md.WriteString(fmt.Sprintf("- **Modifications**: %d\n", diff.Summary.TotalModifications))

	if diff.Summary.HasBreakingChanges {
		md.WriteString("\n⚠️  **Warning: This update contains breaking changes!**\n")
	}
	md.WriteString("\n")

	// Breaking changes
	if len(diff.BreakingChanges) > 0 {
		md.WriteString("## 🚨 Breaking Changes\n\n")
		for _, change := range diff.BreakingChanges {
			md.WriteString(fmt.Sprintf("### %s: %s\n", change.Type, change.Item))
			md.WriteString(fmt.Sprintf("%s\n\n", change.Description))
			if change.OldValue != "" {
				md.WriteString("**Old:**\n```go\n")
				md.WriteString(change.OldValue)
				md.WriteString("\n```\n\n")
			}
			if change.NewValue != "" {
				md.WriteString("**New:**\n```go\n")
				md.WriteString(change.NewValue)
				md.WriteString("\n```\n\n")
			}
		}
	}

	// Additions
	if len(diff.AddedItems) > 0 {
		md.WriteString("## ➕ Additions\n\n")
		for _, item := range diff.AddedItems {
			md.WriteString(fmt.Sprintf("- **%s**: %s - %s\n", item.Type, item.Item, item.Description))
		}
		md.WriteString("\n")
	}

	// Modifications
	if len(diff.ModifiedItems) > 0 {
		md.WriteString("## 📝 Modifications\n\n")
		for _, item := range diff.ModifiedItems {
			md.WriteString(fmt.Sprintf("- **%s**: %s - %s\n", item.Type, item.Item, item.Description))
		}
		md.WriteString("\n")
	}

	return md.String(), nil
}

func formatDiffAsText(diff *contract.ContractDiff) (string, error) {
	var txt strings.Builder

	if diff.Summary.HasBreakingChanges {
		txt.WriteString("⚠️ WARNING: Breaking changes detected!\n\n")
	}

	txt.WriteString(fmt.Sprintf("=== API Contract Diff ===\n"))
	txt.WriteString(fmt.Sprintf("Package: %s\n", diff.PackageName))
	txt.WriteString(fmt.Sprintf("Breaking changes: %d\n", len(diff.BreakingChanges)))
	txt.WriteString(fmt.Sprintf("Added items: %d\n", len(diff.AddedItems)))
	txt.WriteString(fmt.Sprintf("Modified items: %d\n", len(diff.ModifiedItems)))
	txt.WriteString("\n")

	if len(diff.BreakingChanges) > 0 {
		txt.WriteString("BREAKING CHANGES:\n")
		for _, change := range diff.BreakingChanges {
			txt.WriteString(fmt.Sprintf("- %s: %s - %s\n", change.Type, change.Item, change.Description))
		}
		txt.WriteString("\n")
	}

	if len(diff.AddedItems) > 0 {
		txt.WriteString("ADDITIONS:\n")
		for _, item := range diff.AddedItems {
			txt.WriteString(fmt.Sprintf("- %s: %s - %s\n", item.Type, item.Item, item.Description))
		}
		txt.WriteString("\n")
	}

	if len(diff.ModifiedItems) > 0 {
		txt.WriteString("MODIFICATIONS:\n")
		for _, item := range diff.ModifiedItems {
			txt.WriteString(fmt.Sprintf("- %s: %s - %s\n", item.Type, item.Item, item.Description))
		}
		txt.WriteString("\n")
	}

	return txt.String(), nil
}

func runGitDiffContractWithFlags(cmd *cobra.Command, args []string, outputFile, outputFormat string, ignorePositions, ignoreComments, verbose bool, baseline, versionPattern string) error {
	// Import git helper
	gitHelper := git.NewGitHelper(".")
	
	if verbose {
		fmt.Fprintf(os.Stderr, "Using git diff for contract comparison\n")
	}

	var oldRef, newRef, packagePath string
	
	// Parse arguments based on how many were provided
	switch len(args) {
	case 0:
		// Compare latest version tag with current working directory
		if baseline != "" {
			oldRef = baseline
		} else {
			var err error
			oldRef, err = gitHelper.FindLatestVersionTag(versionPattern)
			if err != nil {
				return fmt.Errorf("failed to find latest version tag: %w", err)
			}
		}
		newRef = ""
		packagePath = "."
	case 1:
		// Argument could be package path or new ref
		if strings.HasPrefix(args[0], ".") || strings.HasPrefix(args[0], "/") {
			// It's a package path
			if baseline != "" {
				oldRef = baseline
			} else {
				var err error
				oldRef, err = gitHelper.FindLatestVersionTag(versionPattern)
				if err != nil {
					return fmt.Errorf("failed to find latest version tag: %w", err)
				}
			}
			newRef = ""
			packagePath = args[0]
		} else {
			// It's a ref
			if baseline != "" {
				oldRef = baseline
				newRef = args[0]
			} else {
				oldRef = args[0]
				newRef = ""
			}
			packagePath = "."
		}
	case 2:
		oldRef = args[0]
		newRef = args[1]
		packagePath = "."
	case 3:
		oldRef = args[0]
		newRef = args[1]
		packagePath = args[2]
	default:
		return fmt.Errorf("too many arguments")
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "Comparing: %s -> %s (package: %s)\n", oldRef, newRef, packagePath)
	}

	// Create extractor
	extractor := contract.NewExtractor()

	var diff *contract.ContractDiff
	var err error

	if newRef == "" {
		// Compare ref with current working directory
		oldContract, err := gitHelper.ExtractContractFromRef(oldRef, packagePath, extractor)
		if err != nil {
			return fmt.Errorf("failed to extract contract from %s: %w", oldRef, err)
		}

		// Extract from current working directory
		var targetPath string
		if packagePath == "." || packagePath == "" {
			targetPath = "."
		} else {
			targetPath = packagePath
		}

		var newContract *contract.Contract
		// Check if it's a local directory
		if strings.HasPrefix(targetPath, ".") || strings.HasPrefix(targetPath, "/") {
			newContract, err = extractor.ExtractFromDirectory(targetPath)
		} else {
			newContract, err = extractor.ExtractFromPackage(targetPath)
		}
		
		if err != nil {
			return fmt.Errorf("failed to extract contract from working directory: %w", err)
		}

		// Compare contracts
		differ := contract.NewDiffer()
		differ.IgnorePositions = ignorePositions
		differ.IgnoreComments = ignoreComments

		diff, err = differ.Compare(oldContract, newContract)
		if err != nil {
			return fmt.Errorf("failed to compare contracts: %w", err)
		}
	} else {
		// Compare two refs
		diff, err = gitHelper.CompareRefs(oldRef, newRef, packagePath, extractor)
		if err != nil {
			return err
		}
	}

	// Format and output the diff
	var output string
	switch strings.ToLower(outputFormat) {
	case "json":
		output, err = formatDiffAsJSON(diff)
	case "markdown", "md":
		output, err = formatDiffAsMarkdown(diff)
	case "text":
		output, err = formatDiffAsText(diff)
	default:
		return ErrUnsupportedFormat
	}

	if err != nil {
		return fmt.Errorf("failed to format diff: %w", err)
	}

	// Output the diff
	if outputFile != "" {
		if verbose {
			fmt.Fprintf(os.Stderr, "Saving diff to: %s\n", outputFile)
		}

		if err := os.WriteFile(outputFile, []byte(output), 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}

		fmt.Printf("Contract diff saved to %s\n", outputFile)
	} else {
		fmt.Print(output)
	}

	// Exit with error code if breaking changes detected
	if diff.Summary.HasBreakingChanges {
		if verbose {
			fmt.Fprintf(os.Stderr, "Breaking changes detected!\n")
		}
		os.Exit(1)
	}

	return nil
}

func runListTagsWithFlags(cmd *cobra.Command, args []string, versionPattern string, verbose bool) error {
	packagePath := "."
	if len(args) > 0 {
		packagePath = args[0]
	}

	gitHelper := git.NewGitHelper(packagePath)
	
	if !gitHelper.IsGitRepository() {
		return fmt.Errorf("not a git repository: %s", packagePath)
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "Listing version tags matching pattern: %s\n", versionPattern)
	}

	tags, err := gitHelper.ListVersionTags(versionPattern)
	if err != nil {
		return fmt.Errorf("failed to list version tags: %w", err)
	}

	if len(tags) == 0 {
		fmt.Printf("No version tags found matching pattern: %s\n", versionPattern)
		return nil
	}

	fmt.Printf("Available version tags (%d found):\n\n", len(tags))
	for _, tag := range tags {
		if verbose {
			fmt.Printf("  %s (%s) - %s\n    %s\n", tag.Name, tag.Date.Format("2006-01-02"), tag.Commit[:8], tag.Message)
		} else {
			fmt.Printf("  %s\n", tag.Name)
		}
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "\nUse these tags with 'modcli contract git-diff <old-tag> <new-tag>'\n")
	}

	return nil
}


