package cmd_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/CrisisTextLine/modular/cmd/modcli/cmd"
)

// MockReader is a wrapper around a bytes.Buffer that also implements terminal.FileReader
type MockReader struct {
	*bytes.Buffer
}

// Fd returns a dummy file descriptor to satisfy the terminal.FileReader interface
func (m *MockReader) Fd() uintptr {
	return 0
}

// MockWriter is a wrapper around a bytes.Buffer that also implements terminal.FileWriter
type MockWriter struct {
	*bytes.Buffer
}

// Fd returns a dummy file descriptor to satisfy the terminal.FileWriter interface
func (m *MockWriter) Fd() uintptr {
	return 0
}

// NewMockSurveyIO creates a mock survey IO setup for testing
func NewMockSurveyIO(input string) (io.Reader, io.Writer, io.Writer) {
	inputBuffer := &MockReader{bytes.NewBufferString(input)}
	outputBuffer := &MockWriter{new(bytes.Buffer)}
	errorBuffer := &MockWriter{new(bytes.Buffer)}

	return inputBuffer, outputBuffer, errorBuffer
}

// mockSurveyForTest creates a survey mock for testing
func mockSurveyForTest(t *testing.T, answers string) func() {
	t.Helper()

	// Save original stdin and stdout
	origStdio := cmd.SurveyStdio

	// Create mock IO for the survey
	mockIn := &MockReader{bytes.NewBufferString(answers)}
	mockOut := &MockWriter{new(bytes.Buffer)}
	mockErr := &MockWriter{new(bytes.Buffer)}

	// Set up the survey input stream
	cmd.SurveyStdio = cmd.SurveyIO{
		In:  mockIn,
		Out: mockOut,
		Err: mockErr,
	}

	// Return a cleanup function that restores the original
	return func() {
		cmd.SurveyStdio = origStdio
	}
}
