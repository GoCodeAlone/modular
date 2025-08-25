package eventlogger

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/CrisisTextLine/modular"
)

// capturingLogger implements modular.Logger and stores entries for assertions.
type capturingLogger struct {
	mu      sync.Mutex
	entries []logEntry
}

type logEntry struct {
	level string
	msg   string
	args  []any
}

func (l *capturingLogger) append(level, msg string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, logEntry{level: level, msg: msg, args: args})
}

func (l *capturingLogger) Info(msg string, args ...any)  { l.append("INFO", msg, args...) }
func (l *capturingLogger) Error(msg string, args ...any) { l.append("ERROR", msg, args...) }
func (l *capturingLogger) Warn(msg string, args ...any)  { l.append("WARN", msg, args...) }
func (l *capturingLogger) Debug(msg string, args ...any) { l.append("DEBUG", msg, args...) }

// findErrorContaining returns true if any ERROR entry contains the substring.
func (l *capturingLogger) findErrorContaining(substr string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, e := range l.entries {
		if e.level == "ERROR" && (e.msg == substr || containsArgSubstring(e.args, substr)) {
			return true
		}
	}
	return false
}

func containsArgSubstring(args []any, substr string) bool {
	for _, a := range args {
		if s, ok := a.(string); ok {
			if len(substr) > 0 && contains(s, substr) {
				return true
			}
		}
	}
	return false
}

// small local substring helper (avoid pulling strings package unnecessarily for Contains)
func contains(haystack, needle string) bool {
	return len(needle) == 0 || (len(haystack) >= len(needle) && index(haystack, needle) >= 0)
}

// naive substring search (since tests only use tiny strings) to avoid importing strings
func index(h, n string) int {
	if len(n) == 0 {
		return 0
	}
outer:
	for i := 0; i+len(n) <= len(h); i++ {
		for j := 0; j < len(n); j++ {
			if h[i+j] != n[j] {
				continue outer
			}
		}
		return i
	}
	return -1
}

// TestEventLogger_StopDoesNotEmitAfterShutdown verifies regression (issue #1) capturing that
// previously Stop() emitted an operational event after marking started=false leading to observer errors.
// This test documents current (failing) behavior; it should be updated when fix is applied.
func TestEventLogger_StopDoesNotEmitAfterShutdown(t *testing.T) {
	logger := &capturingLogger{}
	app := modular.NewObservableApplication(modular.NewStdConfigProvider(struct{}{}), logger)

	// Provide explicit config before registering module to avoid defaults override.
	cfg := &EventLoggerConfig{Enabled: true, LogLevel: "INFO", Format: "structured", BufferSize: 10, FlushInterval: 50 * time.Millisecond, OutputTargets: []OutputTargetConfig{{Type: "console", Level: "INFO", Format: "structured", Console: &ConsoleTargetConfig{UseColor: false, Timestamps: false}}}}
	app.RegisterConfigSection(ModuleName, modular.NewStdConfigProvider(cfg))

	mod := NewModule().(*EventLoggerModule)
	app.RegisterModule(mod)

	if err := app.Init(); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	if err := app.Start(); err != nil {
		t.Fatalf("start failed: %v", err)
	}

	// Give a brief window for async startup emissions
	time.Sleep(20 * time.Millisecond)

	if err := app.Stop(); err != nil {
		t.Fatalf("stop failed: %v", err)
	}

	// Allow any async emissions from Stop to propagate
	time.Sleep(20 * time.Millisecond)

	// After fix we expect NO such error.
	if logger.findErrorContaining("event logger not started") {
		t.Fatalf("unexpected 'event logger not started' error during Stop")
	}
}

// TestEventLogger_EarlyLifecycleEventsDoNotError verifies that early application lifecycle events (config.loaded, config.validated)
// do not produce observer error logs from eventlogger when it has not yet fully started.
func TestEventLogger_EarlyLifecycleEventsDoNotError(t *testing.T) {
	logger := &capturingLogger{}
	app := modular.NewObservableApplication(modular.NewStdConfigProvider(struct{}{}), logger)

	cfg := &EventLoggerConfig{Enabled: true, LogLevel: "INFO", Format: "structured", BufferSize: 5, FlushInterval: 100 * time.Millisecond, OutputTargets: []OutputTargetConfig{{Type: "console", Level: "INFO", Format: "structured", Console: &ConsoleTargetConfig{UseColor: false, Timestamps: false}}}}
	app.RegisterConfigSection(ModuleName, modular.NewStdConfigProvider(cfg))

	mod := NewModule().(*EventLoggerModule)
	app.RegisterModule(mod)

	// Manually register observers (normally done during app.Init for ObservableModules)
	if err := mod.RegisterObservers(app); err != nil {
		t.Fatalf("register observers failed: %v", err)
	}

	// Emit lifecycle events that can occur early (before Start) simulating application Init sequence.
	earlyEvents := []string{modular.EventTypeConfigLoaded, modular.EventTypeConfigValidated, modular.EventTypeModuleRegistered}
	for _, et := range earlyEvents {
		evt := modular.NewCloudEvent(et, "application", nil, nil)
		_ = mod.OnEvent(context.Background(), evt) // ignore returned error here; we check logger for side-effects
	}

	// Post-fix expectation: no error log for benign early lifecycle events.
	if logger.findErrorContaining("event logger not started") {
		t.Fatalf("benign early lifecycle events produced 'event logger not started' error")
	}

	// Initialize application (ensures module.Init runs so Start won't panic) then start/stop.
	if err := app.Init(); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	_ = app.Start()
	_ = app.Stop()
}

func TestEventLogger_SynchronousStartupConfigFlag(t *testing.T) {
	logger := &capturingLogger{}
	app := modular.NewObservableApplication(modular.NewStdConfigProvider(struct{}{}), logger)
	cfg := &EventLoggerConfig{Enabled: true, LogLevel: "INFO", Format: "structured", BufferSize: 5, FlushInterval: 100 * time.Millisecond, StartupSync: true, OutputTargets: []OutputTargetConfig{{Type: "console", Level: "INFO", Format: "structured", Console: &ConsoleTargetConfig{UseColor: false, Timestamps: false}}}}
	app.RegisterConfigSection(ModuleName, modular.NewStdConfigProvider(cfg))
	mod := NewModule().(*EventLoggerModule)
	app.RegisterModule(mod)
	if err := app.Init(); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	if err := app.Start(); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	// Without sleep, attempt to emit a test event and ensure no ErrLoggerNotStarted
	evt := modular.NewCloudEvent("sync.startup.test", "test", nil, nil)
	if err := mod.OnEvent(context.Background(), evt); err != nil {
		t.Fatalf("OnEvent failed unexpectedly: %v", err)
	}
	_ = app.Stop()
}

// Helper to simulate an external lifecycle event arrival before Start (if needed in future tests).
func emitDirect(mod *EventLoggerModule, typ string) {
	evt := modular.NewCloudEvent(typ, "application", nil, nil)
	_ = mod.OnEvent(context.Background(), evt)
}
