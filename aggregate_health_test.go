//go:build failing_test

package modular

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAggregateHealthService tests health aggregation behavior
func TestAggregateHealthService_AggregateHealth(t *testing.T) {
	tests := []struct {
		name            string
		reporters       []HealthReporter
		expectedStatus  HealthStatus
		expectedReports int
	}{
		{
			name: "all healthy services return healthy overall",
			reporters: []HealthReporter{
				newTestHealthReporter("service-1", true, nil),
				newTestHealthReporter("service-2", true, nil),
				newTestHealthReporter("service-3", true, nil),
			},
			expectedStatus:  HealthStatusHealthy,
			expectedReports: 3,
		},
		{
			name: "mixed health states return unhealthy overall",
			reporters: []HealthReporter{
				newTestHealthReporter("healthy-service", true, nil),
				newTestHealthReporter("unhealthy-service", false, nil),
				newTestHealthReporter("another-healthy-service", true, nil),
			},
			expectedStatus:  HealthStatusUnhealthy,
			expectedReports: 3,
		},
		{
			name:            "no reporters return healthy by default",
			reporters:       []HealthReporter{},
			expectedStatus:  HealthStatusHealthy,
			expectedReports: 0,
		},
		{
			name: "single unhealthy service makes overall unhealthy",
			reporters: []HealthReporter{
				newTestHealthReporter("failing-service", false, nil),
			},
			expectedStatus:  HealthStatusUnhealthy,
			expectedReports: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create aggregate health service
			aggregator := NewTestAggregateHealthService()

			// Register all reporters
			for _, reporter := range tt.reporters {
				aggregator.RegisterReporter(reporter)
			}

			// Perform health check
			ctx := context.Background()
			result := aggregator.CheckOverallHealth(ctx)

			// Verify results
			assert.Equal(t, tt.expectedStatus, result.OverallStatus, "Overall status should match expected")
			assert.Len(t, result.ServiceHealth, tt.expectedReports, "Should have expected number of service reports")
			assert.WithinDuration(t, time.Now(), result.Timestamp, time.Second, "Timestamp should be recent")
		})
	}
}

// TestAggregateHealthService_ConcurrentAccess tests concurrent access safety
func TestAggregateHealthService_ConcurrentAccess(t *testing.T) {
	t.Run("should handle concurrent health checks safely", func(t *testing.T) {
		aggregator := NewTestAggregateHealthService()

		// Register multiple reporters
		for i := 0; i < 5; i++ {
			reporter := newTestHealthReporter(fmt.Sprintf("service-%d", i), i%2 == 0, nil)
			aggregator.RegisterReporter(reporter)
		}

		// Perform concurrent health checks
		concurrency := 20
		var wg sync.WaitGroup
		results := make(chan *AggregateHealthResult, concurrency)

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ctx := context.Background()
				result := aggregator.CheckOverallHealth(ctx)
				results <- result
			}()
		}

		wg.Wait()
		close(results)

		// Verify all checks completed
		var resultList []*AggregateHealthResult
		for result := range results {
			resultList = append(resultList, result)
		}

		assert.Len(t, resultList, concurrency, "All concurrent checks should complete")

		// All results should be consistent
		for _, result := range resultList {
			assert.Len(t, result.ServiceHealth, 5, "Each result should have all services")
			assert.Equal(t, HealthStatusUnhealthy, result.OverallStatus, "Overall should be unhealthy due to mixed services")
		}
	})
}

// TestAggregateHealthService_TimeoutHandling tests timeout scenarios
func TestAggregateHealthService_TimeoutHandling(t *testing.T) {
	t.Run("should handle reporter timeouts gracefully", func(t *testing.T) {
		aggregator := NewTestAggregateHealthService()

		// Register fast and slow reporters
		fastReporter := newTestHealthReporter("fast-service", true, nil)
		slowReporter := newSlowHealthReporter("slow-service", 200*time.Millisecond)

		aggregator.RegisterReporter(fastReporter)
		aggregator.RegisterReporter(slowReporter)

		// Check health with short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		result := aggregator.CheckOverallHealth(ctx)

		// Should have results for both services
		assert.Len(t, result.ServiceHealth, 2, "Should have results for both services")

		// Fast service should be healthy
		fastResult, exists := result.ServiceHealth["fast-service"]
		require.True(t, exists, "Fast service should have result")
		assert.Equal(t, HealthStatusHealthy, fastResult.Status)

		// Slow service should be unknown due to timeout
		slowResult, exists := result.ServiceHealth["slow-service"]
		require.True(t, exists, "Slow service should have result")
		assert.Equal(t, HealthStatusUnknown, slowResult.Status)

		// Overall should be unhealthy due to unknown service
		assert.Equal(t, HealthStatusUnhealthy, result.OverallStatus)
	})
}

// TestAggregateHealthService_ReporterManagement tests adding/removing reporters
func TestAggregateHealthService_ReporterManagement(t *testing.T) {
	t.Run("should support dynamic reporter registration", func(t *testing.T) {
		aggregator := NewTestAggregateHealthService()

		// Initial health check - no reporters
		ctx := context.Background()
		result := aggregator.CheckOverallHealth(ctx)
		assert.Len(t, result.ServiceHealth, 0, "Should have no service reports initially")

		// Add first reporter
		reporter1 := newTestHealthReporter("service-1", true, nil)
		aggregator.RegisterReporter(reporter1)

		result = aggregator.CheckOverallHealth(ctx)
		assert.Len(t, result.ServiceHealth, 1, "Should have one service report")
		assert.Equal(t, HealthStatusHealthy, result.OverallStatus)

		// Add second reporter
		reporter2 := newTestHealthReporter("service-2", false, nil)
		aggregator.RegisterReporter(reporter2)

		result = aggregator.CheckOverallHealth(ctx)
		assert.Len(t, result.ServiceHealth, 2, "Should have two service reports")
		assert.Equal(t, HealthStatusUnhealthy, result.OverallStatus)

		// Remove unhealthy reporter
		aggregator.RemoveReporter("service-2")

		result = aggregator.CheckOverallHealth(ctx)
		assert.Len(t, result.ServiceHealth, 1, "Should have one service report after removal")
		assert.Equal(t, HealthStatusHealthy, result.OverallStatus)
	})
}

// Test helper implementations

// AggregateHealthResult represents the result of an aggregate health check
type AggregateHealthResult struct {
	OverallStatus HealthStatus            `json:"overall_status"`
	ServiceHealth map[string]HealthResult `json:"service_health"`
	Timestamp     time.Time               `json:"timestamp"`
	Duration      time.Duration           `json:"duration"`
}

// TestAggregateHealthService implements health aggregation for testing
type TestAggregateHealthService struct {
	reporters map[string]HealthReporter
	mutex     sync.RWMutex
}

func NewTestAggregateHealthService() *TestAggregateHealthService {
	return &TestAggregateHealthService{
		reporters: make(map[string]HealthReporter),
	}
}

func (s *TestAggregateHealthService) RegisterReporter(reporter HealthReporter) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.reporters[reporter.HealthCheckName()] = reporter
}

func (s *TestAggregateHealthService) RemoveReporter(name string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	delete(s.reporters, name)
}

func (s *TestAggregateHealthService) CheckOverallHealth(ctx context.Context) *AggregateHealthResult {
	start := time.Now()

	s.mutex.RLock()
	reporters := make(map[string]HealthReporter)
	for name, reporter := range s.reporters {
		reporters[name] = reporter
	}
	s.mutex.RUnlock()

	serviceHealth := make(map[string]HealthResult)

	// Check health of each service concurrently
	var wg sync.WaitGroup
	resultsChan := make(chan serviceHealthResult, len(reporters))

	for name, reporter := range reporters {
		wg.Add(1)
		go func(serviceName string, r HealthReporter) {
			defer wg.Done()

			// Create timeout context for individual service
			serviceCtx, cancel := context.WithTimeout(ctx, r.HealthCheckTimeout())
			defer cancel()

			result := r.CheckHealth(serviceCtx)
			resultsChan <- serviceHealthResult{name: serviceName, result: result}
		}(name, reporter)
	}

	wg.Wait()
	close(resultsChan)

	// Collect results
	for result := range resultsChan {
		serviceHealth[result.name] = result.result
	}

	// Determine overall status
	overallStatus := HealthStatusHealthy
	if len(serviceHealth) == 0 {
		overallStatus = HealthStatusHealthy // Default to healthy when no services
	} else {
		for _, health := range serviceHealth {
			if !health.Status.IsHealthy() {
				overallStatus = HealthStatusUnhealthy
				break
			}
		}
	}

	return &AggregateHealthResult{
		OverallStatus: overallStatus,
		ServiceHealth: serviceHealth,
		Timestamp:     time.Now(),
		Duration:      time.Since(start),
	}
}

type serviceHealthResult struct {
	name   string
	result HealthResult
}
