package modular

import (
    "context"
    "os"
    "syscall"
    "testing"
    "time"
)

// simpleSyncLogger is a minimal logger for tests capturing messages (not exported to avoid API surface increase)
type simpleSyncLogger struct{}

func (l *simpleSyncLogger) Info(string, ...any)  {}
func (l *simpleSyncLogger) Error(string, ...any) {}
func (l *simpleSyncLogger) Warn(string, ...any)  {}
func (l *simpleSyncLogger) Debug(string, ...any) {}

// mockStartStopModule is a test module exercising Start/Stop hooks used by Run
type mockStartStopModule struct {
    started bool
    stopped bool
}

func (m *mockStartStopModule) Name() string { return "mockLifecycle" }
func (m *mockStartStopModule) Init(Application) error { return nil }
func (m *mockStartStopModule) Start(ctx context.Context) error {
    m.started = true
    return nil
}
func (m *mockStartStopModule) Stop(ctx context.Context) error {
    m.stopped = true
    return nil
}

// Ensure interfaces compile-time
var _ Module = (*mockStartStopModule)(nil)
var _ Startable = (*mockStartStopModule)(nil)
var _ Stoppable = (*mockStartStopModule)(nil)

// TestApplicationRunLifecycle covers the Run method which previously had zero coverage.
// It sends a SIGTERM to itself to unblock the Run signal wait.
func TestApplicationRunLifecycle(t *testing.T) {
    // Build minimal app
    appCfg := NewStdConfigProvider(struct{}{})
    app := NewStdApplication(appCfg, &simpleSyncLogger{}).(*StdApplication)

    mod := &mockStartStopModule{}
    app.RegisterModule(mod)

    // Run application in a goroutine (will block until signal)
    done := make(chan error, 1)
    go func() {
        done <- app.Run()
    }()

    // Give some time for Init+Start
    time.Sleep(100 * time.Millisecond)
    if !mod.started {
        t.Fatalf("expected module to be started")
    }

    // Send termination signal to current process to unblock Run
    p, err := os.FindProcess(os.Getpid())
    if err != nil {
        t.Fatalf("failed to find process: %v", err)
    }
    if err := p.Signal(syscall.SIGTERM); err != nil {
        t.Fatalf("failed to send signal: %v", err)
    }

    select {
    case err := <-done:
        if err != nil {
            t.Fatalf("Run returned unexpected error: %v", err)
        }
    case <-time.After(3 * time.Second):
        t.Fatalf("timeout waiting for Run to return")
    }

    if !mod.stopped {
        t.Fatalf("expected module to be stopped after Run completes")
    }
}
