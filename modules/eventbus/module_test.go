package eventbus

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

type mockLogger struct{}

func (l *mockLogger) Debug(msg string, args ...interface{}) {}
func (l *mockLogger) Info(msg string, args ...interface{})  {}
func (l *mockLogger) Warn(msg string, args ...interface{})  {}
func (l *mockLogger) Error(msg string, args ...interface{}) {}

type mockConfigProvider struct{}

func (m *mockConfigProvider) GetConfig() interface{} {
	return nil
}

func TestEventBusModule(t *testing.T) {
	module := NewModule()
	assert.Equal(t, "eventbus", module.Name())

	// Test configuration registration
	app := newMockApp()
	err := module.(*EventBusModule).RegisterConfig(app)
	require.NoError(t, err)

	// Test initialization
	err = module.(*EventBusModule).Init(app)
	require.NoError(t, err)

	// Test services provided
	services := module.(*EventBusModule).ProvidesServices()
	assert.Len(t, services, 1)
	assert.Equal(t, ServiceName, services[0].Name)

	// Test module lifecycle
	ctx := context.Background()
	err = module.(*EventBusModule).Start(ctx)
	require.NoError(t, err)

	err = module.(*EventBusModule).Stop(ctx)
	require.NoError(t, err)
}

func TestEventBusOperations(t *testing.T) {
	// Create the module
	module := NewModule().(*EventBusModule)

	// Initialize with mock app
	app := newMockApp()
	err := module.RegisterConfig(app)
	require.NoError(t, err)
	err = module.Init(app)
	require.NoError(t, err)

	// Start the module
	ctx := context.Background()
	err = module.Start(ctx)
	require.NoError(t, err)

	// Test event publishing and subscribing
	t.Run("PublishSubscribe", func(t *testing.T) {
		eventReceived := make(chan bool, 1)
		testData := map[string]interface{}{"test": "data"}

		// Subscribe to an event
		subscription, err := module.Subscribe(ctx, "test.event", func(ctx context.Context, event Event) error {
			assert.Equal(t, "test.event", event.Topic)
			assert.Equal(t, testData, event.Payload)
			eventReceived <- true
			return nil
		})
		require.NoError(t, err)
		defer func() {
			if cancelErr := subscription.Cancel(); cancelErr != nil {
				t.Logf("Failed to cancel subscription: %v", cancelErr)
			}
		}()

		// Publish an event
		err = module.Publish(ctx, "test.event", testData)
		require.NoError(t, err)

		// Wait for event to be received
		select {
		case <-eventReceived:
			// Event received successfully
		case <-ctx.Done():
			t.Fatal("Event not received within timeout")
		}
	})

	t.Run("MultipleSubscribers", func(t *testing.T) {
		received1 := make(chan bool, 1)
		received2 := make(chan bool, 1)
		testData := map[string]interface{}{"multi": "subscriber"}

		// Subscribe with multiple handlers
		subscription1, err := module.Subscribe(ctx, "multi.event", func(ctx context.Context, event Event) error {
			received1 <- true
			return nil
		})
		require.NoError(t, err)
		defer func() {
			if cancelErr := subscription1.Cancel(); cancelErr != nil {
				t.Logf("Failed to cancel subscription1: %v", cancelErr)
			}
		}()

		subscription2, err := module.Subscribe(ctx, "multi.event", func(ctx context.Context, event Event) error {
			received2 <- true
			return nil
		})
		require.NoError(t, err)
		defer func() {
			if cancelErr := subscription2.Cancel(); cancelErr != nil {
				t.Logf("Failed to cancel subscription2: %v", cancelErr)
			}
		}()

		// Publish event
		err = module.Publish(ctx, "multi.event", testData)
		require.NoError(t, err)

		// Both subscribers should receive the event
		select {
		case <-received1:
		case <-ctx.Done():
			t.Fatal("First subscriber did not receive event")
		}

		select {
		case <-received2:
		case <-ctx.Done():
			t.Fatal("Second subscriber did not receive event")
		}
	})

	t.Run("Unsubscribe", func(t *testing.T) {
		eventReceived := make(chan bool, 1)

		// Subscribe to event
		subscription, err := module.Subscribe(ctx, "unsub.event", func(ctx context.Context, event Event) error {
			eventReceived <- true
			return nil
		})
		require.NoError(t, err)

		// Unsubscribe
		err = module.Unsubscribe(ctx, subscription)
		require.NoError(t, err)

		// Publish event - should not be received
		err = module.Publish(ctx, "unsub.event", map[string]interface{}{"test": "data"})
		require.NoError(t, err)

		// Verify no event was received
		select {
		case <-eventReceived:
			t.Fatal("Event should not have been received after unsubscribe")
		default:
			// Expected - no event received
		}
	})

	t.Run("AsyncSubscribe", func(t *testing.T) {
		eventReceived := make(chan bool, 1)
		testData := map[string]interface{}{"async": "test"}

		// Subscribe async to an event
		subscription, err := module.SubscribeAsync(ctx, "async.event", func(ctx context.Context, event Event) error {
			assert.Equal(t, "async.event", event.Topic)
			assert.Equal(t, testData, event.Payload)
			eventReceived <- true
			return nil
		})
		require.NoError(t, err)
		defer func() {
			if cancelErr := subscription.Cancel(); cancelErr != nil {
				t.Logf("Failed to cancel subscription: %v", cancelErr)
			}
		}()

		// Publish an event
		err = module.Publish(ctx, "async.event", testData)
		require.NoError(t, err)

		// Wait for event to be received
		select {
		case <-eventReceived:
			// Event received successfully
		case <-ctx.Done():
			t.Fatal("Async event not received within timeout")
		}
	})

	// Stop the module
	err = module.Stop(ctx)
	require.NoError(t, err)
}

func TestEventBusConfiguration(t *testing.T) {
	module := NewModule().(*EventBusModule)
	app := newMockApp()

	// Test with custom configuration
	config := &EventBusConfig{
		Engine:      "memory",
		WorkerCount: 10,
	}
	app.RegisterConfigSection(ModuleName, modular.NewStdConfigProvider(config))

	err := module.Init(app)
	require.NoError(t, err)

	// Verify configuration was applied
	assert.NotNil(t, module.eventbus)
}

func TestEventBusServiceProvider(t *testing.T) {
	module := NewModule().(*EventBusModule)
	app := newMockApp()

	err := module.RegisterConfig(app)
	require.NoError(t, err)
	err = module.Init(app)
	require.NoError(t, err)

	// Test service provides
	services := module.ProvidesServices()
	assert.Len(t, services, 1)
	assert.Equal(t, ServiceName, services[0].Name)
	assert.Equal(t, "Event bus for message distribution", services[0].Description)

	// Test required services
	required := module.RequiresServices()
	assert.Empty(t, required)
}
