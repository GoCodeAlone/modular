package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/GoCodeAlone/modular"
)

// ModuleName is the name of this module
const ModuleName = "cache"

// ServiceName is the name of the service provided by this module
const ServiceName = "cache.provider"

// CacheModule represents the cache module
type CacheModule struct {
	name        string
	config      *CacheConfig
	logger      modular.Logger
	cacheEngine CacheEngine
}

// NewModule creates a new instance of the cache module
func NewModule() modular.Module {
	return &CacheModule{
		name: ModuleName,
	}
}

// Name returns the name of the module
func (m *CacheModule) Name() string {
	return m.name
}

// RegisterConfig registers the module's configuration structure
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

// Init initializes the module
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

// Start performs startup logic for the module
func (m *CacheModule) Start(ctx context.Context) error {
	m.logger.Info("Starting cache module")
	err := m.cacheEngine.Connect(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect cache engine: %w", err)
	}
	return nil
}

// Stop performs shutdown logic for the module
func (m *CacheModule) Stop(ctx context.Context) error {
	m.logger.Info("Stopping cache module")
	if err := m.cacheEngine.Close(ctx); err != nil {
		return fmt.Errorf("failed to close cache engine: %w", err)
	}
	return nil
}

// Dependencies returns the names of modules this module depends on
func (m *CacheModule) Dependencies() []string {
	return nil
}

// ProvidesServices declares services provided by this module
func (m *CacheModule) ProvidesServices() []modular.ServiceProvider {
	return []modular.ServiceProvider{
		{
			Name:        ServiceName,
			Description: "Cache service for storing and retrieving data",
			Instance:    m,
		},
	}
}

// RequiresServices declares services required by this module
func (m *CacheModule) RequiresServices() []modular.ServiceDependency {
	return nil
}

// Constructor provides a dependency injection constructor for the module
func (m *CacheModule) Constructor() modular.ModuleConstructor {
	return func(app modular.Application, services map[string]any) (modular.Module, error) {
		return m, nil
	}
}

// Get retrieves a cached item by key
func (m *CacheModule) Get(ctx context.Context, key string) (interface{}, bool) {
	return m.cacheEngine.Get(ctx, key)
}

// Set stores an item in the cache with an optional TTL
func (m *CacheModule) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if ttl == 0 {
		ttl = time.Duration(m.config.DefaultTTL) * time.Second
	}
	if err := m.cacheEngine.Set(ctx, key, value, ttl); err != nil {
		return fmt.Errorf("failed to set cache item: %w", err)
	}
	return nil
}

// Delete removes an item from the cache
func (m *CacheModule) Delete(ctx context.Context, key string) error {
	if err := m.cacheEngine.Delete(ctx, key); err != nil {
		return fmt.Errorf("failed to delete cache item: %w", err)
	}
	return nil
}

// Flush removes all items from the cache
func (m *CacheModule) Flush(ctx context.Context) error {
	if err := m.cacheEngine.Flush(ctx); err != nil {
		return fmt.Errorf("failed to flush cache: %w", err)
	}
	return nil
}

// GetMulti retrieves multiple items from the cache
func (m *CacheModule) GetMulti(ctx context.Context, keys []string) (map[string]interface{}, error) {
	result, err := m.cacheEngine.GetMulti(ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("failed to get multiple cache items: %w", err)
	}
	return result, nil
}

// SetMulti stores multiple items in the cache
func (m *CacheModule) SetMulti(ctx context.Context, items map[string]interface{}, ttl time.Duration) error {
	if ttl == 0 {
		ttl = time.Duration(m.config.DefaultTTL) * time.Second
	}
	if err := m.cacheEngine.SetMulti(ctx, items, ttl); err != nil {
		return fmt.Errorf("failed to set multiple cache items: %w", err)
	}
	return nil
}

// DeleteMulti removes multiple items from the cache
func (m *CacheModule) DeleteMulti(ctx context.Context, keys []string) error {
	if err := m.cacheEngine.DeleteMulti(ctx, keys); err != nil {
		return fmt.Errorf("failed to delete multiple cache items: %w", err)
	}
	return nil
}
