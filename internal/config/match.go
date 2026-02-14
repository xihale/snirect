package config

import patternlib "github.com/xihale/snirect-shared/pattern"

// MatchPattern delegates to the shared pattern matching library
func MatchPattern(pattern, host string) bool {
	return patternlib.MatchPattern(pattern, host)
}
