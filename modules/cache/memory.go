package cache

import (
	"context"
	"sync"
	"time"

	"github.com/CrisisTextLine/modular"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// MemoryCache implements CacheEngine using in-memory storage
type MemoryCache struct {
	config       *CacheConfig
	items        map[string]cacheItem
	mutex        sync.RWMutex
	cleanupCtx   context.Context
	cancelFunc   context.CancelFunc
	eventEmitter func(ctx context.Context, event cloudevents.Event) // Callback for emitting events
	lastCleanup  time.Time                                          // Tracks when cleanup was last run
}

type cacheItem struct {
	value      interface{}
	expiration time.Time
}

// NewMemoryCache creates a new memory cache engine
func NewMemoryCache(config *CacheConfig) *MemoryCache {
	return &MemoryCache{
		config: config,
		items:  make(map[string]cacheItem),
	}
}

// SetEventEmitter sets the event emission callback for the memory cache
func (c *MemoryCache) SetEventEmitter(emitter func(ctx context.Context, event cloudevents.Event)) {
	c.eventEmitter = emitter
}

// ensureCleanupRun ensures cleanup has run recently by triggering it if enough time has passed
func (c *MemoryCache) ensureCleanupRun(ctx context.Context) {
	now := time.Now()
	c.mutex.Lock()
	// Check if cleanup should be triggered based on interval
	if c.lastCleanup.IsZero() || now.Sub(c.lastCleanup) >= c.config.CleanupInterval {
		c.lastCleanup = now
		c.mutex.Unlock()
		// Run cleanup without mutex to avoid deadlock
		c.cleanupExpiredItems(ctx)
	} else {
		c.mutex.Unlock()
	}
}

// TriggerCleanupIfNeeded forces cleanup to run if sufficient time has passed since last cleanup
// This method can be used by tests to ensure cleanup happens naturally without artificial delays
func (c *MemoryCache) TriggerCleanupIfNeeded(ctx context.Context) {
	c.ensureCleanupRun(ctx)
}

// CleanupNow forces an immediate cleanup cycle of expired items and emits corresponding events.
// Intended primarily for tests to deterministically process expirations without waiting for timers.
func (c *MemoryCache) CleanupNow(ctx context.Context) {
	c.cleanupExpiredItems(ctx)
}

// Connect initializes the memory cache
func (c *MemoryCache) Connect(ctx context.Context) error {
	// Validate configuration before use
	if c.config.CleanupInterval <= 0 {
		// Set a sensible default if CleanupInterval is invalid
		c.config.CleanupInterval = 60 * time.Second
	}

	// Start cleanup goroutine with derived context
	c.cleanupCtx, c.cancelFunc = context.WithCancel(ctx)
	go func() {
		c.startCleanupTimer(c.cleanupCtx)
	}()
	return nil
}

// Close stops the memory cache cleanup routine
func (c *MemoryCache) Close(_ context.Context) error {
	if c.cancelFunc != nil {
		c.cancelFunc()
	}
	return nil
}

// Get retrieves an item from the cache
func (c *MemoryCache) Get(ctx context.Context, key string) (interface{}, bool) {
	// Ensure cleanup runs periodically to maintain cache hygiene
	c.ensureCleanupRun(ctx)

	c.mutex.RLock()
	item, found := c.items[key]
	c.mutex.RUnlock()

	if !found {
		return nil, false
	}

	// Check if the item has expired
	if !item.expiration.IsZero() && time.Now().After(item.expiration) {
		c.mutex.Lock()
		delete(c.items, key)
		c.mutex.Unlock()
		return nil, false
	}

	return item.value, true
}

// Set stores an item in the cache
func (c *MemoryCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	// Ensure cleanup runs periodically to maintain cache hygiene
	c.ensureCleanupRun(ctx)

	c.mutex.Lock()
	defer c.mutex.Unlock()

	// If cache is full, reject new items (eviction policy: reject)
	if c.config.MaxItems > 0 && len(c.items) >= c.config.MaxItems {
		_, exists := c.items[key]
		if !exists {
			// Cache is full and this is a new key, emit eviction event
			if c.eventEmitter != nil {
				event := modular.NewCloudEvent(EventTypeCacheEvicted, "cache-service", map[string]interface{}{
					"reason":    "cache_full",
					"max_items": c.config.MaxItems,
					"new_key":   key,
				}, nil)

				c.eventEmitter(ctx, event)
			}
			return ErrCacheFull
		}
	}

	var exp time.Time
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}

	c.items[key] = cacheItem{
		value:      value,
		expiration: exp,
	}

	return nil
}

// Delete removes an item from the cache
func (c *MemoryCache) Delete(_ context.Context, key string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.items, key)
	return nil
}

// Flush removes all items from the cache
func (c *MemoryCache) Flush(_ context.Context) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.items = make(map[string]cacheItem)
	return nil
}

// GetMulti retrieves multiple items from the cache
func (c *MemoryCache) GetMulti(ctx context.Context, keys []string) (map[string]interface{}, error) {
	result := make(map[string]interface{}, len(keys))
	for _, key := range keys {
		if value, found := c.Get(ctx, key); found {
			result[key] = value
		}
	}
	return result, nil
}

// SetMulti stores multiple items in the cache
func (c *MemoryCache) SetMulti(ctx context.Context, items map[string]interface{}, ttl time.Duration) error {
	for key, value := range items {
		if err := c.Set(ctx, key, value, ttl); err != nil {
			return err
		}
	}
	return nil
}

// DeleteMulti removes multiple items from the cache
func (c *MemoryCache) DeleteMulti(ctx context.Context, keys []string) error {
	for _, key := range keys {
		if err := c.Delete(ctx, key); err != nil {
			return err
		}
	}
	return nil
}

// startCleanupTimer starts the cleanup timer for expired items
func (c *MemoryCache) startCleanupTimer(ctx context.Context) {
	// Run cleanup immediately on start
	c.cleanupExpiredItems(ctx)

	ticker := time.NewTicker(c.config.CleanupInterval)
	defer ticker.Stop()

	// Add a secondary shorter ticker for more responsive cleanup during testing
	// This ensures that expired items are cleaned up more promptly
	shortTicker := time.NewTicker(c.config.CleanupInterval / 2)
	defer shortTicker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanupExpiredItems(ctx)
		case <-shortTicker.C:
			// Only run if we have items that might be expired
			c.mutex.RLock()
			hasItems := len(c.items) > 0
			c.mutex.RUnlock()
			if hasItems {
				c.ensureCleanupRun(ctx)
			}
		case <-ctx.Done():
			return
		}
	}
}

// cleanupExpiredItems removes expired items from the cache
func (c *MemoryCache) cleanupExpiredItems(ctx context.Context) {
	now := time.Now()
	c.mutex.Lock()

	// Update last cleanup time
	c.lastCleanup = now

	expiredKeys := make([]string, 0)

	for key, item := range c.items {
		if !item.expiration.IsZero() && now.After(item.expiration) {
			expiredKeys = append(expiredKeys, key)
			delete(c.items, key)
		}
	}

	c.mutex.Unlock()

	// Emit expired events for each expired key (outside mutex to avoid deadlock)
	if c.eventEmitter != nil && len(expiredKeys) > 0 {
		for _, key := range expiredKeys {
			event := modular.NewCloudEvent(EventTypeCacheExpired, "cache-service", map[string]interface{}{
				"cache_key":  key,
				"expired_at": now.Format(time.RFC3339),
				"reason":     "ttl_expired",
			}, nil)

			c.eventEmitter(ctx, event)
		}
	}
}
