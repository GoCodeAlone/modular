package git

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/CrisisTextLine/modular/cmd/modcli/internal/contract"
)

func TestGitHelper_NewGitHelper(t *testing.T) {
	helper := NewGitHelper("/tmp/test")
	if helper.RepoPath != "/tmp/test" {
		t.Errorf("Expected repo path '/tmp/test', got '%s'", helper.RepoPath)
	}
}

func TestGitHelper_ListVersionTags_InvalidPattern(t *testing.T) {
	helper := NewGitHelper("/tmp/test")
	_, err := helper.ListVersionTags("[invalid")
	if err == nil {
		t.Error("Expected error for invalid regex pattern")
	}
}

func TestGitHelper_IsGitRepository_NonExistentPath(t *testing.T) {
	helper := NewGitHelper("/tmp/non-existent-path")
	if helper.IsGitRepository() {
		t.Error("Expected false for non-existent path")
	}
}

func TestGitHelper_ExtractContractFromRef_InvalidRef(t *testing.T) {
	// Create a temporary directory with a simple go file
	tempDir := t.TempDir()
	
	// Create a simple go file
	goFile := filepath.Join(tempDir, "test.go")
	content := `package test

// TestInterface is a test interface
type TestInterface interface {
	TestMethod() error
}
`
	err := os.WriteFile(goFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	helper := NewGitHelper(tempDir)
	extractor := contract.NewExtractor()
	
	// Should fail since tempDir is not a git repository
	_, err = helper.ExtractContractFromRef("invalid-ref", ".", extractor)
	if err == nil {
		t.Error("Expected error for invalid ref in non-git directory")
	}
}

func TestGitHelper_FindLatestVersionTag_NoTags(t *testing.T) {
	helper := NewGitHelper("/tmp/non-git")
	_, err := helper.FindLatestVersionTag(`^v\d+\.\d+\.\d+.*$`)
	if err == nil {
		t.Error("Expected error when no git repository exists")
	}
}

func TestGitHelper_GetAvailableRefs_NonGitRepo(t *testing.T) {
	helper := NewGitHelper("/tmp/non-git")
	refs, err := helper.GetAvailableRefs()
	
	// Should not fail but return empty refs since git commands will fail
	if err != nil {
		t.Logf("Expected behavior - git commands failed: %v", err)
	}
	if len(refs) > 0 {
		t.Errorf("Expected no refs for non-git directory, got %v", refs)
	}
}

// TestVersionPatternMatching tests the version pattern matching logic
func TestVersionPatternMatching(t *testing.T) {
	testCases := []struct {
		name     string
		pattern  string
		tag      string
		expected bool
	}{
		{
			name:     "Standard semantic version",
			pattern:  `^v\d+\.\d+\.\d+.*$`,
			tag:      "v1.2.3",
			expected: true,
		},
		{
			name:     "Semantic version with pre-release",
			pattern:  `^v\d+\.\d+\.\d+.*$`,
			tag:      "v1.2.3-alpha.1",
			expected: true,
		},
		{
			name:     "Non-version tag",
			pattern:  `^v\d+\.\d+\.\d+.*$`,
			tag:      "release-notes",
			expected: false,
		},
		{
			name:     "Custom release pattern",
			pattern:  `^release-\d+\.\d+$`,
			tag:      "release-1.0",
			expected: true,
		},
		{
			name:     "Custom release pattern mismatch",
			pattern:  `^release-\d+\.\d+$`,
			tag:      "v1.0.0",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			regex, err := regexp.Compile(tc.pattern)
			if err != nil {
				t.Fatalf("Invalid pattern: %v", err)
			}
			
			result := regex.MatchString(tc.tag)
			if result != tc.expected {
				t.Errorf("Pattern '%s' matching tag '%s': expected %v, got %v", 
					tc.pattern, tc.tag, tc.expected, result)
			}
		})
	}
}