package cache

// Event type constants for cache module events.
// Following CloudEvents specification reverse domain notation.
const (
	// Cache operation events
	EventTypeCacheGet    = "com.modular.cache.get"
	EventTypeCacheSet    = "com.modular.cache.set"
	EventTypeCacheDelete = "com.modular.cache.delete"
	EventTypeCacheFlush  = "com.modular.cache.flush"

	// Cache state events
	EventTypeCacheHit     = "com.modular.cache.hit"
	EventTypeCacheMiss    = "com.modular.cache.miss"
	EventTypeCacheExpired = "com.modular.cache.expired"
	EventTypeCacheEvicted = "com.modular.cache.evicted"

	// Cache engine events
	EventTypeCacheConnected    = "com.modular.cache.connected"
	EventTypeCacheDisconnected = "com.modular.cache.disconnected"
	EventTypeCacheError        = "com.modular.cache.error"
)
