package tlsutil

import (
	"crypto/x509"
	"snirect/internal/config"
	"testing"
)

func TestMatchHostname(t *testing.T) {
	cert := &x509.Certificate{
		DNSNames: []string{"example.com", "*.example.org"},
	}

	tests := []struct {
		name     string
		hostname string
		policy   config.CertPolicy
		want     bool
	}{
		// 1. Strict Policy
		{
			name:     "Strict - Match",
			hostname: "example.com",
			policy:   config.CertPolicy{Enabled: true, Strict: true},
			want:     true,
		},
		{
			name:     "Strict - Wildcard Match",
			hostname: "foo.example.org",
			policy:   config.CertPolicy{Enabled: true, Strict: true},
			want:     true,
		},
		{
			name:     "Strict - No Match",
			hostname: "other.com",
			policy:   config.CertPolicy{Enabled: true, Strict: true},
			want:     false,
		},

		// 2. Loose Policy (Implicit in MatchHostname when !Strict)
		// Assuming looselyMatch implements eTLD+1 matching
		{
			name:     "Loose - Match exact",
			hostname: "example.com",
			policy:   config.CertPolicy{Enabled: true, Strict: false},
			want:     true,
		},
		{
			name:     "Loose - Match sub of match",
			hostname: "sub.example.com", // eTLD+1 is example.com, cert has example.com
			policy:   config.CertPolicy{Enabled: true, Strict: false},
			want:     true,
		},
		{
			name:     "Loose - Match different sub same root",
			hostname: "a.example.com", // matches example.com
			policy:   config.CertPolicy{Enabled: true, Strict: false},
			want:     true,
		},
		{
			name:     "Loose - No Match",
			hostname: "google.com",
			policy:   config.CertPolicy{Enabled: true, Strict: false},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchHostname(cert, tt.hostname, tt.policy); got != tt.want {
				t.Errorf("MatchHostname() = %v, want %v", got, tt.want)
			}
		})
	}
}
