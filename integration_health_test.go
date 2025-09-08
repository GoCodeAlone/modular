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

// TestHealthAggregationRealApplication tests health aggregation with real application setup
func TestHealthAggregationRealApplication(t *testing.T) {
	t.Run("should aggregate health from registered modules", func(t *testing.T) {
		// Create real application instance
		app := &StdApplication{
			cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
			cfgSections:    make(map[string]ConfigProvider),
			svcRegistry:    make(ServiceRegistry),
			moduleRegistry: make(ModuleRegistry),
			logger:         &logger{t},
		}

		// Register modules that implement health reporting
		dbModule := &testHealthModule{
			name:      "database",
			isHealthy: true,
			timeout:   3 * time.Second,
			details: map[string]interface{}{
				"connection_pool_size": 10,
				"active_connections":   5,
			},
		}

		cacheModule := &testHealthModule{
			name:      "cache",
			isHealthy: false, // Unhealthy cache
			timeout:   2 * time.Second,
			details: map[string]interface{}{
				"cache_hits":   100,
				"cache_misses": 50,
				"error":        "redis connection timeout",
			},
		}

		apiModule := &testHealthModule{
			name:      "api",
			isHealthy: true,
			timeout:   1 * time.Second,
			details: map[string]interface{}{
				"active_requests": 3,
				"uptime_seconds":  3600,
			},
		}

		app.RegisterModule(dbModule)
		app.RegisterModule(cacheModule)
		app.RegisterModule(apiModule)

		// Initialize application
		err := app.Init()
		require.NoError(t, err, "Application initialization should succeed")

		// Simulate health aggregation service
		healthAggregator := NewHealthAggregator(app)
		
		// Collect health from all registered modules
		ctx := context.Background()
		healthSnapshot := healthAggregator.AggregateHealth(ctx)

		// Verify aggregated health results
		require.NotNil(t, healthSnapshot, "Health snapshot should not be nil")
		
		// Should have health results for all 3 modules
		assert.Len(t, healthSnapshot.ModuleHealth, 3, "Should have health results for all modules")

		// Verify individual module health
		dbHealth := healthSnapshot.ModuleHealth["database"]
		assert.Equal(t, HealthStatusHealthy, dbHealth.Status, "Database should be healthy")
		assert.Contains(t, dbHealth.Details, "connection_pool_size")

		cacheHealth := healthSnapshot.ModuleHealth["cache"]
		assert.Equal(t, HealthStatusUnhealthy, cacheHealth.Status, "Cache should be unhealthy")
		assert.Contains(t, cacheHealth.Details, "error")

		apiHealth := healthSnapshot.ModuleHealth["api"]
		assert.Equal(t, HealthStatusHealthy, apiHealth.Status, "API should be healthy")
		assert.Contains(t, apiHealth.Details, "uptime_seconds")

		// Verify overall health determination
		assert.Equal(t, HealthStatusUnhealthy, healthSnapshot.OverallStatus, "Overall health should be unhealthy due to cache")
		assert.WithinDuration(t, time.Now(), healthSnapshot.Timestamp, time.Second, "Timestamp should be recent")
	})

	t.Run("should handle health check timeouts properly", func(t *testing.T) {
		app := &StdApplication{
			cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
			cfgSections:    make(map[string]ConfigProvider),
			svcRegistry:    make(ServiceRegistry),
			moduleRegistry: make(ModuleRegistry),
			logger:         &logger{t},
		}

		// Register module with slow health check
		slowModule := &slowHealthReporter{
			name:    "slow-service",
			delay:   200 * time.Millisecond,
			timeout: 5 * time.Second,
		}

		app.RegisterModule(slowModule)

		healthAggregator := NewHealthAggregator(app)

		// Test with short context timeout
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		healthSnapshot := healthAggregator.AggregateHealth(ctx)

		// Verify timeout handling
		slowHealth := healthSnapshot.ModuleHealth["slow-service"]
		assert.Equal(t, HealthStatusUnknown, slowHealth.Status, "Should return unknown status on timeout")
		assert.Contains(t, slowHealth.Message, "timeout", "Should indicate timeout in message")
	})

	t.Run("should support concurrent health checking", func(t *testing.T) {
		app := &StdApplication{
			cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
			cfgSections:    make(map[string]ConfigProvider),
			svcRegistry:    make(ServiceRegistry),
			moduleRegistry: make(ModuleRegistry),
			logger:         &logger{t},
		}

		// Register multiple modules for concurrent checking
		moduleCount := 5
		for i := 0; i < moduleCount; i++ {
			module := &testHealthModule{
				name:      fmt.Sprintf("service-%d", i),
				isHealthy: i%2 == 0, // Every other service is healthy
				timeout:   1 * time.Second,
				details:   map[string]interface{}{"id": i},
			}
			app.RegisterModule(module)
		}

		healthAggregator := NewHealthAggregator(app)

		// Run concurrent health checks
		ctx := context.Background()
		var wg sync.WaitGroup
		results := make(chan *HealthSnapshot, 10)

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				snapshot := healthAggregator.AggregateHealth(ctx)
				results <- snapshot
			}()
		}

		wg.Wait()
		close(results)

		// Verify all concurrent checks completed
		var snapshots []*HealthSnapshot
		for snapshot := range results {
			snapshots = append(snapshots, snapshot)
		}

		assert.Len(t, snapshots, 10, "All concurrent health checks should complete")

		// Verify consistency across concurrent checks
		for _, snapshot := range snapshots {
			assert.Len(t, snapshot.ModuleHealth, moduleCount, "Each snapshot should have all modules")
			// Overall status should be unhealthy since some services are unhealthy
			assert.Equal(t, HealthStatusUnhealthy, snapshot.OverallStatus)
		}
	})
}

// TestHealthAggregationWithDependencies tests health checking with module dependencies
func TestHealthAggregationWithDependencies(t *testing.T) {
	t.Run("should respect module dependencies in health evaluation", func(t *testing.T) {
		app := &StdApplication{
			cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
			cfgSections:    make(map[string]ConfigProvider),
			svcRegistry:    make(ServiceRegistry),
			moduleRegistry: make(ModuleRegistry),
			logger:         &logger{t},
		}

		// Create modules with dependencies: API depends on DB and Cache
		dbModule := &dependentHealthModule{
			testHealthModule: testHealthModule{
				name:      "database",
				isHealthy: true,
				timeout:   2 * time.Second,
			},
			dependencies: []string{},
		}

		cacheModule := &dependentHealthModule{
			testHealthModule: testHealthModule{
				name:      "cache",
				isHealthy: true,
				timeout:   2 * time.Second,
			},
			dependencies: []string{"database"},
		}

		apiModule := &dependentHealthModule{
			testHealthModule: testHealthModule{
				name:      "api",
				isHealthy: true,
				timeout:   2 * time.Second,
			},
			dependencies: []string{"database", "cache"},
		}

		app.RegisterModule(dbModule)
		app.RegisterModule(cacheModule)
		app.RegisterModule(apiModule)

		healthAggregator := NewHealthAggregatorWithDependencyAwareness(app)

		// Check health with all services healthy
		ctx := context.Background()
		snapshot := healthAggregator.AggregateHealth(ctx)

		assert.Equal(t, HealthStatusHealthy, snapshot.OverallStatus, "All services healthy")

		// Make database unhealthy and check cascading effect
		dbModule.isHealthy = false
		snapshot = healthAggregator.AggregateHealth(ctx)

		// Database should be unhealthy, which should affect overall status
		dbHealth := snapshot.ModuleHealth["database"]
		assert.Equal(t, HealthStatusUnhealthy, dbHealth.Status)

		// Overall status should be unhealthy
		assert.Equal(t, HealthStatusUnhealthy, snapshot.OverallStatus, "Database failure should affect overall health")
	})
}

// TestHealthEventEmission tests health-related event emission during evaluation
func TestHealthEventEmission(t *testing.T) {
	t.Run("should emit health events during evaluation", func(t *testing.T) {
		// Create event tracking system
		eventTracker := &healthEventTracker{
			events: make([]HealthEvent, 0),
		}

		app := &StdApplication{
			cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
			cfgSections:    make(map[string]ConfigProvider),
			svcRegistry:    make(ServiceRegistry),
			moduleRegistry: make(ModuleRegistry),
			logger:         &logger{t},
		}

		// Register modules
		healthyModule := &testHealthModule{
			name:      "healthy-service",
			isHealthy: true,
			timeout:   1 * time.Second,
		}

		unhealthyModule := &testHealthModule{
			name:      "unhealthy-service",
			isHealthy: false,
			timeout:   1 * time.Second,
		}

		app.RegisterModule(healthyModule)
		app.RegisterModule(unhealthyModule)

		// Create health aggregator with event emission
		healthAggregator := NewHealthAggregatorWithEvents(app, eventTracker)

		// Perform health check
		ctx := context.Background()
		snapshot := healthAggregator.AggregateHealth(ctx)

		// Verify events were emitted
		events := eventTracker.GetEvents()
		assert.Greater(t, len(events), 0, "Should emit health events")

		// Should have events for each module check
		var healthyServiceEvent, unhealthyServiceEvent bool
		for _, event := range events {
			switch event.ModuleName {
			case "healthy-service":
				healthyServiceEvent = true
				assert.Equal(t, HealthStatusHealthy, event.Status)
			case "unhealthy-service":
				unhealthyServiceEvent = true
				assert.Equal(t, HealthStatusUnhealthy, event.Status)
			}
		}

		assert.True(t, healthyServiceEvent, "Should emit event for healthy service")
		assert.True(t, unhealthyServiceEvent, "Should emit event for unhealthy service")

		// Verify overall health
		assert.Equal(t, HealthStatusUnhealthy, snapshot.OverallStatus)
	})
}

// Test helper implementations for integration testing

// HealthSnapshot represents aggregated health information
type HealthSnapshot struct {
	OverallStatus  HealthStatus                `json:"overall_status"`
	ModuleHealth   map[string]HealthResult     `json:"module_health"`
	Timestamp      time.Time                   `json:"timestamp"`
	CheckDuration  time.Duration               `json:"check_duration"`
}

// HealthAggregator aggregates health from multiple modules
type HealthAggregator struct {
	app Application
}

func NewHealthAggregator(app Application) *HealthAggregator {
	return &HealthAggregator{app: app}
}

func (ha *HealthAggregator) AggregateHealth(ctx context.Context) *HealthSnapshot {
	start := time.Now()
	
	modules := ha.app.GetModules()
	moduleHealth := make(map[string]HealthResult)
	
	// Check health for each module that implements HealthReporter
	for moduleName, module := range modules {
		if healthReporter, ok := module.(HealthReporter); ok {
			result := healthReporter.CheckHealth(ctx)
			moduleHealth[moduleName] = result
		}
	}
	
	// Determine overall status
	overallStatus := HealthStatusHealthy
	for _, health := range moduleHealth {
		if !health.Status.IsHealthy() {
			overallStatus = HealthStatusUnhealthy
			break
		}
	}
	
	return &HealthSnapshot{
		OverallStatus: overallStatus,
		ModuleHealth:  moduleHealth,
		Timestamp:     time.Now(),
		CheckDuration: time.Since(start),
	}
}

// HealthAggregatorWithDependencyAwareness considers module dependencies
type HealthAggregatorWithDependencyAwareness struct {
	*HealthAggregator
}

func NewHealthAggregatorWithDependencyAwareness(app Application) *HealthAggregatorWithDependencyAwareness {
	return &HealthAggregatorWithDependencyAwareness{
		HealthAggregator: NewHealthAggregator(app),
	}
}

// dependentHealthModule extends health module with dependency information
type dependentHealthModule struct {
	testHealthModule
	dependencies []string
}

func (m *dependentHealthModule) Dependencies() []string {
	return m.dependencies
}

// HealthEvent represents a health-related event
type HealthEvent struct {
	ModuleName string        `json:"module_name"`
	Status     HealthStatus  `json:"status"`
	Message    string        `json:"message"`
	Timestamp  time.Time     `json:"timestamp"`
	Details    map[string]interface{} `json:"details,omitempty"`
}

// healthEventTracker tracks health events for testing
type healthEventTracker struct {
	events []HealthEvent
	mutex  sync.RWMutex
}

func (t *healthEventTracker) EmitHealthEvent(event HealthEvent) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.events = append(t.events, event)
}

func (t *healthEventTracker) GetEvents() []HealthEvent {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	return append([]HealthEvent{}, t.events...)
}

// HealthAggregatorWithEvents emits events during health checking
type HealthAggregatorWithEvents struct {
	*HealthAggregator
	eventTracker *healthEventTracker
}

func NewHealthAggregatorWithEvents(app Application, tracker *healthEventTracker) *HealthAggregatorWithEvents {
	return &HealthAggregatorWithEvents{
		HealthAggregator: NewHealthAggregator(app),
		eventTracker:     tracker,
	}
}

func (ha *HealthAggregatorWithEvents) AggregateHealth(ctx context.Context) *HealthSnapshot {
	start := time.Now()
	
	modules := ha.app.GetModules()
	moduleHealth := make(map[string]HealthResult)
	
	// Check health and emit events for each module
	for moduleName, module := range modules {
		if healthReporter, ok := module.(HealthReporter); ok {
			result := healthReporter.CheckHealth(ctx)
			moduleHealth[moduleName] = result
			
			// Emit health event
			ha.eventTracker.EmitHealthEvent(HealthEvent{
				ModuleName: moduleName,
				Status:     result.Status,
				Message:    result.Message,
				Timestamp:  result.Timestamp,
				Details:    result.Details,
			})
		}
	}
	
	// Determine overall status
	overallStatus := HealthStatusHealthy
	for _, health := range moduleHealth {
		if !health.Status.IsHealthy() {
			overallStatus = HealthStatusUnhealthy
			break
		}
	}
	
	return &HealthSnapshot{
		OverallStatus: overallStatus,
		ModuleHealth:  moduleHealth,
		Timestamp:     time.Now(),
		CheckDuration: time.Since(start),
	}
}