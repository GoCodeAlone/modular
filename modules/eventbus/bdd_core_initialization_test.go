package eventbus

import (
	"context"
	"fmt"
	"time"

	"github.com/GoCodeAlone/modular"
)

// ==============================================================================
// CORE MODULE INITIALIZATION
// ==============================================================================
// This file handles basic module setup, initialization, and core event
// publishing/subscribing functionality.

func (ctx *EventBusBDDTestContext) iHaveAModularApplicationWithEventbusModuleConfigured() error {
	ctx.resetContext()

	// Create application with eventbus config
	logger := &testLogger{}

	// Apply per-app empty feeders instead of mutating global modular.ConfigFeeders

	// Create basic eventbus configuration for testing
	ctx.eventbusConfig = &EventBusConfig{
		Engine:                 "memory",
		MaxEventQueueSize:      1000,
		DefaultEventBufferSize: 10,
		WorkerCount:            5,
		EventTTL:               3600,
		RetentionDays:          7,
	}

	// Create provider with the eventbus config
	eventbusConfigProvider := modular.NewStdConfigProvider(ctx.eventbusConfig)

	// Create app with empty main config
	mainConfigProvider := modular.NewStdConfigProvider(struct{}{})
	ctx.app = modular.NewObservableApplication(mainConfigProvider, logger)
	if cfSetter, ok := ctx.app.(interface{ SetConfigFeeders([]modular.Feeder) }); ok {
		cfSetter.SetConfigFeeders([]modular.Feeder{})
	}

	// Create and register eventbus module
	ctx.module = NewModule().(*EventBusModule)

	// Register the eventbus config section with custom config BEFORE registering module
	// This ensures the module gets the correct config during Init()
	ctx.app.RegisterConfigSection("eventbus", eventbusConfigProvider)

	// Register the module after config section
	ctx.app.RegisterModule(ctx.module)

	return nil
}

// Event observation setup method
func (ctx *EventBusBDDTestContext) iHaveAnEventbusServiceWithEventObservationEnabled() error {
	ctx.resetContext()

	// Create application with eventbus config
	logger := &testLogger{}

	// Apply per-app empty feeders instead of mutating global modular.ConfigFeeders

	// Create basic eventbus configuration for testing
	ctx.eventbusConfig = &EventBusConfig{
		Engine:                 "memory",
		MaxEventQueueSize:      1000,
		DefaultEventBufferSize: 10,
		WorkerCount:            5,
		EventTTL:               3600,
		RetentionDays:          7,
	}

	// Create provider with the eventbus config
	eventbusConfigProvider := modular.NewStdConfigProvider(ctx.eventbusConfig)

	// Create app with empty main config - USE OBSERVABLE for events
	mainConfigProvider := modular.NewStdConfigProvider(struct{}{})
	ctx.app = modular.NewObservableApplication(mainConfigProvider, logger)
	if cfSetter, ok := ctx.app.(interface{ SetConfigFeeders([]modular.Feeder) }); ok {
		cfSetter.SetConfigFeeders([]modular.Feeder{})
	}

	// Create and register eventbus module
	ctx.module = NewModule().(*EventBusModule)

	// Create test event observer
	ctx.eventObserver = newTestEventObserver()

	// Register the eventbus config section with custom config BEFORE registering module
	ctx.app.RegisterConfigSection("eventbus", eventbusConfigProvider)

	// Register the module after config section
	ctx.app.RegisterModule(ctx.module)

	// Register observers AFTER module registration
	if err := ctx.module.RegisterObservers(ctx.app.(modular.Subject)); err != nil {
		return fmt.Errorf("failed to register observers: %w", err)
	}

	// Register our test observer to capture events
	if err := ctx.app.(modular.Subject).RegisterObserver(ctx.eventObserver); err != nil {
		return fmt.Errorf("failed to register test observer: %w", err)
	}

	// Initialize and start the application
	if err := ctx.app.Init(); err != nil {
		return fmt.Errorf("failed to initialize app: %v", err)
	}

	if err := ctx.app.Start(); err != nil {
		return fmt.Errorf("failed to start app: %v", err)
	}

	// Get the eventbus service
	var service interface{}
	if err := ctx.app.GetService("eventbus", &service); err != nil {
		// Try the provider service as fallback
		var eventbusService *EventBusModule
		if err := ctx.app.GetService("eventbus.provider", &eventbusService); err == nil {
			ctx.service = eventbusService
		} else {
			// Final fallback: use the module directly as the service
			ctx.service = ctx.module
		}
	} else {
		// Cast to EventBusModule
		eventbusService, ok := service.(*EventBusModule)
		if !ok {
			return fmt.Errorf("service is not an EventBusModule, got: %T", service)
		}
		ctx.service = eventbusService
	}
	return nil
}

func (ctx *EventBusBDDTestContext) theEventbusModuleIsInitialized() error {
	// Check if we have a module that wasn't registered with the app (multi-engine case)
	if ctx.module != nil && ctx.app != nil {
		// Check if the config was set directly on the app but module not registered
		// This happens in multi-engine tests to avoid config override issues
		if _, err := ctx.app.GetConfigSection("eventbus"); err == nil {
			// Initialize the module directly using the app's config system
			err := ctx.module.Init(ctx.app)
			if err != nil {
				ctx.lastError = err
				return nil
			}
			ctx.service = ctx.module
			ctx.service.Start(context.Background())
			return nil
		}
	}

	// Standard path: Initialize the application - this will read config and set up the router properly
	err := ctx.app.Init()
	if err != nil {
		ctx.lastError = err
		return nil
	}

	// Get the eventbus service
	var eventbusService *EventBusModule
	if err := ctx.app.GetService("eventbus.provider", &eventbusService); err == nil {
		ctx.service = eventbusService
		// Start the eventbus service
		ctx.service.Start(context.Background())
	} else {
		// Fallback: use the module directly as the service
		ctx.service = ctx.module
		// Start the eventbus service
		ctx.service.Start(context.Background())
	}

	return nil
}

func (ctx *EventBusBDDTestContext) theEventbusServiceShouldBeAvailable() error {
	if ctx.service == nil {
		return fmt.Errorf("eventbus service not available")
	}
	return nil
}

func (ctx *EventBusBDDTestContext) theServiceShouldBeConfiguredWithDefaultSettings() error {
	if ctx.service == nil {
		return fmt.Errorf("eventbus service not available")
	}

	if ctx.service.config == nil {
		return fmt.Errorf("eventbus config not available")
	}

	// Verify basic configuration is present
	if ctx.service.config.Engine == "" {
		return fmt.Errorf("eventbus engine not configured")
	}

	return nil
}

func (ctx *EventBusBDDTestContext) iHaveAnEventbusServiceAvailable() error {
	err := ctx.iHaveAModularApplicationWithEventbusModuleConfigured()
	if err != nil {
		return err
	}

	err = ctx.theEventbusModuleIsInitialized()
	if err != nil {
		return err
	}

	// Make sure the service is started
	if ctx.service != nil {
		ctx.service.Start(context.Background())
	}

	return nil
}

func (ctx *EventBusBDDTestContext) iSubscribeToTopicWithAHandler(topic string) error {
	if ctx.service == nil {
		return fmt.Errorf("eventbus service not available")
	}

	// Create a handler that captures events
	handler := func(handlerCtx context.Context, event Event) error {
		ctx.mutex.Lock()
		defer ctx.mutex.Unlock()
		ctx.receivedEvents = append(ctx.receivedEvents, event)
		return nil
	}

	// Store the handler for later reference
	ctx.eventHandlers[topic] = handler

	// Subscribe to the topic
	subscription, err := ctx.service.Subscribe(context.Background(), topic, handler)
	if err != nil {
		ctx.lastError = err
		return fmt.Errorf("failed to subscribe to topic %s: %v", topic, err)
	}

	ctx.subscriptions[topic] = subscription
	ctx.lastSubscription = subscription

	return nil
}

func (ctx *EventBusBDDTestContext) iPublishAnEventToTopicWithPayload(topic, payload string) error {
	if ctx.service == nil {
		return fmt.Errorf("eventbus service not available")
	}

	err := ctx.service.Publish(context.Background(), topic, payload)
	if err != nil {
		ctx.lastError = err
		return fmt.Errorf("failed to publish event: %v", err)
	}

	// Give more time for event processing
	time.Sleep(500 * time.Millisecond)

	return nil
}

func (ctx *EventBusBDDTestContext) theHandlerShouldReceiveTheEvent() error {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()

	if len(ctx.receivedEvents) == 0 {
		return fmt.Errorf("no events received by handler")
	}

	return nil
}

func (ctx *EventBusBDDTestContext) thePayloadShouldMatch(expectedPayload string) error {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()

	if len(ctx.receivedEvents) == 0 {
		return fmt.Errorf("no events received to check payload")
	}

	lastEvent := ctx.receivedEvents[len(ctx.receivedEvents)-1]
	var actual string
	if err := lastEvent.DataAs(&actual); err != nil {
		// Try raw bytes comparison
		if string(lastEvent.Data()) != expectedPayload {
			return fmt.Errorf("payload mismatch: expected %s, got %s", expectedPayload, string(lastEvent.Data()))
		}
		return nil
	}
	if actual != expectedPayload {
		return fmt.Errorf("payload mismatch: expected %s, got %s", expectedPayload, actual)
	}

	return nil
}
