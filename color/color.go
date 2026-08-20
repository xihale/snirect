// Package color holds the ANSI color/style escape sequences shared by the
// logger console handler and the CLI's UI helper. Keeping them in one place
// avoids the two packages redeclaring the same magic strings (and the same
// NO_COLOR check).
package color

import "os"

// enabled controls whether ANSI escapes are emitted. Honors the de-facto
// NO_COLOR convention (https://no-color.org/).
var enabled = true

func init() {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		enabled = false
	}
}

// ANSI escape sequences.
const (
	Reset  = "\033[0m"
	Bold   = "\033[1m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
	Cyan   = "\033[36m"
	Gray   = "\033[90m"
)

// If returns code when color output is enabled, "" otherwise. Wrap a fragment
// with it: fmt.Printf("%slabel%s", If(Bold), If(Reset)).
func If(code string) string {
	if !enabled {
		return ""
	}
	return code
}

// Enabled reports whether color output is on.
func Enabled() bool { return enabled }
