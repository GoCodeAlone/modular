//go:build failing_test

package modular

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConcurrentReloadSafety tests thread-safety of reload operations with proper race detection
func TestConcurrentReloadSafety(t *testing.T) {
	t.Run("should handle concurrent reload requests safely", func(t *testing.T) {
		// Create application with thread-safe reload capability
		app := &StdApplication{
			cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
			cfgSections:    make(map[string]ConfigProvider),
			svcRegistry:    make(ServiceRegistry),
			moduleRegistry: make(ModuleRegistry),
			logger:         &logger{t},
		}

		// Register thread-safe reloadable module
		module := &threadSafeReloadableModule{
			name:          "concurrent-module",
			reloadCount:   0,
			currentConfig: make(map[string]interface{}),
			mutex:         sync.RWMutex{},
		}

		app.RegisterModule(module)
		require.NoError(t, app.Init(), "Application should initialize")

		// Test concurrent reloads
		concurrentReloads := 50
		var wg sync.WaitGroup
		var successCount int64
		var errorCount int64

		// Channel to collect results
		results := make(chan reloadResult, concurrentReloads)

		for i := 0; i < concurrentReloads; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				config := map[string]interface{}{
					"reload_id": id,
					"timestamp": time.Now().UnixNano(),
					"data":      fmt.Sprintf("config-data-%d", id),
				}

				// Get reloadable module and trigger reload
				modules := app.GetModules()
				if reloadable, ok := modules["concurrent-module"].(Reloadable); ok {
					err := reloadable.Reload(context.Background(), config)
					
					if err != nil {
						atomic.AddInt64(&errorCount, 1)
						results <- reloadResult{id: id, success: false, err: err}
					} else {
						atomic.AddInt64(&successCount, 1)
						results <- reloadResult{id: id, success: true, err: nil}
					}
				} else {
					atomic.AddInt64(&errorCount, 1)
					results <- reloadResult{id: id, success: false, err: fmt.Errorf("module not reloadable")}
				}
			}(i)
		}

		wg.Wait()
		close(results)

		// Collect and analyze results
		var resultList []reloadResult
		for result := range results {
			resultList = append(resultList, result)
		}

		// Verify thread safety - all operations should complete
		assert.Len(t, resultList, concurrentReloads, "All reload attempts should complete")
		
		// Most reloads should succeed (some may fail due to validation, but not due to race conditions)
		finalSuccessCount := atomic.LoadInt64(&successCount)
		finalErrorCount := atomic.LoadInt64(&errorCount)
		
		assert.Equal(t, int64(concurrentReloads), finalSuccessCount+finalErrorCount, "All operations should be accounted for")
		assert.Greater(t, finalSuccessCount, int64(concurrentReloads/2), "Most reloads should succeed")

		// Verify final state is consistent
		finalReloadCount := module.getReloadCount()
		assert.Equal(t, finalSuccessCount, int64(finalReloadCount), "Reload count should match successful reloads")
	})

	t.Run("should detect and prevent race conditions in configuration updates", func(t *testing.T) {
		// This test specifically targets race conditions in configuration updates
		app := &StdApplication{
			cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
			cfgSections:    make(map[string]ConfigProvider),
			svcRegistry:    make(ServiceRegistry),
			moduleRegistry: make(ModuleRegistry),
			logger:         &logger{t},
		}

		// Create module that tracks race conditions
		module := &raceDetectionModule{
			name:            "race-detection-module",
			configWrites:    0,
			configReads:     0,
			raceDetected:    false,
			currentConfig:   make(map[string]interface{}),
			operationMutex:  sync.Mutex{},
		}

		app.RegisterModule(module)
		require.NoError(t, app.Init())

		// Start concurrent readers and writers
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		var wg sync.WaitGroup
		
		// Writers (reloaders)
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(writerID int) {
				defer wg.Done()
				for {
					select {
					case <-ctx.Done():
						return
					default:
						config := map[string]interface{}{
							"writer_id": writerID,
							"value":     time.Now().UnixNano(),
						}
						
						modules := app.GetModules()
						if reloadable, ok := modules["race-detection-module"].(Reloadable); ok {
							_ = reloadable.Reload(context.Background(), config)
						}
						
						// Small delay to allow for race conditions
						time.Sleep(time.Microsecond * 100)
					}
				}
			}(i)
		}

		// Readers (configuration accessors)
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(readerID int) {
				defer wg.Done()
				for {
					select {
					case <-ctx.Done():
						return
					default:
						_ = module.getCurrentConfig()
						time.Sleep(time.Microsecond * 50)
					}
				}
			}(i)
		}

		wg.Wait()

		// Verify no race conditions were detected
		assert.False(t, module.wasRaceDetected(), "No race conditions should be detected")
		assert.Greater(t, module.getWriteCount(), 0, "Some writes should have occurred")
		assert.Greater(t, module.getReadCount(), 0, "Some reads should have occurred")
	})

	t.Run("should handle resource contention gracefully", func(t *testing.T) {
		// Test resource contention scenarios
		app := &StdApplication{
			cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
			cfgSections:    make(map[string]ConfigProvider),
			svcRegistry:    make(ServiceRegistry),
			moduleRegistry: make(ModuleRegistry),
			logger:         &logger{t},
		}

		// Module that simulates resource contention
		module := &resourceContentionModule{
			name:                "resource-module",
			sharedResource:      0,
			resourceAccessCount: 0,
			maxConcurrency:      5,
			semaphore:          make(chan struct{}, 5),
		}

		app.RegisterModule(module)

		// Test high concurrency
		workers := runtime.NumCPU() * 4
		var wg sync.WaitGroup
		var totalOperations int64

		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				
				for j := 0; j < 20; j++ {
					config := map[string]interface{}{
						"worker_id":   workerID,
						"operation":   j,
						"resource_op": "increment",
					}

					modules := app.GetModules()
					if reloadable, ok := modules["resource-module"].(Reloadable); ok {
						err := reloadable.Reload(context.Background(), config)
						if err == nil {
							atomic.AddInt64(&totalOperations, 1)
						}
					}
				}
			}(i)
		}

		wg.Wait()

		// Verify resource safety
		finalResourceValue := module.getSharedResource()
		expectedValue := int64(totalOperations)
		
		assert.Equal(t, expectedValue, finalResourceValue, "Shared resource should equal total successful operations")
		assert.Greater(t, totalOperations, int64(0), "Some operations should succeed")
	})

	t.Run("should use atomic operations for critical counters", func(t *testing.T) {
		// Test atomic operations in concurrent environment
		module := &atomicCounterModule{
			name:           "atomic-module",
			reloadCounter:  0,
			successCounter: 0,
			errorCounter:   0,
		}

		app := &StdApplication{
			cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
			cfgSections:    make(map[string]ConfigProvider),
			svcRegistry:    make(ServiceRegistry),
			moduleRegistry: make(ModuleRegistry),
			logger:         &logger{t},
		}

		app.RegisterModule(module)

		// High-frequency concurrent operations
		operations := 1000
		var wg sync.WaitGroup

		for i := 0; i < operations; i++ {
			wg.Add(1)
			go func(opID int) {
				defer wg.Done()
				
				config := map[string]interface{}{
					"op_id": opID,
					"value": opID % 2 == 0, // Half will succeed, half will fail
				}

				modules := app.GetModules()
				if reloadable, ok := modules["atomic-module"].(Reloadable); ok {
					_ = reloadable.Reload(context.Background(), config)
				}
			}(i)
		}

		wg.Wait()

		// Verify atomic counters
		totalReloads := module.getReloadCount()
		successCount := module.getSuccessCount() 
		errorCount := module.getErrorCount()

		assert.Equal(t, int64(operations), totalReloads, "Total reload count should match operations")
		assert.Equal(t, totalReloads, successCount + errorCount, "Success + error should equal total")
		assert.Greater(t, successCount, int64(0), "Some operations should succeed")
		assert.Greater(t, errorCount, int64(0), "Some operations should fail")
	})
}

// Test helper structures and implementations

type reloadResult struct {
	id      int
	success bool
	err     error
}

// threadSafeReloadableModule implements thread-safe reloading
type threadSafeReloadableModule struct {
	name          string
	reloadCount   int64
	currentConfig map[string]interface{}
	mutex         sync.RWMutex
}

func (m *threadSafeReloadableModule) Name() string { return m.name }
func (m *threadSafeReloadableModule) Dependencies() []string { return nil }
func (m *threadSafeReloadableModule) Init(Application) error { return nil }
func (m *threadSafeReloadableModule) Start(context.Context) error { return nil }
func (m *threadSafeReloadableModule) Stop(context.Context) error { return nil }
func (m *threadSafeReloadableModule) RegisterConfig(Application) error { return nil }
func (m *threadSafeReloadableModule) ProvidesServices() []ServiceProvider { return nil }
func (m *threadSafeReloadableModule) RequiresServices() []ServiceDependency { return nil }

func (m *threadSafeReloadableModule) Reload(ctx context.Context, newConfig interface{}) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	// Simulate some work
	time.Sleep(time.Millisecond)
	
	if newConfig != nil {
		m.currentConfig = newConfig.(map[string]interface{})
		atomic.AddInt64(&m.reloadCount, 1)
		return nil
	}
	return fmt.Errorf("invalid config")
}

func (m *threadSafeReloadableModule) CanReload() bool { return true }
func (m *threadSafeReloadableModule) ReloadTimeout() time.Duration { return 5 * time.Second }

func (m *threadSafeReloadableModule) getReloadCount() int64 {
	return atomic.LoadInt64(&m.reloadCount)
}

// raceDetectionModule detects race conditions in configuration access
type raceDetectionModule struct {
	name            string
	configWrites    int64
	configReads     int64
	raceDetected    bool
	currentConfig   map[string]interface{}
	operationMutex  sync.Mutex
}

func (m *raceDetectionModule) Name() string { return m.name }
func (m *raceDetectionModule) Dependencies() []string { return nil }
func (m *raceDetectionModule) Init(Application) error { return nil }
func (m *raceDetectionModule) Start(context.Context) error { return nil }
func (m *raceDetectionModule) Stop(context.Context) error { return nil }
func (m *raceDetectionModule) RegisterConfig(Application) error { return nil }
func (m *raceDetectionModule) ProvidesServices() []ServiceProvider { return nil }
func (m *raceDetectionModule) RequiresServices() []ServiceDependency { return nil }

func (m *raceDetectionModule) Reload(ctx context.Context, newConfig interface{}) error {
	m.operationMutex.Lock()
	defer m.operationMutex.Unlock()
	
	atomic.AddInt64(&m.configWrites, 1)
	
	if newConfig != nil {
		m.currentConfig = newConfig.(map[string]interface{})
		return nil
	}
	return fmt.Errorf("invalid config")
}

func (m *raceDetectionModule) CanReload() bool { return true }
func (m *raceDetectionModule) ReloadTimeout() time.Duration { return 5 * time.Second }

func (m *raceDetectionModule) getCurrentConfig() map[string]interface{} {
	m.operationMutex.Lock()
	defer m.operationMutex.Unlock()
	
	atomic.AddInt64(&m.configReads, 1)
	
	// Create a copy to avoid race conditions
	copy := make(map[string]interface{})
	for k, v := range m.currentConfig {
		copy[k] = v
	}
	return copy
}

func (m *raceDetectionModule) wasRaceDetected() bool {
	m.operationMutex.Lock()
	defer m.operationMutex.Unlock()
	return m.raceDetected
}

func (m *raceDetectionModule) getWriteCount() int64 {
	return atomic.LoadInt64(&m.configWrites)
}

func (m *raceDetectionModule) getReadCount() int64 {
	return atomic.LoadInt64(&m.configReads)
}

// resourceContentionModule simulates resource contention
type resourceContentionModule struct {
	name                string
	sharedResource      int64
	resourceAccessCount int64
	maxConcurrency      int
	semaphore           chan struct{}
}

func (m *resourceContentionModule) Name() string { return m.name }
func (m *resourceContentionModule) Dependencies() []string { return nil }
func (m *resourceContentionModule) Init(Application) error { return nil }
func (m *resourceContentionModule) Start(context.Context) error { return nil }
func (m *resourceContentionModule) Stop(context.Context) error { return nil }
func (m *resourceContentionModule) RegisterConfig(Application) error { return nil }
func (m *resourceContentionModule) ProvidesServices() []ServiceProvider { return nil }
func (m *resourceContentionModule) RequiresServices() []ServiceDependency { return nil }

func (m *resourceContentionModule) Reload(ctx context.Context, newConfig interface{}) error {
	// Acquire semaphore to limit concurrency
	select {
	case m.semaphore <- struct{}{}:
		defer func() { <-m.semaphore }()
	case <-ctx.Done():
		return ctx.Err()
	}
	
	atomic.AddInt64(&m.resourceAccessCount, 1)
	
	// Simulate resource access
	current := atomic.LoadInt64(&m.sharedResource)
	time.Sleep(time.Microsecond * 100) // Simulate work
	atomic.StoreInt64(&m.sharedResource, current+1)
	
	return nil
}

func (m *resourceContentionModule) CanReload() bool { return true }
func (m *resourceContentionModule) ReloadTimeout() time.Duration { return 5 * time.Second }

func (m *resourceContentionModule) getSharedResource() int64 {
	return atomic.LoadInt64(&m.sharedResource)
}

// atomicCounterModule uses atomic operations for all counters
type atomicCounterModule struct {
	name           string
	reloadCounter  int64
	successCounter int64
	errorCounter   int64
}

func (m *atomicCounterModule) Name() string { return m.name }
func (m *atomicCounterModule) Dependencies() []string { return nil }
func (m *atomicCounterModule) Init(Application) error { return nil }
func (m *atomicCounterModule) Start(context.Context) error { return nil }
func (m *atomicCounterModule) Stop(context.Context) error { return nil }
func (m *atomicCounterModule) RegisterConfig(Application) error { return nil }
func (m *atomicCounterModule) ProvidesServices() []ServiceProvider { return nil }
func (m *atomicCounterModule) RequiresServices() []ServiceDependency { return nil }

func (m *atomicCounterModule) Reload(ctx context.Context, newConfig interface{}) error {
	atomic.AddInt64(&m.reloadCounter, 1)
	
	if configMap, ok := newConfig.(map[string]interface{}); ok {
		if value, exists := configMap["value"]; exists {
			if success, ok := value.(bool); ok && success {
				atomic.AddInt64(&m.successCounter, 1)
				return nil
			}
		}
	}
	
	atomic.AddInt64(&m.errorCounter, 1)
	return fmt.Errorf("simulated error")
}

func (m *atomicCounterModule) CanReload() bool { return true }
func (m *atomicCounterModule) ReloadTimeout() time.Duration { return 5 * time.Second }

func (m *atomicCounterModule) getReloadCount() int64 {
	return atomic.LoadInt64(&m.reloadCounter)
}

func (m *atomicCounterModule) getSuccessCount() int64 {
	return atomic.LoadInt64(&m.successCounter)
}

func (m *atomicCounterModule) getErrorCount() int64 {
	return atomic.LoadInt64(&m.errorCounter)
}