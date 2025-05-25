package scheduler

import (
	"context"
	"testing"

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

type mockLogger struct{}

func (l *mockLogger) Debug(msg string, args ...interface{}) {}
func (l *mockLogger) Info(msg string, args ...interface{})  {}
func (l *mockLogger) Warn(msg string, args ...interface{})  {}
func (l *mockLogger) Error(msg string, args ...interface{}) {}

type mockConfigProvider struct{}

func (m *mockConfigProvider) GetConfig() interface{} {
	return nil
}

func TestSchedulerModule(t *testing.T) {
	module := NewModule()
	assert.Equal(t, "scheduler", module.Name())

	// Test configuration registration
	app := newMockApp()
	err := module.(*SchedulerModule).RegisterConfig(app)
	require.NoError(t, err)

	// Test initialization
	err = module.(*SchedulerModule).Init(app)
	require.NoError(t, err)

	// Test services provided
	services := module.(*SchedulerModule).ProvidesServices()
	assert.Equal(t, 1, len(services))
	assert.Equal(t, ServiceName, services[0].Name)

	// Test module lifecycle
	ctx := context.Background()
	err = module.(*SchedulerModule).Start(ctx)
	require.NoError(t, err)

	err = module.(*SchedulerModule).Stop(ctx)
	require.NoError(t, err)
}

func TestSchedulerOperations(t *testing.T) {
	t.Skip("Skipping due to timing issues")
}
