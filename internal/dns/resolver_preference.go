package dns

import (
	"sync"
	"time"
)

// preferenceCacheEntry stores a preferred IP and its metadata.
type preferenceCacheEntry struct {
	ip        string
	latency   time.Duration // Measured latency (for debugging/stats only)
	testedAt  time.Time
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
func (c *preferenceCache) set(host, ip string, latency time.Duration, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict if limit reached (simple random eviction)
	if c.limit > 0 && len(c.entries) >= c.limit {
		for k := range c.entries {
			delete(c.entries, k)
			break
		}
	}

	now := time.Now()
	c.entries[host] = preferenceCacheEntry{
		ip:        ip,
		latency:   latency,
		testedAt:  now,
		expiresAt: now.Add(ttl),
	}
}

// invalidate removes the cached preference for the given host.
func (c *preferenceCache) invalidate(host string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, host)
}

// clear removes all entries.
func (c *preferenceCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]preferenceCacheEntry)
}

// stats returns cache statistics (for debugging).
func (c *preferenceCache) stats() (size int, hits, misses int) {
	c.mu.RLock()
	size = len(c.entries)
	c.mu.RUnlock()
	// Note: hits/misses tracking would need atomic counters if we want accuracy
	// For now just return size. Can be extended later.
	return size, 0, 0
}
