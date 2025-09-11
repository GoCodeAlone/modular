package cache

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/GoCodeAlone/modular"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCacheModule_HealthCheck_MemoryCache(t *testing.T) {
	// RED PHASE: Write failing test for memory cache health check

	// Create a cache module with memory engine
	module := &CacheModule{
		name: "cache",
		config: &CacheConfig{
			Engine:          "memory",
			DefaultTTL:      300,
			MaxItems:        1000,
			CleanupInterval: 60,
		},
	}

	// Initialize the cache engine by setting up the memory cache directly
	memCache := NewMemoryCache(module.config)
	module.cacheEngine = memCache

	// Act: Perform health check
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	reports, err := module.HealthCheck(ctx)

	// Assert: Should return healthy status for memory cache
	assert.NoError(t, err)
	assert.NotEmpty(t, reports)

	// Find the cache health report
	var cacheReport *modular.HealthReport
	for i, report := range reports {
		if report.Module == "cache" {
			cacheReport = &reports[i]
			break
		}
	}

	require.NotNil(t, cacheReport, "Expected cache health report")
	assert.Equal(t, "cache", cacheReport.Module)
	assert.Equal(t, "memory", cacheReport.Component)
	assert.Equal(t, modular.HealthStatusHealthy, cacheReport.Status)
	assert.NotEmpty(t, cacheReport.Message)
	assert.False(t, cacheReport.Optional)
	assert.WithinDuration(t, time.Now(), cacheReport.CheckedAt, 5*time.Second)

	// Memory cache should include item count and capacity in details
	assert.Contains(t, cacheReport.Details, "item_count")
	assert.Contains(t, cacheReport.Details, "max_items")
	assert.Contains(t, cacheReport.Details, "engine")
	assert.Equal(t, "memory", cacheReport.Details["engine"])
}

func TestCacheModule_HealthCheck_RedisCache_Healthy(t *testing.T) {
	// RED PHASE: Write failing test for Redis cache health check

	// Create a cache module with Redis engine
	module := &CacheModule{
		name: "cache",
		config: &CacheConfig{
			Engine:        "redis",
			DefaultTTL:    300,
			RedisURL:      "redis://localhost:6379",
			RedisPassword: "",
			RedisDB:       0,
		},
	}

	// Initialize the cache engine by setting up Redis cache directly
	redisCache := NewRedisCache(module.config)
	module.cacheEngine = redisCache

	// Test Redis connection - skip test if Redis not available
	ctx := context.Background()
	if err := redisCache.Connect(ctx); err != nil {
		t.Skip("Redis not available for testing")
		return
	}

	// Act: Perform health check
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	reports, err := module.HealthCheck(ctx)

	// Assert: Should return status based on Redis connectivity
	assert.NoError(t, err)
	assert.NotEmpty(t, reports)

	// Find the cache health report
	var cacheReport *modular.HealthReport
	for i, report := range reports {
		if report.Module == "cache" {
			cacheReport = &reports[i]
			break
		}
	}

	require.NotNil(t, cacheReport, "Expected cache health report")
	assert.Equal(t, "cache", cacheReport.Module)
	assert.Equal(t, "redis", cacheReport.Component)
	assert.NotEmpty(t, cacheReport.Message)
	assert.False(t, cacheReport.Optional)
	assert.WithinDuration(t, time.Now(), cacheReport.CheckedAt, 5*time.Second)

	// Redis cache should include connection info in details
	assert.Contains(t, cacheReport.Details, "redis_url")
	assert.Contains(t, cacheReport.Details, "redis_db")
	assert.Contains(t, cacheReport.Details, "engine")
	assert.Equal(t, "redis", cacheReport.Details["engine"])
}

func TestCacheModule_HealthCheck_UnhealthyCache(t *testing.T) {
	// RED PHASE: Test unhealthy cache scenario

	// Create a cache module without initializing engine
	module := &CacheModule{
		name: "cache",
		config: &CacheConfig{
			Engine: "memory",
		},
		cacheEngine: nil, // No engine initialized - should be unhealthy
	}

	// Act: Perform health check
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	reports, err := module.HealthCheck(ctx)

	// Assert: Should return unhealthy status
	assert.NoError(t, err)
	assert.NotEmpty(t, reports)

	// Find the cache health report
	var cacheReport *modular.HealthReport
	for i, report := range reports {
		if report.Module == "cache" {
			cacheReport = &reports[i]
			break
		}
	}

	require.NotNil(t, cacheReport, "Expected cache health report")
	assert.Equal(t, "cache", cacheReport.Module)
	assert.Equal(t, modular.HealthStatusUnhealthy, cacheReport.Status)
	assert.Contains(t, cacheReport.Message, "not initialized")
	assert.False(t, cacheReport.Optional)
}

func TestCacheModule_HealthCheck_WithCacheUsage(t *testing.T) {
	// RED PHASE: Test health check with cache operations

	// Create a cache module with memory engine
	module := &CacheModule{
		name: "cache",
		config: &CacheConfig{
			Engine:          "memory",
			DefaultTTL:      300,
			MaxItems:        10, // Small limit to test capacity
			CleanupInterval: 60,
		},
		logger: &testLogger{}, // Add test logger to avoid nil pointer
	}

	// Initialize the cache engine by setting up the memory cache directly
	memCache := NewMemoryCache(module.config)
	module.cacheEngine = memCache

	// Connect the cache engine
	ctx := context.Background()
	err := memCache.Connect(ctx)
	require.NoError(t, err)

	// Add some items to test usage reporting directly via cache engine
	for i := 0; i < 5; i++ {
		err := memCache.Set(ctx, fmt.Sprintf("key%d", i), fmt.Sprintf("value%d", i), time.Hour)
		require.NoError(t, err)
	}

	// Act: Perform health check
	reports, err := module.HealthCheck(ctx)

	// Assert: Should show usage information
	assert.NoError(t, err)
	assert.NotEmpty(t, reports)

	var cacheReport *modular.HealthReport
	for i, report := range reports {
		if report.Module == "cache" {
			cacheReport = &reports[i]
			break
		}
	}

	require.NotNil(t, cacheReport, "Expected cache health report")
	assert.Equal(t, modular.HealthStatusHealthy, cacheReport.Status)

	// Check that usage information is included
	assert.Contains(t, cacheReport.Details, "item_count")
	itemCount, ok := cacheReport.Details["item_count"].(int)
	assert.True(t, ok)
	assert.Equal(t, 5, itemCount)
}

func TestCacheModule_HealthCheck_HighCapacityUsage(t *testing.T) {
	// RED PHASE: Test degraded status when cache is near capacity

	// Create a cache module with very small capacity
	module := &CacheModule{
		name: "cache",
		config: &CacheConfig{
			Engine:          "memory",
			DefaultTTL:      300,
			MaxItems:        5, // Very small limit
			CleanupInterval: 60,
		},
		logger: &testLogger{}, // Add test logger to avoid nil pointer
	}

	// Initialize the cache engine by setting up the memory cache directly
	memCache := NewMemoryCache(module.config)
	module.cacheEngine = memCache

	// Connect the cache engine
	ctx := context.Background()
	err := memCache.Connect(ctx)
	require.NoError(t, err)

	// Fill cache to near capacity (90%+ should be degraded) directly via cache engine
	for i := 0; i < 5; i++ { // Fill to 100%
		err := memCache.Set(ctx, fmt.Sprintf("key%d", i), fmt.Sprintf("value%d", i), time.Hour)
		require.NoError(t, err)
	}

	// Act: Perform health check
	reports, err := module.HealthCheck(ctx)

	// Assert: Should show degraded status due to high usage
	assert.NoError(t, err)
	assert.NotEmpty(t, reports)

	var cacheReport *modular.HealthReport
	for i, report := range reports {
		if report.Module == "cache" {
			cacheReport = &reports[i]
			break
		}
	}

	require.NotNil(t, cacheReport, "Expected cache health report")
	// Should be degraded when at or near capacity (could be "cache full" or "usage high")
	assert.Equal(t, modular.HealthStatusDegraded, cacheReport.Status)
	// Message could be either "cache full" or "usage high"
	hasExpectedMessage := strings.Contains(cacheReport.Message, "usage high") ||
		strings.Contains(cacheReport.Message, "cache full")
	assert.True(t, hasExpectedMessage, "Expected message about high usage or full cache, got: %s", cacheReport.Message)
}

func TestCacheModule_HealthCheck_WithContext(t *testing.T) {
	// RED PHASE: Test context cancellation handling

	module := &CacheModule{
		name: "cache",
		config: &CacheConfig{
			Engine: "memory",
		},
	}

	// Initialize the cache engine by setting up the memory cache directly
	memCache := NewMemoryCache(module.config)
	module.cacheEngine = memCache

	// Act: Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	reports, err := module.HealthCheck(ctx)

	// Assert: Should handle context cancellation gracefully
	if err != nil {
		assert.Contains(t, err.Error(), "context")
	} else {
		// If no error, reports should still be valid
		assert.NotNil(t, reports)
	}
}

// Test helper to verify the module implements HealthProvider interface
func TestCacheModule_ImplementsHealthProvider(t *testing.T) {
	// Verify that CacheModule implements HealthProvider interface
	module := &CacheModule{
		name: "cache",
		config: &CacheConfig{
			Engine: "memory",
		},
	}

	// This should compile without errors if the interface is properly implemented
	var _ modular.HealthProvider = module

	// Also verify method signatures exist (will fail to compile if missing)
	ctx := context.Background()
	reports, err := module.HealthCheck(ctx)

	// Error is expected since module is not initialized, but method should exist
	assert.NoError(t, err)
	assert.NotNil(t, reports)
}
