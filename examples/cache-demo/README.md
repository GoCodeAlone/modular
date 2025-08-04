# Cache Module Demo

This example demonstrates how to use the cache module for both in-memory and Redis caching with TTL support and cache operations.

## Overview

The example sets up:
- In-memory cache with configurable TTL
- Redis cache configuration (when available)
- Cache operations: Set, Get, Delete, Clear
- HTTP API endpoints to interact with the cache
- Automatic expiration handling

## Features Demonstrated

1. **Multi-Backend Caching**: Both in-memory and Redis support
2. **TTL Support**: Time-to-live for cache entries
3. **Cache Operations**: Basic CRUD operations on cache
4. **HTTP Integration**: RESTful API for cache management
5. **Configuration**: Configurable cache backends and settings

## API Endpoints

- `POST /api/cache/:key` - Set a value in cache with optional TTL
- `GET /api/cache/:key` - Get a value from cache
- `DELETE /api/cache/:key` - Delete a value from cache
- `DELETE /api/cache` - Clear all cache entries
- `GET /api/cache/stats` - Get cache statistics

## Running the Example

1. Start the application:
   ```bash
   go run main.go
   ```

2. The application will start on port 8080

## Testing Cache Operations

### Set a value in cache
```bash
curl -X POST http://localhost:8080/api/cache/mykey \
  -H "Content-Type: application/json" \
  -d '{"value": "Hello, World!", "ttl": 3600}'
```

### Get a value from cache
```bash
curl http://localhost:8080/api/cache/mykey
```

### Set with different TTL
```bash
curl -X POST http://localhost:8080/api/cache/shortlived \
  -H "Content-Type: application/json" \
  -d '{"value": "This expires in 10 seconds", "ttl": 10}'
```

### Delete a specific key
```bash
curl -X DELETE http://localhost:8080/api/cache/mykey
```

### Clear all cache entries
```bash
curl -X DELETE http://localhost:8080/api/cache
```

### Get cache statistics
```bash
curl http://localhost:8080/api/cache/stats
```

## Configuration

The cache module is configured in `config.yaml`:

```yaml
cache:
  backend: "memory"  # or "redis"
  default_ttl: 3600  # 1 hour in seconds
  memory:
    cleanup_interval: 600  # cleanup every 10 minutes
  redis:
    address: "localhost:6379"
    password: ""
    db: 0
    max_retries: 3
    pool_size: 10
```

## Cache Backends

### In-Memory Cache
- Fast access for single-instance applications
- Automatic cleanup of expired entries
- Configurable cleanup intervals
- Memory-efficient with TTL support

### Redis Cache
- Distributed caching for multi-instance applications
- Persistent storage with Redis features
- Connection pooling and retry logic
- Production-ready scalability

## Error Handling

The example includes proper error handling for:
- Cache backend connection failures
- Key not found scenarios
- Invalid TTL values
- Serialization/deserialization errors
- Network issues with Redis

This demonstrates how to integrate caching capabilities into modular applications for improved performance.