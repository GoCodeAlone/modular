// Package cache provides a flexible caching module for the modular framework.
//
// This module supports multiple cache backends including in-memory and Redis,
// with configurable TTL, cleanup intervals, and connection management. It provides
// a unified interface for caching operations across different storage engines.
//
// # Supported Cache Engines
//
// The cache module supports the following engines:
//   - "memory": In-memory cache with LRU eviction and TTL support
//   - "redis": Redis-based cache with connection pooling and persistence
//
// # Configuration
//
// The module can be configured through the CacheConfig structure:
//
//	config := &CacheConfig{
//	    Engine:           "memory",     // or "redis"
//	    DefaultTTL:       300,          // 5 minutes default TTL
//	    CleanupInterval:  60,           // cleanup every minute
//	    MaxItems:         10000,        // max items for memory cache
//	    RedisURL:         "redis://localhost:6379", // for Redis engine
//	    RedisPassword:    "",           // Redis password if required
//	    RedisDB:          0,            // Redis database number
//	    ConnectionMaxAge: 60,           // connection max age in seconds
//	}
//
// # Service Registration
//
// The module registers itself as a service that can be injected into other modules:
//
//	// Get the cache service
//	cacheService := app.GetService("cache.provider").(*CacheModule)
//
//	// Use the cache
//	err := cacheService.Set(ctx, "key", "value", time.Minute*5)
//	value, found := cacheService.Get(ctx, "key")
//
// # Usage Examples
//
// Basic caching operations:
//
//	// Set a value with default TTL
//	err := cache.Set(ctx, "user:123", userData, 0)
//
//	// Set a value with custom TTL
//	err := cache.Set(ctx, "session:abc", sessionData, time.Hour)
//
//	// Get a value
//	value, found := cache.Get(ctx, "user:123")
//	if found {
//	    user := value.(UserData)
//	    // use user data
//	}
//
//	// Batch operations
//	items := map[string]interface{}{
//	    "key1": "value1",
//	    "key2": "value2",
//	}
//	err := cache.SetMulti(ctx, items, time.Minute*10)
//
//	results, err := cache.GetMulti(ctx, []string{"key1", "key2"})
package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/CrisisTextLine/modular"
)

// ModuleName is the unique identifier for the cache module.
const ModuleName = "cache"

// ServiceName is the name of the service provided by this module.
// Other modules can use this name to request the cache service through dependency injection.
const ServiceName = "cache.provider"

// CacheModule provides caching functionality for the modular framework.
// It supports multiple cache backends (memory and Redis) and provides a unified
// interface for caching operations including TTL management, batch operations,
// and automatic cleanup.
//
// The module implements the following interfaces:
//   - modular.Module: Basic module lifecycle
//   - modular.Configurable: Configuration management
//   - modular.ServiceAware: Service dependency management
//   - modular.Startable: Startup logic
//   - modular.Stoppable: Shutdown logic
//
// Cache operations are thread-safe and support context cancellation.
type CacheModule struct {
	name        string
	config      *CacheConfig
	logger      modular.Logger
	cacheEngine CacheEngine
}

// NewModule creates a new instance of the cache module.
// This is the primary constructor for the cache module and should be used
// when registering the module with the application.
//
// Example:
//
//	app.RegisterModule(cache.NewModule())
func NewModule() modular.Module {
	return &CacheModule{
		name: ModuleName,
	}
}

// Name returns the unique identifier for this module.
// This name is used for service registration, dependency resolution,
// and configuration section identification.
func (m *CacheModule) Name() string {
	return m.name
}

// RegisterConfig registers the module's configuration structure.
// This method is called during application initialization to register
// the default configuration values for the cache module.
//
// Default configuration:
//   - Engine: "memory"
//   - DefaultTTL: 300 seconds (5 minutes)
//   - CleanupInterval: 60 seconds (1 minute)
//   - MaxItems: 10000
//   - Redis settings: empty/default values
func (m *CacheModule) RegisterConfig(app modular.Application) error {
	// Register the configuration with default values
	defaultConfig := &CacheConfig{
		Engine:           "memory",
		DefaultTTL:       300,
		CleanupInterval:  60,
		MaxItems:         10000,
		RedisURL:         "",
		RedisPassword:    "",
		RedisDB:          0,
		ConnectionMaxAge: 60,
	}

	app.RegisterConfigSection(m.Name(), modular.NewStdConfigProvider(defaultConfig))
	return nil
}

// Init initializes the cache module with the application context.
// This method is called after all modules have been registered and their
// configurations loaded. It sets up the cache engine based on the configuration.
//
// The initialization process:
//  1. Retrieves the module's configuration
//  2. Sets up logging
//  3. Initializes the appropriate cache engine (memory or Redis)
//  4. Logs the initialization status
//
// Supported cache engines:
//   - "memory": In-memory cache with LRU eviction
//   - "redis": Redis-based distributed cache
//   - fallback: defaults to memory cache for unknown engines
func (m *CacheModule) Init(app modular.Application) error {
	// Retrieve the registered config section for access
	cfg, err := app.GetConfigSection(m.name)
	if err != nil {
		return fmt.Errorf("failed to get config section for cache module: %w", err)
	}

	m.config = cfg.GetConfig().(*CacheConfig)
	m.logger = app.Logger()

	// Initialize the appropriate cache engine based on configuration
	switch m.config.Engine {
	case "memory":
		m.cacheEngine = NewMemoryCache(m.config)
		m.logger.Info("Initialized memory cache engine", "maxItems", m.config.MaxItems)
	case "redis":
		m.cacheEngine = NewRedisCache(m.config)
		m.logger.Info("Initialized Redis cache engine", "url", m.config.RedisURL)
	default:
		m.cacheEngine = NewMemoryCache(m.config)
		m.logger.Warn("Unknown cache engine specified, using memory cache", "specified", m.config.Engine)
	}

	m.logger.Info("Cache module initialized")
	return nil
}

// Start performs startup logic for the module.
// This method establishes connections to the cache backend and prepares
// the cache for operations. It's called after all modules have been initialized.
//
// For memory cache: No external connections are needed
// For Redis cache: Establishes connection pool to the Redis server
func (m *CacheModule) Start(ctx context.Context) error {
	m.logger.Info("Starting cache module")
	err := m.cacheEngine.Connect(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect cache engine: %w", err)
	}
	return nil
}

// Stop performs shutdown logic for the module.
// This method gracefully closes all connections and cleans up resources.
// It's called during application shutdown to ensure proper cleanup.
//
// The shutdown process:
//  1. Logs the shutdown initiation
//  2. Closes cache engine connections
//  3. Cleans up any background processes
func (m *CacheModule) Stop(ctx context.Context) error {
	m.logger.Info("Stopping cache module")
	if err := m.cacheEngine.Close(ctx); err != nil {
		return fmt.Errorf("failed to close cache engine: %w", err)
	}
	return nil
}

// Dependencies returns the names of modules this module depends on.
// The cache module has no dependencies and can be started independently.
func (m *CacheModule) Dependencies() []string {
	return nil
}

// ProvidesServices declares services provided by this module.
// The cache module provides a cache service that can be injected into other modules.
//
// Provided services:
//   - "cache.provider": The main cache service interface
func (m *CacheModule) ProvidesServices() []modular.ServiceProvider {
	return []modular.ServiceProvider{
		{
			Name:        ServiceName,
			Description: "Cache service for storing and retrieving data",
			Instance:    m,
		},
	}
}

// RequiresServices declares services required by this module.
// The cache module operates independently and requires no external services.
func (m *CacheModule) RequiresServices() []modular.ServiceDependency {
	return nil
}

// Constructor provides a dependency injection constructor for the module.
// This method is used by the dependency injection system to create
// the module instance with any required services.
func (m *CacheModule) Constructor() modular.ModuleConstructor {
	return func(app modular.Application, services map[string]any) (modular.Module, error) {
		return m, nil
	}
}

// Get retrieves a cached item by key.
// Returns the cached value and a boolean indicating whether the key was found.
// If the key doesn't exist or has expired, returns (nil, false).
//
// Example:
//
//	value, found := cache.Get(ctx, "user:123")
//	if found {
//	    user := value.(UserData)
//	    // process user data
//	}
func (m *CacheModule) Get(ctx context.Context, key string) (interface{}, bool) {
	return m.cacheEngine.Get(ctx, key)
}

// Set stores an item in the cache with an optional TTL.
// If ttl is 0, uses the default TTL from configuration.
// The value can be any serializable type.
//
// Example:
//
//	// Use default TTL
//	err := cache.Set(ctx, "user:123", userData, 0)
//
//	// Use custom TTL
//	err := cache.Set(ctx, "session:abc", sessionData, time.Hour)
func (m *CacheModule) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if ttl == 0 {
		ttl = time.Duration(m.config.DefaultTTL) * time.Second
	}
	if err := m.cacheEngine.Set(ctx, key, value, ttl); err != nil {
		return fmt.Errorf("failed to set cache item: %w", err)
	}
	return nil
}

// Delete removes an item from the cache.
// Returns an error if the deletion fails, but not if the key doesn't exist.
//
// Example:
//
//	err := cache.Delete(ctx, "user:123")
//	if err != nil {
//	    // handle deletion error
//	}
func (m *CacheModule) Delete(ctx context.Context, key string) error {
	if err := m.cacheEngine.Delete(ctx, key); err != nil {
		return fmt.Errorf("failed to delete cache item: %w", err)
	}
	return nil
}

// Flush removes all items from the cache.
// This operation is irreversible and should be used with caution.
// Useful for cache invalidation or testing scenarios.
//
// Example:
//
//	err := cache.Flush(ctx)
//	if err != nil {
//	    // handle flush error
//	}
func (m *CacheModule) Flush(ctx context.Context) error {
	if err := m.cacheEngine.Flush(ctx); err != nil {
		return fmt.Errorf("failed to flush cache: %w", err)
	}
	return nil
}

// GetMulti retrieves multiple items from the cache in a single operation.
// Returns a map of key-value pairs for found items and an error if the operation fails.
// Missing keys are simply not included in the result map.
//
// Example:
//
//	keys := []string{"user:123", "user:456", "user:789"}
//	results, err := cache.GetMulti(ctx, keys)
//	if err != nil {
//	    // handle error
//	}
//	for key, value := range results {
//	    // process found values
//	}
func (m *CacheModule) GetMulti(ctx context.Context, keys []string) (map[string]interface{}, error) {
	result, err := m.cacheEngine.GetMulti(ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("failed to get multiple cache items: %w", err)
	}
	return result, nil
}

// SetMulti stores multiple items in the cache in a single operation.
// All items will use the same TTL. If ttl is 0, uses the default TTL from configuration.
// This is more efficient than multiple Set calls for batch operations.
//
// Example:
//
//	items := map[string]interface{}{
//	    "user:123": userData1,
//	    "user:456": userData2,
//	    "session:abc": sessionData,
//	}
//	err := cache.SetMulti(ctx, items, time.Minute*30)
func (m *CacheModule) SetMulti(ctx context.Context, items map[string]interface{}, ttl time.Duration) error {
	if ttl == 0 {
		ttl = time.Duration(m.config.DefaultTTL) * time.Second
	}
	if err := m.cacheEngine.SetMulti(ctx, items, ttl); err != nil {
		return fmt.Errorf("failed to set multiple cache items: %w", err)
	}
	return nil
}

// DeleteMulti removes multiple items from the cache in a single operation.
// This is more efficient than multiple Delete calls for batch operations.
// Does not return an error for keys that don't exist.
//
// Example:
//
//	keys := []string{"user:123", "user:456", "expired:key"}
//	err := cache.DeleteMulti(ctx, keys)
//	if err != nil {
//	    // handle deletion error
//	}
func (m *CacheModule) DeleteMulti(ctx context.Context, keys []string) error {
	if err := m.cacheEngine.DeleteMulti(ctx, keys); err != nil {
		return fmt.Errorf("failed to delete multiple cache items: %w", err)
	}
	return nil
}
