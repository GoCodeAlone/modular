package cache

// CacheConfig defines the configuration for the cache module.
// This structure contains all the settings needed to configure both
// memory and Redis cache engines.
//
// Configuration can be provided through JSON, YAML, or environment variables.
// The struct tags define the mapping for each configuration source and
// validation rules.
//
// Example JSON configuration:
//
//	{
//	  "engine": "redis",
//	  "defaultTTL": 600,
//	  "cleanupInterval": 300,
//	  "maxItems": 50000,
//	  "redisURL": "redis://localhost:6379",
//	  "redisPassword": "mypassword",
//	  "redisDB": 1
//	}
//
// Example environment variables:
//
//	CACHE_ENGINE=memory
//	CACHE_DEFAULT_TTL=300
//	CACHE_MAX_ITEMS=10000
type CacheConfig struct {
	// Engine specifies the cache engine to use.
	// Supported values: "memory", "redis"
	// Default: "memory"
	Engine string `json:"engine" yaml:"engine" env:"ENGINE" validate:"oneof=memory redis"`

	// DefaultTTL is the default time-to-live for cache entries in seconds.
	// Used when no explicit TTL is provided in cache operations.
	// Must be at least 1 second.
	DefaultTTL int `json:"defaultTTL" yaml:"defaultTTL" env:"DEFAULT_TTL" validate:"min=1"`

	// CleanupInterval is how often to clean up expired items (in seconds).
	// Only applicable to memory cache engine.
	// Must be at least 1 second.
	CleanupInterval int `json:"cleanupInterval" yaml:"cleanupInterval" env:"CLEANUP_INTERVAL" validate:"min=1"`

	// MaxItems is the maximum number of items to store in memory cache.
	// When this limit is reached, least recently used items are evicted.
	// Only applicable to memory cache engine.
	// Must be at least 1.
	MaxItems int `json:"maxItems" yaml:"maxItems" env:"MAX_ITEMS" validate:"min=1"`

	// RedisURL is the connection URL for Redis server.
	// Format: redis://[username:password@]host:port[/database]
	// Only required when using Redis engine.
	// Example: "redis://localhost:6379", "redis://user:pass@localhost:6379/1"
	RedisURL string `json:"redisURL" yaml:"redisURL" env:"REDIS_URL"`

	// RedisPassword is the password for Redis authentication.
	// Optional if Redis server doesn't require authentication.
	RedisPassword string `json:"redisPassword" yaml:"redisPassword" env:"REDIS_PASSWORD"`

	// RedisDB is the Redis database number to use.
	// Redis supports multiple databases (0-15 by default).
	// Must be non-negative.
	RedisDB int `json:"redisDB" yaml:"redisDB" env:"REDIS_DB" validate:"min=0"`

	// ConnectionMaxAge is the maximum age of a connection in seconds.
	// Connections older than this will be closed and recreated.
	// Helps prevent connection staleness in long-running applications.
	// Must be at least 1 second.
	ConnectionMaxAge int `json:"connectionMaxAge" yaml:"connectionMaxAge" env:"CONNECTION_MAX_AGE" validate:"min=1"`
}
