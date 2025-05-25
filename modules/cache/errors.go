package cache

import (
	"errors"
)

// Error definitions
var (
	// ErrCacheFull is returned when the memory cache is full and cannot store new items
	ErrCacheFull = errors.New("cache is full")

	// ErrInvalidKey is returned when the key is invalid
	ErrInvalidKey = errors.New("invalid cache key")

	// ErrInvalidValue is returned when the value cannot be stored in the cache
	ErrInvalidValue = errors.New("invalid cache value")

	// ErrNotFound is returned when a key is not found in the cache
	ErrNotFound = errors.New("key not found in cache")

	// ErrNotConnected is returned when an operation is attempted on a cache that is not connected
	ErrNotConnected = errors.New("cache not connected")
)