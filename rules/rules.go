package rules

import (
	"sort"
	"strings"
	"sync"

	"github.com/xihale/snirect/certpolicy"
)

// MatchHost reports whether host matches a rule key.
//
// Supported key forms (covering every key in builtin_rules.go):
//   - exact: "example.com", "1.2.3.4"
//   - "*.example.com": matches example.com itself and any subdomain
//   - "*example.com"  : matches example.com and any string ending in example.com
//     (the undotted-prefix form used by ~260 rules; equivalent to a suffix test
//     on "example.com" so it also covers "www.example.com", "sub.example.com").
//
// Both host and pattern are compared lowercased with a trailing dot stripped.
// Exported so tlsutil's allowlist matcher can share the exact same semantics.
func MatchHost(pattern, host string) bool {
	host = strings.ToLower(strings.TrimSpace(strings.TrimSuffix(host, ".")))
	pattern = strings.ToLower(strings.TrimSpace(strings.TrimSuffix(pattern, ".")))
	if pattern == "" || host == "" {
		return false
	}

	switch {
	case strings.HasPrefix(pattern, "*."):
		domain := pattern[2:]
		return host == domain || strings.HasSuffix(host, "."+domain)
	case strings.HasPrefix(pattern, "*"):
		return strings.HasSuffix(host, pattern[1:])
	default:
		return host == pattern
	}
}

// LoadRules returns the compile-time built-in rules.
//
// builtinRules is a package-level value whose maps are hand-maintained in
// builtin_rules.go. We rebuild the pre-sorted key indexes here so the maps are
// the single source of truth — hand-syncing parallel sorted slices silently
// breaks pattern matching (a map entry without a matching key in the slice is
// invisible to GetCertVerify/GetAlterHostname/GetHost). All Rules accessors
// take a read lock, so the single shared instance is safe to hand out.
func LoadRules() (*Rules, error) {
	builtinRules.Init()
	return builtinRules, nil
}

// Rules represents all rules for SNI spoofing and certificate handling.
type Rules struct {
	mu sync.RWMutex

	// SNI alteration rules: pattern -> target SNI
	AlterHostname map[string]string

	// Certificate verification rules: pattern -> policy
	CertVerify map[string]interface{}

	// Static hosts mapping: pattern -> IP
	Hosts map[string]string

	// Ignore certificate expiry: pattern -> bool
	IgnoreExpiry map[string]bool

	// Pre-computed sorted keys for efficient matching
	alterHostnameKeys []string
	certVerifyKeys    []string
	hostsKeys         []string
	ignoreExpiryKeys  []string
}

// NewRules creates a new empty Rules instance.
func NewRules() *Rules {
	return &Rules{
		AlterHostname: make(map[string]string),
		CertVerify:    make(map[string]interface{}),
		Hosts:         make(map[string]string),
		IgnoreExpiry:  make(map[string]bool),
	}
}

// Init initializes rules for efficient matching.
func (r *Rules) Init() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.initLocked()
}

// initLocked rebuilds sorted keys. Caller must hold r.mu.
func (r *Rules) initLocked() {
	if r.AlterHostname == nil {
		r.AlterHostname = make(map[string]string)
	}
	if r.CertVerify == nil {
		r.CertVerify = make(map[string]interface{})
	}
	if r.Hosts == nil {
		r.Hosts = make(map[string]string)
	}
	if r.IgnoreExpiry == nil {
		r.IgnoreExpiry = make(map[string]bool)
	}

	r.alterHostnameKeys = getSortedKeys(r.AlterHostname)
	r.certVerifyKeys = getSortedKeys(r.CertVerify)
	r.hostsKeys = getSortedKeys(r.Hosts)
	r.ignoreExpiryKeys = getSortedKeys(r.IgnoreExpiry)
}

// getSortedKeys returns keys sorted by length (longest first) for pattern matching.
func getSortedKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// Sort by length descending so more specific patterns match first
	sort.Slice(keys, func(i, j int) bool {
		return len(keys[j]) < len(keys[i])
	})
	return keys
}

// GetAlterHostname returns the target SNI for a host, or false if no rule matches.
func (r *Rules) GetAlterHostname(host string) (string, bool) {
	if r == nil {
		return "", false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Exact match first
	if val, ok := r.AlterHostname[host]; ok {
		return val, true
	}

	// Pattern matching
	for _, k := range r.alterHostnameKeys {
		if MatchHost(k, host) {
			return r.AlterHostname[k], true
		}
	}

	return "", false
}

// GetHost returns the mapped IP for a host, or false if no rule matches.
func (r *Rules) GetHost(host string) (string, bool) {
	if r == nil {
		return "", false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Exact match first
	if val, ok := r.Hosts[host]; ok {
		return val, true
	}

	// Pattern matching
	for _, k := range r.hostsKeys {
		if MatchHost(k, host) {
			return r.Hosts[k], true
		}
	}

	return "", false
}

// GetCertVerify returns the certificate verification policy for a host, or false if no rule matches.
func (r *Rules) GetCertVerify(host string) (certpolicy.CertPolicy, bool) {
	if r == nil {
		return certpolicy.CertPolicy{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Exact match first
	if val, ok := r.CertVerify[host]; ok {
		p, _ := certpolicy.ParseCertPolicy(val)
		return p, true
	}

	// Pattern matching
	for _, k := range r.certVerifyKeys {
		if MatchHost(k, host) {
			p, _ := certpolicy.ParseCertPolicy(r.CertVerify[k])
			return p, true
		}
	}

	return certpolicy.CertPolicy{}, false
}

// GetIgnoreExpiry returns whether certificate expiry should be ignored for a host.
func (r *Rules) GetIgnoreExpiry(host string) bool {
	if r == nil {
		return false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Exact match first
	if val, ok := r.IgnoreExpiry[host]; ok {
		return val
	}

	// Pattern matching
	for _, k := range r.ignoreExpiryKeys {
		if MatchHost(k, host) {
			return r.IgnoreExpiry[k]
		}
	}

	return false
}

// Merge merges another Rules instance into this one.
func (r *Rules) Merge(other *Rules) {
	r.mu.Lock()
	defer r.mu.Unlock()

	other.mu.RLock()
	defer other.mu.RUnlock()

	if r.AlterHostname == nil {
		r.AlterHostname = make(map[string]string)
	}
	if r.CertVerify == nil {
		r.CertVerify = make(map[string]interface{})
	}
	if r.Hosts == nil {
		r.Hosts = make(map[string]string)
	}
	if r.IgnoreExpiry == nil {
		r.IgnoreExpiry = make(map[string]bool)
	}

	for k, v := range other.AlterHostname {
		r.AlterHostname[k] = v
	}
	for k, v := range other.CertVerify {
		r.CertVerify[k] = v
	}
	for k, v := range other.Hosts {
		r.Hosts[k] = v
	}
	for k, v := range other.IgnoreExpiry {
		r.IgnoreExpiry[k] = v
	}

	r.initLocked()
}
