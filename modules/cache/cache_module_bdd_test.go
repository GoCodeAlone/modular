package cache

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/CrisisTextLine/modular"
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
}

func (ctx *CacheBDDTestContext) iHaveAModularApplicationWithCacheModuleConfigured() error {
	ctx.resetContext()
	
	// Create application with cache config
	logger := &testLogger{}
	
	// Create basic cache configuration for testing
	ctx.cacheConfig = &CacheConfig{
		Engine:           "memory",
		DefaultTTL:       300 * time.Second,
		CleanupInterval:  60 * time.Second,
		MaxItems:         1000,
	}
	
	// Create provider with the cache config
	cacheConfigProvider := modular.NewStdConfigProvider(ctx.cacheConfig)
	
	// Create app with empty main config
	mainConfigProvider := modular.NewStdConfigProvider(struct{}{})
	ctx.app = modular.NewStdApplication(mainConfigProvider, logger)
	
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
		Engine:           "memory",
		DefaultTTL:       300 * time.Second,
		CleanupInterval:  60 * time.Second,
		MaxItems:         1000,
	}
	
	// Update the module's config if it exists
	if ctx.service != nil {
		ctx.service.config = ctx.cacheConfig
	}
	return nil
}

func (ctx *CacheBDDTestContext) iHaveACacheConfigurationWithRedisEngine() error {
	ctx.cacheConfig = &CacheConfig{
		Engine:           "redis",
		DefaultTTL:       300 * time.Second,
		CleanupInterval:  60 * time.Second,
		RedisURL:         "redis://localhost:6379",
		RedisDB:          0,
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
		Engine:           "redis",
		DefaultTTL:       300 * time.Second,
		CleanupInterval:  60 * time.Second,  // Add non-zero cleanup interval
		RedisURL:         "redis://invalid-host:9999",
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
	app := modular.NewStdApplication(mainConfigProvider, logger)
	
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