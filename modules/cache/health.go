package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/GoCodeAlone/modular"
)

// Static errors for cache health checks
var (
	ErrCacheSetTestFailed      = errors.New("cache: failed to set test value")
	ErrCacheRetrieveTestFailed = errors.New("cache: failed to retrieve test value")
	ErrCacheTestValueMismatch  = errors.New("cache: retrieved value does not match set value")
)

// HealthCheck implements the HealthProvider interface for the cache module.
// This method checks the health of the configured cache engine (memory or Redis)
// and returns detailed reports about cache status, usage, and performance.
//
// The health check performs the following operations:
//   - Validates that the cache engine is initialized
//   - Tests basic cache connectivity
//   - Reports cache usage statistics and capacity information
//   - Provides performance and configuration details
//
// Returns:
//   - Slice of HealthReport objects with cache status information
//   - Error if the health check operation itself fails
func (m *CacheModule) HealthCheck(ctx context.Context) ([]modular.HealthReport, error) {
	reports := make([]modular.HealthReport, 0)
	checkTime := time.Now()

	// Create base report structure
	report := modular.HealthReport{
		Module:        "cache",
		Component:     m.config.Engine,
		CheckedAt:     checkTime,
		ObservedSince: checkTime,
		Optional:      false, // Cache is typically not optional for readiness
		Details:       make(map[string]any),
	}

	// Check if cache engine is initialized
	if m.cacheEngine == nil {
		report.Status = modular.HealthStatusUnhealthy
		report.Message = "cache engine not initialized"
		report.Details["engine"] = m.config.Engine
		report.Details["initialized"] = false
		reports = append(reports, report)
		return reports, nil
	}

	// Test basic cache connectivity and operations
	if err := m.testCacheConnectivity(ctx, &report); err != nil {
		report.Status = modular.HealthStatusUnhealthy
		report.Message = fmt.Sprintf("cache connectivity test failed: %v", err)
		report.Details["connectivity_error"] = err.Error()
		reports = append(reports, report)
		return reports, nil
	}

	// Collect cache statistics and usage information
	m.collectCacheStatistics(&report)

	// Determine overall health status based on usage and performance
	m.evaluateHealthStatus(&report)

	reports = append(reports, report)
	return reports, nil
}

// testCacheConnectivity tests basic cache operations to ensure the cache is working
func (m *CacheModule) testCacheConnectivity(ctx context.Context, report *modular.HealthReport) error {
	// Test key for health check
	healthKey := "health_check_" + fmt.Sprintf("%d", time.Now().Unix())
	healthValue := "health_test_value"

	// Try to set a value
	startTime := time.Now()
	if err := m.cacheEngine.Set(ctx, healthKey, healthValue, time.Minute); err != nil {
		// If cache is full, that's not necessarily unhealthy - just indicate degraded performance
		if err.Error() == "cache is full" {
			report.Details["operation_failed"] = "set_cache_full"
			report.Details["cache_full"] = true
			setGetDuration := time.Since(startTime)
			report.Details["set_get_duration_ms"] = setGetDuration.Milliseconds()
			return nil // Not a failure, just full
		}
		report.Details["operation_failed"] = "set"
		return fmt.Errorf("%w", ErrCacheSetTestFailed)
	}

	// Try to get the value back
	retrievedValue, found := m.cacheEngine.Get(ctx, healthKey)
	setGetDuration := time.Since(startTime)

	if !found {
		report.Details["operation_failed"] = "get"
		return ErrCacheRetrieveTestFailed
	}

	if retrievedValue != healthValue {
		report.Details["operation_failed"] = "value_mismatch"
		return ErrCacheTestValueMismatch
	}

	// Clean up test key
	_ = m.cacheEngine.Delete(ctx, healthKey)

	// Record performance metrics
	report.Details["set_get_duration_ms"] = setGetDuration.Milliseconds()
	report.Details["connectivity_test"] = "passed"

	return nil
}

// collectCacheStatistics gathers usage and performance statistics from the cache engine
func (m *CacheModule) collectCacheStatistics(report *modular.HealthReport) {
	// Add basic configuration information
	report.Details["engine"] = m.config.Engine
	report.Details["default_ttl_seconds"] = m.config.DefaultTTL
	report.Details["initialized"] = true

	// Engine-specific statistics
	switch m.config.Engine {
	case "memory":
		if memCache, ok := m.cacheEngine.(*MemoryCache); ok {
			m.collectMemoryCacheStats(memCache, report)
		}
	case "redis":
		if redisCache, ok := m.cacheEngine.(*RedisCache); ok {
			m.collectRedisCacheStats(redisCache, report)
		}
	}
}

// collectMemoryCacheStats collects statistics specific to memory cache
func (m *CacheModule) collectMemoryCacheStats(memCache *MemoryCache, report *modular.HealthReport) {
	// Get basic memory cache information - simulate item count from items map size
	memCache.mutex.RLock()
	itemCount := len(memCache.items)
	memCache.mutex.RUnlock()

	report.Details["item_count"] = itemCount
	report.Details["max_items"] = m.config.MaxItems

	// Calculate usage percentage
	if m.config.MaxItems > 0 {
		usagePercent := float64(itemCount) / float64(m.config.MaxItems) * 100.0
		report.Details["usage_percent"] = usagePercent
	}
}

// collectRedisCacheStats collects statistics specific to Redis cache
func (m *CacheModule) collectRedisCacheStats(redisCache *RedisCache, report *modular.HealthReport) {
	report.Details["redis_url"] = m.config.RedisURL
	report.Details["redis_db"] = m.config.RedisDB

	// Basic Redis configuration information - stats methods may not be available yet
	report.Details["connection_type"] = "redis"
}

// evaluateHealthStatus determines the overall health status based on collected metrics
func (m *CacheModule) evaluateHealthStatus(report *modular.HealthReport) {
	// Start with healthy status
	report.Status = modular.HealthStatusHealthy

	// Check if cache is full
	if isFull, ok := report.Details["cache_full"].(bool); ok && isFull {
		report.Status = modular.HealthStatusDegraded
		report.Message = "cache full: unable to accept new items"
		return
	}

	// Check for memory cache capacity issues
	if m.config.Engine == "memory" && m.config.MaxItems > 0 {
		if itemCount, ok := report.Details["item_count"].(int); ok {
			usagePercent := float64(itemCount) / float64(m.config.MaxItems) * 100.0

			if usagePercent >= 95.0 {
				report.Status = modular.HealthStatusDegraded
				report.Message = fmt.Sprintf("cache usage high: %d/%d items (%.1f%%)",
					itemCount, m.config.MaxItems, usagePercent)
				return
			} else if usagePercent >= 90.0 {
				report.Status = modular.HealthStatusDegraded
				report.Message = fmt.Sprintf("cache usage high: %d/%d items (%.1f%%)",
					itemCount, m.config.MaxItems, usagePercent)
				return
			}
		}
	}

	// Check performance metrics
	if duration, ok := report.Details["set_get_duration_ms"].(int64); ok {
		if duration > 1000 { // More than 1 second for basic operations
			report.Status = modular.HealthStatusDegraded
			report.Message = fmt.Sprintf("cache operations slow: %dms for set/get", duration)
			return
		}
	}

	// If we get here, cache is healthy
	report.Message = fmt.Sprintf("cache healthy: %s engine operational", m.config.Engine)
}

// GetHealthTimeout returns the maximum time needed for health checks to complete.
// Cache health checks involve basic set/get operations which should be fast.
func (m *CacheModule) GetHealthTimeout() time.Duration {
	// Base timeout for cache operations
	baseTimeout := 3 * time.Second

	// Redis might need slightly more time for network operations
	if m.config.Engine == "redis" {
		return baseTimeout + 2*time.Second
	}

	return baseTimeout
}

// IsHealthy is a convenience method that returns true if the cache is healthy.
// This is useful for quick health status checks without detailed reports.
func (m *CacheModule) IsHealthy(ctx context.Context) bool {
	reports, err := m.HealthCheck(ctx)
	if err != nil {
		return false
	}

	for _, report := range reports {
		if report.Status != modular.HealthStatusHealthy {
			return false
		}
	}

	return true
}
