package modular

import "testing"

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
