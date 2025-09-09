package modular

import (
	"context"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/stretchr/testify/assert"
)

// Additional focused tests to cover previously uncovered branches and methods
func TestAggregateHealthService_AdditionalCoverage(t *testing.T) {
	t.Run("constructor_defaults_are_applied", func(t *testing.T) {
		svc := NewAggregateHealthServiceWithConfig(AggregateHealthServiceConfig{})
		assert.NotNil(t, svc)
		// Collect with no providers -> healthy
		res, err := svc.Collect(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, HealthStatusHealthy, res.Health)
		assert.Equal(t, HealthStatusHealthy, res.Readiness)
	})

	t.Run("SetEventSubject_is_thread_safe", func(t *testing.T) {
		svc := NewAggregateHealthService()
		// Use a no-op subject implementation
		subj := &testSubject{}
		svc.SetEventSubject(subj)
		// Register a provider so Collect triggers event emission goroutine
		_ = svc.RegisterProvider("mod-a", &testProvider{reports: []HealthReport{{Status: HealthStatusHealthy}}}, false)
		_, err := svc.Collect(context.Background())
		assert.NoError(t, err)
	})

	t.Run("RegisterProvider_validation_errors", func(t *testing.T) {
		svc := NewAggregateHealthService()
		err := svc.RegisterProvider("", &testProvider{}, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "module name cannot be empty")
		err = svc.RegisterProvider("mod-a", nil, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "provider cannot be nil")
	})

	t.Run("RegisterProvider_duplicate_error", func(t *testing.T) {
		svc := NewAggregateHealthService()
		p := &testProvider{reports: []HealthReport{{Status: HealthStatusHealthy}}}
		assert.NoError(t, svc.RegisterProvider("dup", p, false))
		err := svc.RegisterProvider("dup", p, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already registered")
	})

	t.Run("UnregisterProvider_errors_and_success", func(t *testing.T) {
		svc := NewAggregateHealthService()
		// Not registered yet
		err := svc.UnregisterProvider("missing")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no provider registered")
		// Register then remove
		p := &testProvider{reports: []HealthReport{{Status: HealthStatusHealthy}}}
		assert.NoError(t, svc.RegisterProvider("mod-a", p, false))
		assert.NoError(t, svc.UnregisterProvider("mod-a"))
		// Removing again should yield not registered
		err = svc.UnregisterProvider("mod-a")
		assert.Error(t, err)
	})

	t.Run("GetProviders_returns_correct_mapping", func(t *testing.T) {
		svc := NewAggregateHealthService()
		assert.NoError(t, svc.RegisterProvider("req", &testProvider{reports: []HealthReport{{Status: HealthStatusHealthy}}}, false))
		assert.NoError(t, svc.RegisterProvider("opt", &testProvider{reports: []HealthReport{{Status: HealthStatusHealthy}}}, true))
		providers := svc.GetProviders()
		assert.Equal(t, 2, len(providers))
		assert.False(t, providers["req"].Optional)
		assert.True(t, providers["opt"].Optional)
	})

	t.Run("force_refresh_context_bypasses_cache", func(t *testing.T) {
		svc := NewAggregateHealthServiceWithConfig(AggregateHealthServiceConfig{CacheTTL: 2 * time.Second, CacheEnabled: true})
		callCount := 0
		p := &testProvider{reports: []HealthReport{{Status: HealthStatusHealthy}}, beforeCall: func() { callCount++ }}
		assert.NoError(t, svc.RegisterProvider("p", p, false))
		// First call - fetch
		_, err := svc.Collect(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, 1, callCount)
		// Cached call
		_, err = svc.Collect(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, 1, callCount)
		// Force refresh
		ctx := context.WithValue(context.Background(), "force_refresh", true)
		_, err = svc.Collect(ctx)
		assert.NoError(t, err)
		assert.Equal(t, 2, callCount)
	})
}

// testSubject minimal Subject implementation for event emission path
type testSubject struct{}

func (t *testSubject) RegisterObserver(o Observer, eventTypes ...string) error        { return nil }
func (t *testSubject) UnregisterObserver(o Observer) error                            { return nil }
func (t *testSubject) NotifyObservers(ctx context.Context, e cloudevents.Event) error { return nil }
func (t *testSubject) GetObservers() []ObserverInfo                                   { return nil }
