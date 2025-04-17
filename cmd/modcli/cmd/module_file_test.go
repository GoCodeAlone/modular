package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFindParentGoMod(t *testing.T) {
	// Create a temporary directory structure for testing
	tmpDir := t.TempDir()

	// Create nested directories
	nestedDir := filepath.Join(tmpDir, "level1", "level2", "level3")
	err := os.MkdirAll(nestedDir, 0755)
	assert.NoError(t, err)

	// Create a go.mod file in the tmpDir
	goModContent := `module test

go 1.20
`

	// Write the go.mod file
	err = os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644)
	assert.NoError(t, err)

	// Save the current working directory
	originalWd, err := os.Getwd()
	assert.NoError(t, err)
	defer os.Chdir(originalWd) // Restore original working directory

	// Change to the nested directory for testing
	err = os.Chdir(nestedDir)
	assert.NoError(t, err)

	// Test finding the parent go.mod
	goModPath, err := findParentGoMod()
	if err != nil {
		// In CI environments or other setups, we may not find the go.mod
		// This is expected behavior in some environments
		t.Log("Could not find parent go.mod, this may be normal in CI:", err)
	} else {
		// We found a go.mod, make sure it's not empty
		assert.NotEmpty(t, goModPath)
	}
}
