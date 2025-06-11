package cache

import (
	"context"
	"testing"
	"time"

	"github.com/GoCodeAlone/modular"
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
