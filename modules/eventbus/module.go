package eventbus

import (
	"context"
	"fmt"
	"sync"

	"github.com/GoCodeAlone/modular"
)

// ModuleName is the name of this module
const ModuleName = "eventbus"

// ServiceName is the name of the service provided by this module
const ServiceName = "eventbus.provider"

// EventBusModule represents the event bus module
type EventBusModule struct {
	name      string
	config    *EventBusConfig
	logger    modular.Logger
	eventbus  EventBus
	mutex     sync.RWMutex
	isStarted bool
}

// NewModule creates a new instance of the event bus module
func NewModule() modular.Module {
	return &EventBusModule{
		name: ModuleName,
	}
}

// Name returns the name of the module
func (m *EventBusModule) Name() string {
	return m.name
}

// RegisterConfig registers the module's configuration structure
func (m *EventBusModule) RegisterConfig(app modular.Application) error {
	// Register the configuration with default values
	defaultConfig := &EventBusConfig{
		Engine:                 "memory",
		MaxEventQueueSize:      1000,
		DefaultEventBufferSize: 10,
		WorkerCount:            5,
		EventTTL:               3600,
		RetentionDays:          7,
		ExternalBrokerURL:      "",
		ExternalBrokerUser:     "",
		ExternalBrokerPassword: "",
	}

	app.RegisterConfigSection(m.Name(), modular.NewStdConfigProvider(defaultConfig))
	return nil
}

// Init initializes the module
func (m *EventBusModule) Init(app modular.Application) error {
	// Retrieve the registered config section for access
	cfg, err := app.GetConfigSection(m.name)
	if err != nil {
		return fmt.Errorf("failed to get config section '%s': %w", m.name, err)
	}

	m.config = cfg.GetConfig().(*EventBusConfig)
	m.logger = app.Logger()

	// Initialize the event bus based on configuration
	switch m.config.Engine {
	case "memory":
		m.eventbus = NewMemoryEventBus(m.config)
		m.logger.Info("Using memory event bus")
	default:
		m.eventbus = NewMemoryEventBus(m.config)
		m.logger.Warn("Unknown event bus engine specified, using memory engine", "specified", m.config.Engine)
	}

	m.logger.Info("Event bus module initialized")
	return nil
}

// Start performs startup logic for the module
func (m *EventBusModule) Start(ctx context.Context) error {
	m.logger.Info("Starting event bus module")

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.isStarted {
		return nil
	}

	// Start the event bus
	err := m.eventbus.Start(ctx)
	if err != nil {
		return err
	}

	m.isStarted = true
	m.logger.Info("Event bus started")
	return nil
}

// Stop performs shutdown logic for the module
func (m *EventBusModule) Stop(ctx context.Context) error {
	m.logger.Info("Stopping event bus module")

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isStarted {
		return nil
	}

	// Stop the event bus
	err := m.eventbus.Stop(ctx)
	if err != nil {
		return err
	}

	m.isStarted = false
	m.logger.Info("Event bus stopped")
	return nil
}

// Dependencies returns the names of modules this module depends on
func (m *EventBusModule) Dependencies() []string {
	return nil
}

// ProvidesServices declares services provided by this module
func (m *EventBusModule) ProvidesServices() []modular.ServiceProvider {
	return []modular.ServiceProvider{
		{
			Name:        ServiceName,
			Description: "Event bus for message distribution",
			Instance:    m,
		},
	}
}

// RequiresServices declares services required by this module
func (m *EventBusModule) RequiresServices() []modular.ServiceDependency {
	return nil
}

// Constructor provides a dependency injection constructor for the module
func (m *EventBusModule) Constructor() modular.ModuleConstructor {
	return func(app modular.Application, services map[string]any) (modular.Module, error) {
		return m, nil
	}
}

// Publish publishes an event to the event bus
func (m *EventBusModule) Publish(ctx context.Context, topic string, payload interface{}) error {
	event := Event{
		Topic:   topic,
		Payload: payload,
	}
	return m.eventbus.Publish(ctx, event)
}

// Subscribe subscribes to a topic on the event bus
func (m *EventBusModule) Subscribe(ctx context.Context, topic string, handler EventHandler) (Subscription, error) {
	return m.eventbus.Subscribe(ctx, topic, handler)
}

// SubscribeAsync subscribes to a topic with asynchronous event handling
func (m *EventBusModule) SubscribeAsync(ctx context.Context, topic string, handler EventHandler) (Subscription, error) {
	return m.eventbus.SubscribeAsync(ctx, topic, handler)
}

// Unsubscribe cancels a subscription
func (m *EventBusModule) Unsubscribe(ctx context.Context, subscription Subscription) error {
	return m.eventbus.Unsubscribe(ctx, subscription)
}

// Topics returns a list of all active topics
func (m *EventBusModule) Topics() []string {
	return m.eventbus.Topics()
}

// SubscriberCount returns the number of subscribers for a topic
func (m *EventBusModule) SubscriberCount(topic string) int {
	return m.eventbus.SubscriberCount(topic)
}