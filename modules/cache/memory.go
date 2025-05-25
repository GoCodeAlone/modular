package cache

import (
	"context"
	"sync"
	"time"
)

// MemoryCache implements CacheEngine using in-memory storage
type MemoryCache struct {
	config     *CacheConfig
	items      map[string]cacheItem
	mutex      sync.RWMutex
	cleanupCtx context.Context
	cancelFunc context.CancelFunc
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

// Connect initializes the memory cache
func (c *MemoryCache) Connect(_ context.Context) error {
	// Start cleanup goroutine
	c.cleanupCtx, c.cancelFunc = context.WithCancel(context.Background())
	go c.startCleanupTimer(c.cleanupCtx)
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
func (c *MemoryCache) Get(_ context.Context, key string) (interface{}, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	item, found := c.items[key]
	if !found {
		return nil, false
	}

	// Check if the item has expired
	if !item.expiration.IsZero() && time.Now().After(item.expiration) {
		return nil, false
	}

	return item.value, true
}

// Set stores an item in the cache
func (c *MemoryCache) Set(_ context.Context, key string, value interface{}, ttl time.Duration) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// If cache is full, reject new items
	if c.config.MaxItems > 0 && len(c.items) >= c.config.MaxItems && c.items[key].value == nil {
		return ErrCacheFull
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
	ticker := time.NewTicker(time.Duration(c.config.CleanupInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanupExpiredItems()
		case <-ctx.Done():
			return
		}
	}
}

// cleanupExpiredItems removes expired items from the cache
func (c *MemoryCache) cleanupExpiredItems() {
	now := time.Now()
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for key, item := range c.items {
		if !item.expiration.IsZero() && now.After(item.expiration) {
			delete(c.items, key)
		}
	}
}