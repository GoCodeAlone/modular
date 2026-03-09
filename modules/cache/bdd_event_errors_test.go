package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/GoCodeAlone/modular"
)

// Event observation BDD test steps for error scenarios

func (ctx *CacheBDDTestContext) theCacheEngineEncountersAConnectionError() error {
	// Set up a Redis configuration that will actually fail to connect
	// This uses an invalid URL that will trigger a real connection error
	ctx.cacheConfig = &CacheConfig{
		Engine:          "redis",
		DefaultTTL:      300 * time.Second,
		CleanupInterval: 60 * time.Second,
		RedisURL:        "redis://localhost:99999", // Invalid port to trigger real error
		RedisDB:         0,
	}
	return nil
}

func (ctx *CacheBDDTestContext) iAttemptToStartTheCacheModule() error {
	// Create a new module with the error-prone configuration
	module := &CacheModule{}
	config := ctx.cacheConfig
	if config == nil {
		config = &CacheConfig{
			Engine:          "redis",
			DefaultTTL:      300 * time.Second,
			CleanupInterval: 60 * time.Second,
			RedisURL:        "redis://invalid-host:6379",
			RedisDB:         0,
		}
	}

	module.config = config
	module.logger = &testLogger{}

	// Initialize the cache engine
	switch module.config.Engine {
	case "memory":
		module.cacheEngine = NewMemoryCache(module.config)
	case "redis":
		module.cacheEngine = NewRedisCache(module.config)
	default:
		module.cacheEngine = NewMemoryCache(module.config)
	}

	// Set up event observer
	if ctx.eventObserver == nil {
		ctx.eventObserver = newTestEventObserver()
	}

	// Register observer with module if we have an app context
	if ctx.app != nil {
		if err := ctx.app.(modular.Subject).RegisterObserver(ctx.eventObserver); err != nil {
			return fmt.Errorf("failed to register event observer: %w", err)
		}
		// Set up the module as an observable that can emit events
		if err := module.RegisterObservers(ctx.app.(modular.Subject)); err != nil {
			return fmt.Errorf("failed to register module observers: %w", err)
		}
	}

	// Try to start - this should fail and emit error event for invalid Redis URL
	ctx.lastError = module.Start(context.Background())
	return nil // Don't return the error, just capture it
}

func (ctx *CacheBDDTestContext) aCacheErrorEventShouldBeEmitted() error {
	time.Sleep(100 * time.Millisecond) // Give time for async event emission

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeCacheError {
			return nil
		}
	}

	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	return fmt.Errorf("cache error event not found. Captured events: %v", eventTypes)
}

func (ctx *CacheBDDTestContext) theErrorEventShouldContainConnectionErrorDetails() error {
	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeCacheError {
			// Check if the event data contains error information
			data := event.Data()
			if data != nil {
				// Parse the JSON data
				var eventData map[string]interface{}
				if err := event.DataAs(&eventData); err == nil {
					// Look for error-related fields
					if errorMsg, hasError := eventData["error"]; hasError {
						if operation, hasOp := eventData["operation"]; hasOp {
							// Validate that it's actually a connection-related error
							if _, ok := errorMsg.(string); ok {
								if opStr, ok := operation.(string); ok {
									// Check if this looks like a connection error
									if opStr == "connect" || opStr == "start" {
										return nil
									}
								}
							}
						}
					}
				}
			}
		}
	}
	return fmt.Errorf("error event does not contain proper connection error details (error, operation)")
}
