package cache

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/GoCodeAlone/modular"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// Cache BDD Test Context
type CacheBDDTestContext struct {
	app            modular.Application
	module         *CacheModule
	service        *CacheModule
	cacheConfig    *CacheConfig
	lastError      error
	cachedValue    interface{}
	cacheHit       bool
	multipleItems  map[string]interface{}
	multipleResult map[string]interface{}
	capturedEvents []cloudevents.Event
	eventObserver  *testEventObserver
}

// testEventObserver captures events for testing
type testEventObserver struct {
	events []cloudevents.Event
	id     string
	mu     *sync.Mutex
}

func newTestEventObserver() *testEventObserver {
	return &testEventObserver{
		id: "test-observer-cache",
		mu: &sync.Mutex{},
	}
}

func (o *testEventObserver) OnEvent(ctx context.Context, event cloudevents.Event) error {
	o.mu.Lock()
	o.events = append(o.events, event)
	o.mu.Unlock()
	return nil
}

func (o *testEventObserver) ObserverID() string {
	return o.id
}

func (o *testEventObserver) GetEvents() []cloudevents.Event {
	o.mu.Lock()
	defer o.mu.Unlock()
	// Return a copy to avoid race with concurrent appends
	out := make([]cloudevents.Event, len(o.events))
	copy(out, o.events)
	return out
}

func (o *testEventObserver) ClearEvents() {
	o.mu.Lock()
	o.events = nil
	o.mu.Unlock()
}

func (ctx *CacheBDDTestContext) resetContext() {
	ctx.app = nil
	ctx.module = nil
	ctx.service = nil
	ctx.cacheConfig = nil
	ctx.lastError = nil
	ctx.cachedValue = nil
	ctx.cacheHit = false
	ctx.multipleItems = make(map[string]interface{})
	ctx.multipleResult = make(map[string]interface{})
	ctx.capturedEvents = nil
	ctx.eventObserver = newTestEventObserver()
}

func (ctx *CacheBDDTestContext) iHaveAModularApplicationWithCacheModuleConfigured() error {
	ctx.resetContext()

	// Create application with cache config
	logger := &testLogger{}

	// Create basic cache configuration for testing
	ctx.cacheConfig = &CacheConfig{
		Engine:          "memory",
		DefaultTTL:      300 * time.Second,
		CleanupInterval: 60 * time.Second,
		MaxItems:        1000,
	}

	// Create provider with the cache config
	cacheConfigProvider := modular.NewStdConfigProvider(ctx.cacheConfig)

	// Create app with empty main config
	mainConfigProvider := modular.NewStdConfigProvider(struct{}{})
	ctx.app = modular.NewObservableApplication(mainConfigProvider, logger)

	// Create and register cache module
	ctx.module = NewModule().(*CacheModule)

	// Register the cache config section first
	ctx.app.RegisterConfigSection("cache", cacheConfigProvider)

	// Register the module
	ctx.app.RegisterModule(ctx.module)

	// Initialize
	if err := ctx.app.Init(); err != nil {
		return fmt.Errorf("failed to initialize app: %v", err)
	}

	return nil
}

func (ctx *CacheBDDTestContext) theCacheModuleIsInitialized() error {
	// Module should already be initialized in the background step
	return nil
}

func (ctx *CacheBDDTestContext) theCacheServiceShouldBeAvailable() error {
	var cacheService *CacheModule
	if err := ctx.app.GetService("cache.provider", &cacheService); err != nil {
		return fmt.Errorf("failed to get cache service: %v", err)
	}

	ctx.service = cacheService
	return nil
}

func (ctx *CacheBDDTestContext) iHaveACacheServiceAvailable() error {
	if ctx.service == nil {
		var cacheService *CacheModule
		if err := ctx.app.GetService("cache.provider", &cacheService); err != nil {
			return fmt.Errorf("failed to get cache service: %v", err)
		}
		ctx.service = cacheService
	}
	return nil
}

// Test logger for BDD tests
type testLogger struct{}

func (l *testLogger) Debug(msg string, keysAndValues ...interface{}) {}
func (l *testLogger) Info(msg string, keysAndValues ...interface{})  {}
func (l *testLogger) Warn(msg string, keysAndValues ...interface{})  {}
func (l *testLogger) Error(msg string, keysAndValues ...interface{}) {}

// Event validation step - ensures all registered events are emitted during testing
func (ctx *CacheBDDTestContext) allRegisteredEventsShouldBeEmittedDuringTesting() error {
	// Get all registered event types from the module
	registeredEvents := ctx.module.GetRegisteredEventTypes()

	// Create event validation observer
	validator := modular.NewEventValidationObserver("event-validator", registeredEvents)
	_ = validator // Use validator to avoid unused variable error

	// Check which events were emitted during testing
	emittedEvents := make(map[string]bool)
	for _, event := range ctx.eventObserver.GetEvents() {
		emittedEvents[event.Type()] = true
	}

	// Check for missing events (skip events that are non-deterministic or covered in heavy scenarios)
	var missingEvents []string
	for _, eventType := range registeredEvents {
		if eventType == EventTypeCacheExpired || eventType == EventTypeCacheEvicted || eventType == EventTypeCacheError {
			continue
		}
		if !emittedEvents[eventType] {
			missingEvents = append(missingEvents, eventType)
		}
	}

	if len(missingEvents) > 0 {
		return fmt.Errorf("the following registered events were not emitted during testing: %v", missingEvents)
	}

	return nil
}
