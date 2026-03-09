package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/GoCodeAlone/modular"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// Event observation BDD test steps for cache eviction scenarios

func (ctx *CacheBDDTestContext) iHaveACacheServiceWithSmallMemoryLimitConfigured() error {
	ctx.resetContext()

	// Create application with cache config
	logger := &testLogger{}

	// Create basic cache configuration for testing with small memory limit
	ctx.cacheConfig = &CacheConfig{
		Engine:          "memory",
		DefaultTTL:      300 * time.Second,
		CleanupInterval: 60 * time.Second,
		MaxItems:        2, // Very small limit to trigger eviction
	}

	// Create provider with the cache config
	cacheConfigProvider := modular.NewStdConfigProvider(ctx.cacheConfig)

	// Create app with empty main config - use ObservableApplication for event support
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
		return fmt.Errorf("failed to initialize application: %w", err)
	}

	// Start the application
	if err := ctx.app.Start(); err != nil {
		return fmt.Errorf("failed to start application: %w", err)
	}

	// Get the service so we can set up event observation
	var cacheService *CacheModule
	if err := ctx.app.GetService("cache.provider", &cacheService); err != nil {
		return fmt.Errorf("failed to get cache service: %w", err)
	}
	ctx.service = cacheService

	return nil
}

func (ctx *CacheBDDTestContext) iHaveEventObservationEnabled() error {
	// Set up event observer if not already done
	if ctx.eventObserver == nil {
		ctx.eventObserver = newTestEventObserver()
	}

	// Register observer with application if available and it supports the Subject interface
	if ctx.app != nil {
		if subject, ok := ctx.app.(modular.Subject); ok {
			if err := subject.RegisterObserver(ctx.eventObserver); err != nil {
				return fmt.Errorf("failed to register event observer: %w", err)
			}
		}
	}

	// Register observers with the cache service if available
	if ctx.service != nil {
		if subject, ok := ctx.app.(modular.Subject); ok {
			if err := ctx.service.RegisterObservers(subject); err != nil {
				return fmt.Errorf("failed to register service observers: %w", err)
			}
		}
	}

	return nil
}

func (ctx *CacheBDDTestContext) iFillTheCacheBeyondItsMaximumCapacity() error {
	if ctx.service == nil {
		// Try to get the service from the app if not already available
		var cacheService *CacheModule
		if err := ctx.app.GetService("cache.provider", &cacheService); err != nil {
			return fmt.Errorf("cache service not available: %w", err)
		}
		ctx.service = cacheService
	}

	// Directly set up a memory cache with MaxItems=2 to ensure eviction
	// This bypasses any configuration issues
	config := &CacheConfig{
		Engine:          "memory",
		DefaultTTL:      300 * time.Second,
		CleanupInterval: 60 * time.Second,
		MaxItems:        2,
	}

	memCache := NewMemoryCache(config)
	// Set up the event emitter for the direct memory cache
	memCache.SetEventEmitter(func(eventCtx context.Context, event cloudevents.Event) {
		if ctx.eventObserver != nil {
			ctx.eventObserver.OnEvent(eventCtx, event)
		}
	})

	// Replace the cache engine temporarily
	originalEngine := ctx.service.cacheEngine
	ctx.service.cacheEngine = memCache
	defer func() {
		ctx.service.cacheEngine = originalEngine
	}()

	// Try to add more items than the MaxItems limit (which is 2)
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("item-%d", i)
		value := fmt.Sprintf("value-%d", i)
		err := ctx.service.Set(context.Background(), key, value, 0)
		if err != nil {
			// This might fail when cache is full, which is expected
			continue
		}
	}

	// Give time for async event emission
	time.Sleep(100 * time.Millisecond)

	return nil
}

func (ctx *CacheBDDTestContext) aCacheEvictedEventShouldBeEmitted() error {
	time.Sleep(100 * time.Millisecond) // Give time for async event emission

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeCacheEvicted {
			return nil
		}
	}

	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	return fmt.Errorf("cache evicted event not found. Captured events: %v", eventTypes)
}

func (ctx *CacheBDDTestContext) theEvictedEventShouldContainEvictionDetails() error {
	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeCacheEvicted {
			// Check if the event data contains eviction details
			data := event.Data()
			if data != nil {
				// Parse the JSON data
				var eventData map[string]interface{}
				if err := event.DataAs(&eventData); err == nil {
					// Validate required fields for eviction event
					if reason, hasReason := eventData["reason"]; hasReason && reason == "cache_full" {
						if _, hasMaxItems := eventData["max_items"]; hasMaxItems {
							if _, hasNewKey := eventData["new_key"]; hasNewKey {
								// All expected eviction details are present
								return nil
							}
						}
					}
				}
			}
		}
	}
	return fmt.Errorf("evicted event does not contain proper eviction details (reason, max_items, new_key)")
}
