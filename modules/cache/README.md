# Cache Module

[![Go Reference](https://pkg.go.dev/badge/github.com/GoCodeAlone/modular/modules/cache.svg)](https://pkg.go.dev/github.com/GoCodeAlone/modular/modules/cache)

The Cache Module provides caching functionality for Modular applications. It offers different cache backend options including in-memory and Redis (placeholder implementation).

## Features

- Multiple cache engine options (memory, Redis)
- Support for time-to-live (TTL) expirations
- Automatic cache cleanup for expired items
- Basic cache operations (get, set, delete)
- Bulk operations (getMulti, setMulti, deleteMulti)

## Installation

```go
import (
    "github.com/GoCodeAlone/modular"
    "github.com/GoCodeAlone/modular/modules/cache"
)

// Register the cache module with your Modular application
app.RegisterModule(cache.NewModule())
```

## Configuration

The cache module can be configured using the following options:

```yaml
cache:
  engine: memory            # Cache engine to use: "memory" or "redis"
  defaultTTL: 300           # Default TTL in seconds if not specified (300s = 5 minutes)
  cleanupInterval: 60       # How often to clean up expired items (60s = 1 minute)
  maxItems: 10000           # Maximum items to store in memory cache
  redisURL: ""              # Redis connection URL (for Redis engine)
  redisPassword: ""         # Redis password (for Redis engine)
  redisDB: 0                # Redis database number (for Redis engine)
  connectionMaxAge: 60      # Maximum age of connections in seconds
```

## Usage

### Accessing the Cache Service

```go
// In your module's Init function
func (m *MyModule) Init(app modular.Application) error {
    var cacheService *cache.CacheModule
    err := app.GetService("cache.provider", &cacheService)
    if err != nil {
        return fmt.Errorf("failed to get cache service: %w", err)
    }
    
    // Now you can use the cache service
    m.cache = cacheService
    return nil
}
```

### Using Interface-Based Service Matching

```go
// Define the service dependency
func (m *MyModule) RequiresServices() []modular.ServiceDependency {
    return []modular.ServiceDependency{
        {
            Name:               "cache",
            Required:           true,
            MatchByInterface:   true,
            SatisfiesInterface: reflect.TypeOf((*cache.CacheEngine)(nil)).Elem(),
        },
    }
}

// Access the service in your constructor
func (m *MyModule) Constructor() modular.ModuleConstructor {
    return func(app modular.Application, services map[string]any) (modular.Module, error) {
        cacheService := services["cache"].(cache.CacheEngine)
        return &MyModule{cache: cacheService}, nil
    }
}
```

### Basic Operations

```go
// Set a value in the cache with a TTL
err := cacheService.Set(ctx, "user:123", userData, 5*time.Minute)
if err != nil {
    // Handle error
}

// Get a value from the cache
value, found := cacheService.Get(ctx, "user:123")
if !found {
    // Cache miss, fetch from primary source
} else {
    userData = value.(UserData)
}

// Delete a value from the cache
err := cacheService.Delete(ctx, "user:123")
if err != nil {
    // Handle error
}
```

### Bulk Operations

```go
// Get multiple values
keys := []string{"user:123", "user:456", "user:789"}
results, err := cacheService.GetMulti(ctx, keys)
if err != nil {
    // Handle error
}

// Set multiple values
items := map[string]interface{}{
    "user:123": userData1,
    "user:456": userData2,
}
err := cacheService.SetMulti(ctx, items, 10*time.Minute)
if err != nil {
    // Handle error
}

// Delete multiple values
err := cacheService.DeleteMulti(ctx, keys)
if err != nil {
    // Handle error
}
```

## Implementation Notes

- The in-memory cache uses Go's built-in concurrency primitives for thread safety
- Redis implementation is provided as a placeholder and would require a Redis client implementation
- The cache automatically cleans up expired items at configurable intervals

## Testing

The cache module includes comprehensive tests for the memory cache implementation.