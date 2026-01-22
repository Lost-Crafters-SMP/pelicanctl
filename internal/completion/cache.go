package completion

import (
	"sync"
	"time"
)

const (
	defaultCacheTTL = 5 * time.Minute
)

type cacheEntry struct {
	data      []string
	timestamp time.Time
}

var (
	cache     sync.Map
	cacheTTL  = defaultCacheTTL
	cacheLock sync.RWMutex
)

// getCacheKey generates a cache key from API type and resource type.
func getCacheKey(apiType, resourceType string) string {
	return apiType + ":" + resourceType
}

// getCached retrieves cached data if it's still valid.
func getCached(key string) []string {
	entry, ok := cache.Load(key)
	if !ok {
		return nil
	}

	ce, ok := entry.(cacheEntry)
	if !ok {
		return nil
	}
	if time.Since(ce.timestamp) > cacheTTL {
		cache.Delete(key)
		return nil
	}

	return ce.data
}

// setCached stores data in the cache.
func setCached(key string, data []string) {
	cache.Store(key, cacheEntry{
		data:      data,
		timestamp: time.Now(),
	})
}

// SetCacheTTL sets the cache TTL (for testing or configuration).
func SetCacheTTL(ttl time.Duration) {
	cacheLock.Lock()
	defer cacheLock.Unlock()
	cacheTTL = ttl
}
