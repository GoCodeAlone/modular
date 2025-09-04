package git

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/CrisisTextLine/modular/cmd/modcli/internal/contract"
)

// GitHelper provides functionality to work with git repositories for contract extraction
type GitHelper struct {
	RepoPath string
}

// NewGitHelper creates a new GitHelper for the given repository path
func NewGitHelper(repoPath string) *GitHelper {
	return &GitHelper{
		RepoPath: repoPath,
	}
}

// TagInfo represents information about a git tag
type TagInfo struct {
	Name      string
	Commit    string
	Date      time.Time
	Message   string
	IsVersion bool
}

// ListVersionTags lists all version tags in the repository, sorted by version
func (g *GitHelper) ListVersionTags(pattern string) ([]TagInfo, error) {
	// Default version pattern if none provided
	if pattern == "" {
		pattern = `^v\d+\.\d+\.\d+.*$`
	}

	versionRegex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid version pattern: %w", err)
	}

	// Get all tags with their info
	cmd := exec.Command("git", "tag", "-l", "--sort=-version:refname", "--format=%(refname:short)|%(objectname)|%(creatordate:iso8601)|%(subject)")
	cmd.Dir = g.RepoPath
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list git tags: %w", err)
	}

	var tags []TagInfo
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) < 3 {
			continue
		}

		tagName := parts[0]
		commit := parts[1]
		dateStr := parts[2]
		message := ""
		if len(parts) > 3 {
			message = parts[3]
		}

		date, err := time.Parse("2006-01-02 15:04:05 -0700", dateStr)
		if err != nil {
			// Try alternative format
			if date, err = time.Parse(time.RFC3339, dateStr); err != nil {
				continue
			}
		}

		tag := TagInfo{
			Name:      tagName,
			Commit:    commit,
			Date:      date,
			Message:   message,
			IsVersion: versionRegex.MatchString(tagName),
		}

		if tag.IsVersion {
			tags = append(tags, tag)
		}
	}

	return tags, scanner.Err()
}

// ExtractContractFromRef extracts API contract from a specific git reference
func (g *GitHelper) ExtractContractFromRef(ref, packagePath string, extractor *contract.Extractor) (*contract.Contract, error) {
	// Create temporary directory for checkout
	tempDir, err := os.MkdirTemp("", "modcli-git-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Clone the specific reference to temp directory
	cmd := exec.Command("git", "clone", "--depth=1", "--branch", ref, g.RepoPath, tempDir)
	if err := cmd.Run(); err != nil {
		// Try with checkout instead if clone fails
		if err := g.checkoutRefToTemp(ref, tempDir); err != nil {
			return nil, fmt.Errorf("failed to checkout ref %s: %w", ref, err)
		}
	}

	// Determine the package path in the temp directory
	var targetPath string
	if packagePath == "." || packagePath == "" {
		targetPath = tempDir
	} else {
		targetPath = filepath.Join(tempDir, strings.TrimPrefix(packagePath, "./"))
	}

	// Extract contract from the temporary directory
	apiContract, err := extractor.ExtractFromDirectory(targetPath)
	if err != nil {
		return nil, fmt.Errorf("failed to extract contract from ref %s: %w", ref, err)
	}

	// Add version information to contract
	if apiContract != nil {
		apiContract.Version = ref
	}

	return apiContract, nil
}

// checkoutRefToTemp checks out a specific ref to a temporary directory using git worktree
func (g *GitHelper) checkoutRefToTemp(ref, tempDir string) error {
	// Remove the temp directory first
	os.RemoveAll(tempDir)

	// Create git worktree for the specific ref
	cmd := exec.Command("git", "worktree", "add", "--detach", tempDir, ref)
	cmd.Dir = g.RepoPath
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create worktree: %w", err)
	}

	// Clean up worktree after use (defer in calling function handles this)
	// We'll need to clean this up in the caller
	return nil
}

// IsGitRepository checks if the given path is inside a git repository
func (g *GitHelper) IsGitRepository() bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = g.RepoPath
	err := cmd.Run()
	return err == nil
}

// GetCurrentRef gets the current git reference (branch or tag)
func (g *GitHelper) GetCurrentRef() (string, error) {
	// Try to get current branch first
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = g.RepoPath
	output, err := cmd.Output()
	if err == nil {
		ref := strings.TrimSpace(string(output))
		if ref != "HEAD" {
			return ref, nil
		}
	}

	// If detached HEAD, try to get tag
	cmd = exec.Command("git", "describe", "--tags", "--exact-match", "HEAD")
	cmd.Dir = g.RepoPath
	output, err = cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(output)), nil
	}

	// Fall back to commit hash
	cmd = exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = g.RepoPath
	output, err = cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current ref: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// CompareRefs extracts contracts from two git references and compares them
func (g *GitHelper) CompareRefs(oldRef, newRef, packagePath string, extractor *contract.Extractor) (*contract.ContractDiff, error) {
	// Extract contracts from both refs
	oldContract, err := g.ExtractContractFromRef(oldRef, packagePath, extractor)
	if err != nil {
		return nil, fmt.Errorf("failed to extract contract from %s: %w", oldRef, err)
	}

	newContract, err := g.ExtractContractFromRef(newRef, packagePath, extractor)
	if err != nil {
		return nil, fmt.Errorf("failed to extract contract from %s: %w", newRef, err)
	}

	// Compare the contracts
	differ := contract.NewDiffer()
	differ.IgnorePositions = true // Usually ignore positions for git comparisons

	diff, err := differ.Compare(oldContract, newContract)
	if err != nil {
		return nil, fmt.Errorf("failed to compare contracts: %w", err)
	}

	return diff, nil
}

// FindLatestVersionTag finds the latest version tag matching the pattern
func (g *GitHelper) FindLatestVersionTag(pattern string) (string, error) {
	tags, err := g.ListVersionTags(pattern)
	if err != nil {
		return "", err
	}

	if len(tags) == 0 {
		return "", fmt.Errorf("no version tags found matching pattern: %s", pattern)
	}

	// Tags are already sorted by version:refname in descending order
	return tags[0].Name, nil
}

// GetAvailableRefs returns a list of available refs (branches and tags)
func (g *GitHelper) GetAvailableRefs() ([]string, error) {
	var refs []string

	// Get branches
	cmd := exec.Command("git", "branch", "-r", "--format=%(refname:short)")
	cmd.Dir = g.RepoPath
	output, err := cmd.Output()
	if err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(output)))
		for scanner.Scan() {
			branch := strings.TrimSpace(scanner.Text())
			if branch != "" && !strings.Contains(branch, "HEAD") {
				// Remove origin/ prefix for remote branches
				branch = strings.TrimPrefix(branch, "origin/")
				refs = append(refs, branch)
			}
		}
	}

	// Get tags
	cmd = exec.Command("git", "tag", "-l")
	cmd.Dir = g.RepoPath
	output, err = cmd.Output()
	if err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(output)))
		for scanner.Scan() {
			tag := strings.TrimSpace(scanner.Text())
			if tag != "" {
				refs = append(refs, tag)
			}
		}
	}

	// Remove duplicates and sort
	refMap := make(map[string]bool)
	var uniqueRefs []string
	for _, ref := range refs {
		if !refMap[ref] {
			refMap[ref] = true
			uniqueRefs = append(uniqueRefs, ref)
		}
	}

	sort.Strings(uniqueRefs)
	return uniqueRefs, nil
}