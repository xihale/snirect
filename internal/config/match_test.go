package config

import "testing"

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		pattern string
		host    string
		want    bool
	}{
		{"#comment", "example.com", false},
		{"example.com", "example.com", true},
		{"example.com", "www.example.com", false},
		{"$example.com", "example.com", true},
		{"$example.com", "www.example.com", false},
		{"*example.com", "example.com", true},
		{"*example.com", "www.example.com", true},
		{"*example.com", "anotherexample.com", true}, // Suffix match
		{"*.example.com", "example.com", true},
		{"*.example.com", "www.example.com", true},
		{"*.example.com", "anotherexample.com", false},
		{"example*", "example.com", true},
		{"example*", "example.org", true},
		{"example*", "google.com", false},
		{"$*google.com", "www.google.com", true},   // $ is stripped, then matches as wildcard
		{"$example.com", "www.example.com", false}, // $ stripped, example.com doesn't match www.example.com
	}

	for _, tt := range tests {
		got := MatchPattern(tt.pattern, tt.host)
		if got != tt.want {
			t.Errorf("MatchPattern(%q, %q) = %v, want %v", tt.pattern, tt.host, got, tt.want)
		}
	}
}
