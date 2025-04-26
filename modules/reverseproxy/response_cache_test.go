package reverseproxy

import (
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewResponseCache(t *testing.T) {
	ttl := 10 * time.Minute
	maxSize := 100
	cleanupInterval := 30 * time.Second

	rc := newResponseCache(ttl, maxSize, cleanupInterval)
	defer rc.Close() // Close to stop the cleanup goroutine

	assert.NotNil(t, rc, "Response cache should be created")
	assert.Equal(t, ttl, rc.defaultTTL, "Default TTL should be set correctly")
	assert.Equal(t, maxSize, rc.maxCacheSize, "Max size should be set correctly")
	assert.Equal(t, cleanupInterval, rc.cleanupInterval, "Cleanup interval should be set correctly")
	assert.NotNil(t, rc.cache, "Cache map should be initialized")
	assert.NotNil(t, rc.stopCleanup, "Cleanup stop channel should be initialized")
	assert.NotNil(t, rc.cacheable, "Cacheable function should be initialized")
}

func TestSetAndGetResponseCache(t *testing.T) {
	rc := newResponseCache(10*time.Minute, 100, 1*time.Hour)
	defer rc.Close()

	key := "test-key"
	statusCode := 200
	headers := http.Header{"Content-Type": []string{"application/json"}}
	body := []byte(`{"test":"data"}`)

	// Set the cache entry
	rc.Set(key, statusCode, headers, body, 0) // Use default TTL

	// Get the cache entry
	cachedResp, found := rc.Get(key)

	assert.True(t, found, "Cache entry should be found")
	assert.Equal(t, statusCode, cachedResp.StatusCode, "Status code should match")
	assert.Equal(t, headers.Get("Content-Type"), cachedResp.Headers.Get("Content-Type"), "Headers should match")
	assert.Equal(t, body, cachedResp.Body, "Body should match")
}

func TestExpiredCache(t *testing.T) {
	rc := newResponseCache(10*time.Millisecond, 100, 1*time.Hour) // Very short TTL for testing
	defer rc.Close()

	key := "test-key"
	rc.Set(key, 200, http.Header{}, []byte("test"), 0)

	// Wait for expiration
	time.Sleep(20 * time.Millisecond)

	// Try to get expired entry
	_, found := rc.Get(key)
	assert.False(t, found, "Expired cache entry should not be found")

	// Check it was removed
	rc.mutex.RLock()
	_, exists := rc.cache[key]
	rc.mutex.RUnlock()
	assert.False(t, exists, "Expired entry should be removed from cache")
}

func TestCustomTTL(t *testing.T) {
	rc := newResponseCache(10*time.Minute, 100, 1*time.Hour)
	defer rc.Close()

	// Set cache with custom short TTL
	rc.Set("short", 200, http.Header{}, []byte("short"), 10*time.Millisecond)
	// Set cache with custom long TTL
	rc.Set("long", 200, http.Header{}, []byte("long"), 10*time.Hour)

	// Wait for short TTL to expire
	time.Sleep(20 * time.Millisecond)

	_, shortFound := rc.Get("short")
	longResp, longFound := rc.Get("long")

	assert.False(t, shortFound, "Short TTL entry should expire")
	assert.True(t, longFound, "Long TTL entry should not expire")
	assert.Equal(t, []byte("long"), longResp.Body, "Long TTL entry data should be intact")
}

func TestLRUEviction(t *testing.T) {
	rc := newResponseCache(10*time.Minute, 3, 1*time.Hour) // Very small size for testing
	defer rc.Close()

	// Fill the cache
	rc.Set("key1", 200, http.Header{}, []byte("data1"), 0)
	rc.Set("key2", 200, http.Header{}, []byte("data2"), 0)
	rc.Set("key3", 200, http.Header{}, []byte("data3"), 0)

	// Access key1 to make it most recently used
	rc.Get("key1")
	time.Sleep(5 * time.Millisecond) // Ensure access times are different

	// Access key2 to make it second most recently used
	rc.Get("key2")
	time.Sleep(5 * time.Millisecond) // Ensure access times are different

	// Add a new entry, should evict key3 (least recently used)
	rc.Set("key4", 200, http.Header{}, []byte("data4"), 0)

	// Check what's in the cache
	_, key1Found := rc.Get("key1")
	_, key2Found := rc.Get("key2")
	_, key3Found := rc.Get("key3")
	_, key4Found := rc.Get("key4")

	assert.True(t, key1Found, "Most recently used entry should not be evicted")
	assert.True(t, key2Found, "Second most recently used entry should not be evicted")
	assert.False(t, key3Found, "Least recently used entry should be evicted")
	assert.True(t, key4Found, "Newly added entry should be in cache")
}

func TestCacheableCheck(t *testing.T) {
	rc := newResponseCache(10*time.Minute, 100, 1*time.Hour)
	defer rc.Close()

	// Test default cacheable function
	getRequest := &http.Request{Method: http.MethodGet}
	postRequest := &http.Request{Method: http.MethodPost}

	assert.True(t, rc.IsCacheable(getRequest, 200), "GET request with 200 response should be cacheable")
	assert.False(t, rc.IsCacheable(getRequest, 500), "GET request with non-200 response should not be cacheable")
	assert.False(t, rc.IsCacheable(postRequest, 200), "Non-GET request should not be cacheable")

	// Set custom cacheable function
	rc.SetCacheableCheck(func(r *http.Request, statusCode int) bool {
		// Cache any successful response
		return statusCode >= 200 && statusCode < 300
	})

	assert.True(t, rc.IsCacheable(postRequest, 201), "Custom function should allow POST with 201")
	assert.True(t, rc.IsCacheable(getRequest, 204), "Custom function should allow GET with 204")
	assert.False(t, rc.IsCacheable(getRequest, 404), "Custom function should reject 404")
}

func TestGenerateKey(t *testing.T) {
	rc := newResponseCache(10*time.Minute, 100, 1*time.Hour)
	defer rc.Close()

	// Create test requests
	url1, _ := url.Parse("http://example.com/api/resource?q=test")
	req1 := &http.Request{
		Method: "GET",
		URL:    url1,
		Header: http.Header{},
	}

	url2, _ := url.Parse("http://example.com/api/resource?q=test")
	req2 := &http.Request{
		Method: "GET",
		URL:    url2,
		Header: http.Header{},
	}

	url3, _ := url.Parse("http://example.com/api/resource?q=different")
	req3 := &http.Request{
		Method: "GET",
		URL:    url3,
		Header: http.Header{},
	}

	// Same request with different Accept header
	url4, _ := url.Parse("http://example.com/api/resource?q=test")
	req4 := &http.Request{
		Method: "GET",
		URL:    url4,
		Header: http.Header{"Accept": []string{"application/json"}},
	}

	// Generate keys
	key1 := rc.GenerateKey(req1)
	key2 := rc.GenerateKey(req2)
	key3 := rc.GenerateKey(req3)
	key4 := rc.GenerateKey(req4)

	// Identical requests should have identical keys
	assert.Equal(t, key1, key2, "Identical requests should have identical cache keys")

	// Different requests should have different keys
	assert.NotEqual(t, key1, key3, "Different requests should have different cache keys")

	// Headers should affect the key
	assert.NotEqual(t, key1, key4, "Different headers should result in different cache keys")
}

func TestCleanup(t *testing.T) {
	rc := newResponseCache(10*time.Millisecond, 100, 50*time.Millisecond) // Short TTL and cleanup interval
	defer rc.Close()

	// Add several entries
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("key%d", i)
		rc.Set(key, 200, http.Header{}, []byte("data"), 0)
	}

	// Initially, all items should be there
	rc.mutex.RLock()
	initialCount := len(rc.cache)
	rc.mutex.RUnlock()
	assert.Equal(t, 5, initialCount, "All items should be in cache initially")

	// Wait for cleanup to run (longer than cleanup interval)
	time.Sleep(100 * time.Millisecond)

	// After cleanup, all items should be gone due to expiration
	rc.mutex.RLock()
	afterCleanupCount := len(rc.cache)
	rc.mutex.RUnlock()
	assert.Equal(t, 0, afterCleanupCount, "All items should be removed after cleanup")
}

func TestConcurrentAccess(t *testing.T) {
	rc := newResponseCache(10*time.Second, 1000, 1*time.Hour)
	defer rc.Close()

	// Use waitgroups to coordinate concurrent access
	var wg sync.WaitGroup
	iterations := 100

	// Start multiple writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < iterations; j++ {
				key := fmt.Sprintf("writer-%d-key-%d", id, j)
				rc.Set(key, 200, http.Header{}, []byte("data"), 0)
			}
		}(i)
	}

	// Start multiple readers concurrently
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < iterations; j++ {
				key := fmt.Sprintf("writer-%d-key-%d", id%5, j)
				rc.Get(key) // Result doesn't matter, just testing concurrency
			}
		}(i)
	}

	// Wait for all goroutines to finish
	wg.Wait()

	// If we got here without deadlocks or panics, the test passes
	// Additional verification can be done by counting items
	rc.mutex.RLock()
	count := len(rc.cache)
	rc.mutex.RUnlock()

	assert.True(t, count > 0, "Cache should contain items after concurrent operations")
	assert.True(t, count <= 1000, "Cache should not exceed max size")
}
