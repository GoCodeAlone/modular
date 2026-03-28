package eventbus

import (
	"context"
	"fmt"
	"time"

	"github.com/GoCodeAlone/modular"
)

// ==============================================================================
// MULTI-ENGINE SCENARIOS
// ==============================================================================
// This file handles multi-engine configuration, routing, custom engines,
// and engine-specific behaviors and error handling.

// Multi-engine scenario implementations

func (ctx *EventBusBDDTestContext) iHaveAMultiEngineEventbusConfiguration() error {
	// Configure with memory and custom engines
	config := &EventBusConfig{
		Engines: []EngineConfig{
			{
				Name: "memory",
				Type: "memory",
				Config: map[string]interface{}{
					"maxEventQueueSize":      500,
					"defaultEventBufferSize": 5,
					"workerCount":            3,
				},
			},
			{
				Name: "custom",
				Type: "custom",
				Config: map[string]interface{}{
					"enableMetrics":     true,
					"maxEventQueueSize": 1000,
				},
			},
		},
		Routing: []RoutingRule{
			{
				Topics: []string{"user.*", "auth.*"},
				Engine: "memory",
			},
			{
				Topics: []string{"*"},
				Engine: "custom",
			},
		},
	}

	// Store config for later use by theEventbusModuleIsInitialized
	ctx.eventbusConfig = config

	// Create and configure application
	logger := &testLogger{}
	mainConfigProvider := modular.NewStdConfigProvider(struct{}{})
	ctx.app = modular.NewObservableApplication(mainConfigProvider, logger)

	// Register config section with custom multi-engine config
	ctx.app.RegisterConfigSection("eventbus", modular.NewStdConfigProvider(config))

	ctx.module = NewModule().(*EventBusModule)
	// Don't call app.RegisterModule to avoid config override
	// theEventbusModuleIsInitialized will handle initialization
	return nil
}

func (ctx *EventBusBDDTestContext) bothEnginesShouldBeAvailable() error {
	if ctx.service == nil || ctx.service.router == nil {
		return fmt.Errorf("eventbus router not initialized")
	}

	engineNames := ctx.service.router.GetEngineNames()
	if len(engineNames) != 2 {
		return fmt.Errorf("expected 2 engines, got %d: %v", len(engineNames), engineNames)
	}

	hasMemory, hasCustom := false, false
	for _, name := range engineNames {
		if name == "memory" {
			hasMemory = true
		} else if name == "custom" {
			hasCustom = true
		}
	}

	if !hasMemory || !hasCustom {
		return fmt.Errorf("expected memory and custom engines, got: %v", engineNames)
	}

	return nil
}

func (ctx *EventBusBDDTestContext) theEngineRouterShouldBeConfiguredCorrectly() error {
	if ctx.service == nil || ctx.service.router == nil {
		return fmt.Errorf("eventbus router not initialized")
	}

	// Test routing for specific topics
	memoryEngine := ctx.service.router.GetEngineForTopic("user.created")
	customEngine := ctx.service.router.GetEngineForTopic("analytics.pageview")

	if memoryEngine != "memory" {
		return fmt.Errorf("expected user.created to route to memory engine, got %s", memoryEngine)
	}

	if customEngine != "custom" {
		return fmt.Errorf("expected analytics.pageview to route to custom engine, got %s", customEngine)
	}

	return nil
}

func (ctx *EventBusBDDTestContext) iHaveAMultiEngineEventbusWithTopicRouting() error {
	// Set up multi-engine configuration
	err := ctx.iHaveAMultiEngineEventbusConfiguration()
	if err != nil {
		return err
	}

	// Initialize the eventbus module
	return ctx.theEventbusModuleIsInitialized()
}

func (ctx *EventBusBDDTestContext) iPublishAnEventToTopic(topic string) error {
	if ctx.service == nil {
		return fmt.Errorf("service not available")
	}

	// Store the topic for routing verification
	if ctx.publishedTopics == nil {
		ctx.publishedTopics = make(map[string]bool)
	}
	ctx.publishedTopics[topic] = true

	// Start the service if not already started
	if !ctx.service.isStarted.Load() {
		err := ctx.service.Start(context.Background())
		if err != nil {
			return fmt.Errorf("failed to start eventbus: %w", err)
		}
	}

	return ctx.service.Publish(context.Background(), topic, fmt.Sprintf("test-payload-%s", topic))
}

func (ctx *EventBusBDDTestContext) topicShouldBeRoutedToMemoryEngine(topic string) error {
	if ctx.service == nil || ctx.service.router == nil {
		return fmt.Errorf("eventbus router not initialized")
	}

	actualEngine := ctx.service.router.GetEngineForTopic(topic)
	if actualEngine != "memory" {
		return fmt.Errorf("expected %s to be routed to memory engine, got %s", topic, actualEngine)
	}

	return nil
}

func (ctx *EventBusBDDTestContext) topicShouldBeRoutedToCustomEngine(topic string) error {
	if ctx.service == nil || ctx.service.router == nil {
		return fmt.Errorf("eventbus router not initialized")
	}

	actualEngine := ctx.service.router.GetEngineForTopic(topic)
	if actualEngine != "custom" {
		return fmt.Errorf("expected %s to be routed to custom engine, got %s", topic, actualEngine)
	}

	return nil
}

func (ctx *EventBusBDDTestContext) iRegisterACustomEngineType(engineType string) error {
	// Register a test engine type
	RegisterEngine(engineType, func(config map[string]interface{}) (EventBus, error) {
		return NewCustomMemoryEventBus(config)
	})
	ctx.customEngineType = engineType
	return nil
}

func (ctx *EventBusBDDTestContext) iConfigureEventbusToUseCustomEngine() error {
	if ctx.customEngineType == "" {
		return fmt.Errorf("custom engine type not registered")
	}

	config := &EventBusConfig{
		Engines: []EngineConfig{
			{
				Name: "testengine",
				Type: ctx.customEngineType,
				Config: map[string]interface{}{
					"enableMetrics": true,
				},
			},
		},
	}

	logger := &testLogger{}
	mainConfigProvider := modular.NewStdConfigProvider(struct{}{})
	app := modular.NewObservableApplication(mainConfigProvider, logger)

	// Register config section with custom engine config
	app.RegisterConfigSection("eventbus", modular.NewStdConfigProvider(config))

	module := NewModule().(*EventBusModule)
	// Initialize module directly to avoid config override from RegisterModule
	err := module.Init(app)
	if err != nil {
		return fmt.Errorf("failed to initialize module: %w", err)
	}

	ctx.service = module
	ctx.app = app
	return nil
}

func (ctx *EventBusBDDTestContext) theCustomEngineShouldBeUsed() error {
	if ctx.service == nil || ctx.service.router == nil {
		return fmt.Errorf("eventbus router not initialized")
	}

	engineNames := ctx.service.router.GetEngineNames()
	if len(engineNames) != 1 || engineNames[0] != "testengine" {
		return fmt.Errorf("expected testengine, got %v", engineNames)
	}

	return nil
}

func (ctx *EventBusBDDTestContext) eventsShouldBeHandledByCustomImplementation() error {
	// Verify that events are processed by the custom engine
	// Start the service and test a simple publish/subscribe
	err := ctx.service.Start(context.Background())
	if err != nil {
		return fmt.Errorf("failed to start eventbus: %w", err)
	}

	received := make(chan bool, 1)
	_, err = ctx.service.Subscribe(context.Background(), "test.topic", func(ctx context.Context, event Event) error {
		// Check if event has custom engine metadata
		if metadata, ok := event.Extensions()["engine"]; ok && metadata == "custom-memory" {
			received <- true
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	err = ctx.service.Publish(context.Background(), "test.topic", "test-data")
	if err != nil {
		return fmt.Errorf("failed to publish: %w", err)
	}

	select {
	case <-received:
		return nil
	case <-time.After(1 * time.Second):
		return fmt.Errorf("event not processed by custom engine")
	}
}
