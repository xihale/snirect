package rules

import (
	"sort"
	"strings"
	"sync"
)

// normalizeHost normalizes a hostname by trimming a trailing dot, trimming
// whitespace, and lowercasing. Clean lowercase ASCII hosts (with or without
// a single trailing dot) take a zero-allocation fast path.
func normalizeHost(s string) string {
	orig := s
	if len(s) > 0 && s[len(s)-1] == '.' {
		s = s[:len(s)-1]
	}
	n := len(s)
	if n == 0 {
		return ""
	}
	if s[0] <= ' ' || s[n-1] <= ' ' {
		return normalizeHostSlow(orig)
	}
	for i := 0; i < n; i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || c >= 0x80 {
			return normalizeHostSlow(orig)
		}
	}
	return s
}

func normalizeHostSlow(s string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(s), "."))
}

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
	host = normalizeHost(host)
	pattern = normalizeHost(pattern)
	if pattern == "" || host == "" {
		return false
	}

	switch {
	case strings.HasPrefix(pattern, "*."):
		domain := pattern[2:]
		return host == domain || (len(host) > len(domain) && host[len(host)-len(domain)-1] == '.' && strings.HasSuffix(host, domain))
	case strings.HasPrefix(pattern, "*"):
		return strings.HasSuffix(host, pattern[1:])
	default:
		return host == pattern
	}
}

var builtinOnce sync.Once

// LoadRules returns the compile-time built-in rules.
//
// builtinRules is a package-level value whose maps are hand-maintained in
// builtin_rules.go. We build the exact maps and reverse-suffix wildcard tries
// once via sync.Once so concurrent callers pay zero initialization overhead.
func LoadRules() (*Rules, error) {
	builtinOnce.Do(builtinRules.Init)
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

	// Pre-computed exact maps and reverse-suffix wildcard tries built in Init().
	initialized bool
	alterIdx    ruleIndex[string]
	certIdx     ruleIndex[CertPolicy]
	hostsIdx    ruleIndex[string]
	ignoreIdx   ruleIndex[bool]
}

type ruleIndex[T any] struct {
	exact map[string]T
	root  *trieNode[T]
	items []wildcardItem[T]
}

type wildcardItem[T any] struct {
	pattern string
	value   T
}

type trieEdge[T any] struct {
	b    byte
	node *trieNode[T]
}

type trieNode[T any] struct {
	anyRank  int
	dotRank  int
	oneByte  byte
	oneChild *trieNode[T]
	ascii    *[128]*trieNode[T]
	nonASCII []trieEdge[T]
}

func newTrieNode[T any]() *trieNode[T] {
	return &trieNode[T]{
		anyRank: -1,
		dotRank: -1,
	}
}

func (n *trieNode[T]) getOrAddChild(c byte) *trieNode[T] {
	if n.oneChild == nil && n.ascii == nil && len(n.nonASCII) == 0 {
		child := newTrieNode[T]()
		n.oneByte = c
		n.oneChild = child
		return child
	}
	if n.oneChild != nil {
		if n.oneByte == c {
			return n.oneChild
		}
		// Promote single-child fast path into ASCII table / nonASCII slice.
		existingByte, existingChild := n.oneByte, n.oneChild
		n.oneByte = 0
		n.oneChild = nil
		n.insertChild(existingByte, existingChild)
	}
	if c < 128 {
		if n.ascii == nil {
			n.ascii = new([128]*trieNode[T])
		}
		if child := n.ascii[c]; child != nil {
			return child
		}
		child := newTrieNode[T]()
		n.ascii[c] = child
		return child
	}
	for _, e := range n.nonASCII {
		if e.b == c {
			return e.node
		}
	}
	child := newTrieNode[T]()
	n.nonASCII = append(n.nonASCII, trieEdge[T]{b: c, node: child})
	return child
}

func (n *trieNode[T]) insertChild(c byte, child *trieNode[T]) {
	if c < 128 {
		if n.ascii == nil {
			n.ascii = new([128]*trieNode[T])
		}
		n.ascii[c] = child
		return
	}
	n.nonASCII = append(n.nonASCII, trieEdge[T]{b: c, node: child})
}

func (n *trieNode[T]) findNonASCII(c byte) *trieNode[T] {
	for _, e := range n.nonASCII {
		if e.b == c {
			return e.node
		}
	}
	return nil
}

func buildRuleIndex[V any, T any](m map[string]V, transform func(V) T) ruleIndex[T] {
	rawKeys := make([]string, 0, len(m))
	for k := range m {
		rawKeys = append(rawKeys, k)
	}
	sort.Strings(rawKeys)

	exact := make(map[string]T)
	var wildcards []wildcardItem[T]
	for _, k := range rawKeys {
		norm := normalizeHost(k)
		if norm == "" {
			continue
		}
		val := transform(m[k])
		if strings.HasPrefix(norm, "*") {
			wildcards = append(wildcards, wildcardItem[T]{pattern: norm, value: val})
		} else {
			exact[norm] = val
		}
	}

	sort.Slice(wildcards, func(i, j int) bool {
		if len(wildcards[i].pattern) != len(wildcards[j].pattern) {
			return len(wildcards[j].pattern) < len(wildcards[i].pattern)
		}
		return wildcards[i].pattern < wildcards[j].pattern
	})

	idx := ruleIndex[T]{
		exact: exact,
		items: wildcards,
	}
	if len(wildcards) == 0 {
		return idx
	}

	root := newTrieNode[T]()
	for rank, item := range wildcards {
		var suffix string
		var isDot bool
		if strings.HasPrefix(item.pattern, "*.") {
			suffix = item.pattern[2:]
			isDot = true
		} else {
			suffix = item.pattern[1:]
		}
		node := root
		for p := len(suffix) - 1; p >= 0; p-- {
			node = node.getOrAddChild(suffix[p])
		}
		if isDot {
			if node.dotRank < 0 || rank < node.dotRank {
				node.dotRank = rank
			}
		} else {
			if node.anyRank < 0 || rank < node.anyRank {
				node.anyRank = rank
			}
		}
	}
	idx.root = root
	return idx
}

func (idx *ruleIndex[T]) lookup(host string) (T, bool) {
	var zero T
	if host == "" {
		return zero, false
	}
	if v, ok := idx.exact[host]; ok {
		return v, true
	}
	if idx.root == nil {
		return zero, false
	}

	node := idx.root
	bestRank := len(idx.items)
	if node.anyRank >= 0 && node.anyRank < bestRank {
		bestRank = node.anyRank
	}
	if node.dotRank >= 0 && host[len(host)-1] == '.' && node.dotRank < bestRank {
		bestRank = node.dotRank
	}

	for k := len(host) - 1; k >= 0; k-- {
		c := host[k]
		if node.oneChild != nil && node.oneByte == c {
			node = node.oneChild
		} else if node.ascii != nil && c < 128 {
			node = node.ascii[c]
		} else {
			node = node.findNonASCII(c)
		}
		if node == nil {
			break
		}
		if node.anyRank >= 0 && node.anyRank < bestRank {
			bestRank = node.anyRank
		}
		if node.dotRank >= 0 && (k == 0 || host[k-1] == '.') && node.dotRank < bestRank {
			bestRank = node.dotRank
		}
	}

	if bestRank < len(idx.items) {
		return idx.items[bestRank].value, true
	}
	return zero, false
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

// initLocked rebuilds exact maps and wildcard tries. Caller must hold r.mu.
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

	idStr := func(v string) string { return v }
	idBool := func(v bool) bool { return v }
	parseCert := func(v interface{}) CertPolicy {
		p, _ := ParseCertPolicy(v)
		return p
	}

	r.alterIdx = buildRuleIndex(r.AlterHostname, idStr)
	r.certIdx = buildRuleIndex(r.CertVerify, parseCert)
	r.hostsIdx = buildRuleIndex(r.Hosts, idStr)
	r.ignoreIdx = buildRuleIndex(r.IgnoreExpiry, idBool)
	r.initialized = true
}

// getSortedKeys returns keys sorted by length (longest first), with
// deterministic lexicographical tie-breaking for equal lengths.
func getSortedKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[j]) != len(keys[i]) {
			return len(keys[j]) < len(keys[i])
		}
		return keys[i] < keys[j]
	})
	return keys
}

// GetAlterHostname returns the target SNI for a host, or false if no rule matches.
func (r *Rules) GetAlterHostname(host string) (string, bool) {
	if r == nil {
		return "", false
	}
	norm := normalizeHost(host)
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.initialized {
		return r.alterIdx.lookup(norm)
	}
	if norm != "" {
		if val, ok := r.AlterHostname[norm]; ok {
			return val, true
		}
	}
	if val, ok := r.AlterHostname[host]; ok {
		return val, true
	}
	return "", false
}

// GetHost returns the mapped IP for a host, or false if no rule matches.
func (r *Rules) GetHost(host string) (string, bool) {
	if r == nil {
		return "", false
	}
	norm := normalizeHost(host)
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.initialized {
		return r.hostsIdx.lookup(norm)
	}
	if norm != "" {
		if val, ok := r.Hosts[norm]; ok {
			return val, true
		}
	}
	if val, ok := r.Hosts[host]; ok {
		return val, true
	}
	return "", false
}

// GetCertVerify returns the certificate verification policy for a host, or false if no rule matches.
func (r *Rules) GetCertVerify(host string) (CertPolicy, bool) {
	if r == nil {
		return CertPolicy{}, false
	}
	norm := normalizeHost(host)
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.initialized {
		return r.certIdx.lookup(norm)
	}
	if norm != "" {
		if val, ok := r.CertVerify[norm]; ok {
			p, _ := ParseCertPolicy(val)
			return p, true
		}
	}
	if val, ok := r.CertVerify[host]; ok {
		p, _ := ParseCertPolicy(val)
		return p, true
	}
	return CertPolicy{}, false
}

// GetIgnoreExpiry returns whether certificate expiry should be ignored for a host.
func (r *Rules) GetIgnoreExpiry(host string) bool {
	if r == nil {
		return false
	}
	norm := normalizeHost(host)
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.initialized {
		v, _ := r.ignoreIdx.lookup(norm)
		return v
	}
	if norm != "" {
		if val, ok := r.IgnoreExpiry[norm]; ok {
			return val
		}
	}
	if val, ok := r.IgnoreExpiry[host]; ok {
		return val
	}
	return false
}
