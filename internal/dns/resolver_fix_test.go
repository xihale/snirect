package dns

import (
	"snirect/internal/config"
	"testing"
)

func TestResolver_Invalidate(t *testing.T) {
	// Initialize resolver with empty config
	r := NewResolver(&config.Config{}, &config.Rules{})

	host := "example.com"
	ip := "1.2.3.4"

	// Set cache manually (simulating resolution for A record / Type 1)
	r.setCache(host, ip, 1)

	// Verify it's there
	if _, ok := r.getCache(host, 1); !ok {
		t.Fatal("Cache should have entry")
	}

	// Invalidate
	r.Invalidate(host)

	// Verify it's GONE
	// This assertion fails if Invalidate only tries to delete "example.com" instead of "example.com:1"
	if val, ok := r.getCache(host, 1); ok {
		t.Errorf("BUG: Cache Invalidation FAILED: Key %s still exists with value %s", host, val)
	}
}
