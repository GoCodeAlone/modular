package reverseproxy

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"sync"
	"time"
)

// CachedResponse represents a cached HTTP response
type CachedResponse struct {
	StatusCode     int
	Headers        http.Header
	Body           []byte
	LastAccessed   time.Time
	ExpirationTime time.Time
}

// responseCache implements a simple cache for HTTP responses
type responseCache struct {
	cache        map[string]*CachedResponse
	mutex        sync.RWMutex
	defaultTTL   time.Duration
	maxCacheSize int
	cacheable    func(r *http.Request, statusCode int) bool
	stopCleanup  chan struct{}
}

// newResponseCache creates a new response cache with the specified TTL and max size
//
//nolint:unused // Used in tests
func newResponseCache(defaultTTL time.Duration, maxCacheSize int, cleanupInterval time.Duration) *responseCache {
	rc := &responseCache{
		cache:        make(map[string]*CachedResponse),
		defaultTTL:   defaultTTL,
		maxCacheSize: maxCacheSize,
		stopCleanup:  make(chan struct{}),
		cacheable: func(r *http.Request, statusCode int) bool {
			// Only cache GET requests with 200 OK responses by default
			return r.Method == http.MethodGet && statusCode == http.StatusOK
		},
	}

	return rc
}

// Set adds or updates a response in the cache
func (rc *responseCache) Set(key string, statusCode int, headers http.Header, body []byte, ttl time.Duration) {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()

	// If at max capacity, evict least recently used item
	if len(rc.cache) >= rc.maxCacheSize {
		rc.evictLRU()
	}

	// Use default TTL if none provided
	if ttl <= 0 {
		ttl = rc.defaultTTL
	}

	// Make a copy of headers to avoid reference issues
	headerCopy := make(http.Header)
	for k, v := range headers {
		headerCopy[k] = v
	}

	rc.cache[key] = &CachedResponse{
		StatusCode:     statusCode,
		Headers:        headerCopy,
		Body:           body,
		LastAccessed:   time.Now(),
		ExpirationTime: time.Now().Add(ttl),
	}
}

// Get retrieves a response from the cache if it exists and is valid
func (rc *responseCache) Get(key string) (*CachedResponse, bool) {
	rc.mutex.RLock()
	cachedResp, found := rc.cache[key]
	rc.mutex.RUnlock()

	if !found {
		return nil, false
	}

	// Check if the response has expired
	if time.Now().After(cachedResp.ExpirationTime) {
		rc.mutex.Lock()
		delete(rc.cache, key)
		rc.mutex.Unlock()
		return nil, false
	}

	// Update last accessed time
	rc.mutex.Lock()
	cachedResp.LastAccessed = time.Now()
	rc.mutex.Unlock()

	return cachedResp, true
}

// GenerateKey creates a cache key from an HTTP request
func (rc *responseCache) GenerateKey(r *http.Request) string {
	// Create a hash of the method, URL, and relevant headers
	h := sha256.New()
	_, _ = io.WriteString(h, r.Method)
	_, _ = io.WriteString(h, r.URL.String())

	// Include relevant caching headers like Accept and Accept-Encoding
	if accept := r.Header.Get("Accept"); accept != "" {
		_, _ = io.WriteString(h, accept)
	}
	if acceptEncoding := r.Header.Get("Accept-Encoding"); acceptEncoding != "" {
		_, _ = io.WriteString(h, acceptEncoding)
	}

	return hex.EncodeToString(h.Sum(nil))
}

// SetCacheableCheck sets a function that determines if a request/response is cacheable
func (rc *responseCache) SetCacheableCheck(fn func(r *http.Request, statusCode int) bool) {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()
	rc.cacheable = fn
}

// IsCacheable checks if a request/response is cacheable according to the configured function
func (rc *responseCache) IsCacheable(r *http.Request, statusCode int) bool {
	rc.mutex.RLock()
	defer rc.mutex.RUnlock()
	return rc.cacheable(r, statusCode)
}

// evictLRU evicts the least recently used item from the cache
// Must be called with lock held
func (rc *responseCache) evictLRU() {
	var oldestKey string
	var oldestTime time.Time

	// Find the oldest item
	first := true
	for k, v := range rc.cache {
		if first || v.LastAccessed.Before(oldestTime) {
			oldestKey = k
			oldestTime = v.LastAccessed
			first = false
		}
	}

	// Remove it
	if oldestKey != "" {
		delete(rc.cache, oldestKey)
	}
}

// cleanup removes expired items from the cache
func (rc *responseCache) cleanup() {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()

	now := time.Now()
	for k, v := range rc.cache {
		if now.After(v.ExpirationTime) {
			delete(rc.cache, k)
		}
	}
}

// periodicCleanup runs a cleanup on the cache at regular intervals
// Close stops the periodic cleanup goroutine
func (rc *responseCache) Close() {
	close(rc.stopCleanup)
}
