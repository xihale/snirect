package dns

import (
	"sync"
	"time"
)

// preferenceCacheEntry stores a preferred IP and its expiry.
type preferenceCacheEntry struct {
	ip        string
	expiresAt time.Time
}

// preferenceCache caches IP preference results based on latency testing.
// It is thread-safe and has configurable size limits.
type preferenceCache struct {
	mu      sync.RWMutex
	entries map[string]preferenceCacheEntry
	limit   int // 0 = unlimited
}

// newPreferenceCache creates a new cache with the given size limit.
func newPreferenceCache(limit int) *preferenceCache {
	return &preferenceCache{
		entries: make(map[string]preferenceCacheEntry),
		limit:   limit,
	}
}

// get returns the cached preferred IP for the given host, if valid.
func (c *preferenceCache) get(host string) (ip string, ok bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[host]
	if !exists || time.Now().After(entry.expiresAt) {
		return "", false
	}
	return entry.ip, true
}

// set stores a preference entry with the given TTL.
func (c *preferenceCache) set(host, ip string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict if limit reached (simple random eviction)
	if c.limit > 0 && len(c.entries) >= c.limit {
		for k := range c.entries {
			delete(c.entries, k)
			break
		}
	}

	c.entries[host] = preferenceCacheEntry{
		ip:        ip,
		expiresAt: time.Now().Add(ttl),
	}
}

// invalidate removes the cached preference for the given host.
func (c *preferenceCache) invalidate(host string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, host)
}

// size returns the number of entries currently in the cache.
func (c *preferenceCache) size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}
