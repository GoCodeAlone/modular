package eventbus

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/CrisisTextLine/modular"
	cevent "github.com/cloudevents/sdk-go/v2/event"
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

// Context returns a background context
func (a *mockApp) Context() context.Context { return context.Background() }

// GetServicesByModule mock implementation returns empty slice
func (a *mockApp) GetServicesByModule(moduleName string) []string { return []string{} }

// GetServiceEntry mock implementation returns nil
func (a *mockApp) GetServiceEntry(serviceName string) (*modular.ServiceRegistryEntry, bool) {
	return nil, false
}

// GetServicesByInterface mock implementation returns empty slice
func (a *mockApp) GetServicesByInterface(interfaceType reflect.Type) []*modular.ServiceRegistryEntry {
	return []*modular.ServiceRegistryEntry{}
}

// GetModule returns a module by name (mock implementation)
func (a *mockApp) GetModule(name string) modular.Module {
	for _, m := range a.modules {
		if m.Name() == name {
			return m
		}
	}
	return nil
}

// GetAllModules returns all registered modules (mock implementation)
func (a *mockApp) GetAllModules() map[string]modular.Module {
	result := make(map[string]modular.Module)
	for _, m := range a.modules {
		result[m.Name()] = m
	}
	return result
}

// StartTime returns the application start time (mock implementation)
func (a *mockApp) StartTime() time.Time {
	return time.Time{}
}

// OnConfigLoaded registers a config loaded hook (mock implementation)
func (a *mockApp) OnConfigLoaded(hook func(app modular.Application) error) {}

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
	assert.Equal(t, 1, len(services))
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
			assert.Equal(t, "test.event", event.Type())
			var received map[string]interface{}
			require.NoError(t, event.DataAs(&received))
			assert.Equal(t, testData, received)
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
			assert.Equal(t, "async.event", event.Type())
			var received map[string]interface{}
			require.NoError(t, event.DataAs(&received))
			assert.Equal(t, testData, received)
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
	assert.NotNil(t, module.router)
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

func TestPublishEvent_RoutesPreBuiltCloudEvent(t *testing.T) {
	// Setup: create and start the module (same pattern as TestEventBusOperations)
	module := NewModule().(*EventBusModule)
	app := newMockApp()
	err := module.RegisterConfig(app)
	require.NoError(t, err)
	err = module.Init(app)
	require.NoError(t, err)
	ctx := context.Background()
	err = module.Start(ctx)
	require.NoError(t, err)
	defer module.Stop(ctx)

	// Create a pre-built CloudEvent with custom extensions
	event := cevent.New()
	event.SetType("test.event")
	event.SetSource("/test/source")
	event.SetID("test-id-123")
	event.SetTime(time.Now())
	event.SetData("application/json", map[string]string{"key": "value"})
	event.SetExtension("encryption", "aes-256-gcm")
	event.SetExtension("encryptedfields", `["key"]`)

	// Subscribe to capture the event
	eventReceived := make(chan Event, 1)
	subscription, err := module.Subscribe(ctx, "test.event", func(ctx context.Context, e Event) error {
		eventReceived <- e
		return nil
	})
	require.NoError(t, err)
	defer subscription.Cancel()

	// Act
	err = module.PublishEvent(ctx, event)
	require.NoError(t, err)

	// Assert
	select {
	case received := <-eventReceived:
		assert.Equal(t, "test.event", received.Type())
		assert.Equal(t, "/test/source", received.Source())
		assert.Equal(t, "test-id-123", received.ID())
		// Extensions should be preserved (not overwritten by module)
		assert.Equal(t, "aes-256-gcm", received.Extensions()["encryption"])
		assert.Equal(t, `["key"]`, received.Extensions()["encryptedfields"])
	case <-time.After(5 * time.Second):
		t.Fatal("Event not received within timeout")
	}
}
