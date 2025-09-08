package integration

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	modular "github.com/GoCodeAlone/modular"
)

// TestDynamicReloadHealthInterplay tests T029: Integration dynamic reload + health interplay
// This test verifies that dynamic configuration reloads work correctly with health checks
// and that health status is properly updated when configuration changes affect module health.
//
// NOTE: This test demonstrates the integration pattern for future dynamic reload and
// health aggregation functionality. The actual implementation is not yet available,
// so this test shows the expected interface and behavior.
func TestDynamicReloadHealthInterplay(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create application
	app := modular.NewStdApplication(modular.NewStdConfigProvider(&struct{}{}), logger)

	// Register modules that support dynamic reload and health checks
	reloadableModule := &testReloadableModule{
		name:   "reloadableModule",
		config: &testReloadableConfig{Enabled: true, Timeout: 5},
		health: &testHealthStatus{status: "healthy", lastCheck: time.Now()},
	}

	healthAggregator := &testHealthAggregatorModule{
		name:    "healthAggregator",
		modules: make(map[string]*testHealthStatus),
	}

	app.RegisterModule(reloadableModule)
	app.RegisterModule(healthAggregator)

	// Initialize application
	err := app.Init()
	if err != nil {
		t.Fatalf("Application initialization failed: %v", err)
	}

	// Start application
	err = app.Start()
	if err != nil {
		t.Fatalf("Application start failed: %v", err)
	}
	defer app.Stop()

	// Register module with health aggregator
	healthAggregator.registerModule("reloadableModule", reloadableModule.health)

	// Verify initial health status
	initialHealth := healthAggregator.getAggregatedHealth()
	if initialHealth.overallStatus != "healthy" {
		t.Errorf("Expected initial health to be 'healthy', got: %s", initialHealth.overallStatus)
	}

	t.Log("✅ Initial health status verified as healthy")

	// Test case 1: Valid configuration reload
	t.Run("ValidConfigReload", func(t *testing.T) {
		// Prepare new valid configuration
		newConfig := &testReloadableConfig{
			Enabled: true,
			Timeout: 10, // Increased timeout
		}

		// Perform dynamic reload
		reloadResult := reloadableModule.reloadConfig(newConfig)
		if !reloadResult.success {
			t.Errorf("Valid config reload failed: %s", reloadResult.error)
		}

		// Verify health status remains healthy after valid reload
		time.Sleep(100 * time.Millisecond) // Allow health check to update
		healthAfterReload := healthAggregator.getAggregatedHealth()

		if healthAfterReload.overallStatus != "healthy" {
			t.Errorf("Expected health to remain 'healthy' after valid reload, got: %s", healthAfterReload.overallStatus)
		}

		t.Log("✅ Health remains healthy after valid configuration reload")
	})

	// Test case 2: Invalid configuration reload triggers health degradation
	t.Run("InvalidConfigReloadHealthDegradation", func(t *testing.T) {
		// Prepare invalid configuration
		invalidConfig := &testReloadableConfig{
			Enabled: true,
			Timeout: -1, // Invalid negative timeout
		}

		// Perform dynamic reload
		reloadResult := reloadableModule.reloadConfig(invalidConfig)
		if reloadResult.success {
			t.Error("Invalid config reload should have failed but succeeded")
		}

		// Verify health status degrades after invalid reload
		time.Sleep(100 * time.Millisecond) // Allow health check to update
		healthAfterBadReload := healthAggregator.getAggregatedHealth()

		if healthAfterBadReload.overallStatus == "healthy" {
			t.Error("Expected health to degrade after invalid config reload")
		}

		t.Logf("✅ Health properly degraded after invalid reload: %s", healthAfterBadReload.overallStatus)
	})

	// Test case 3: Health recovery after fixing configuration
	t.Run("HealthRecoveryAfterFix", func(t *testing.T) {
		// Fix configuration
		fixedConfig := &testReloadableConfig{
			Enabled: true,
			Timeout: 30,
		}

		// Perform reload with fixed config
		reloadResult := reloadableModule.reloadConfig(fixedConfig)
		if !reloadResult.success {
			t.Errorf("Fixed config reload failed: %s", reloadResult.error)
		}

		// Verify health recovery
		time.Sleep(200 * time.Millisecond) // Allow health check to update
		healthAfterFix := healthAggregator.getAggregatedHealth()

		if healthAfterFix.overallStatus != "healthy" {
			t.Errorf("Expected health to recover after config fix, got: %s", healthAfterFix.overallStatus)
		}

		t.Log("✅ Health properly recovered after configuration fix")
	})

	// Test case 4: Concurrent reload and health check operations
	t.Run("ConcurrentReloadAndHealthCheck", func(t *testing.T) {
		var wg sync.WaitGroup
		results := make([]string, 0)
		resultsMutex := sync.Mutex{}

		// Start multiple concurrent reloads
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(iteration int) {
				defer wg.Done()

				config := &testReloadableConfig{
					Enabled: true,
					Timeout: 5 + iteration,
				}

				result := reloadableModule.reloadConfig(config)

				resultsMutex.Lock()
				if result.success {
					results = append(results, "reload-success")
				} else {
					results = append(results, "reload-failure")
				}
				resultsMutex.Unlock()
			}(i)
		}

		// Start concurrent health checks
		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				health := healthAggregator.getAggregatedHealth()

				resultsMutex.Lock()
				results = append(results, "health-check:"+health.overallStatus)
				resultsMutex.Unlock()
			}()
		}

		// Wait for all operations to complete
		done := make(chan bool)
		go func() {
			wg.Wait()
			done <- true
		}()

		select {
		case <-done:
			t.Log("✅ All concurrent operations completed")
		case <-time.After(5 * time.Second):
			t.Fatal("Test timed out waiting for concurrent operations")
		}

		// Verify no race conditions or deadlocks occurred
		if len(results) != 8 { // 5 reloads + 3 health checks
			t.Errorf("Expected 8 operation results, got %d", len(results))
		}

		t.Logf("✅ Concurrent reload and health check operations: %v", results)
	})
}

// testReloadableConfig represents configuration that can be reloaded
type testReloadableConfig struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
	Timeout int  `yaml:"timeout" json:"timeout"`
}

// testHealthStatus represents health status information
type testHealthStatus struct {
	status    string
	lastCheck time.Time
	mutex     sync.RWMutex
}

// testReloadResult contains result of configuration reload
type testReloadResult struct {
	success bool
	error   string
}

// testAggregatedHealth contains aggregated health information
type testAggregatedHealth struct {
	overallStatus string
	moduleCount   int
	lastUpdated   time.Time
}

// testReloadableModule simulates a module that supports dynamic configuration reload
type testReloadableModule struct {
	name   string
	config *testReloadableConfig
	health *testHealthStatus
	mutex  sync.RWMutex
}

func (m *testReloadableModule) Name() string {
	return m.name
}

func (m *testReloadableModule) Init(app modular.Application) error {
	return nil
}

func (m *testReloadableModule) Start(ctx context.Context) error {
	return nil
}

func (m *testReloadableModule) Stop(ctx context.Context) error {
	return nil
}

// reloadConfig simulates dynamic configuration reload
func (m *testReloadableModule) reloadConfig(newConfig *testReloadableConfig) testReloadResult {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Validate new configuration
	if newConfig.Timeout < 0 {
		// Invalid config - update health status
		m.health.mutex.Lock()
		m.health.status = "unhealthy"
		m.health.lastCheck = time.Now()
		m.health.mutex.Unlock()

		return testReloadResult{
			success: false,
			error:   "invalid timeout value",
		}
	}

	// Apply new configuration
	m.config = newConfig

	// Update health status to healthy
	m.health.mutex.Lock()
	m.health.status = "healthy"
	m.health.lastCheck = time.Now()
	m.health.mutex.Unlock()

	return testReloadResult{
		success: true,
		error:   "",
	}
}

// testHealthAggregatorModule simulates a health aggregation module
type testHealthAggregatorModule struct {
	name    string
	modules map[string]*testHealthStatus
	mutex   sync.RWMutex
}

func (m *testHealthAggregatorModule) Name() string {
	return m.name
}

func (m *testHealthAggregatorModule) Init(app modular.Application) error {
	return nil
}

func (m *testHealthAggregatorModule) Start(ctx context.Context) error {
	return nil
}

func (m *testHealthAggregatorModule) Stop(ctx context.Context) error {
	return nil
}

// registerModule registers a module for health monitoring
func (m *testHealthAggregatorModule) registerModule(moduleName string, health *testHealthStatus) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.modules[moduleName] = health
}

// getAggregatedHealth returns aggregated health status
func (m *testHealthAggregatorModule) getAggregatedHealth() testAggregatedHealth {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	overallStatus := "healthy"
	moduleCount := len(m.modules)

	// Check health of all registered modules
	for _, health := range m.modules {
		health.mutex.RLock()
		if health.status != "healthy" {
			overallStatus = health.status
		}
		health.mutex.RUnlock()
	}

	return testAggregatedHealth{
		overallStatus: overallStatus,
		moduleCount:   moduleCount,
		lastUpdated:   time.Now(),
	}
}
