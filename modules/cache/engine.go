package cache

import (
	"context"
	"time"
)

// CacheEngine defines the interface for cache engine implementations.
// This interface abstracts the underlying storage mechanism, allowing
// the cache module to support multiple backends (memory, Redis, etc.)
// through a common API.
//
// All operations are context-aware to support cancellation and timeouts.
// Implementations should be thread-safe and handle concurrent access properly.
//
// Cache engines are responsible for:
//   - Connection management to the underlying storage
//   - Data serialization/deserialization
//   - TTL handling and expiration
//   - Error handling and recovery
type CacheEngine interface {
	// Connect establishes connection to the cache backend.
	// This method is called during module startup and should prepare
	// the engine for cache operations. For memory caches, this might
	// initialize internal data structures. For network-based caches
	// like Redis, this establishes the connection pool.
	//
	// The context can be used to handle connection timeouts.
	Connect(ctx context.Context) error

	// Close closes the connection to the cache backend.
	// This method is called during module shutdown and should cleanup
	// all resources, close network connections, and stop background
	// processes. The method should be idempotent - safe to call multiple times.
	//
	// The context can be used to handle graceful shutdown timeouts.
	Close(ctx context.Context) error

	// Get retrieves an item from the cache.
	// Returns the cached value and a boolean indicating whether the key was found.
	// If the key doesn't exist or has expired, returns (nil, false).
	//
	// The returned value should be the same type that was stored.
	// The context can be used for operation timeouts.
	Get(ctx context.Context, key string) (interface{}, bool)

	// Set stores an item in the cache with a TTL.
	// The value can be any serializable type. The TTL determines how long
	// the item should remain in the cache before expiring.
	//
	// If TTL is 0 or negative, the item should use the default TTL or
	// never expire, depending on the implementation.
	//
	// The context can be used for operation timeouts.
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error

	// Delete removes an item from the cache.
	// Should not return an error if the key doesn't exist.
	// Only returns errors for actual operation failures.
	//
	// The context can be used for operation timeouts.
	Delete(ctx context.Context, key string) error

	// Flush removes all items from the cache.
	// This operation should be atomic - either all items are removed
	// or none are. Should be used with caution as it's irreversible.
	//
	// The context can be used for operation timeouts.
	Flush(ctx context.Context) error

	// GetMulti retrieves multiple items from the cache in a single operation.
	// Returns a map containing only the keys that were found.
	// Missing or expired keys are not included in the result.
	//
	// This operation should be more efficient than multiple Get calls
	// for network-based caches.
	//
	// The context can be used for operation timeouts.
	GetMulti(ctx context.Context, keys []string) (map[string]interface{}, error)

	// SetMulti stores multiple items in the cache with a TTL.
	// All items use the same TTL value. This operation should be atomic
	// where possible - either all items are stored or none are.
	//
	// This operation should be more efficient than multiple Set calls
	// for network-based caches.
	//
	// The context can be used for operation timeouts.
	SetMulti(ctx context.Context, items map[string]interface{}, ttl time.Duration) error

	// DeleteMulti removes multiple items from the cache.
	// Should not return an error for keys that don't exist.
	// Only returns errors for actual operation failures.
	//
	// This operation should be more efficient than multiple Delete calls
	// for network-based caches.
	//
	// The context can be used for operation timeouts.
	DeleteMulti(ctx context.Context, keys []string) error
}
