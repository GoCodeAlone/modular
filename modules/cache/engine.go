package cache

import (
	"context"
	"time"
)

// CacheEngine defines the interface for cache engine implementations
type CacheEngine interface {
	// Connect establishes connection to the cache backend
	Connect(ctx context.Context) error

	// Close closes the connection to the cache backend
	Close(ctx context.Context) error

	// Get retrieves an item from the cache
	Get(ctx context.Context, key string) (interface{}, bool)

	// Set stores an item in the cache with a TTL
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error

	// Delete removes an item from the cache
	Delete(ctx context.Context, key string) error

	// Flush removes all items from the cache
	Flush(ctx context.Context) error

	// GetMulti retrieves multiple items from the cache
	GetMulti(ctx context.Context, keys []string) (map[string]interface{}, error)

	// SetMulti stores multiple items in the cache with a TTL
	SetMulti(ctx context.Context, items map[string]interface{}, ttl time.Duration) error

	// DeleteMulti removes multiple items from the cache
	DeleteMulti(ctx context.Context, keys []string) error
}
