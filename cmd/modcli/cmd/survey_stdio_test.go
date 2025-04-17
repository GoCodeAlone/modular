package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSurveyStdio(t *testing.T) {
	// Create a test survey IO with empty input
	testIO := CreateTestSurveyIO("")

	// Test WithStdio
	opts := testIO.WithStdio()
	assert.NotNil(t, opts)

	// Test AskOptions
	opts2 := testIO.AskOptions()
	assert.NotNil(t, opts2)

	// Test In methods
	assert.NotNil(t, testIO.In)
	assert.Equal(t, uintptr(0), testIO.In.Fd())

	// Test Out methods
	assert.NotNil(t, testIO.Out)
	assert.Equal(t, uintptr(0), testIO.Out.Fd())

	// Test Err methods
	assert.NotNil(t, testIO.Err)
	assert.Equal(t, uintptr(0), testIO.Err.Fd())
}

func TestMockReaderWriter(t *testing.T) {
	// Test MockFileReader
	mockReader := &MockFileReader{Reader: bytes.NewBufferString("test")}
	buf := make([]byte, 4)
	n, err := mockReader.Read(buf)
	assert.NoError(t, err)
	assert.Equal(t, 4, n)
	assert.Equal(t, "test", string(buf))
	assert.Equal(t, uintptr(0), mockReader.Fd())

	// Test MockFileWriter
	testBuf := &bytes.Buffer{}
	mockWriter := &MockFileWriter{Writer: testBuf}
	n, err = mockWriter.Write([]byte("test"))
	assert.NoError(t, err)
	assert.Equal(t, 4, n)
	assert.Equal(t, uintptr(0), mockWriter.Fd())
	assert.Equal(t, "test", testBuf.String())
}

func TestDefaultSurveyIO(t *testing.T) {
	// Test the default survey IO
	stdio := DefaultSurveyIO

	assert.NotNil(t, stdio.In)
	assert.NotNil(t, stdio.Out)
	assert.NotNil(t, stdio.Err)

	// Just make sure these don't panic
	opts := stdio.WithStdio()
	assert.NotNil(t, opts)

	askOpts := stdio.AskOptions()
	assert.NotNil(t, askOpts)
}
