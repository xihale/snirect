package config

import (
	"testing"
)

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		host    string
		want    bool
	}{
		// Basic exact match
		{
			name:    "Exact match",
			pattern: "example.com",
			host:    "example.com",
			want:    true,
		},
		{
			name:    "Exact no match",
			pattern: "example.com",
			host:    "www.example.com",
			want:    false,
		},

		// Wildcard at end
		{
			name:    "Wildcard end match",
			pattern: "example*",
			host:    "example.com",
			want:    true,
		},
		{
			name:    "Wildcard end match 2",
			pattern: "example*",
			host:    "example.org",
			want:    true,
		},
		{
			name:    "Wildcard end no match",
			pattern: "example*",
			host:    "test.com",
			want:    false,
		},

		// Wildcard at beginning
		{
			name:    "Wildcard start match",
			pattern: "*example.com",
			host:    "example.com",
			want:    true,
		},
		{
			name:    "Wildcard start match subdomain",
			pattern: "*example.com",
			host:    "www.example.com",
			want:    true,
		},
		{
			name:    "Wildcard start no match different domain",
			pattern: "*example.com",
			host:    "test.com",
			want:    false,
		},

		// *. pattern (subdomain wildcard - matches domain and all subdomains)
		{
			name:    "Dot wildcard match subdomain",
			pattern: "*.example.com",
			host:    "www.example.com",
			want:    true,
		},
		{
			name:    "Dot wildcard match root",
			pattern: "*.example.com",
			host:    "example.com",
			want:    true,
		},
		{
			name:    "Dot wildcard no match different domain",
			pattern: "*.example.com",
			host:    "test.com",
			want:    false,
		},

		// Comment/Ignore prefixes
		{
			name:    "# prefix always ignore",
			pattern: "#example.com",
			host:    "example.com",
			want:    false,
		},
		{
			name:    "$ prefix always ignore",
			pattern: "$example.com",
			host:    "example.com",
			want:    false,
		},
		{
			name:    "^ prefix always ignore",
			pattern: "^example.com",
			host:    "example.com",
			want:    false,
		},

		// Exclusion operator (^)
		{
			name:    "Exclusion match not excluded",
			pattern: "*wik*.org^*wiki*edia.org",
			host:    "wikinews.org",
			want:    true,
		},
		{
			name:    "Exclusion excluded by pattern",
			pattern: "*wik*.org^*wiki*edia.org",
			host:    "wikipedia.org",
			want:    false,
		},
		{
			name:    "Exclusion excluded by subdomain",
			pattern: "*wik*.org^*wiki*edia.org",
			host:    "zh.wikipedia.org",
			want:    false,
		},
		{
			name:    "Exclusion not matched include",
			pattern: "*wik*.org^*wiki*edia.org",
			host:    "test.com",
			want:    false,
		},
		{
			name:    "Exclusion match another",
			pattern: "*wik*.org^*wiki*edia.org",
			host:    "wiktionary.org",
			want:    true,
		},
		{
			name:    "Exclusion with subdomain",
			pattern: "*wik*.org^*wiki*edia.org",
			host:    "en.wiktionary.org",
			want:    true,
		},

		// Exclusion with empty exclude part
		{
			name:    "Exclusion with empty exclude part matches",
			pattern: "*example.com^",
			host:    "test.example.com",
			want:    true,
		},

		// Multiple wildcards
		{
			name:    "Multiple wildcards match",
			pattern: "*a*b*c*",
			host:    "1a2b3c4",
			want:    true,
		},

		// Real-world exclusion patterns from config
		{
			name:    "Yahoo exclusion match allowed",
			pattern: "*.yahoo.com^*.media.yahoo.com",
			host:    "www.yahoo.com",
			want:    true,
		},
		{
			name:    "Yahoo exclusion match subdomain",
			pattern: "*.yahoo.com^*.media.yahoo.com",
			host:    "search.yahoo.com",
			want:    true,
		},
		{
			name:    "Yahoo exclusion excluded media",
			pattern: "*.yahoo.com^*.media.yahoo.com",
			host:    "media.yahoo.com",
			want:    false,
		},
		{
			name:    "Yahoo exclusion excluded media subdomain",
			pattern: "*.yahoo.com^*.media.yahoo.com",
			host:    "images.media.yahoo.com",
			want:    false,
		},
		{
			name:    "Wikimedia exclusion match",
			pattern: "*wikimedia.org^lists.wikimedia.org",
			host:    "upload.wikimedia.org",
			want:    true,
		},
		{
			name:    "Wikimedia exclusion excluded",
			pattern: "*wikimedia.org^lists.wikimedia.org",
			host:    "lists.wikimedia.org",
			want:    false,
		},

		// Middle wildcard (rare but used in config)
		{
			name:    "Middle wildcard match",
			pattern: "disney.*.edge.bamgrid.com",
			host:    "disney.test.edge.bamgrid.com",
			want:    true,
		},
		{
			name:    "Middle wildcard no match",
			pattern: "disney.*.edge.bamgrid.com",
			host:    "disney.edge.bamgrid.com",
			want:    false,
		},

		// Wildcard with subdomain prefix (from config: *.media.tumblr.com)
		{
			name:    "Wildcard with subdomain prefix match subdomain",
			pattern: "*.media.tumblr.com",
			host:    "64.media.tumblr.com",
			want:    true,
		},
		{
			name:    "Wildcard with subdomain prefix match root",
			pattern: "*.media.tumblr.com",
			host:    "media.tumblr.com",
			want:    true,
		},

		// Edge case: wildcard with short parent domain (*.w.wiki from config)
		{
			name:    "Short parent wildcard match subdomain",
			pattern: "*.w.wiki",
			host:    "en.w.wiki",
			want:    true,
		},
		{
			name:    "Short parent wildcard match root",
			pattern: "*.w.wiki",
			host:    "w.wiki",
			want:    true,
		},

		// Multi-level specific subdomains (from config)
		{
			name:    "Multi-level wildcard match subdomain",
			pattern: "*.buy.yahoo.com",
			host:    "store.buy.yahoo.com",
			want:    true,
		},
		{
			name:    "Multi-level wildcard match root",
			pattern: "*.buy.yahoo.com",
			host:    "buy.yahoo.com",
			want:    true,
		},
		{
			name:    "Multi-level wildcard no match parent",
			pattern: "*.buy.yahoo.com",
			host:    "yahoo.com",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchPattern(tt.pattern, tt.host); got != tt.want {
				t.Errorf("MatchPattern(%q, %q) = %v, want %v",
					tt.pattern, tt.host, got, tt.want)
			}
		})
	}
}

func TestMatchInclude(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		host    string
		want    bool
	}{
		// Test basic patterns without exclusion logic
		{
			name:    "Basic exact match",
			pattern: "example.com",
			host:    "example.com",
			want:    true,
		},
		{
			name:    "Dot wildcard exact domain",
			pattern: "*.example.com",
			host:    "example.com",
			want:    true,
		},
		{
			name:    "Dot wildcard subdomain",
			pattern: "*.example.com",
			host:    "www.example.com",
			want:    true,
		},
		{
			name:    "Dot wildcard deep subdomain",
			pattern: "*.example.com",
			host:    "a.b.c.example.com",
			want:    true,
		},
		{
			name:    "Dot wildcard no match different domain",
			pattern: "*.example.com",
			host:    "test.com",
			want:    false,
		},
		{
			name:    "Dot wildcard no match subdomain of different domain",
			pattern: "*.example.com",
			host:    "test.example.org",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchInclude(tt.pattern, tt.host); got != tt.want {
				t.Errorf("matchInclude(%q, %q) = %v, want %v",
					tt.pattern, tt.host, got, tt.want)
			}
		})
	}
}
