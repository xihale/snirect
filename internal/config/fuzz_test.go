package config

import (
	"testing"

	"github.com/pelletier/go-toml/v2"
)

// FuzzUnmarshalConfig tests toml.Unmarshal with random inputs to catch panics.
func FuzzUnmarshalConfig(f *testing.F) {
	// Seed with known valid and edge-case configs to improve corpus efficiency
	f.Add([]byte("server.port = 7654\n"))
	f.Add([]byte(""))                                 // empty config
	f.Add([]byte("server.port = -1\n"))               // negative port
	f.Add([]byte("dns.nameserver = [\"8.8.8.8\"]\n")) // DNS config
	f.Add([]byte("[update]\nupstream_rate_limit = 10\n"))
	f.Add([]byte("check_hostname = false\n"))
	f.Add([]byte("[hosts]\n\"example.com\" = \"93.184.216.34\"\n"))

	f.Fuzz(func(t *testing.T, data []byte) {
		var cfg Config
		_ = toml.Unmarshal(data, &cfg)
	})
}
