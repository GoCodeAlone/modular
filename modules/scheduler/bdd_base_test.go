package scheduler

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/GoCodeAlone/modular"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// Scheduler BDD Test Context
type SchedulerBDDTestContext struct {
	app           modular.Application
	module        *SchedulerModule
	service       *SchedulerModule
	config        *SchedulerConfig
	lastError     error
	jobID         string
	jobCompleted  bool
	jobResults    []string
	eventObserver *testEventObserver
	scheduledAt   time.Time
	started       bool
}

// testEventObserver captures CloudEvents during testing
type testEventObserver struct {
	events []cloudevents.Event
	mu     sync.RWMutex
}

func newTestEventObserver() *testEventObserver {
	return &testEventObserver{
		events: make([]cloudevents.Event, 0),
	}
}

func (t *testEventObserver) OnEvent(ctx context.Context, event cloudevents.Event) error {
	// Clone before locking to minimize time under write lock; clone is cheap
	cloned := event.Clone()
	t.mu.Lock()
	t.events = append(t.events, cloned)
	t.mu.Unlock()
	return nil
}

func (t *testEventObserver) ObserverID() string {
	return "test-observer-scheduler"
}

func (t *testEventObserver) GetEvents() []cloudevents.Event {
	t.mu.RLock()
	defer t.mu.RUnlock()
	events := make([]cloudevents.Event, len(t.events))
	copy(events, t.events)
	return events
}

func (t *testEventObserver) ClearEvents() {
	t.mu.Lock()
	t.events = make([]cloudevents.Event, 0)
	t.mu.Unlock()
}

func (ctx *SchedulerBDDTestContext) resetContext() {
	ctx.app = nil
	ctx.module = nil
	ctx.service = nil
	ctx.config = nil
	ctx.lastError = nil
	ctx.jobID = ""
	ctx.jobCompleted = false
	ctx.jobResults = nil
	ctx.started = false
}

// ensureAppStarted starts the application once per scenario so scheduled jobs can execute and emit events
func (ctx *SchedulerBDDTestContext) ensureAppStarted() error {
	if ctx.started {
		return nil
	}
	if ctx.app == nil {
		return fmt.Errorf("application not initialized")
	}
	if err := ctx.app.Start(); err != nil {
		return err
	}
	ctx.started = true
	return nil
}

func (ctx *SchedulerBDDTestContext) setupSchedulerModule() error {
	logger := &testLogger{}
	mainConfigProvider := modular.NewStdConfigProvider(struct{}{})
	ctx.app = modular.NewStdApplication(mainConfigProvider, logger)
	if cfSetter, ok := ctx.app.(interface{ SetConfigFeeders([]modular.Feeder) }); ok {
		cfSetter.SetConfigFeeders([]modular.Feeder{})
	}

	// Create and register scheduler module
	module := NewModule()
	ctx.module = module.(*SchedulerModule)

	// Register the scheduler config section with current config
	schedulerConfigProvider := modular.NewStdConfigProvider(ctx.config)
	ctx.app.RegisterConfigSection("scheduler", schedulerConfigProvider)

	// Register the module
	ctx.app.RegisterModule(ctx.module)

	// Initialize the application
	err := ctx.app.Init()
	if err != nil {
		ctx.lastError = err
		return err
	}

	return nil
}

// Test helper structures
// testLogger captures logs for assertion. We treat Warn/Error as potential test failures
// unless explicitly whitelisted (expected for a negative scenario like an intentional
// job failure or shutdown timeout). This helps ensure new warnings/errors are surfaced.
type testLogger struct {
	mu    sync.RWMutex
	debug []string
	info  []string
	warn  []string
	error []string
}

func (l *testLogger) record(dst *[]string, msg string, kv []interface{}) {
	b := strings.Builder{}
	b.WriteString(msg)
	if len(kv) > 0 {
		b.WriteString(" | ")
		for i := 0; i < len(kv); i += 2 {
			if i+1 < len(kv) {
				b.WriteString(fmt.Sprintf("%v=%v ", kv[i], kv[i+1]))
			} else {
				b.WriteString(fmt.Sprintf("%v", kv[i]))
			}
		}
	}
	l.mu.Lock()
	*dst = append(*dst, strings.TrimSpace(b.String()))
	l.mu.Unlock()
}

func (l *testLogger) Debug(msg string, keysAndValues ...interface{}) {
	l.record(&l.debug, msg, keysAndValues)
}
func (l *testLogger) Info(msg string, keysAndValues ...interface{}) {
	l.record(&l.info, msg, keysAndValues)
}
func (l *testLogger) Warn(msg string, keysAndValues ...interface{}) {
	l.record(&l.warn, msg, keysAndValues)
}
func (l *testLogger) Error(msg string, keysAndValues ...interface{}) {
	l.record(&l.error, msg, keysAndValues)
}
func (l *testLogger) With(keysAndValues ...interface{}) modular.Logger { return l }

// unexpectedWarningsOrErrors returns unexpected warn/error logs (excluding allowlist substrings)
func (l *testLogger) unexpectedWarningsOrErrors(allowlist []string) []string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	var out []string
	isAllowed := func(entry string) bool {
		for _, allow := range allowlist {
			if strings.Contains(entry, allow) {
				return true
			}
		}
		return false
	}
	for _, w := range l.warn {
		if !isAllowed(w) {
			out = append(out, "WARN: "+w)
		}
	}
	for _, e := range l.error {
		if !isAllowed(e) {
			out = append(out, "ERROR: "+e)
		}
	}
	return out
}
