package config

import (
	"path"
	"strings"
)

func MatchPattern(pattern, host string) bool {
	// Comment/Ignore prefixes: #, $, or ^ at the start means ignore this pattern
	if strings.HasPrefix(pattern, "#") || strings.HasPrefix(pattern, "$") || strings.HasPrefix(pattern, "^") {
		return false
	}

	// Handle exclusion operator (^) in the middle of the pattern
	// e.g., "*wik*.org^*wiki*edia.org" matches wikinews.org but excludes wikipedia.org
	if idx := strings.Index(pattern, "^"); idx != -1 {
		includePart := pattern[:idx]
		excludePart := pattern[idx+1:]

		// If the host doesn't match the include part, no match
		if !matchInclude(includePart, host) {
			return false
		}

		// If host matches the exclude part, it's excluded
		if excludePart != "" && matchInclude(excludePart, host) {
			return false
		}

		// Host matches include part and is not excluded
		return true
	}

	return matchInclude(pattern, host)
}

// matchInclude performs the actual pattern matching without exclusion logic
func matchInclude(pattern, host string) bool {
	// Special case: *.example.com matches the domain and all subdomains
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
