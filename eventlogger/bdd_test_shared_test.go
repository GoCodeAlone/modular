package eventlogger

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/GoCodeAlone/modular"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// EventLogger BDD Test Context
type EventLoggerBDDTestContext struct {
	app           modular.Application
	module        *EventLoggerModule
	service       *EventLoggerModule
	config        *EventLoggerConfig
	lastError     error
	loggedEvents  []cloudevents.Event
	tempDir       string
	outputLogs    []string
	testConsole   *testConsoleOutput
	testFile      *testFileOutput
	eventObserver *testEventObserver
	// fastEmit enables burst emission without per-event sleep (used to deterministically trigger buffer full events)
	fastEmit bool
}

// Test event observer for capturing emitted events
type testEventObserver struct {
	mu     sync.Mutex
	events []cloudevents.Event
}

func newTestEventObserver() *testEventObserver {
	return &testEventObserver{
		events: make([]cloudevents.Event, 0),
	}
}

func (t *testEventObserver) OnEvent(ctx context.Context, event cloudevents.Event) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.events = append(t.events, event.Clone())
	return nil
}

func (t *testEventObserver) ObserverID() string {
	return "test-observer-eventlogger"
}

func (t *testEventObserver) GetEvents() []cloudevents.Event {
	t.mu.Lock()
	defer t.mu.Unlock()
	events := make([]cloudevents.Event, len(t.events))
	copy(events, t.events)
	return events
}

func (t *testEventObserver) ClearEvents() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.events = make([]cloudevents.Event, 0)
}

func (ctx *EventLoggerBDDTestContext) resetContext() {
	if ctx.tempDir != "" {
		_ = os.RemoveAll(ctx.tempDir)
	}
	if ctx.app != nil {
		_ = ctx.app.Stop()
		// Give some time for cleanup
		time.Sleep(10 * time.Millisecond)
	}

	ctx.app = nil
	ctx.module = nil
	ctx.service = nil
	ctx.config = nil
	ctx.lastError = nil
	ctx.loggedEvents = nil
	ctx.tempDir = ""
	ctx.outputLogs = nil
	ctx.testConsole = nil
	ctx.testFile = nil
	ctx.eventObserver = nil
}

// Test helper structures
type testLogger struct{}

func (l *testLogger) Debug(msg string, keysAndValues ...interface{})   {}
func (l *testLogger) Info(msg string, keysAndValues ...interface{})    {}
func (l *testLogger) Warn(msg string, keysAndValues ...interface{})    {}
func (l *testLogger) Error(msg string, keysAndValues ...interface{})   {}
func (l *testLogger) With(keysAndValues ...interface{}) modular.Logger { return l }

// baseTestOutput provides common functionality for test output implementations
type baseTestOutput struct {
	logs  []string
	mutex sync.Mutex
}

func (b *baseTestOutput) Start(ctx context.Context) error {
	return nil
}

func (b *baseTestOutput) Stop(ctx context.Context) error {
	return nil
}

func (b *baseTestOutput) Flush() error {
	return nil
}

func (b *baseTestOutput) GetLogs() []string {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	result := make([]string, len(b.logs))
	copy(result, b.logs)
	return result
}

func (b *baseTestOutput) appendLog(logLine string) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.logs = append(b.logs, logLine)
}

type testConsoleOutput struct {
	baseTestOutput
}

func (t *testConsoleOutput) WriteEvent(entry *LogEntry) error {
	// Format the entry as it would appear in console output
	logLine := fmt.Sprintf("[%s] %s %s", entry.Timestamp.Format("2006-01-02 15:04:05"), entry.Level, entry.Type)
	if entry.Source != "" {
		logLine += fmt.Sprintf("\n  Source: %s", entry.Source)
	}
	if entry.Data != nil {
		logLine += fmt.Sprintf("\n  Data: %v", entry.Data)
	}
	if len(entry.Metadata) > 0 {
		logLine += "\n  Metadata:"
		for k, v := range entry.Metadata {
			logLine += fmt.Sprintf("\n    %s: %s", k, v)
		}
	}
	t.appendLog(logLine)
	return nil
}

type testFileOutput struct {
	baseTestOutput
}

func (t *testFileOutput) WriteEvent(entry *LogEntry) error {
	// Format the entry as JSON for file output
	logLine := fmt.Sprintf(`{"timestamp":"%s","level":"%s","type":"%s","source":"%s","data":%v}`,
		entry.Timestamp.Format("2006-01-02T15:04:05Z07:00"), entry.Level, entry.Type, entry.Source, entry.Data)
	t.appendLog(logLine)
	return nil
}

// Faulty output target for testing error scenarios
type faultyOutputTarget struct{}

func (f *faultyOutputTarget) Start(ctx context.Context) error {
	return nil
}

func (f *faultyOutputTarget) Stop(ctx context.Context) error {
	return nil
}

func (f *faultyOutputTarget) WriteEvent(entry *LogEntry) error {
	return fmt.Errorf("simulated output target failure")
}

func (f *faultyOutputTarget) Flush() error {
	return fmt.Errorf("simulated flush failure")
}

// Helper functions

// Helper function to check if a log entry contains a specific event type
func containsEventType(logEntry, eventType string) bool {
	// Use Go's built-in string search for better reliability
	return strings.Contains(logEntry, eventType)
}

// Helper function to extract event type from a formatted log line
func extractEventTypeFromLog(logLine string) string {
	// Log format: [timestamp] LEVEL TYPE
	// Look for pattern after the second space
	parts := strings.SplitN(logLine, " ", 3)
	if len(parts) >= 3 {
		// The third part should start with the event type
		typePart := strings.TrimSpace(parts[2])
		// Extract just the event type (before any newline)
		if idx := strings.Index(typePart, "\n"); idx >= 0 {
			return typePart[:idx]
		}
		return typePart
	}
	return ""
}
