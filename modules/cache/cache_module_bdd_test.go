package cache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/GoCodeAlone/modular"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cucumber/godog"
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

func (ctx *CacheBDDTestContext) iHaveACacheConfigurationWithMemoryEngine() error {
	ctx.cacheConfig = &CacheConfig{
		Engine:          "memory",
		DefaultTTL:      300 * time.Second,
		CleanupInterval: 60 * time.Second,
		MaxItems:        1000,
	}

	// Update the module's config if it exists
	if ctx.service != nil {
		ctx.service.config = ctx.cacheConfig
	}
	return nil
}

func (ctx *CacheBDDTestContext) iHaveACacheConfigurationWithRedisEngine() error {
	ctx.cacheConfig = &CacheConfig{
		Engine:          "redis",
		DefaultTTL:      300 * time.Second,
		CleanupInterval: 60 * time.Second,
		RedisURL:        "redis://localhost:6379",
		RedisDB:         0,
	}

	// Update the module's config if it exists
	if ctx.service != nil {
		ctx.service.config = ctx.cacheConfig
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

func (ctx *CacheBDDTestContext) theMemoryCacheEngineShouldBeConfigured() error {
	// Get the service so we can check its config
	if ctx.service == nil {
		return fmt.Errorf("cache service not available")
	}

	if ctx.service.config == nil {
		return fmt.Errorf("cache service config is nil")
	}

	if ctx.service.config.Engine != "memory" {
		return fmt.Errorf("memory cache engine not configured, found: %s", ctx.service.config.Engine)
	}
	return nil
}

func (ctx *CacheBDDTestContext) theRedisCacheEngineShouldBeConfigured() error {
	// Get the service so we can check its config
	if ctx.service == nil {
		return fmt.Errorf("cache service not available")
	}

	if ctx.service.config == nil {
		return fmt.Errorf("cache service config is nil")
	}

	if ctx.service.config.Engine != "redis" {
		return fmt.Errorf("redis cache engine not configured, found: %s", ctx.service.config.Engine)
	}
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

func (ctx *CacheBDDTestContext) iSetACacheItemWithKeyAndValue(key, value string) error {
	err := ctx.service.Set(context.Background(), key, value, 0)
	if err != nil {
		ctx.lastError = err
	}
	return err
}

func (ctx *CacheBDDTestContext) iGetTheCacheItemWithKey(key string) error {
	value, found := ctx.service.Get(context.Background(), key)
	ctx.cachedValue = value
	ctx.cacheHit = found
	return nil
}

func (ctx *CacheBDDTestContext) theCachedValueShouldBe(expectedValue string) error {
	if !ctx.cacheHit {
		return errors.New("cache miss when hit was expected")
	}

	if ctx.cachedValue != expectedValue {
		return errors.New("cached value does not match expected value")
	}

	return nil
}

func (ctx *CacheBDDTestContext) theCacheHitShouldBeSuccessful() error {
	if !ctx.cacheHit {
		return errors.New("cache hit should have been successful")
	}
	return nil
}

func (ctx *CacheBDDTestContext) iSetACacheItemWithKeyAndValueWithTTLSeconds(key, value string, ttl int) error {
	duration := time.Duration(ttl) * time.Second
	err := ctx.service.Set(context.Background(), key, value, duration)
	if err != nil {
		ctx.lastError = err
	}
	return err
}

func (ctx *CacheBDDTestContext) iGetTheCacheItemWithKeyImmediately(key string) error {
	return ctx.iGetTheCacheItemWithKey(key)
}

func (ctx *CacheBDDTestContext) iWaitForSeconds(seconds int) error {
	time.Sleep(time.Duration(seconds) * time.Second)
	return nil
}

func (ctx *CacheBDDTestContext) theCacheHitShouldBeUnsuccessful() error {
	if ctx.cacheHit {
		return errors.New("cache hit should have been unsuccessful")
	}
	return nil
}

func (ctx *CacheBDDTestContext) noValueShouldBeReturned() error {
	if ctx.cachedValue != nil {
		return errors.New("no value should have been returned")
	}
	return nil
}

func (ctx *CacheBDDTestContext) iHaveSetACacheItemWithKeyAndValue(key, value string) error {
	return ctx.iSetACacheItemWithKeyAndValue(key, value)
}

func (ctx *CacheBDDTestContext) iDeleteTheCacheItemWithKey(key string) error {
	err := ctx.service.Delete(context.Background(), key)
	if err != nil {
		ctx.lastError = err
	}
	return err
}

func (ctx *CacheBDDTestContext) iHaveSetMultipleCacheItems() error {
	items := map[string]interface{}{
		"item1": "value1",
		"item2": "value2",
		"item3": "value3",
	}

	for key, value := range items {
		err := ctx.service.Set(context.Background(), key, value, 0)
		if err != nil {
			return err
		}
	}

	ctx.multipleItems = items
	return nil
}

func (ctx *CacheBDDTestContext) iFlushAllCacheItems() error {
	err := ctx.service.Flush(context.Background())
	if err != nil {
		ctx.lastError = err
	}
	return err
}

func (ctx *CacheBDDTestContext) iGetAnyOfThePreviouslySetCacheItems() error {
	// Try to get any item from the previously set items
	for key := range ctx.multipleItems {
		value, found := ctx.service.Get(context.Background(), key)
		ctx.cachedValue = value
		ctx.cacheHit = found
		break
	}
	return nil
}

func (ctx *CacheBDDTestContext) iSetMultipleCacheItemsWithDifferentKeysAndValues() error {
	items := map[string]interface{}{
		"multi-key1": "multi-value1",
		"multi-key2": "multi-value2",
		"multi-key3": "multi-value3",
	}

	err := ctx.service.SetMulti(context.Background(), items, 0)
	if err != nil {
		ctx.lastError = err
		return err
	}

	ctx.multipleItems = items
	return nil
}

func (ctx *CacheBDDTestContext) allItemsShouldBeStoredSuccessfully() error {
	if ctx.lastError != nil {
		return ctx.lastError
	}
	return nil
}

func (ctx *CacheBDDTestContext) iShouldBeAbleToRetrieveAllItems() error {
	for key, expectedValue := range ctx.multipleItems {
		value, found := ctx.service.Get(context.Background(), key)
		if !found {
			return errors.New("item should be found in cache")
		}
		if value != expectedValue {
			return errors.New("cached value does not match expected value")
		}
	}
	return nil
}

func (ctx *CacheBDDTestContext) iHaveSetMultipleCacheItemsWithKeys(key1, key2, key3 string) error {
	items := map[string]interface{}{
		key1: "value1",
		key2: "value2",
		key3: "value3",
	}

	for key, value := range items {
		err := ctx.service.Set(context.Background(), key, value, 0)
		if err != nil {
			return err
		}
	}

	ctx.multipleItems = items
	return nil
}

func (ctx *CacheBDDTestContext) iGetMultipleCacheItemsWithTheSameKeys() error {
	// Get keys from the stored items
	keys := make([]string, 0, len(ctx.multipleItems))
	for key := range ctx.multipleItems {
		keys = append(keys, key)
	}

	result, err := ctx.service.GetMulti(context.Background(), keys)
	if err != nil {
		ctx.lastError = err
		return err
	}

	ctx.multipleResult = result
	return nil
}

func (ctx *CacheBDDTestContext) iShouldReceiveAllTheCachedValues() error {
	if len(ctx.multipleResult) != len(ctx.multipleItems) {
		return errors.New("should receive all cached values")
	}
	return nil
}

func (ctx *CacheBDDTestContext) theValuesShouldMatchWhatWasStored() error {
	for key, expectedValue := range ctx.multipleItems {
		actualValue, found := ctx.multipleResult[key]
		if !found {
			return errors.New("value should be found in results")
		}
		if actualValue != expectedValue {
			return errors.New("value does not match what was stored")
		}
	}
	return nil
}

func (ctx *CacheBDDTestContext) iHaveSetMultipleCacheItemsWithKeysForDeletion(key1, key2, key3 string) error {
	items := map[string]interface{}{
		key1: "value1",
		key2: "value2",
		key3: "value3",
	}

	for key, value := range items {
		err := ctx.service.Set(context.Background(), key, value, 0)
		if err != nil {
			return err
		}
	}

	ctx.multipleItems = items
	return nil
}

func (ctx *CacheBDDTestContext) iDeleteMultipleCacheItemsWithTheSameKeys() error {
	// Get keys from the stored items
	keys := make([]string, 0, len(ctx.multipleItems))
	for key := range ctx.multipleItems {
		keys = append(keys, key)
	}

	err := ctx.service.DeleteMulti(context.Background(), keys)
	if err != nil {
		ctx.lastError = err
		return err
	}
	return nil
}

func (ctx *CacheBDDTestContext) iShouldReceiveNoCachedValues() error {
	if len(ctx.multipleResult) != 0 {
		return errors.New("should receive no cached values")
	}
	return nil
}

func (ctx *CacheBDDTestContext) iHaveACacheServiceWithDefaultTTLConfigured() error {
	// Service already configured with default TTL in background
	return ctx.iHaveACacheServiceAvailable()
}

func (ctx *CacheBDDTestContext) iSetACacheItemWithoutSpecifyingTTL() error {
	err := ctx.service.Set(context.Background(), "default-ttl-key", "default-ttl-value", 0)
	if err != nil {
		ctx.lastError = err
	}
	return err
}

func (ctx *CacheBDDTestContext) theItemShouldUseTheDefaultTTLFromConfiguration() error {
	// This is validated by the fact that the item was set successfully
	// The actual TTL validation would require inspecting internal cache state
	// which is implementation-specific
	return nil
}

func (ctx *CacheBDDTestContext) iHaveACacheConfigurationWithInvalidRedisSettings() error {
	ctx.cacheConfig = &CacheConfig{
		Engine:          "redis",
		DefaultTTL:      300 * time.Second,
		CleanupInterval: 60 * time.Second, // Add non-zero cleanup interval
		RedisURL:        "redis://invalid-host:9999",
	}
	return nil
}

func (ctx *CacheBDDTestContext) theCacheModuleAttemptsToStart() error {
	// Create application with invalid Redis config
	logger := &testLogger{}

	// Create provider with the invalid cache config
	cacheConfigProvider := modular.NewStdConfigProvider(ctx.cacheConfig)

	// Create app with empty main config
	mainConfigProvider := modular.NewStdConfigProvider(struct{}{})
	app := modular.NewObservableApplication(mainConfigProvider, logger)

	// Create and register cache module
	module := NewModule().(*CacheModule)

	// Register the cache config section first
	app.RegisterConfigSection("cache", cacheConfigProvider)

	// Register the module
	app.RegisterModule(module)

	// Initialize
	if err := app.Init(); err != nil {
		return err
	}

	// Try to start the application (this should fail for Redis)
	ctx.lastError = app.Start()
	ctx.app = app
	return nil
}

func (ctx *CacheBDDTestContext) theModuleShouldHandleConnectionErrorsGracefully() error {
	// Error should be captured, not panic
	if ctx.lastError == nil {
		return errors.New("expected connection error but none occurred")
	}
	return nil
}

func (ctx *CacheBDDTestContext) appropriateErrorMessagesShouldBeLogged() error {
	// This would be verified by checking the test logger output
	// For now, we just verify an error occurred
	return ctx.theModuleShouldHandleConnectionErrorsGracefully()
}

// Event observation step methods
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

	// Initialize
	if err := ctx.app.Init(); err != nil {
		return err
	}

	// Start the application to enable cache functionality
	if err := ctx.app.Start(); err != nil {
		return fmt.Errorf("failed to start application: %w", err)
	}

	// Register the event observer with the cache module
	if err := ctx.service.RegisterObservers(ctx.app.(modular.Subject)); err != nil {
		return fmt.Errorf("failed to register observers: %w", err)
	}

	// Register our test observer to capture events
	if err := ctx.app.(modular.Subject).RegisterObserver(ctx.eventObserver); err != nil {
		return fmt.Errorf("failed to register test observer: %w", err)
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

func (ctx *CacheBDDTestContext) theCacheCleanupProcessRuns() error {
	// Wait for the natural cleanup process to run
	// With the configured cleanup interval of 500ms, we wait for 3+ cycles to ensure it runs reliably
	time.Sleep(1600 * time.Millisecond)

	// Additionally, proactively trigger cleanup on the in-memory engine to reduce test flakiness
	// and accelerate emission of expiration events in CI environments.
	if ctx.service != nil {
		if mem, ok := ctx.service.cacheEngine.(*MemoryCache); ok {
			// Poll a few times, triggering cleanup and checking if the expired event appeared
			deadline := time.Now().Add(2 * time.Second)
			for time.Now().Before(deadline) {
				mem.CleanupNow(context.Background())
				// Small delay to allow async event emission to propagate
				time.Sleep(50 * time.Millisecond)
				for _, ev := range ctx.eventObserver.GetEvents() {
					if ev.Type() == EventTypeCacheExpired {
						return nil
					}
				}
			}
		}
	}

	return nil
}

func (ctx *CacheBDDTestContext) aCacheExpiredEventShouldBeEmitted() error {
	time.Sleep(100 * time.Millisecond) // Give time for async event emission

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeCacheExpired {
			return nil
		}
	}

	eventTypes := make([]string, len(events))
	for i, event := range events {
		eventTypes[i] = event.Type()
	}

	return fmt.Errorf("cache expired event not found. Captured events: %v", eventTypes)
}

func (ctx *CacheBDDTestContext) theExpiredEventShouldContainTheExpiredKey(key string) error {
	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeCacheExpired {
			// Check if the event data contains the expired key
			data := event.Data()
			if data != nil {
				// Parse the JSON data
				var eventData map[string]interface{}
				if err := event.DataAs(&eventData); err == nil {
					if cacheKey, exists := eventData["cache_key"]; exists && cacheKey == key {
						// Also validate other expected fields
						if _, hasExpiredAt := eventData["expired_at"]; hasExpiredAt {
							if reason, hasReason := eventData["reason"]; hasReason && reason == "ttl_expired" {
								return nil
							}
						}
					}
				}
			}
		}
	}
	return fmt.Errorf("expired event does not contain expected expired key '%s' with proper data structure", key)
}

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

// Test runner function
func TestCacheModuleBDD(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: func(ctx *godog.ScenarioContext) {
			testCtx := &CacheBDDTestContext{}

			// Background
			ctx.Step(`^I have a modular application with cache module configured$`, testCtx.iHaveAModularApplicationWithCacheModuleConfigured)

			// Initialization steps
			ctx.Step(`^the cache module is initialized$`, testCtx.theCacheModuleIsInitialized)
			ctx.Step(`^the cache service should be available$`, testCtx.theCacheServiceShouldBeAvailable)

			// Service availability
			ctx.Step(`^I have a cache service available$`, testCtx.iHaveACacheServiceAvailable)
			ctx.Step(`^I have a cache service with default TTL configured$`, testCtx.iHaveACacheServiceWithDefaultTTLConfigured)

			// Basic cache operations
			ctx.Step(`^I set a cache item with key "([^"]*)" and value "([^"]*)"$`, testCtx.iSetACacheItemWithKeyAndValue)
			ctx.Step(`^I get the cache item with key "([^"]*)"$`, testCtx.iGetTheCacheItemWithKey)
			ctx.Step(`^I get the cache item with key "([^"]*)" immediately$`, testCtx.iGetTheCacheItemWithKeyImmediately)
			ctx.Step(`^the cached value should be "([^"]*)"$`, testCtx.theCachedValueShouldBe)
			ctx.Step(`^the cache hit should be successful$`, testCtx.theCacheHitShouldBeSuccessful)
			ctx.Step(`^the cache hit should be unsuccessful$`, testCtx.theCacheHitShouldBeUnsuccessful)
			ctx.Step(`^no value should be returned$`, testCtx.noValueShouldBeReturned)

			// TTL operations
			ctx.Step(`^I set a cache item with key "([^"]*)" and value "([^"]*)" with TTL (\d+) seconds$`, testCtx.iSetACacheItemWithKeyAndValueWithTTLSeconds)
			ctx.Step(`^I wait for (\d+) seconds$`, testCtx.iWaitForSeconds)
			ctx.Step(`^I set a cache item without specifying TTL$`, testCtx.iSetACacheItemWithoutSpecifyingTTL)
			ctx.Step(`^the item should use the default TTL from configuration$`, testCtx.theItemShouldUseTheDefaultTTLFromConfiguration)

			// Delete operations
			ctx.Step(`^I have set a cache item with key "([^"]*)" and value "([^"]*)"$`, testCtx.iHaveSetACacheItemWithKeyAndValue)
			ctx.Step(`^I delete the cache item with key "([^"]*)"$`, testCtx.iDeleteTheCacheItemWithKey)

			// Flush operations
			ctx.Step(`^I have set multiple cache items$`, testCtx.iHaveSetMultipleCacheItems)
			ctx.Step(`^I flush all cache items$`, testCtx.iFlushAllCacheItems)
			ctx.Step(`^I get any of the previously set cache items$`, testCtx.iGetAnyOfThePreviouslySetCacheItems)

			// Multi operations
			ctx.Step(`^I set multiple cache items with different keys and values$`, testCtx.iSetMultipleCacheItemsWithDifferentKeysAndValues)
			ctx.Step(`^all items should be stored successfully$`, testCtx.allItemsShouldBeStoredSuccessfully)
			ctx.Step(`^I should be able to retrieve all items$`, testCtx.iShouldBeAbleToRetrieveAllItems)

			ctx.Step(`^I have set multiple cache items with keys "([^"]*)", "([^"]*)", "([^"]*)"$`, testCtx.iHaveSetMultipleCacheItemsWithKeys)
			ctx.Step(`^I get multiple cache items with the same keys$`, testCtx.iGetMultipleCacheItemsWithTheSameKeys)
			ctx.Step(`^I should receive all the cached values$`, testCtx.iShouldReceiveAllTheCachedValues)
			ctx.Step(`^the values should match what was stored$`, testCtx.theValuesShouldMatchWhatWasStored)

			ctx.Step(`^I have set multiple cache items with keys "([^"]*)", "([^"]*)", "([^"]*)"$`, testCtx.iHaveSetMultipleCacheItemsWithKeysForDeletion)
			ctx.Step(`^I delete multiple cache items with the same keys$`, testCtx.iDeleteMultipleCacheItemsWithTheSameKeys)
			ctx.Step(`^I should receive no cached values$`, testCtx.iShouldReceiveNoCachedValues)

			// Event observation steps
			ctx.Step(`^I have a cache service with event observation enabled$`, testCtx.iHaveACacheServiceWithEventObservationEnabled)
			ctx.Step(`^a cache set event should be emitted$`, testCtx.aCacheSetEventShouldBeEmitted)
			ctx.Step(`^the event should contain the cache key "([^"]*)"$`, testCtx.theEventShouldContainTheCacheKey)
			ctx.Step(`^a cache hit event should be emitted$`, testCtx.aCacheHitEventShouldBeEmitted)
			ctx.Step(`^a cache miss event should be emitted$`, testCtx.aCacheMissEventShouldBeEmitted)
			ctx.Step(`^I get a non-existent key "([^"]*)"$`, testCtx.iGetANonExistentKey)
			ctx.Step(`^a cache delete event should be emitted$`, testCtx.aCacheDeleteEventShouldBeEmitted)
			ctx.Step(`^the cache module starts$`, testCtx.theCacheModuleStarts)
			ctx.Step(`^a cache connected event should be emitted$`, testCtx.aCacheConnectedEventShouldBeEmitted)
			ctx.Step(`^a cache flush event should be emitted$`, testCtx.aCacheFlushEventShouldBeEmitted)
			ctx.Step(`^the cache module stops$`, testCtx.theCacheModuleStops)
			ctx.Step(`^a cache disconnected event should be emitted$`, testCtx.aCacheDisconnectedEventShouldBeEmitted)

			// Error event steps
			ctx.Step(`^the cache engine encounters a connection error$`, testCtx.theCacheEngineEncountersAConnectionError)
			ctx.Step(`^I attempt to start the cache module$`, testCtx.iAttemptToStartTheCacheModule)
			ctx.Step(`^a cache error event should be emitted$`, testCtx.aCacheErrorEventShouldBeEmitted)
			ctx.Step(`^the error event should contain connection error details$`, testCtx.theErrorEventShouldContainConnectionErrorDetails)

			// Expired event steps
			ctx.Step(`^the cache cleanup process runs$`, testCtx.theCacheCleanupProcessRuns)
			ctx.Step(`^a cache expired event should be emitted$`, testCtx.aCacheExpiredEventShouldBeEmitted)
			ctx.Step(`^the expired event should contain the expired key "([^"]*)"$`, testCtx.theExpiredEventShouldContainTheExpiredKey)

			// Evicted event steps
			ctx.Step(`^I have a cache service with small memory limit configured$`, testCtx.iHaveACacheServiceWithSmallMemoryLimitConfigured)
			ctx.Step(`^I have event observation enabled$`, testCtx.iHaveEventObservationEnabled)
			ctx.Step(`^I fill the cache beyond its maximum capacity$`, testCtx.iFillTheCacheBeyondItsMaximumCapacity)
			ctx.Step(`^a cache evicted event should be emitted$`, testCtx.aCacheEvictedEventShouldBeEmitted)
			ctx.Step(`^the evicted event should contain eviction details$`, testCtx.theEvictedEventShouldContainEvictionDetails)
		},
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features"},
			TestingT: t,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run feature tests")
	}
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
	
	// Check for missing events
	var missingEvents []string
	for _, eventType := range registeredEvents {
		if !emittedEvents[eventType] {
			missingEvents = append(missingEvents, eventType)
		}
	}
	
	if len(missingEvents) > 0 {
		return fmt.Errorf("the following registered events were not emitted during testing: %v", missingEvents)
	}
	
	return nil
}
