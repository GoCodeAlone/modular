package main

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/CrisisTextLine/modular/cmd/modcli/cmd"
)

func TestMainVersionFlag(t *testing.T) {
	// Save original command-line arguments and restore them after the test
	originalArgs := os.Args
	originalExit := cmd.OsExit
	defer func() {
		os.Args = originalArgs
		cmd.OsExit = originalExit
	}()

	// Mock the os.Exit function to prevent the test from exiting
	exitCalled := false
	cmd.OsExit = func(code int) {
		exitCalled = true
	}

	// Capture stdout to verify version output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Set up arguments to test the version flag
	os.Args = []string{"modcli", "--version"}

	// Call main - this should print version info to stdout
	main()

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Verify output contains version info
	if output == "" && !exitCalled {
		t.Errorf("Version flag didn't produce any output or call exit")
	}
}
