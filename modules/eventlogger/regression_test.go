package eventlogger

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/CrisisTextLine/modular"
	cloudevents "github.com/cloudevents/sdk-go/v2"
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

// TestEventLogger_NoiseReductionForModuleSpecificEarlyEvents verifies that module-specific
// early lifecycle events are queued instead of producing errors when the eventlogger
// has not yet started. This test validates the "queue until ready" approach that
// addresses the issue described in #80.
func TestEventLogger_NoiseReductionForModuleSpecificEarlyEvents(t *testing.T) {
	logger := &capturingLogger{}

	// Create a simple eventlogger module without full application init
	mod := NewModule().(*EventLoggerModule)

	// Set up minimal config to initialize the module
	cfg := &EventLoggerConfig{
		Enabled:       true,
		LogLevel:      "INFO",
		Format:        "structured",
		BufferSize:    5,
		FlushInterval: 100 * time.Millisecond,
		OutputTargets: []OutputTargetConfig{{
			Type:    "console",
			Level:   "INFO",
			Format:  "structured",
			Console: &ConsoleTargetConfig{UseColor: false, Timestamps: false},
		}},
	}
	mod.config = cfg
	mod.logger = logger

	// Initialize channels like the Init() method would, but don't start
	mod.eventChan = make(chan cloudevents.Event, mod.config.BufferSize)
	mod.stopChan = make(chan struct{})
	mod.eventQueue = make([]cloudevents.Event, 0)
	mod.queueMaxSize = 1000

	// At this point, the eventlogger is initialized but NOT started (mod.started is still false).

	// Clear any existing log entries before the test to get clean results
	logger.mu.Lock()
	logger.entries = nil
	logger.mu.Unlock()

	// Emit the specific noisy event types mentioned in the issue directly to the module
	noisyEarlyEvents := []string{
		"com.modular.chimux.config.loaded",       // module-specific config event
		"com.modular.chimux.config.validated",    // module-specific config event
		"com.modular.chimux.router.created",      // specific example from issue
		"com.modular.httpserver.cors.configured", // specific example from issue
		"com.modular.reverseproxy.config.loaded", // another module-specific config
		"com.modular.scheduler.config.validated", // another module-specific config
	}

	errorCount := 0
	for _, et := range noisyEarlyEvents {
		evt := modular.NewCloudEvent(et, "test-module", nil, nil)
		err := mod.OnEvent(context.Background(), evt)
		if err != nil {
			errorCount++
			t.Logf("Event %s returned error: %v", et, err)
		} else {
			t.Logf("Event %s was silently dropped (no error)", et)
		}
	}

	// With the queue-until-ready approach: no errors for any early lifecycle events
	if errorCount > 0 {
		t.Fatalf("module-specific early lifecycle events should be queued (not produce errors) with queue-until-ready approach, but got %d errors", errorCount)
	}

	t.Logf("✓ All %d module-specific early lifecycle events were silently queued without errors", len(noisyEarlyEvents))
}

// TestEventLogger_AllEventsQueuedWhenNotStarted verifies that ALL events
// are queued when the logger is not started (queue-until-ready approach).
func TestEventLogger_AllEventsQueuedWhenNotStarted(t *testing.T) {
	logger := &capturingLogger{}

	// Create a simple eventlogger module without full application init
	mod := NewModule().(*EventLoggerModule)

	// Set up minimal config to initialize the module
	cfg := &EventLoggerConfig{
		Enabled:       true,
		LogLevel:      "INFO",
		Format:        "structured",
		BufferSize:    5,
		FlushInterval: 100 * time.Millisecond,
		OutputTargets: []OutputTargetConfig{{
			Type:    "console",
			Level:   "INFO",
			Format:  "structured",
			Console: &ConsoleTargetConfig{UseColor: false, Timestamps: false},
		}},
	}
	mod.config = cfg
	mod.logger = logger

	// Initialize channels like the Init() method would, but don't start
	mod.eventChan = make(chan cloudevents.Event, mod.config.BufferSize)
	mod.stopChan = make(chan struct{})
	mod.eventQueue = make([]cloudevents.Event, 0)
	mod.queueMaxSize = 1000

	// Test events that should NOT be treated as benign
	nonBenignEvents := []string{
		"com.mycompany.custom.event",          // Random custom event
		"user.created",                        // Business logic event
		"payment.processed",                   // Business logic event
		"com.modular.chimux.request.received", // Runtime operational event (not early lifecycle)
	}

	errorCount := 0
	for _, et := range nonBenignEvents {
		evt := modular.NewCloudEvent(et, "test-module", nil, nil)
		err := mod.OnEvent(context.Background(), evt)
		if err != nil {
			errorCount++
			t.Logf("Event %s correctly returned error: %v", et, err)
		} else {
			t.Logf("Event %s was queued (no error)", et)
		}
	}

	// With the queue-until-ready approach, ALL events should be queued (no errors)
	if errorCount != 0 {
		t.Fatalf("expected all events to be queued (0 errors), but got %d errors", errorCount)
	}

	// Verify events were actually queued
	mod.mutex.RLock()
	queueSize := len(mod.eventQueue)
	mod.mutex.RUnlock()

	if queueSize != len(nonBenignEvents) {
		t.Fatalf("Expected %d events in queue, got %d", len(nonBenignEvents), queueSize)
	}

	t.Logf("✓ All %d events were successfully queued for processing when logger starts", len(nonBenignEvents))
}

// TestEventLogger_QueuedEventsProcessedOnStart verifies that events queued before
// Start() are processed when the logger starts up.
func TestEventLogger_QueuedEventsProcessedOnStart(t *testing.T) {
	logger := &capturingLogger{}

	// Create a mock application
	configProvider := modular.NewStdConfigProvider(&struct{}{})
	app := modular.NewObservableApplication(configProvider, logger)

	// Register and initialize the eventlogger module
	mod := NewModule()
	app.RegisterModule(mod)

	// Initialize to set up the module
	if err := app.Init(); err != nil {
		t.Fatalf("Failed to initialize application: %v", err)
	}

	// Get the eventlogger module instance
	var eventLogger *EventLoggerModule
	err := app.GetService("eventlogger.observer", &eventLogger)
	if err != nil {
		t.Fatalf("Failed to get eventlogger service: %v", err)
	}

	// At this point, the eventlogger is initialized but not started

	// Clear logger entries to get clean test results
	logger.mu.Lock()
	logger.entries = nil
	logger.mu.Unlock()

	// Emit some events before starting - these should be queued
	preStartEvents := []string{
		"com.modular.chimux.router.created",
		"user.registered",
		"com.modular.httpserver.cors.configured",
	}

	for _, et := range preStartEvents {
		evt := modular.NewCloudEvent(et, "test-source", map[string]interface{}{"test": "data"}, nil)
		err := eventLogger.OnEvent(context.Background(), evt)
		if err != nil {
			t.Errorf("Unexpected error queueing event %s: %v", et, err)
		}
	}

	// Verify events are queued (at least the ones we emitted)
	eventLogger.mutex.RLock()
	queueSize := len(eventLogger.eventQueue)
	eventLogger.mutex.RUnlock()

	if queueSize < len(preStartEvents) {
		t.Fatalf("Expected at least %d events in queue, got %d", len(preStartEvents), queueSize)
	}

	t.Logf("Queue has %d events (at least %d are ours)", queueSize, len(preStartEvents))

	// Now start the logger - this should process the queued events
	if err := app.Start(); err != nil {
		t.Fatalf("Failed to start application: %v", err)
	}

	// Wait a bit for async processing
	time.Sleep(100 * time.Millisecond)

	// Verify the queue was cleared
	eventLogger.mutex.RLock()
	queueSizeAfterStart := len(eventLogger.eventQueue)
	eventLogger.mutex.RUnlock()

	if queueSizeAfterStart != 0 {
		t.Errorf("Expected queue to be empty after start, but has %d events", queueSizeAfterStart)
	}

	// The important validation is that the queue was cleared, indicating events were processed
	t.Logf("✓ Queue was processed on start (queue cleared: %d → %d)", queueSize, queueSizeAfterStart)

	// Cleanup
	_ = app.Stop()
}

// Helper to simulate an external lifecycle event arrival before Start (if needed in future tests).
func emitDirect(mod *EventLoggerModule, typ string) {
	evt := modular.NewCloudEvent(typ, "application", nil, nil)
	_ = mod.OnEvent(context.Background(), evt)
}
