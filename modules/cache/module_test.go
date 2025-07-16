package cache

import (
	"context"
	"testing"
	"time"

	"github.com/CrisisTextLine/modular"
	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockApp struct {
	configSections map[string]modular.ConfigProvider
	logger         modular.Logger
	configProvider modular.ConfigProvider
	modules        []modular.Module
	services       modular.ServiceRegistry
}

func newMockApp() *mockApp {
	return &mockApp{
		configSections: make(map[string]modular.ConfigProvider),
		logger:         &mockLogger{},
		configProvider: &mockConfigProvider{},
		services:       make(modular.ServiceRegistry),
	}
}

func (a *mockApp) RegisterConfigSection(name string, provider modular.ConfigProvider) {
	a.configSections[name] = provider
}

func (a *mockApp) GetConfigSection(name string) (modular.ConfigProvider, error) {
	return a.configSections[name], nil
}

func (a *mockApp) ConfigSections() map[string]modular.ConfigProvider {
	return a.configSections
}

func (a *mockApp) Logger() modular.Logger {
	return a.logger
}

func (a *mockApp) SetLogger(logger modular.Logger) {
	a.logger = logger
}

func (a *mockApp) ConfigProvider() modular.ConfigProvider {
	return a.configProvider
}

func (a *mockApp) SvcRegistry() modular.ServiceRegistry {
	return a.services
}

func (a *mockApp) RegisterModule(module modular.Module) {
	a.modules = append(a.modules, module)
}

func (a *mockApp) RegisterService(name string, service any) error {
	a.services[name] = service
	return nil
}

func (a *mockApp) GetService(name string, target any) error {
	return nil
}

func (a *mockApp) Init() error {
	return nil
}

func (a *mockApp) Start() error {
	return nil
}

func (a *mockApp) Stop() error {
	return nil
}

func (a *mockApp) Run() error {
	return nil
}

func (a *mockApp) IsVerboseConfig() bool {
	return false
}

func (a *mockApp) SetVerboseConfig(verbose bool) {
	// No-op in mock
}

type mockConfigProvider struct{}

func (m *mockConfigProvider) GetConfig() interface{} {
	return nil
}

type mockLogger struct{}

func (l *mockLogger) Debug(msg string, args ...interface{}) {}
func (l *mockLogger) Info(msg string, args ...interface{})  {}
func (l *mockLogger) Warn(msg string, args ...interface{})  {}
func (l *mockLogger) Error(msg string, args ...interface{}) {}

func TestCacheModule(t *testing.T) {
	module := NewModule()
	assert.Equal(t, "cache", module.Name())

	// Test configuration registration
	app := newMockApp()
	err := module.(*CacheModule).RegisterConfig(app)
	require.NoError(t, err)

	// Test initialization
	err = module.(*CacheModule).Init(app)
	require.NoError(t, err)

	// Test services provided
	services := module.(*CacheModule).ProvidesServices()
	assert.Equal(t, 1, len(services))
	assert.Equal(t, ServiceName, services[0].Name)
}

func TestMemoryCacheOperations(t *testing.T) {
	// Create the module
	module := NewModule().(*CacheModule)

	// Initialize with mock app
	app := newMockApp()
	err := module.RegisterConfig(app)
	require.NoError(t, err)
	err = module.Init(app)
	require.NoError(t, err)

	// Ensure we have a memory cache
	assert.IsType(t, &MemoryCache{}, module.cacheEngine)

	// Start the module
	ctx := context.Background()
	err = module.Start(ctx)
	require.NoError(t, err)

	// Test basic operations
	err = module.Set(ctx, "test-key", "test-value", time.Minute)
	assert.NoError(t, err)

	value, found := module.Get(ctx, "test-key")
	assert.True(t, found)
	assert.Equal(t, "test-value", value)

	err = module.Delete(ctx, "test-key")
	assert.NoError(t, err)

	_, found = module.Get(ctx, "test-key")
	assert.False(t, found)

	// Test multi operations
	items := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	err = module.SetMulti(ctx, items, time.Minute)
	assert.NoError(t, err)

	results, err := module.GetMulti(ctx, []string{"key1", "key2", "key4"})
	assert.NoError(t, err)
	assert.Equal(t, "value1", results["key1"])
	assert.Equal(t, "value2", results["key2"])
	assert.NotContains(t, results, "key4")

	err = module.Flush(ctx)
	assert.NoError(t, err)

	_, found = module.Get(ctx, "key1")
	assert.False(t, found)

	// Stop the module
	err = module.Stop(ctx)
	require.NoError(t, err)
}

func TestExpiration(t *testing.T) {
	// Create the module
	module := NewModule().(*CacheModule)

	// Initialize with mock app and minimal TTL
	app := newMockApp()
	err := module.RegisterConfig(app)
	require.NoError(t, err)

	// Override config for faster expiration
	config := &CacheConfig{
		Engine:          "memory",
		DefaultTTL:      1, // 1 second
		CleanupInterval: 1, // 1 second
		MaxItems:        100,
	}
	app.RegisterConfigSection(ModuleName, modular.NewStdConfigProvider(config))

	err = module.Init(app)
	require.NoError(t, err)
	ctx := context.Background()
	err = module.Start(ctx)
	require.NoError(t, err)

	// Set with short TTL
	err = module.Set(ctx, "expires-quickly", "value", time.Second)
	assert.NoError(t, err)

	// Verify it exists
	_, found := module.Get(ctx, "expires-quickly")
	assert.True(t, found)

	// Wait for expiration
	time.Sleep(2 * time.Second)

	// Verify it's gone
	_, found = module.Get(ctx, "expires-quickly")
	assert.False(t, found)

	err = module.Stop(ctx)
	require.NoError(t, err)
}

// TestRedisConfiguration tests Redis configuration handling without actual Redis connection
func TestRedisConfiguration(t *testing.T) {
	// Create the module
	module := NewModule().(*CacheModule)

	// Initialize with Redis config
	app := newMockApp()
	err := module.RegisterConfig(app)
	require.NoError(t, err)

	// Override config for Redis
	config := &CacheConfig{
		Engine:           "redis",
		DefaultTTL:       300,
		CleanupInterval:  60,
		MaxItems:         10000,
		RedisURL:         "redis://localhost:6379",
		RedisPassword:    "",
		RedisDB:          0,
		ConnectionMaxAge: 60,
	}
	app.RegisterConfigSection(ModuleName, modular.NewStdConfigProvider(config))

	err = module.Init(app)
	require.NoError(t, err)

	// Ensure we have a Redis cache
	assert.IsType(t, &RedisCache{}, module.cacheEngine)

	// Note: We don't start the module here as it would require an actual Redis connection
}

// TestRedisOperationsWithMockBehavior tests Redis cache operations that don't require a real connection
func TestRedisOperationsWithMockBehavior(t *testing.T) {
	config := &CacheConfig{
		Engine:           "redis",
		DefaultTTL:       300,
		CleanupInterval:  60,
		MaxItems:         10000,
		RedisURL:         "redis://localhost:6379",
		RedisPassword:    "",
		RedisDB:          0,
		ConnectionMaxAge: 60,
	}

	cache := NewRedisCache(config)
	ctx := context.Background()

	// Test operations without connection (should return appropriate errors)
	_, found := cache.Get(ctx, "test-key")
	assert.False(t, found)

	err := cache.Set(ctx, "test-key", "test-value", time.Minute)
	assert.Equal(t, ErrNotConnected, err)

	err = cache.Delete(ctx, "test-key")
	assert.Equal(t, ErrNotConnected, err)

	err = cache.Flush(ctx)
	assert.Equal(t, ErrNotConnected, err)

	_, err = cache.GetMulti(ctx, []string{"key1", "key2"})
	assert.Equal(t, ErrNotConnected, err)

	err = cache.SetMulti(ctx, map[string]interface{}{"key1": "value1"}, time.Minute)
	assert.Equal(t, ErrNotConnected, err)

	err = cache.DeleteMulti(ctx, []string{"key1", "key2"})
	assert.Equal(t, ErrNotConnected, err)

	// Test close without connection
	err = cache.Close(ctx)
	assert.NoError(t, err)
}

// TestRedisConfigurationEdgeCases tests edge cases in Redis configuration
func TestRedisConfigurationEdgeCases(t *testing.T) {
	config := &CacheConfig{
		Engine:           "redis",
		DefaultTTL:       300,
		CleanupInterval:  60,
		MaxItems:         10000,
		RedisURL:         "invalid-url",
		RedisPassword:    "test-password",
		RedisDB:          1,
		ConnectionMaxAge: 120,
	}

	cache := NewRedisCache(config)
	ctx := context.Background()

	// Test connection with invalid URL
	err := cache.Connect(ctx)
	assert.Error(t, err)
}

// TestRedisMultiOperationsEmptyInputs tests multi operations with empty inputs
func TestRedisMultiOperationsEmptyInputs(t *testing.T) {
	config := &CacheConfig{
		Engine:           "redis",
		DefaultTTL:       300,
		CleanupInterval:  60,
		MaxItems:         10000,
		RedisURL:         "redis://localhost:6379",
		RedisPassword:    "",
		RedisDB:          0,
		ConnectionMaxAge: 60,
	}

	cache := NewRedisCache(config)
	ctx := context.Background()

	// Test GetMulti with empty keys - should return empty map (no connection needed)
	results, err := cache.GetMulti(ctx, []string{})
	assert.NoError(t, err)
	assert.Equal(t, map[string]interface{}{}, results)

	// Test SetMulti with empty items - should succeed (no connection needed)
	err = cache.SetMulti(ctx, map[string]interface{}{}, time.Minute)
	assert.NoError(t, err)

	// Test DeleteMulti with empty keys - should succeed (no connection needed)
	err = cache.DeleteMulti(ctx, []string{})
	assert.NoError(t, err)
}

// TestRedisConnectWithPassword tests connection configuration with password
func TestRedisConnectWithPassword(t *testing.T) {
	config := &CacheConfig{
		Engine:           "redis",
		DefaultTTL:       300,
		CleanupInterval:  60,
		MaxItems:         10000,
		RedisURL:         "redis://localhost:6379",
		RedisPassword:    "test-password",
		RedisDB:          1,
		ConnectionMaxAge: 120,
	}

	cache := NewRedisCache(config)
	ctx := context.Background()

	// Test connection with password and different DB - this will fail since no Redis server
	// but will exercise the connection configuration code paths
	err := cache.Connect(ctx)
	assert.Error(t, err) // Expected to fail without Redis server

	// Test Close when client is nil initially
	err = cache.Close(ctx)
	assert.NoError(t, err)
}

// TestRedisJSONMarshaling tests JSON marshaling error scenarios
func TestRedisJSONMarshaling(t *testing.T) {
	// Start a test Redis server
	s := miniredis.RunT(t)
	defer s.Close()

	config := &CacheConfig{
		Engine:           "redis",
		DefaultTTL:       300,
		CleanupInterval:  60,
		MaxItems:         10000,
		RedisURL:         "redis://" + s.Addr(),
		RedisPassword:    "",
		RedisDB:          0,
		ConnectionMaxAge: 60,
	}

	cache := NewRedisCache(config)
	ctx := context.Background()

	// Connect to the test Redis server
	err := cache.Connect(ctx)
	require.NoError(t, err)
	defer cache.Close(ctx)

	// Test Set with invalid JSON value (function cannot be marshaled)
	err = cache.Set(ctx, "test-key", func() {}, time.Minute)
	assert.Equal(t, ErrInvalidValue, err)

	// Test SetMulti with values that cause JSON marshaling errors
	invalidItems := map[string]interface{}{
		"valid-key":   "valid-value",
		"invalid-key": func() {}, // Functions cannot be marshaled to JSON
	}

	err = cache.SetMulti(ctx, invalidItems, time.Minute)
	assert.Equal(t, ErrInvalidValue, err)
}

// TestRedisFullOperations tests Redis operations with a test server
func TestRedisFullOperations(t *testing.T) {
	// Start a test Redis server
	s := miniredis.RunT(t)
	defer s.Close()

	config := &CacheConfig{
		Engine:           "redis",
		DefaultTTL:       300,
		CleanupInterval:  60,
		MaxItems:         10000,
		RedisURL:         "redis://" + s.Addr(),
		RedisPassword:    "",
		RedisDB:          0,
		ConnectionMaxAge: 60,
	}

	cache := NewRedisCache(config)
	ctx := context.Background()

	// Test Connect
	err := cache.Connect(ctx)
	require.NoError(t, err)

	// Test Set and Get
	err = cache.Set(ctx, "test-key", "test-value", time.Minute)
	assert.NoError(t, err)

	value, found := cache.Get(ctx, "test-key")
	assert.True(t, found)
	assert.Equal(t, "test-value", value)

	// Test Delete
	err = cache.Delete(ctx, "test-key")
	assert.NoError(t, err)

	_, found = cache.Get(ctx, "test-key")
	assert.False(t, found)

	// Test SetMulti and GetMulti
	items := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
		"key3": map[string]string{"nested": "value"},
	}

	err = cache.SetMulti(ctx, items, time.Minute)
	assert.NoError(t, err)

	results, err := cache.GetMulti(ctx, []string{"key1", "key2", "key3", "nonexistent"})
	assert.NoError(t, err)
	assert.Equal(t, "value1", results["key1"])
	assert.Equal(t, float64(42), results["key2"]) // JSON unmarshaling returns numbers as float64
	assert.Equal(t, map[string]interface{}{"nested": "value"}, results["key3"])
	assert.NotContains(t, results, "nonexistent")

	// Test DeleteMulti
	err = cache.DeleteMulti(ctx, []string{"key1", "key2"})
	assert.NoError(t, err)

	// Verify deletions
	_, found = cache.Get(ctx, "key1")
	assert.False(t, found)
	_, found = cache.Get(ctx, "key2")
	assert.False(t, found)
	value, found = cache.Get(ctx, "key3")
	assert.True(t, found)
	assert.Equal(t, map[string]interface{}{"nested": "value"}, value)

	// Test Flush
	err = cache.Flush(ctx)
	assert.NoError(t, err)

	_, found = cache.Get(ctx, "key3")
	assert.False(t, found)

	// Test Close
	err = cache.Close(ctx)
	assert.NoError(t, err)
}

// TestRedisGetJSONUnmarshalError tests JSON unmarshaling errors in Get
func TestRedisGetJSONUnmarshalError(t *testing.T) {
	// Start a test Redis server
	s := miniredis.RunT(t)
	defer s.Close()

	config := &CacheConfig{
		Engine:           "redis",
		DefaultTTL:       300,
		CleanupInterval:  60,
		MaxItems:         10000,
		RedisURL:         "redis://" + s.Addr(),
		RedisPassword:    "",
		RedisDB:          0,
		ConnectionMaxAge: 60,
	}

	cache := NewRedisCache(config)
	ctx := context.Background()

	// Connect to the test Redis server
	err := cache.Connect(ctx)
	require.NoError(t, err)
	defer cache.Close(ctx)

	// Manually insert invalid JSON into Redis
	s.Set("invalid-json", "this is not valid JSON {")

	// Try to get the invalid JSON value
	value, found := cache.Get(ctx, "invalid-json")
	assert.False(t, found)
	assert.Nil(t, value)
}

// TestRedisGetWithServerError tests Get with server errors
func TestRedisGetWithServerError(t *testing.T) {
	// Start a test Redis server
	s := miniredis.RunT(t)

	config := &CacheConfig{
		Engine:           "redis",
		DefaultTTL:       300,
		CleanupInterval:  60,
		MaxItems:         10000,
		RedisURL:         "redis://" + s.Addr(),
		RedisPassword:    "",
		RedisDB:          0,
		ConnectionMaxAge: 60,
	}

	cache := NewRedisCache(config)
	ctx := context.Background()

	// Connect to the test Redis server
	err := cache.Connect(ctx)
	require.NoError(t, err)

	// Close the server to simulate connection error
	s.Close()

	// Try to get a value when server is down
	value, found := cache.Get(ctx, "test-key")
	assert.False(t, found)
	assert.Nil(t, value)

	// Try GetMulti when server is down
	results, err := cache.GetMulti(ctx, []string{"key1", "key2"})
	assert.Error(t, err)
	assert.Nil(t, results)

	// Close cache
	cache.Close(ctx)
}
