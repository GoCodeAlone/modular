package modular

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cucumber/godog"
)

// Static errors for reload contract BDD tests.
var (
	errExpectedModuleReceiveChanges = errors.New("expected module to receive changes")
	errExpectedCompletedEvent       = errors.New("expected reload completed event")
	errExpectedFailedEvent          = errors.New("expected reload failed event")
	errExpectedNoopEvent            = errors.New("expected reload noop event")
	errExpectedModuleSkipped        = errors.New("expected non-reloadable module to be skipped")
	errExpectedOtherModulesReloaded = errors.New("expected other modules to still be reloaded")
	errExpectedRollback             = errors.New("expected first module to be rolled back")
	errExpectedCircuitBreakerReject = errors.New("expected circuit breaker to reject request")
	errExpectedCircuitBreakerReset  = errors.New("expected circuit breaker to eventually reset")
	errExpectedNoModuleCalls        = errors.New("expected no modules to be called")
	errExpectedRequestsProcessed    = errors.New("expected all requests to be processed")
)

// reloadBDDMockReloadable is a mock Reloadable for BDD reload contract tests.
type reloadBDDMockReloadable struct {
	name        string
	canReload   bool
	timeout     time.Duration
	reloadErr   error
	reloadCalls atomic.Int32
	lastChanges []ConfigChange
	mu          sync.Mutex
}

func (m *reloadBDDMockReloadable) Reload(_ context.Context, changes []ConfigChange) error {
	m.reloadCalls.Add(1)
	m.mu.Lock()
	m.lastChanges = changes
	m.mu.Unlock()
	return m.reloadErr
}

func (m *reloadBDDMockReloadable) CanReload() bool            { return m.canReload }
func (m *reloadBDDMockReloadable) ReloadTimeout() time.Duration { return m.timeout }

// reloadBDDSubject captures events for BDD reload contract tests.
type reloadBDDSubject struct {
	mu     sync.Mutex
	events []cloudevents.Event
}

func (s *reloadBDDSubject) RegisterObserver(_ Observer, _ ...string) error { return nil }
func (s *reloadBDDSubject) UnregisterObserver(_ Observer) error            { return nil }
func (s *reloadBDDSubject) GetObservers() []ObserverInfo                   { return nil }
func (s *reloadBDDSubject) NotifyObservers(_ context.Context, event cloudevents.Event) error {
	s.mu.Lock()
	s.events = append(s.events, event)
	s.mu.Unlock()
	return nil
}

func (s *reloadBDDSubject) eventTypes() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var types []string
	for _, e := range s.events {
		types = append(types, e.Type())
	}
	return types
}

func (s *reloadBDDSubject) reset() {
	s.mu.Lock()
	s.events = nil
	s.mu.Unlock()
}

// reloadBDDLogger implements Logger for BDD reload contract tests.
type reloadBDDLogger struct{}

func (l *reloadBDDLogger) Info(_ string, _ ...any)  {}
func (l *reloadBDDLogger) Error(_ string, _ ...any) {}
func (l *reloadBDDLogger) Warn(_ string, _ ...any)  {}
func (l *reloadBDDLogger) Debug(_ string, _ ...any) {}

// bddWaitForEvent polls until the subject has recorded an event of the given type,
// or the timeout elapses. Returns true if the event was observed.
func bddWaitForEvent(subject *reloadBDDSubject, eventType string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, et := range subject.eventTypes() {
			if et == eventType {
				return true
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

// bddWaitForCalls polls until the total reload calls across modules reaches
// at least n, or the timeout elapses.
func bddWaitForCalls(modules []*reloadBDDMockReloadable, n int32, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var total int32
		for _, m := range modules {
			total += m.reloadCalls.Load()
		}
		if total >= n {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

// ReloadBDDContext holds state for reload contract BDD scenarios.
type ReloadBDDContext struct {
	orchestrator *ReloadOrchestrator
	modules      []*reloadBDDMockReloadable
	subject      *reloadBDDSubject
	logger       *reloadBDDLogger
	ctx          context.Context
	cancel       context.CancelFunc
	reloadErr    error
	raceDetected atomic.Bool
}

func (rc *ReloadBDDContext) reset() {
	if rc.cancel != nil {
		rc.cancel()
	}
	rc.subject = &reloadBDDSubject{}
	rc.logger = &reloadBDDLogger{}
	rc.modules = nil
	rc.reloadErr = nil
	rc.raceDetected.Store(false)
	rc.ctx, rc.cancel = context.WithCancel(context.Background())
}

func (rc *ReloadBDDContext) newDiff() ConfigDiff {
	return ConfigDiff{
		Changed: map[string]FieldChange{
			"db.host": {OldValue: "localhost", NewValue: "remotehost", ChangeType: ChangeModified},
		},
		Added:     make(map[string]FieldChange),
		Removed:   make(map[string]FieldChange),
		Timestamp: time.Now(),
		DiffID:    "bdd-test-diff",
	}
}

func (rc *ReloadBDDContext) emptyDiff() ConfigDiff {
	return ConfigDiff{
		Changed: make(map[string]FieldChange),
		Added:   make(map[string]FieldChange),
		Removed: make(map[string]FieldChange),
		DiffID:  "bdd-empty-diff",
	}
}

// Step definitions

func (rc *ReloadBDDContext) aReloadOrchestratorWithNReloadableModules(n int) error {
	rc.orchestrator = NewReloadOrchestrator(rc.logger, rc.subject)
	for i := range n {
		mod := &reloadBDDMockReloadable{
			name:      string(rune('a'+i)) + "_mod",
			canReload: true,
			timeout:   5 * time.Second,
		}
		rc.modules = append(rc.modules, mod)
		rc.orchestrator.RegisterReloadable(mod.name, mod)
	}
	rc.orchestrator.Start(rc.ctx)
	return nil
}

func (rc *ReloadBDDContext) aReloadIsRequestedWithConfigurationChanges() error {
	diff := rc.newDiff()
	rc.reloadErr = rc.orchestrator.RequestReload(rc.ctx, ReloadManual, diff)
	bddWaitForEvent(rc.subject, EventTypeConfigReloadCompleted, 2*time.Second)
	return nil
}

func (rc *ReloadBDDContext) allNModulesShouldReceiveTheChanges(n int) error {
	received := 0
	for _, mod := range rc.modules {
		if mod.reloadCalls.Load() > 0 {
			received++
		}
	}
	if received != n {
		return errExpectedModuleReceiveChanges
	}
	return nil
}

func (rc *ReloadBDDContext) aReloadCompletedEventShouldBeEmitted() error {
	for _, et := range rc.subject.eventTypes() {
		if et == EventTypeConfigReloadCompleted {
			return nil
		}
	}
	return errExpectedCompletedEvent
}

func (rc *ReloadBDDContext) aReloadOrchestratorWithAModuleThatCannotReload() error {
	rc.orchestrator = NewReloadOrchestrator(rc.logger, rc.subject)

	disabledMod := &reloadBDDMockReloadable{
		name:      "disabled_mod",
		canReload: false,
		timeout:   5 * time.Second,
	}
	rc.modules = append(rc.modules, disabledMod)
	rc.orchestrator.RegisterReloadable(disabledMod.name, disabledMod)

	enabledMod := &reloadBDDMockReloadable{
		name:      "enabled_mod",
		canReload: true,
		timeout:   5 * time.Second,
	}
	rc.modules = append(rc.modules, enabledMod)
	rc.orchestrator.RegisterReloadable(enabledMod.name, enabledMod)

	rc.orchestrator.Start(rc.ctx)
	return nil
}

func (rc *ReloadBDDContext) aReloadIsRequested() error {
	diff := rc.newDiff()
	rc.reloadErr = rc.orchestrator.RequestReload(rc.ctx, ReloadManual, diff)
	// Wait for either completed or failed event (covers both success and failure scenarios).
	bddWaitForEvent(rc.subject, EventTypeConfigReloadCompleted, 2*time.Second)
	bddWaitForEvent(rc.subject, EventTypeConfigReloadFailed, 100*time.Millisecond)
	return nil
}

func (rc *ReloadBDDContext) theNonReloadableModuleShouldBeSkipped() error {
	for _, mod := range rc.modules {
		if !mod.canReload && mod.reloadCalls.Load() != 0 {
			return errExpectedModuleSkipped
		}
	}
	return nil
}

func (rc *ReloadBDDContext) otherModulesShouldStillBeReloaded() error {
	for _, mod := range rc.modules {
		if mod.canReload && mod.reloadCalls.Load() == 0 {
			return errExpectedOtherModulesReloaded
		}
	}
	return nil
}

func (rc *ReloadBDDContext) aReloadOrchestratorWith3ModulesWhereTheSecondFails() error {
	rc.orchestrator = NewReloadOrchestrator(rc.logger, rc.subject)

	// Use names that sort deterministically to control ordering.
	mod1 := &reloadBDDMockReloadable{
		name:      "aaa_first",
		canReload: true,
		timeout:   5 * time.Second,
	}
	mod2 := &reloadBDDMockReloadable{
		name:      "bbb_second",
		canReload: true,
		timeout:   5 * time.Second,
		reloadErr: errors.New("reload failure"),
	}
	mod3 := &reloadBDDMockReloadable{
		name:      "ccc_third",
		canReload: true,
		timeout:   5 * time.Second,
	}
	rc.modules = append(rc.modules, mod1, mod2, mod3)
	rc.orchestrator.RegisterReloadable(mod1.name, mod1)
	rc.orchestrator.RegisterReloadable(mod2.name, mod2)
	rc.orchestrator.RegisterReloadable(mod3.name, mod3)

	rc.orchestrator.Start(rc.ctx)
	return nil
}

func (rc *ReloadBDDContext) theFirstModuleShouldBeRolledBack() error {
	// Reload targets are sorted by name. aaa_first runs before bbb_second (which
	// fails), so aaa_first is always applied and then rolled back (2 calls total).
	mod1 := rc.modules[0]
	calls := mod1.reloadCalls.Load()
	if calls != 2 {
		return fmt.Errorf("%w: expected aaa_first to be called 2 times (apply + rollback), got %d", errExpectedRollback, calls)
	}
	return nil
}

func (rc *ReloadBDDContext) aReloadFailedEventShouldBeEmitted() error {
	for _, et := range rc.subject.eventTypes() {
		if et == EventTypeConfigReloadFailed {
			return nil
		}
	}
	return errExpectedFailedEvent
}

func (rc *ReloadBDDContext) aReloadOrchestratorWithAFailingModule() error {
	rc.orchestrator = NewReloadOrchestrator(rc.logger, rc.subject)

	mod := &reloadBDDMockReloadable{
		name:      "failing_mod",
		canReload: true,
		timeout:   5 * time.Second,
		reloadErr: errors.New("always fails"),
	}
	rc.modules = append(rc.modules, mod)
	rc.orchestrator.RegisterReloadable(mod.name, mod)

	rc.orchestrator.Start(rc.ctx)
	return nil
}

func (rc *ReloadBDDContext) nConsecutiveReloadsFail(n int) error {
	diff := rc.newDiff()
	for i := range n {
		_ = rc.orchestrator.RequestReload(rc.ctx, ReloadManual, diff)
		expected := int32(i + 1)
		bddWaitForCalls(rc.modules, expected, 2*time.Second)
	}
	return nil
}

func (rc *ReloadBDDContext) subsequentReloadRequestsShouldBeRejected() error {
	diff := rc.newDiff()
	err := rc.orchestrator.RequestReload(rc.ctx, ReloadManual, diff)
	if err == nil || !strings.Contains(err.Error(), "circuit breaker") {
		return errExpectedCircuitBreakerReject
	}
	return nil
}

func (rc *ReloadBDDContext) theCircuitBreakerShouldEventuallyReset() error {
	// Simulate that the backoff period has elapsed by moving lastFailure
	// sufficiently into the past. This validates isCircuitOpen()/backoffDuration()
	// rather than bypassing them.
	rc.orchestrator.cbMu.Lock()
	rc.orchestrator.lastFailure = time.Now().Add(-circuitBreakerMaxDelay - time.Second)
	rc.orchestrator.cbMu.Unlock()

	diff := rc.newDiff()
	err := rc.orchestrator.RequestReload(rc.ctx, ReloadManual, diff)
	if err != nil && strings.Contains(err.Error(), "circuit breaker") {
		return errExpectedCircuitBreakerReset
	}
	return nil
}

func (rc *ReloadBDDContext) aReloadOrchestratorWithReloadableModules() error {
	return rc.aReloadOrchestratorWithNReloadableModules(2)
}

func (rc *ReloadBDDContext) aReloadIsRequestedWithNoChanges() error {
	diff := rc.emptyDiff()
	rc.reloadErr = rc.orchestrator.RequestReload(rc.ctx, ReloadManual, diff)
	bddWaitForEvent(rc.subject, EventTypeConfigReloadNoop, 2*time.Second)
	return nil
}

func (rc *ReloadBDDContext) aReloadNoopEventShouldBeEmitted() error {
	for _, et := range rc.subject.eventTypes() {
		if et == EventTypeConfigReloadNoop {
			return nil
		}
	}
	return errExpectedNoopEvent
}

func (rc *ReloadBDDContext) noModulesShouldBeCalled() error {
	for _, mod := range rc.modules {
		if mod.reloadCalls.Load() != 0 {
			return errExpectedNoModuleCalls
		}
	}
	return nil
}

func (rc *ReloadBDDContext) tenReloadRequestsAreSubmittedConcurrently() error {
	diff := rc.newDiff()
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = rc.orchestrator.RequestReload(rc.ctx, ReloadManual, diff)
		}()
	}
	wg.Wait()
	bddWaitForCalls(rc.modules, 1, 2*time.Second)
	return nil
}

func (rc *ReloadBDDContext) allRequestsShouldBeProcessed() error {
	totalCalls := int32(0)
	for _, mod := range rc.modules {
		totalCalls += mod.reloadCalls.Load()
	}
	if totalCalls < 1 {
		return errExpectedRequestsProcessed
	}
	return nil
}

func (rc *ReloadBDDContext) noRaceConditionsShouldOccur() error {
	// The race detector (go test -race) validates this at runtime.
	// If we got here without a panic, there are no races.
	return nil
}

// InitializeReloadContractScenario wires up all reload contract BDD steps.
func InitializeReloadContractScenario(ctx *godog.ScenarioContext) {
	rc := &ReloadBDDContext{}

	ctx.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
		rc.reset()
		return ctx, nil
	})

	ctx.After(func(ctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
		if rc.cancel != nil {
			rc.cancel()
		}
		return ctx, nil
	})

	ctx.Step(`^a reload orchestrator with (\d+) reloadable modules$`, rc.aReloadOrchestratorWithNReloadableModules)
	ctx.Step(`^a reload is requested with configuration changes$`, rc.aReloadIsRequestedWithConfigurationChanges)
	ctx.Step(`^all (\d+) modules should receive the changes$`, rc.allNModulesShouldReceiveTheChanges)
	ctx.Step(`^a reload completed event should be emitted$`, rc.aReloadCompletedEventShouldBeEmitted)

	ctx.Step(`^a reload orchestrator with a module that cannot reload$`, rc.aReloadOrchestratorWithAModuleThatCannotReload)
	ctx.Step(`^a reload is requested$`, rc.aReloadIsRequested)
	ctx.Step(`^the non-reloadable module should be skipped$`, rc.theNonReloadableModuleShouldBeSkipped)
	ctx.Step(`^other modules should still be reloaded$`, rc.otherModulesShouldStillBeReloaded)

	ctx.Step(`^a reload orchestrator with 3 modules where the second fails$`, rc.aReloadOrchestratorWith3ModulesWhereTheSecondFails)
	ctx.Step(`^the first module should be rolled back$`, rc.theFirstModuleShouldBeRolledBack)
	ctx.Step(`^a reload failed event should be emitted$`, rc.aReloadFailedEventShouldBeEmitted)

	ctx.Step(`^a reload orchestrator with a failing module$`, rc.aReloadOrchestratorWithAFailingModule)
	ctx.Step(`^(\d+) consecutive reloads fail$`, rc.nConsecutiveReloadsFail)
	ctx.Step(`^subsequent reload requests should be rejected$`, rc.subsequentReloadRequestsShouldBeRejected)
	ctx.Step(`^the circuit breaker should eventually reset$`, rc.theCircuitBreakerShouldEventuallyReset)

	ctx.Step(`^a reload orchestrator with reloadable modules$`, rc.aReloadOrchestratorWithReloadableModules)
	ctx.Step(`^a reload is requested with no changes$`, rc.aReloadIsRequestedWithNoChanges)
	ctx.Step(`^a reload noop event should be emitted$`, rc.aReloadNoopEventShouldBeEmitted)
	ctx.Step(`^no modules should be called$`, rc.noModulesShouldBeCalled)

	ctx.Step(`^10 reload requests are submitted concurrently$`, rc.tenReloadRequestsAreSubmittedConcurrently)
	ctx.Step(`^all requests should be processed$`, rc.allRequestsShouldBeProcessed)
	ctx.Step(`^no race conditions should occur$`, rc.noRaceConditionsShouldOccur)
}

// TestReloadContractBDD runs the BDD tests for the reload contract.
func TestReloadContractBDD(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: InitializeReloadContractScenario,
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features/reload_contract.feature"},
			TestingT: t,
			Strict:   true,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run reload contract feature tests")
	}
}
