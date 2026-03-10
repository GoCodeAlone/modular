package modular

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestSlogAdapter_ImplementsLogger(t *testing.T) {
	var _ Logger = (*SlogAdapter)(nil)
}

func TestSlogAdapter_DelegatesToSlog(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)
	adapter := NewSlogAdapter(logger)

	adapter.Info("test info", "key", "value")
	adapter.Error("test error", "err", "fail")
	adapter.Warn("test warn")
	adapter.Debug("test debug")

	output := buf.String()
	for _, msg := range []string{"test info", "test error", "test warn", "test debug"} {
		if !strings.Contains(output, msg) {
			t.Errorf("expected %q in output", msg)
		}
	}
}

func TestSlogAdapter_With(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	adapter := NewSlogAdapter(slog.New(handler)).With("module", "test")
	adapter.Info("with test")
	if !strings.Contains(buf.String(), "module=test") {
		t.Errorf("expected module=test in output, got: %s", buf.String())
	}
}

func TestSlogAdapter_WithGroup(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	adapter := NewSlogAdapter(slog.New(handler)).WithGroup("mygroup")
	adapter.Info("group test", "key", "val")
	if !strings.Contains(buf.String(), "mygroup") {
		t.Errorf("expected mygroup in output, got: %s", buf.String())
	}
}
