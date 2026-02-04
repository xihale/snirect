package config

import (
	"path/filepath"
	"strings"
)

func MatchPattern(pattern, host string) bool {
	if strings.HasPrefix(pattern, "#") {
		return false
	}
	pattern = strings.TrimPrefix(pattern, "$")

	if matched, _ := filepath.Match(pattern, host); matched {
		return true
	}

	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[1:]
		if host == suffix[1:] || strings.HasSuffix(host, suffix) {
			return true
		}
	}

	return host == pattern
}
