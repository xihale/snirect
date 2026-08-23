package update

import (
	"strconv"
	"strings"
)

type ver struct {
	major, minor, patch int
}

// parseVer takes a git-describe or release tag ("v1.5.0", "1.5.0-12-gabc",
// "0.0.0-dev") and returns the leading major.minor.patch. Extra pre-release
// / describe suffixes are ignored so a build sitting on v1.5.0-12-gabc is
// treated as 1.5.0, not as older than v1.5.0.
func parseVer(s string) (ver, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "v")
	s = strings.TrimPrefix(s, "V")
	if s == "" {
		return ver{}, false
	}
	head, _, _ := strings.Cut(s, "-")
	head, _, _ = strings.Cut(head, "+")
	parts := strings.Split(head, ".")
	if len(parts) == 0 || parts[0] == "" {
		return ver{}, false
	}
	var nums [3]int
	for i := 0; i < 3 && i < len(parts); i++ {
		n, err := strconv.Atoi(parts[i])
		if err != nil {
			return ver{}, false
		}
		nums[i] = n
	}
	return ver{nums[0], nums[1], nums[2]}, true
}

// Newer reports whether latest is a higher major.minor.patch than current.
// An unparseable current (empty, garbage) is treated as older than a real
// latest. An unparseable latest is never newer.
func Newer(latest, current string) bool {
	l, ok := parseVer(latest)
	if !ok {
		return false
	}
	c, ok := parseVer(current)
	if !ok {
		return true
	}
	if l.major != c.major {
		return l.major > c.major
	}
	if l.minor != c.minor {
		return l.minor > c.minor
	}
	return l.patch > c.patch
}
