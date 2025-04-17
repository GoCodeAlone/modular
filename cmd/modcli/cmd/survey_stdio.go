// survey_stdio.go - Contains utilities for handling survey I/O consistently
package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
)

// SurveyIO represents the standard input/output streams for surveys
type SurveyIO struct {
	In  terminal.FileReader
	Out terminal.FileWriter
	Err terminal.FileWriter
}

// DefaultSurveyIO provides standard IO for interactive prompts
var DefaultSurveyIO = SurveyIO{
	In:  os.Stdin,
	Out: os.Stdout,
	Err: os.Stderr,
}

// WithStdio returns survey.WithStdio option
func (s SurveyIO) WithStdio() survey.AskOpt {
	return survey.WithStdio(s.In, s.Out, s.Err)
}

// AskOptions returns an array of survey options to use with AskOne
func (s SurveyIO) AskOptions() []survey.AskOpt {
	return []survey.AskOpt{survey.WithStdio(s.In, s.Out, s.Err)}
}

// MockReader is a helper to create a terminal.FileReader for testing
type MockFileReader struct {
	Reader io.Reader
}

func (m *MockFileReader) Read(p []byte) (n int, err error) {
	return m.Reader.Read(p)
}

func (m *MockFileReader) Fd() uintptr {
	return 0 // Dummy value
}

// MockWriter is a helper to create a terminal.FileWriter for testing
type MockFileWriter struct {
	Writer io.Writer
}

func (m *MockFileWriter) Write(p []byte) (n int, err error) {
	return m.Writer.Write(p)
}

func (m *MockFileWriter) Fd() uintptr {
	return 0 // Dummy value
}

// CreateTestSurveyIO creates a SurveyIO instance for testing with the given input
func CreateTestSurveyIO(input string) SurveyIO {
	mockReader := &MockFileReader{strings.NewReader(input)}
	mockOutWriter := &MockFileWriter{new(bytes.Buffer)}
	mockErrWriter := &MockFileWriter{new(bytes.Buffer)}

	return SurveyIO{
		In:  mockReader,
		Out: mockOutWriter,
		Err: mockErrWriter,
	}
}
