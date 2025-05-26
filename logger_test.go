package modular

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/mock"
)

type logger struct {
	t *testing.T
}

func (l *logger) getCallerInfo() string {
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		return "unknown"
	}
	wd, err := os.Getwd()
	if err != nil {
		wd = "."
	}
	relPath, err := filepath.Rel(wd, file)
	if err != nil {
		relPath = file
	}
	return fmt.Sprintf("%s:%d", relPath, line)
}

func (l *logger) Info(msg string, args ...any) {
	dir := l.getCallerInfo()
	l.t.Log(fmt.Sprintf("[%s] %s", dir, msg), args)
}

func (l *logger) Error(msg string, args ...any) {
	dir := l.getCallerInfo()
	l.t.Error(fmt.Sprintf("[%s] %s", dir, msg), args)
}

func (l *logger) Warn(msg string, args ...any) {
	dir := l.getCallerInfo()
	l.t.Log(fmt.Sprintf("[%s] %s", dir, msg), args)
}

func (l *logger) Debug(msg string, args ...any) {
	dir := l.getCallerInfo()
	l.t.Log(fmt.Sprintf("[%s] %s", dir, msg), args)
}

// MockLogger
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(msg string, args ...interface{}) {
	m.Called(msg, args)
}

func (m *MockLogger) Info(msg string, args ...interface{}) {
	m.Called(msg, args)
}

func (m *MockLogger) Warn(msg string, args ...interface{}) {
	m.Called(msg, args)
}

func (m *MockLogger) Error(msg string, args ...interface{}) {
	m.Called(msg, args)
}
