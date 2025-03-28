package modular

import (
	"github.com/stretchr/testify/mock"
	"testing"
)

type logger struct {
	t *testing.T
}

func (l *logger) Info(msg string, args ...any) {
	l.t.Log(msg, args)
}

func (l *logger) Error(msg string, args ...any) {
	l.t.Error(msg, args)
}

func (l *logger) Warn(msg string, args ...any) {
	l.t.Error(msg, args)
}

func (l *logger) Debug(msg string, args ...any) {
	l.t.Log(msg, args)
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
