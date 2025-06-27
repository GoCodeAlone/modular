package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache implements CacheEngine using Redis
type RedisCache struct {
	config *CacheConfig
	client *redis.Client
}

// NewRedisCache creates a new Redis cache engine
func NewRedisCache(config *CacheConfig) *RedisCache {
	return &RedisCache{
		config: config,
	}
}

// Connect establishes connection to Redis
func (c *RedisCache) Connect(ctx context.Context) error {
	opts, err := redis.ParseURL(c.config.RedisURL)
	if err != nil {
		return fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	if c.config.RedisPassword != "" {
		opts.Password = c.config.RedisPassword
	}

	opts.DB = c.config.RedisDB
	opts.ConnMaxLifetime = time.Duration(c.config.ConnectionMaxAge) * time.Second

	c.client = redis.NewClient(opts)

	// Test the connection
	if err := c.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("failed to ping Redis server: %w", err)
	}
	return nil
}

// Close closes the connection to Redis
func (c *RedisCache) Close(ctx context.Context) error {
	if c.client != nil {
		if err := c.client.Close(); err != nil {
			return fmt.Errorf("failed to close Redis connection: %w", err)
		}
	}
	return nil
}

// Get retrieves an item from the Redis cache
func (c *RedisCache) Get(ctx context.Context, key string) (interface{}, bool) {
	if c.client == nil {
		return nil, false
	}

	val, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, false
		}
		return nil, false
	}

	var result interface{}
	if err := json.Unmarshal([]byte(val), &result); err != nil {
		return nil, false
	}

	return result, true
}

// Set stores an item in the Redis cache with a TTL
func (c *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if c.client == nil {
		return ErrNotConnected
	}

	data, err := json.Marshal(value)
	if err != nil {
		return ErrInvalidValue
	}

	if err := c.client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("failed to set Redis key %s: %w", key, err)
	}
	return nil
}

// Delete removes an item from the Redis cache
func (c *RedisCache) Delete(ctx context.Context, key string) error {
	if c.client == nil {
		return ErrNotConnected
	}

	if err := c.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete Redis key %s: %w", key, err)
	}
	return nil
}

// Flush removes all items from the Redis cache
func (c *RedisCache) Flush(ctx context.Context) error {
	if c.client == nil {
		return ErrNotConnected
	}

	if err := c.client.FlushDB(ctx).Err(); err != nil {
		return fmt.Errorf("failed to flush Redis database: %w", err)
	}
	return nil
}

// GetMulti retrieves multiple items from the Redis cache
func (c *RedisCache) GetMulti(ctx context.Context, keys []string) (map[string]interface{}, error) {
	if len(keys) == 0 {
		return make(map[string]interface{}), nil
	}

	if c.client == nil {
		return nil, ErrNotConnected
	}

	vals, err := c.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get multiple Redis keys: %w", err)
	}

	result := make(map[string]interface{}, len(keys))
	for i, val := range vals {
		if val != nil {
			var value interface{}
			if str, ok := val.(string); ok {
				if err := json.Unmarshal([]byte(str), &value); err == nil {
					result[keys[i]] = value
				}
			}
		}
	}

	return result, nil
}

// SetMulti stores multiple items in the Redis cache with a TTL
func (c *RedisCache) SetMulti(ctx context.Context, items map[string]interface{}, ttl time.Duration) error {
	if len(items) == 0 {
		return nil
	}

	if c.client == nil {
		return ErrNotConnected
	}

	pipe := c.client.Pipeline()
	for key, value := range items {
		data, err := json.Marshal(value)
		if err != nil {
			return ErrInvalidValue
		}
		pipe.Set(ctx, key, data, ttl)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to execute Redis pipeline for SetMulti: %w", err)
	}
	return nil
}

// DeleteMulti removes multiple items from the Redis cache
func (c *RedisCache) DeleteMulti(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	if c.client == nil {
		return ErrNotConnected
	}

	if err := c.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("failed to delete multiple Redis keys: %w", err)
	}
	return nil
}
