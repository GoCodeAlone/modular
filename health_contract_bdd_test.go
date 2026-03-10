package modular

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cucumber/godog"
)

// Static errors for health contract BDD tests.
var (
	errExpectedOverallHealthy       = errors.New("expected overall status to be healthy")
	errExpectedOverallUnhealthy     = errors.New("expected overall status to be unhealthy")
	errExpectedReadinessHealthy     = errors.New("expected readiness to be healthy")
	errExpectedReadinessUnhealthy   = errors.New("expected readiness to be unhealthy")
	errExpectedPanicUnhealthy       = errors.New("expected panicking provider to report unhealthy")
	errExpectedOtherProvidersChecked = errors.New("expected other providers to still be checked")
	errExpectedDegradedStatus       = errors.New("expected provider status to be degraded")
	errExpectedSingleCall           = errors.New("expected provider to be called only once")
	errExpectedRefreshCall          = errors.New("expected provider to be called again on refresh")
	errExpectedStatusChangedEvent   = errors.New("expected health status changed event")
)

// healthBDDProvider is a configurable mock HealthProvider for BDD tests.
type healthBDDProvider struct {
	reports   []HealthReport
	err       error
	callCount atomic.Int32
	panicMsg  string
	mu        sync.Mutex
}

func (p *healthBDDProvider) HealthCheck(_ context.Context) ([]HealthReport, error) {
	p.callCount.Add(1)
	if p.panicMsg != "" {
		panic(p.panicMsg)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.err != nil {
		return nil, p.err
	}
	reports := make([]HealthReport, len(p.reports))
	copy(reports, p.reports)
	for i := range reports {
		reports[i].CheckedAt = time.Now()
	}
	return reports, nil
}

func (p *healthBDDProvider) setReports(reports []HealthReport) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.reports = reports
}

// bddTemporaryError implements the Temporary() bool interface for degraded status.
type bddTemporaryError struct {
	msg string
}

func (e *bddTemporaryError) Error() string   { return e.msg }
func (e *bddTemporaryError) Temporary() bool { return true }

// healthBDDSubject captures events for BDD health contract tests.
type healthBDDSubject struct {
	mu     sync.Mutex
	events []cloudevents.Event
}

func (s *healthBDDSubject) RegisterObserver(_ Observer, _ ...string) error { return nil }
func (s *healthBDDSubject) UnregisterObserver(_ Observer) error            { return nil }
func (s *healthBDDSubject) GetObservers() []ObserverInfo                   { return nil }
func (s *healthBDDSubject) NotifyObservers(_ context.Context, event cloudevents.Event) error {
	s.mu.Lock()
	s.events = append(s.events, event)
	s.mu.Unlock()
	return nil
}

func (s *healthBDDSubject) hasEventType(eventType string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.events {
		if e.Type() == eventType {
			return true
		}
	}
	return false
}



// HealthBDDContext holds state for health contract BDD scenarios.
type HealthBDDContext struct {
	service    *AggregateHealthService
	subject    *healthBDDSubject
	providers  map[string]*healthBDDProvider
	result     *AggregatedHealth
	checkErr   error
}

func (hc *HealthBDDContext) reset() {
	hc.subject = &healthBDDSubject{}
	hc.providers = make(map[string]*healthBDDProvider)
	hc.service = nil
	hc.result = nil
	hc.checkErr = nil
}

func (hc *HealthBDDContext) ensureService() {
	if hc.service == nil {
		hc.service = NewAggregateHealthService(
			WithSubject(hc.subject),
			WithCacheTTL(250*time.Millisecond),
		)
	}
}

// Step definitions

func (hc *HealthBDDContext) aHealthServiceWithOneHealthyProvider() error {
	hc.ensureService()
	p := &healthBDDProvider{
		reports: []HealthReport{{
			Module:    "healthy-mod",
			Component: "main",
			Status:    StatusHealthy,
			Message:   "ok",
		}},
	}
	hc.providers["healthy"] = p
	hc.service.AddProvider("healthy", p)
	return nil
}

func (hc *HealthBDDContext) healthIsChecked() error {
	hc.result, hc.checkErr = hc.service.Check(context.Background())
	return nil
}

func (hc *HealthBDDContext) theOverallStatusShouldBe(expected string) error {
	if hc.result.Health.String() != expected {
		if expected == "healthy" {
			return errExpectedOverallHealthy
		}
		return errExpectedOverallUnhealthy
	}
	return nil
}

func (hc *HealthBDDContext) readinessShouldBe(expected string) error {
	if hc.result.Readiness.String() != expected {
		if expected == "healthy" {
			return errExpectedReadinessHealthy
		}
		return errExpectedReadinessUnhealthy
	}
	return nil
}

func (hc *HealthBDDContext) aHealthServiceWithOneHealthyAndOneUnhealthyProvider() error {
	hc.ensureService()
	healthy := &healthBDDProvider{
		reports: []HealthReport{{
			Module:    "healthy-mod",
			Component: "main",
			Status:    StatusHealthy,
			Message:   "ok",
		}},
	}
	unhealthy := &healthBDDProvider{
		reports: []HealthReport{{
			Module:    "unhealthy-mod",
			Component: "main",
			Status:    StatusUnhealthy,
			Message:   "down",
		}},
	}
	hc.providers["healthy"] = healthy
	hc.providers["unhealthy"] = unhealthy
	hc.service.AddProvider("healthy", healthy)
	hc.service.AddProvider("unhealthy", unhealthy)
	return nil
}

func (hc *HealthBDDContext) theOverallHealthShouldBe(expected string) error {
	return hc.theOverallStatusShouldBe(expected)
}

func (hc *HealthBDDContext) aHealthServiceWithOneHealthyRequiredAndOneUnhealthyOptionalProvider() error {
	hc.ensureService()
	required := &healthBDDProvider{
		reports: []HealthReport{{
			Module:    "required-mod",
			Component: "main",
			Status:    StatusHealthy,
			Message:   "ok",
			Optional:  false,
		}},
	}
	optional := &healthBDDProvider{
		reports: []HealthReport{{
			Module:    "optional-mod",
			Component: "aux",
			Status:    StatusUnhealthy,
			Message:   "not critical",
			Optional:  true,
		}},
	}
	hc.providers["required"] = required
	hc.providers["optional"] = optional
	hc.service.AddProvider("required", required)
	hc.service.AddProvider("optional", optional)
	return nil
}

func (hc *HealthBDDContext) aHealthServiceWithAProviderThatPanics() error {
	hc.ensureService()
	panicker := &healthBDDProvider{
		panicMsg: "something went terribly wrong",
	}
	stable := &healthBDDProvider{
		reports: []HealthReport{{
			Module:    "stable-mod",
			Component: "main",
			Status:    StatusHealthy,
			Message:   "ok",
		}},
	}
	hc.providers["panicker"] = panicker
	hc.providers["stable"] = stable
	hc.service.AddProvider("panicker", panicker)
	hc.service.AddProvider("stable", stable)
	return nil
}

func (hc *HealthBDDContext) thePanickingProviderShouldReport(expected string) error {
	for _, r := range hc.result.Reports {
		if r.Component == "panic-recovery" {
			if r.Status.String() != expected {
				return errExpectedPanicUnhealthy
			}
			return nil
		}
	}
	return errExpectedPanicUnhealthy
}

func (hc *HealthBDDContext) otherProvidersShouldStillBeChecked() error {
	for _, r := range hc.result.Reports {
		if r.Module == "stable-mod" {
			return nil
		}
	}
	return errExpectedOtherProvidersChecked
}

func (hc *HealthBDDContext) aHealthServiceWithAProviderReturningATemporaryError() error {
	hc.ensureService()
	p := &healthBDDProvider{
		err: &bddTemporaryError{msg: "transient issue"},
	}
	hc.providers["temp-err"] = p
	hc.service.AddProvider("temp-err", p)
	return nil
}

func (hc *HealthBDDContext) theProviderStatusShouldBe(expected string) error {
	for _, r := range hc.result.Reports {
		if r.Status.String() == expected {
			return nil
		}
	}
	return errExpectedDegradedStatus
}

func (hc *HealthBDDContext) aHealthServiceWithA100msCacheTTL() error {
	hc.service = NewAggregateHealthService(
		WithSubject(hc.subject),
		WithCacheTTL(100*time.Millisecond),
	)
	return nil
}

func (hc *HealthBDDContext) aHealthyProvider() error {
	p := &healthBDDProvider{
		reports: []HealthReport{{
			Module:    "cached-mod",
			Component: "main",
			Status:    StatusHealthy,
			Message:   "ok",
		}},
	}
	hc.providers["cached"] = p
	hc.service.AddProvider("cached", p)
	return nil
}

func (hc *HealthBDDContext) healthIsCheckedTwiceWithin50ms() error {
	hc.result, hc.checkErr = hc.service.Check(context.Background())
	if hc.checkErr != nil {
		return hc.checkErr
	}
	// Second check within cache TTL
	time.Sleep(10 * time.Millisecond)
	hc.result, hc.checkErr = hc.service.Check(context.Background())
	return nil
}

func (hc *HealthBDDContext) theProviderShouldOnlyBeCalledOnce() error {
	p := hc.providers["cached"]
	if p.callCount.Load() != 1 {
		return errExpectedSingleCall
	}
	return nil
}

func (hc *HealthBDDContext) aHealthServiceWithCachedResults() error {
	hc.service = NewAggregateHealthService(
		WithSubject(hc.subject),
		WithCacheTTL(10*time.Second),
	)
	p := &healthBDDProvider{
		reports: []HealthReport{{
			Module:    "refresh-mod",
			Component: "main",
			Status:    StatusHealthy,
			Message:   "ok",
		}},
	}
	hc.providers["refresh"] = p
	hc.service.AddProvider("refresh", p)
	// Prime the cache
	_, _ = hc.service.Check(context.Background())
	return nil
}

func (hc *HealthBDDContext) healthIsCheckedWithForceRefresh() error {
	ctx := context.WithValue(context.Background(), ForceHealthRefreshKey, true)
	hc.result, hc.checkErr = hc.service.Check(ctx)
	return nil
}

func (hc *HealthBDDContext) theProviderShouldBeCalledAgain() error {
	p := hc.providers["refresh"]
	if p.callCount.Load() < 2 {
		return errExpectedRefreshCall
	}
	return nil
}

func (hc *HealthBDDContext) aHealthServiceWithAProviderThatTransitionsFromHealthyToUnhealthy() error {
	hc.ensureService()
	p := &healthBDDProvider{
		reports: []HealthReport{{
			Module:    "transitioning-mod",
			Component: "main",
			Status:    StatusHealthy,
			Message:   "ok",
		}},
	}
	hc.providers["transitioning"] = p
	hc.service.AddProvider("transitioning", p)

	// Do initial check to establish healthy baseline, then invalidate cache.
	_, _ = hc.service.Check(context.Background())
	hc.service.invalidateCache()

	// Transition to unhealthy.
	p.setReports([]HealthReport{{
		Module:    "transitioning-mod",
		Component: "main",
		Status:    StatusUnhealthy,
		Message:   "went down",
	}})
	return nil
}

func (hc *HealthBDDContext) healthIsCheckedAfterTheTransition() error {
	hc.result, hc.checkErr = hc.service.Check(context.Background())
	return nil
}

func (hc *HealthBDDContext) aHealthStatusChangedEventShouldBeEmitted() error {
	if hc.subject.hasEventType(EventTypeHealthStatusChanged) {
		return nil
	}
	return errExpectedStatusChangedEvent
}

// InitializeHealthContractScenario wires up all health contract BDD steps.
func InitializeHealthContractScenario(ctx *godog.ScenarioContext) {
	hc := &HealthBDDContext{}

	ctx.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
		hc.reset()
		return ctx, nil
	})

	ctx.Step(`^a health service with one healthy provider$`, hc.aHealthServiceWithOneHealthyProvider)
	ctx.Step(`^health is checked$`, hc.healthIsChecked)
	ctx.Step(`^the overall status should be "([^"]*)"$`, hc.theOverallStatusShouldBe)
	ctx.Step(`^readiness should be "([^"]*)"$`, hc.readinessShouldBe)

	ctx.Step(`^a health service with one healthy and one unhealthy provider$`, hc.aHealthServiceWithOneHealthyAndOneUnhealthyProvider)
	ctx.Step(`^the overall health should be "([^"]*)"$`, hc.theOverallHealthShouldBe)

	ctx.Step(`^a health service with one healthy required and one unhealthy optional provider$`, hc.aHealthServiceWithOneHealthyRequiredAndOneUnhealthyOptionalProvider)

	ctx.Step(`^a health service with a provider that panics$`, hc.aHealthServiceWithAProviderThatPanics)
	ctx.Step(`^the panicking provider should report "([^"]*)"$`, hc.thePanickingProviderShouldReport)
	ctx.Step(`^other providers should still be checked$`, hc.otherProvidersShouldStillBeChecked)

	ctx.Step(`^a health service with a provider returning a temporary error$`, hc.aHealthServiceWithAProviderReturningATemporaryError)
	ctx.Step(`^the provider status should be "([^"]*)"$`, hc.theProviderStatusShouldBe)

	ctx.Step(`^a health service with a 100ms cache TTL$`, hc.aHealthServiceWithA100msCacheTTL)
	ctx.Step(`^a healthy provider$`, hc.aHealthyProvider)
	ctx.Step(`^health is checked twice within 50ms$`, hc.healthIsCheckedTwiceWithin50ms)
	ctx.Step(`^the provider should only be called once$`, hc.theProviderShouldOnlyBeCalledOnce)

	ctx.Step(`^a health service with cached results$`, hc.aHealthServiceWithCachedResults)
	ctx.Step(`^health is checked with force refresh$`, hc.healthIsCheckedWithForceRefresh)
	ctx.Step(`^the provider should be called again$`, hc.theProviderShouldBeCalledAgain)

	ctx.Step(`^a health service with a provider that transitions from healthy to unhealthy$`, hc.aHealthServiceWithAProviderThatTransitionsFromHealthyToUnhealthy)
	ctx.Step(`^health is checked after the transition$`, hc.healthIsCheckedAfterTheTransition)
	ctx.Step(`^a health status changed event should be emitted$`, hc.aHealthStatusChangedEventShouldBeEmitted)
}

// TestHealthContractBDD runs the BDD tests for the health contract.
func TestHealthContractBDD(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: InitializeHealthContractScenario,
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features/health_contract.feature"},
			TestingT: t,
			Strict:   true,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run health contract feature tests")
	}
}
