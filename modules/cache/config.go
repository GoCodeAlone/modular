package cache

// CacheConfig defines the configuration for the cache module
type CacheConfig struct {
	// Engine specifies the cache engine to use ("memory" or "redis")
	Engine string `json:"engine" yaml:"engine" env:"ENGINE" validate:"oneof=memory redis"`

	// DefaultTTL is the default time-to-live for cache entries in seconds
	DefaultTTL int `json:"defaultTTL" yaml:"defaultTTL" env:"DEFAULT_TTL" validate:"min=1"`

	// CleanupInterval is how often to clean up expired items (in seconds)
	CleanupInterval int `json:"cleanupInterval" yaml:"cleanupInterval" env:"CLEANUP_INTERVAL" validate:"min=1"`

	// MaxItems is the maximum number of items to store in memory cache
	MaxItems int `json:"maxItems" yaml:"maxItems" env:"MAX_ITEMS" validate:"min=1"`

	// Redis-specific configuration
	RedisURL      string `json:"redisURL" yaml:"redisURL" env:"REDIS_URL"`
	RedisPassword string `json:"redisPassword" yaml:"redisPassword" env:"REDIS_PASSWORD"`
	RedisDB       int    `json:"redisDB" yaml:"redisDB" env:"REDIS_DB" validate:"min=0"`

	// ConnectionMaxAge is the maximum age of a connection in seconds
	ConnectionMaxAge int `json:"connectionMaxAge" yaml:"connectionMaxAge" env:"CONNECTION_MAX_AGE" validate:"min=1"`
}
