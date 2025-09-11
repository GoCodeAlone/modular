package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CrisisTextLine/modular/cmd/modcli/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSimpleModuleGeneration is a minimal test to debug survey input issues
func TestSimpleModuleGeneration(t *testing.T) {
	// Create a temporary directory for output
	testDir, err := os.MkdirTemp("", "modcli-simple-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(testDir)

	// Save original setOptionsFn
	origSetOptionsFn := cmd.SetOptionsFn
	defer func() {
		cmd.SetOptionsFn = origSetOptionsFn
	}()

	// Set up the options directly instead of using survey
	cmd.SetOptionsFn = func(options *cmd.ModuleOptions) bool {
		// Set basic module properties
		options.PackageName = strings.ToLower(options.ModuleName)

		// Set options based on our test requirements
		options.HasConfig = false
		options.IsTenantAware = false
		options.HasDependencies = false
		options.HasStartupLogic = false
		options.HasShutdownLogic = false
		options.ProvidesServices = false
		options.RequiresServices = false
		options.GenerateTests = true

		return true // Return true to indicate we've handled the options
	}

	// Create the command
	moduleCmd := cmd.NewGenerateModuleCommand()

	// Set up args - specify all required options
	moduleCmd.SetArgs([]string{
		"--name", "Simple",
		"--output", testDir,
	})

	// Execute the command
	var outBuf, errBuf bytes.Buffer
	moduleCmd.SetOut(&outBuf)
	moduleCmd.SetErr(&errBuf)

	err = moduleCmd.Execute()
	if err != nil {
		t.Logf("Command output: %s", outBuf.String())
		t.Logf("Command error: %s", errBuf.String())
		t.Fatalf("Module generation failed: %v", err)
	}

	// Verify basic files were created
	packageDir := filepath.Join(testDir, "simple")
	moduleFile := filepath.Join(packageDir, "module.go")
	assert.FileExists(t, moduleFile, "module.go file should exist")
}
