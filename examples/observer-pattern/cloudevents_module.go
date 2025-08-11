package main

import (
	"context"
	"fmt"
	"time"

	"github.com/CrisisTextLine/modular"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// CloudEventsModule demonstrates CloudEvents usage in the Observer pattern.
type CloudEventsModule struct {
	name   string
	app    modular.Application
	logger modular.Logger
}

// CloudEventsConfig holds configuration for the CloudEvents demo module.
type CloudEventsConfig struct {
	Enabled        bool   `yaml:"enabled" json:"enabled" default:"true" desc:"Enable CloudEvents demo"`
	DemoInterval   string `yaml:"demoInterval" json:"demoInterval" default:"10s" desc:"Interval between demo events"`
	EventNamespace string `yaml:"eventNamespace" json:"eventNamespace" default:"com.example.demo" desc:"Namespace for demo events"`
}

// NewCloudEventsModule creates a new CloudEvents demonstration module.
func NewCloudEventsModule() modular.Module {
	return &CloudEventsModule{
		name: "cloudevents-demo",
	}
}

// Name returns the module name.
func (m *CloudEventsModule) Name() string {
	return m.name
}

// RegisterConfig registers the module's configuration.
func (m *CloudEventsModule) RegisterConfig(app modular.Application) error {
	defaultConfig := &CloudEventsConfig{
		Enabled:        true,
		DemoInterval:   "10s",
		EventNamespace: "com.example.demo",
	}
	app.RegisterConfigSection(m.name, modular.NewStdConfigProvider(defaultConfig))
	return nil
}

// Init initializes the module.
func (m *CloudEventsModule) Init(app modular.Application) error {
	m.app = app
	m.logger = app.Logger()
	m.logger.Info("CloudEvents demo module initialized")
	return nil
}

// Start starts the CloudEvents demonstration.
func (m *CloudEventsModule) Start(ctx context.Context) error {
	cfg, err := m.app.GetConfigSection(m.name)
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	config := cfg.GetConfig().(*CloudEventsConfig)
	if !config.Enabled {
		m.logger.Info("CloudEvents demo is disabled")
		return nil
	}

	interval, err := time.ParseDuration(config.DemoInterval)
	if err != nil {
		return fmt.Errorf("invalid demo interval: %w", err)
	}

	// Start demonstration in background
	go m.runDemo(ctx, config, interval)

	m.logger.Info("CloudEvents demo started", "interval", interval)
	return nil
}

// Stop stops the module.
func (m *CloudEventsModule) Stop(ctx context.Context) error {
	m.logger.Info("CloudEvents demo stopped")
	return nil
}

// Dependencies returns module dependencies.
func (m *CloudEventsModule) Dependencies() []string {
	return nil
}

// runDemo runs the CloudEvents demonstration.
func (m *CloudEventsModule) runDemo(ctx context.Context, config *CloudEventsConfig, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	counter := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			counter++
			m.emitDemoCloudEvent(ctx, config, counter)
		}
	}
}

// emitDemoCloudEvent emits a demonstration CloudEvent.
func (m *CloudEventsModule) emitDemoCloudEvent(ctx context.Context, config *CloudEventsConfig, counter int) {
	// Check if the application supports CloudEvents (cast to ObservableApplication)
	observableApp, ok := m.app.(*modular.ObservableApplication)
	if !ok {
		m.logger.Warn("Application does not support CloudEvents")
		return
	}

	// Create a CloudEvent
	event := modular.NewCloudEvent(
		config.EventNamespace+".heartbeat",
		"cloudevents-demo",
		map[string]interface{}{
			"counter":   counter,
			"timestamp": time.Now().Unix(),
			"message":   fmt.Sprintf("Demo CloudEvent #%d", counter),
		},
		map[string]interface{}{
			"demo":    "true",
			"version": "1.0",
		},
	)

	// Set additional CloudEvent attributes
	event.SetSubject("demo-heartbeat")

	// Emit the CloudEvent
	if err := observableApp.NotifyObservers(ctx, event); err != nil {
		m.logger.Error("Failed to emit CloudEvent", "error", err)
	} else {
		m.logger.Debug("CloudEvent emitted", "id", event.ID(), "type", event.Type())
	}

	// Emit another CloudEvent for comparison
	heartbeatEvent := modular.NewCloudEvent(
		"com.example.demo.heartbeat",
		"cloudevents-demo",
		map[string]interface{}{"counter": counter, "demo": true},
		map[string]interface{}{"demo_type": "heartbeat"},
	)

	if err := observableApp.NotifyObservers(ctx, heartbeatEvent); err != nil {
		m.logger.Error("Failed to emit heartbeat event", "error", err)
	}
}

// RegisterObservers implements ObservableModule to register for events.
func (m *CloudEventsModule) RegisterObservers(subject modular.Subject) error {
	// Register to receive all events for demonstration
	return fmt.Errorf("failed to register observer: %w", subject.RegisterObserver(m))
}

// EmitEvent implements ObservableModule for CloudEvents.
func (m *CloudEventsModule) EmitEvent(ctx context.Context, event cloudevents.Event) error {
	if observableApp, ok := m.app.(*modular.ObservableApplication); ok {
		return fmt.Errorf("failed to notify observers: %w", observableApp.NotifyObservers(ctx, event))
	}
	return errApplicationDoesNotSupportCloudEvents
}

// OnEvent implements Observer interface to receive CloudEvents.
func (m *CloudEventsModule) OnEvent(ctx context.Context, event cloudevents.Event) error {
	// Only log certain events to avoid noise
	if event.Type() == modular.EventTypeApplicationStarted || event.Type() == modular.EventTypeApplicationStopped {
		m.logger.Info("Received CloudEvent", "type", event.Type(), "source", event.Source(), "id", event.ID())
	}
	return nil
}

// ObserverID returns the observer identifier.
func (m *CloudEventsModule) ObserverID() string {
	return m.name + "-observer"
}
