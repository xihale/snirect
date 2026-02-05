package config

import (
	"path"
	"strings"
)

func MatchPattern(pattern, host string) bool {
	if strings.HasPrefix(pattern, "#") {
		return false
	}

	// Strip strict prefix if present. In Snirect, $ usually indicates an exact match
	// or a specific rule type, but for domain matching we treat the remainder as the pattern.
	pattern = strings.TrimPrefix(pattern, "$")

	// Special case: *.example.com matches example.com and all subdomains (*.example.com)
	if strings.HasPrefix(pattern, "*.") {
		domain := pattern[2:]
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}

	// Standard glob match (handles *example.com, example*, and other wildcards)
	if matched, _ := path.Match(pattern, host); matched {
		return true
	}

	return host == pattern
}
