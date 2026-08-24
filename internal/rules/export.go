package rules

import (
	"sort"
	"strings"
)

// ExportDomains returns the normalized, minimized set of host patterns that
// snirect knows how to handle — the union of all keys across the four rule
// maps (AlterHostname, CertVerify, Hosts, IgnoreExpiry). This is exactly the
// domain set an upstream router (dae, clash, ...) should send snirect's way:
// a host matching no pattern here gains nothing from being routed to snirect.
//
// Patterns are normalized following MatchHost's semantics:
//   - "*.example.com" and the undotted-prefix "*example.com" both become the
//     suffix "example.com" (matches the domain itself and any subdomain);
//   - a bare "example.com" stays exact.
//
// The undotted form technically also matches strings like "notexample.com",
// which DOMAIN-SUFFIX / suffix: do not; upstream rule lists only use it to
// mean "the domain and its subdomains", so the suffix mapping is the right
// one for routing purposes.
//
// The result is minimized so exported lists stay short without changing what
// they cover: a suffix nested under a wider suffix is dropped (sukebei.nyaa.si
// under nyaa.si), and an exact host covered by any suffix is dropped
// (api.github.com under github.com). Exact hosts with no covering suffix —
// e.g. the docker.io family, where snirect only knows specific vhosts — are
// kept, so the export never routes hosts snirect has no rule for. Both lists
// are sorted alphabetically for stable output.
func (r *Rules) ExportDomains() (suffixes, exacts []string) {
	if r == nil {
		return nil, nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	suffixSet := make(map[string]struct{})
	exactSet := make(map[string]struct{})
	normalize := func(pattern string) {
		p := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(pattern, ".")))
		switch {
		case strings.HasPrefix(p, "*."):
			suffixSet[p[2:]] = struct{}{}
		case strings.HasPrefix(p, "*"):
			suffixSet[p[1:]] = struct{}{}
		case p != "":
			exactSet[p] = struct{}{}
		}
	}
	for k := range r.AlterHostname {
		normalize(k)
	}
	for k := range r.CertVerify {
		normalize(k)
	}
	for k := range r.Hosts {
		normalize(k)
	}
	for k := range r.IgnoreExpiry {
		normalize(k)
	}

	suffixes = make([]string, 0, len(suffixSet))
	for s := range suffixSet {
		// Nested under a different, wider suffix? Drop it — matching the
		// wider one already routes every host this one would.
		wider := false
		for w := range suffixSet {
			if w != s && strings.HasSuffix(s, "."+w) {
				wider = true
				break
			}
		}
		if !wider {
			suffixes = append(suffixes, s)
		}
	}

	exacts = make([]string, 0, len(exactSet))
	for e := range exactSet {
		covered := false
		for s := range suffixSet {
			if e == s || strings.HasSuffix(e, "."+s) {
				covered = true
				break
			}
		}
		if !covered {
			exacts = append(exacts, e)
		}
	}

	sort.Strings(suffixes)
	sort.Strings(exacts)
	return suffixes, exacts
}

// FormatDAE renders the domain set as a dae routing block:
//
//	domain(
//	    suffix: github.com
//	    full: docker.io
//	) -> proxy
//
// Conditions inside one domain() are OR-ed by dae, so a single block is all a
// user needs to paste into their routing section. Suffixes come first, then
// exact hosts, each group alphabetically.
func FormatDAE(suffixes, exacts []string, policy string) string {
	var b strings.Builder
	b.WriteString("domain(\n")
	for _, s := range suffixes {
		b.WriteString("    suffix: ")
		b.WriteString(s)
		b.WriteString("\n")
	}
	for _, e := range exacts {
		b.WriteString("    full: ")
		b.WriteString(e)
		b.WriteString("\n")
	}
	b.WriteString(") -> ")
	b.WriteString(policy)
	b.WriteString("\n")
	return b.String()
}

// FormatClash renders the domain set as clash/mihomo rule lines:
//
//	- DOMAIN-SUFFIX,github.com,proxy
//	- DOMAIN,docker.io,proxy
//
// Ready to paste into a rules list. Suffixes come first, then exact hosts,
// each group alphabetically.
func FormatClash(suffixes, exacts []string, policy string) string {
	var b strings.Builder
	for _, s := range suffixes {
		b.WriteString("- DOMAIN-SUFFIX,")
		b.WriteString(s)
		b.WriteString(",")
		b.WriteString(policy)
		b.WriteString("\n")
	}
	for _, e := range exacts {
		b.WriteString("- DOMAIN,")
		b.WriteString(e)
		b.WriteString(",")
		b.WriteString(policy)
		b.WriteString("\n")
	}
	return b.String()
}
