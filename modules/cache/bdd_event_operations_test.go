package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/GoCodeAlone/modular"
)

// Event observation BDD test steps for basic operations

func (ctx *CacheBDDTestContext) iHaveACacheServiceWithEventObservationEnabled() error {
	ctx.resetContext()

	// Create application with cache config - use ObservableApplication for event support
	logger := &testLogger{}
	mainConfigProvider := modular.NewStdConfigProvider(struct{}{})
	ctx.app = modular.NewObservableApplication(mainConfigProvider, logger)

	// Create basic cache configuration for testing with shorter cleanup interval
	// for scenarios that might need to test expiration behavior
	ctx.cacheConfig = &CacheConfig{
		Engine:          "memory",
		DefaultTTL:      300 * time.Second,
		CleanupInterval: 500 * time.Millisecond, // Much shorter for testing
		MaxItems:        1000,
	}

	cacheConfigProvider := modular.NewStdConfigProvider(ctx.cacheConfig)

	// Create cache module
	ctx.module = NewModule().(*CacheModule)
	ctx.service = ctx.module

	// Register the cache config section first
	ctx.app.RegisterConfigSection("cache", cacheConfigProvider)

	// Register the module
	ctx.app.RegisterModule(ctx.module)

	// Initialize (module.Init runs, setting up cache engine)
	if err := ctx.app.Init(); err != nil {
		return err
	}

	// Register module observers BEFORE starting so lifecycle events (connected) are captured
	if err := ctx.service.RegisterObservers(ctx.app.(modular.Subject)); err != nil {
		return fmt.Errorf("failed to register observers: %w", err)
	}

	// Register test observer prior to Start to observe startup events
	if err := ctx.app.(modular.Subject).RegisterObserver(ctx.eventObserver); err != nil {
		return fmt.Errorf("failed to register test observer: %w", err)
	}

	// Now start the application (connected event will be emitted asynchronously and captured)
	if err := ctx.app.Start(); err != nil {
		return fmt.Errorf("failed to start application: %w", err)
	}

	return nil
}

func (ctx *CacheBDDTestContext) aCacheSetEventShouldBeEmitted() error {
	time.Sleep(100 * time.Millisecond) // Give time for async event emission

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeCacheSet {
			return nil
		}
	}

	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	return fmt.Errorf("cache set event not found. Captured events: %v", eventTypes)
}

func (ctx *CacheBDDTestContext) theEventShouldContainTheCacheKey(expectedKey string) error {
	time.Sleep(100 * time.Millisecond) // Give time for async event emission

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeCacheSet {
			var eventData map[string]interface{}
			if err := event.DataAs(&eventData); err != nil {
				continue
			}
			if cacheKey, ok := eventData["cache_key"]; ok && cacheKey == expectedKey {
				return nil
			}
		}
	}

	return fmt.Errorf("cache set event with key %s not found", expectedKey)
}

func (ctx *CacheBDDTestContext) aCacheHitEventShouldBeEmitted() error {
	time.Sleep(100 * time.Millisecond) // Give time for async event emission

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeCacheHit {
			return nil
		}
	}

	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	return fmt.Errorf("cache hit event not found. Captured events: %v", eventTypes)
}

func (ctx *CacheBDDTestContext) aCacheMissEventShouldBeEmitted() error {
	time.Sleep(100 * time.Millisecond) // Give time for async event emission

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeCacheMiss {
			return nil
		}
	}

	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	return fmt.Errorf("cache miss event not found. Captured events: %v", eventTypes)
}

func (ctx *CacheBDDTestContext) iGetANonExistentKey(key string) error {
	return ctx.iGetTheCacheItemWithKey(key)
}

func (ctx *CacheBDDTestContext) aCacheDeleteEventShouldBeEmitted() error {
	time.Sleep(100 * time.Millisecond) // Give time for async event emission

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeCacheDelete {
			return nil
		}
	}

	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	return fmt.Errorf("cache delete event not found. Captured events: %v", eventTypes)
}

func (ctx *CacheBDDTestContext) theCacheModuleStarts() error {
	// The module should already be started from the setup
	// This step is just to indicate the lifecycle event
	return nil
}

func (ctx *CacheBDDTestContext) aCacheConnectedEventShouldBeEmitted() error {
	time.Sleep(100 * time.Millisecond) // Give time for async event emission

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeCacheConnected {
			return nil
		}
	}

	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	return fmt.Errorf("cache connected event not found. Captured events: %v", eventTypes)
}

func (ctx *CacheBDDTestContext) aCacheFlushEventShouldBeEmitted() error {
	time.Sleep(100 * time.Millisecond) // Give time for async event emission

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeCacheFlush {
			return nil
		}
	}

	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	return fmt.Errorf("cache flush event not found. Captured events: %v", eventTypes)
}

func (ctx *CacheBDDTestContext) theCacheModuleStops() error {
	// Stop the cache module to trigger disconnected event
	if ctx.service != nil {
		return ctx.service.Stop(context.Background())
	}
	return nil
}

func (ctx *CacheBDDTestContext) aCacheDisconnectedEventShouldBeEmitted() error {
	time.Sleep(100 * time.Millisecond) // Give time for async event emission

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeCacheDisconnected {
			return nil
		}
	}

	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	return fmt.Errorf("cache disconnected event not found. Captured events: %v", eventTypes)
}
