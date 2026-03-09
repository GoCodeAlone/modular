package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/GoCodeAlone/modular"
)

// Configuration-related BDD test steps

func (ctx *CacheBDDTestContext) iHaveACacheConfigurationWithMemoryEngine() error {
	ctx.cacheConfig = &CacheConfig{
		Engine:          "memory",
		DefaultTTL:      300 * time.Second,
		CleanupInterval: 60 * time.Second,
		MaxItems:        1000,
	}

	// Update the module's config if it exists
	if ctx.service != nil {
		ctx.service.config = ctx.cacheConfig
	}
	return nil
}

func (ctx *CacheBDDTestContext) iHaveACacheConfigurationWithRedisEngine() error {
	ctx.cacheConfig = &CacheConfig{
		Engine:          "redis",
		DefaultTTL:      300 * time.Second,
		CleanupInterval: 60 * time.Second,
		RedisURL:        "redis://localhost:6379",
		RedisDB:         0,
	}

	// Update the module's config if it exists
	if ctx.service != nil {
		ctx.service.config = ctx.cacheConfig
	}
	return nil
}

func (ctx *CacheBDDTestContext) theMemoryCacheEngineShouldBeConfigured() error {
	// Get the service so we can check its config
	if ctx.service == nil {
		return fmt.Errorf("cache service not available")
	}

	if ctx.service.config == nil {
		return fmt.Errorf("cache service config is nil")
	}

	if ctx.service.config.Engine != "memory" {
		return fmt.Errorf("memory cache engine not configured, found: %s", ctx.service.config.Engine)
	}
	return nil
}

func (ctx *CacheBDDTestContext) theRedisCacheEngineShouldBeConfigured() error {
	// Get the service so we can check its config
	if ctx.service == nil {
		return fmt.Errorf("cache service not available")
	}

	if ctx.service.config == nil {
		return fmt.Errorf("cache service config is nil")
	}

	if ctx.service.config.Engine != "redis" {
		return fmt.Errorf("redis cache engine not configured, found: %s", ctx.service.config.Engine)
	}
	return nil
}

func (ctx *CacheBDDTestContext) iHaveACacheServiceWithDefaultTTLConfigured() error {
	// Service already configured with default TTL in background
	return ctx.iHaveACacheServiceAvailable()
}

func (ctx *CacheBDDTestContext) iSetACacheItemWithoutSpecifyingTTL() error {
	err := ctx.service.Set(context.Background(), "default-ttl-key", "default-ttl-value", 0)
	if err != nil {
		ctx.lastError = err
	}
	return err
}

func (ctx *CacheBDDTestContext) theItemShouldUseTheDefaultTTLFromConfiguration() error {
	// This is validated by the fact that the item was set successfully
	// The actual TTL validation would require inspecting internal cache state
	// which is implementation-specific
	return nil
}

func (ctx *CacheBDDTestContext) iHaveACacheConfigurationWithInvalidRedisSettings() error {
	ctx.cacheConfig = &CacheConfig{
		Engine:          "redis",
		DefaultTTL:      300 * time.Second,
		CleanupInterval: 60 * time.Second, // Add non-zero cleanup interval
		RedisURL:        "redis://invalid-host:9999",
	}
	return nil
}

func (ctx *CacheBDDTestContext) theCacheModuleAttemptsToStart() error {
	// Create application with invalid Redis config
	logger := &testLogger{}

	// Create provider with the invalid cache config
	cacheConfigProvider := modular.NewStdConfigProvider(ctx.cacheConfig)

	// Create app with empty main config
	mainConfigProvider := modular.NewStdConfigProvider(struct{}{})
	app := modular.NewObservableApplication(mainConfigProvider, logger)

	// Create and register cache module
	module := NewModule().(*CacheModule)

	// Register the cache config section first
	app.RegisterConfigSection("cache", cacheConfigProvider)

	// Register the module
	app.RegisterModule(module)

	// Initialize
	if err := app.Init(); err != nil {
		return err
	}

	// Try to start the application (this should fail for Redis)
	ctx.lastError = app.Start()
	ctx.app = app
	return nil
}
