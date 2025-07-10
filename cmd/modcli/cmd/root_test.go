package cmd_test

import (
	"bytes"
	"testing"

	"github.com/CrisisTextLine/modular/cmd/modcli/cmd"
	"github.com/stretchr/testify/assert"
)

func TestRootCommand(t *testing.T) {
	rootCmd := cmd.NewRootCommand()
	assert.NotNil(t, rootCmd)
	assert.Equal(t, "modcli", rootCmd.Use)

	// Ensure help doesn't panic
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"--help"})
	err := rootCmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "Modular CLI")
}

func TestGenerateCommand(t *testing.T) {
	genCmd := cmd.NewGenerateCommand()
	assert.NotNil(t, genCmd)
	assert.Equal(t, "generate", genCmd.Use)

	// Ensure help doesn't panic
	buf := new(bytes.Buffer)
	genCmd.SetOut(buf)
	genCmd.SetArgs([]string{"--help"})
	err := genCmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "Generate modules, configurations, and other components")
}

func TestVersionInfo(t *testing.T) {
	version := cmd.PrintVersion()
	assert.Contains(t, version, "Modular CLI v")
}
