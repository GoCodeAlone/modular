package modular

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestAggregateHealthServiceBasic tests basic functionality without build tags
func TestAggregateHealthServiceBasic(t *testing.T) {
	t.Run("should_create_service_with_default_config", func(t *testing.T) {
		service := NewAggregateHealthService()
		assert.NotNil(t, service)

		// Test with no providers - should return healthy by default
		ctx := context.Background()
		result, err := service.Collect(ctx)
		assert.NoError(t, err)
		assert.Equal(t, HealthStatusHealthy, result.Health)
		assert.Equal(t, HealthStatusHealthy, result.Readiness)
		assert.Empty(t, result.Reports)
	})

	t.Run("should_register_and_collect_from_provider", func(t *testing.T) {
		service := NewAggregateHealthService()
		provider := &testProvider{
			reports: []HealthReport{
				{
					Module:    "test-module",
					Status:    HealthStatusHealthy,
					Message:   "All good",
					CheckedAt: time.Now(),
				},
			},
		}

		err := service.RegisterProvider("test-module", provider, false)
		assert.NoError(t, err)

		ctx := context.Background()
		result, err := service.Collect(ctx)
		assert.NoError(t, err)
		assert.Equal(t, HealthStatusHealthy, result.Health)
		assert.Equal(t, HealthStatusHealthy, result.Readiness)
		assert.Len(t, result.Reports, 1)
		assert.Equal(t, "test-module", result.Reports[0].Module)
	})

	t.Run("should_aggregate_multiple_providers", func(t *testing.T) {
		service := NewAggregateHealthService()

		// Healthy provider
		healthyProvider := &testProvider{
			reports: []HealthReport{
				{Module: "healthy", Status: HealthStatusHealthy, Message: "OK"},
			},
		}

		// Unhealthy provider
		unhealthyProvider := &testProvider{
			reports: []HealthReport{
				{Module: "unhealthy", Status: HealthStatusUnhealthy, Message: "Error"},
			},
		}

		err := service.RegisterProvider("healthy", healthyProvider, false)
		assert.NoError(t, err)

		err = service.RegisterProvider("unhealthy", unhealthyProvider, false)
		assert.NoError(t, err)

		ctx := context.Background()
		result, err := service.Collect(ctx)
		assert.NoError(t, err)

		// Should be unhealthy overall due to one unhealthy provider
		assert.Equal(t, HealthStatusUnhealthy, result.Health)
		assert.Equal(t, HealthStatusUnhealthy, result.Readiness)
		assert.Len(t, result.Reports, 2)
	})

	t.Run("should_handle_optional_providers_for_readiness", func(t *testing.T) {
		service := NewAggregateHealthService()

		// Required healthy provider
		requiredProvider := &testProvider{
			reports: []HealthReport{
				{Module: "required", Status: HealthStatusHealthy, Message: "OK"},
			},
		}

		// Optional unhealthy provider
		optionalProvider := &testProvider{
			reports: []HealthReport{
				{Module: "optional", Status: HealthStatusUnhealthy, Message: "Error"},
			},
		}

		err := service.RegisterProvider("required", requiredProvider, false) // Not optional
		assert.NoError(t, err)

		err = service.RegisterProvider("optional", optionalProvider, true) // Optional
		assert.NoError(t, err)

		ctx := context.Background()
		result, err := service.Collect(ctx)
		assert.NoError(t, err)

		// Health should be unhealthy (includes all providers)
		assert.Equal(t, HealthStatusUnhealthy, result.Health)
		// Readiness should be healthy (only required providers affect readiness)
		assert.Equal(t, HealthStatusHealthy, result.Readiness)
		assert.Len(t, result.Reports, 2)
	})

	t.Run("should_handle_provider_errors", func(t *testing.T) {
		service := NewAggregateHealthService()

		// Provider that returns an error
		errorProvider := &testProvider{
			err: errors.New("provider failed"),
		}

		err := service.RegisterProvider("error-module", errorProvider, false)
		assert.NoError(t, err)

		ctx := context.Background()
		result, err := service.Collect(ctx)
		assert.NoError(t, err)

		// Should handle error and create an unhealthy report
		assert.Equal(t, HealthStatusUnhealthy, result.Health)
		assert.Len(t, result.Reports, 1)
		assert.Contains(t, result.Reports[0].Message, "Health check failed")
		assert.Equal(t, HealthStatusUnhealthy, result.Reports[0].Status)
	})

	t.Run("should_handle_panics_in_providers", func(t *testing.T) {
		service := NewAggregateHealthService()

		// Provider that panics
		panicProvider := &testProvider{
			shouldPanic: true,
		}

		err := service.RegisterProvider("panic-module", panicProvider, false)
		assert.NoError(t, err)

		ctx := context.Background()
		result, err := service.Collect(ctx)
		assert.NoError(t, err)

		// Should recover from panic and create an unhealthy report
		assert.Equal(t, HealthStatusUnhealthy, result.Health)
		assert.Len(t, result.Reports, 1)
		assert.Contains(t, result.Reports[0].Message, "panicked")
		assert.Equal(t, HealthStatusUnhealthy, result.Reports[0].Status)
	})

	t.Run("should_cache_results", func(t *testing.T) {
		service := NewAggregateHealthServiceWithConfig(AggregateHealthServiceConfig{
			CacheTTL:     100 * time.Millisecond,
			CacheEnabled: true,
		})

		callCount := 0
		provider := &testProvider{
			reports: []HealthReport{
				{Module: "test", Status: HealthStatusHealthy, Message: "OK"},
			},
			beforeCall: func() {
				callCount++
			},
		}

		err := service.RegisterProvider("test", provider, false)
		assert.NoError(t, err)

		ctx := context.Background()

		// First call should hit provider
		_, err = service.Collect(ctx)
		assert.NoError(t, err)
		assert.Equal(t, 1, callCount)

		// Second call should use cache
		_, err = service.Collect(ctx)
		assert.NoError(t, err)
		assert.Equal(t, 1, callCount) // Should still be 1

		// Wait for cache to expire
		time.Sleep(150 * time.Millisecond)

		// Third call should hit provider again
		_, err = service.Collect(ctx)
		assert.NoError(t, err)
		assert.Equal(t, 2, callCount)
	})
}

// testProvider is a test implementation of HealthProvider
type testProvider struct {
	reports     []HealthReport
	err         error
	shouldPanic bool
	beforeCall  func()
}

func (p *testProvider) HealthCheck(ctx context.Context) ([]HealthReport, error) {
	if p.beforeCall != nil {
		p.beforeCall()
	}

	if p.shouldPanic {
		panic("test panic")
	}

	if p.err != nil {
		return nil, p.err
	}

	// Fill in default values
	for i := range p.reports {
		if p.reports[i].CheckedAt.IsZero() {
			p.reports[i].CheckedAt = time.Now()
		}
		if p.reports[i].ObservedSince.IsZero() {
			p.reports[i].ObservedSince = time.Now()
		}
	}

	return p.reports, nil
}

// TestTemporaryError tests error handling for temporary errors
type temporaryError struct {
	msg string
}

func (e temporaryError) Error() string {
	return e.msg
}

func (e temporaryError) Temporary() bool {
	return true
}

func TestAggregateHealthService_TemporaryErrors(t *testing.T) {
	service := NewAggregateHealthService()

	// Provider that returns a temporary error
	tempErrorProvider := &testProvider{
		err: temporaryError{msg: "temporary connection issue"},
	}

	err := service.RegisterProvider("temp-error", tempErrorProvider, false)
	assert.NoError(t, err)

	ctx := context.Background()
	result, err := service.Collect(ctx)
	assert.NoError(t, err)

	// Temporary errors should result in degraded status
	assert.Equal(t, HealthStatusDegraded, result.Health)
	assert.Len(t, result.Reports, 1)
	assert.Equal(t, HealthStatusDegraded, result.Reports[0].Status)
}
