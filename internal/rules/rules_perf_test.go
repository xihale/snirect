package rules

import (
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
)

// referenceLookup implements the specification linear matcher (normalized exact
// match first, then sorted wildcard keys longest-first with lexicographical tie-break).
func referenceLookup[V any](m map[string]V, host string) (V, bool) {
	var zero V
	norm := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if norm == "" {
		return zero, false
	}

	exact := make(map[string]V)
	var wildcards []string
	wildVals := make(map[string]V)
	for k, v := range m {
		nk := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(k), "."))
		if nk == "" {
			continue
		}
		if strings.HasPrefix(nk, "*") {
			wildcards = append(wildcards, nk)
			wildVals[nk] = v
		} else {
			exact[nk] = v
		}
	}
	if v, ok := exact[norm]; ok {
		return v, true
	}
	sort.Slice(wildcards, func(i, j int) bool {
		if len(wildcards[j]) != len(wildcards[i]) {
			return len(wildcards[j]) < len(wildcards[i])
		}
		return wildcards[i] < wildcards[j]
	})
	for _, k := range wildcards {
		if MatchHost(k, norm) {
			return wildVals[k], true
		}
	}
	return zero, false
}

func TestRules_EquivalenceAcrossBuiltinRules(t *testing.T) {
	r, err := LoadRules()
	if err != nil {
		t.Fatalf("LoadRules() error = %v", err)
	}

	// Build a diverse set of candidate hosts derived from every builtin key,
	// exercising exact matches, subdomains, undotted suffix matches, misses,
	// trailing dots, and mixed casing.
	candidates := map[string]struct{}{
		"example.com":                     {},
		"unknown.invalid":                 {},
		"EXAMPLE.COM.":                    {},
		"":                                {},
		".":                               {},
		"   ":                             {},
		"www.youtube.com":                 {},
		"WWW.YOUTUBE.COM.":                {},
		"r1---sn-4g5e6nez.googlevideo.com": {},
		"notyoutube.com":                  {},
		"mygithub.com":                    {},
		"api.github.com.":                 {},
	}

	addFromMap := func(keys []string) {
		for _, k := range keys {
			base := strings.TrimPrefix(strings.TrimPrefix(k, "*."), "*")
			if base == "" {
				continue
			}
			candidates[base] = struct{}{}
			candidates[base+"."] = struct{}{}
			candidates["sub."+base] = struct{}{}
			candidates["a.b."+base] = struct{}{}
			candidates["x"+base] = struct{}{}
			candidates[strings.ToUpper("sub."+base)+". "] = struct{}{}
			candidates[" "+strings.ToUpper(base)+"."] = struct{}{}
		}
	}

	allKeys := make([]string, 0, len(r.AlterHostname)+len(r.Hosts)+len(r.CertVerify)+len(r.IgnoreExpiry))
	for k := range r.AlterHostname {
		allKeys = append(allKeys, k)
	}
	for k := range r.Hosts {
		allKeys = append(allKeys, k)
	}
	for k := range r.CertVerify {
		allKeys = append(allKeys, k)
	}
	for k := range r.IgnoreExpiry {
		allKeys = append(allKeys, k)
	}
	addFromMap(allKeys)

	for host := range candidates {
		gotAlt, gotAltOK := r.GetAlterHostname(host)
		wantAlt, wantAltOK := referenceLookup(r.AlterHostname, host)
		if gotAlt != wantAlt || gotAltOK != wantAltOK {
			t.Fatalf("GetAlterHostname(%q) = (%q, %v), want (%q, %v)", host, gotAlt, gotAltOK, wantAlt, wantAltOK)
		}

		gotHost, gotHostOK := r.GetHost(host)
		wantHost, wantHostOK := referenceLookup(r.Hosts, host)
		if gotHost != wantHost || gotHostOK != wantHostOK {
			t.Fatalf("GetHost(%q) = (%q, %v), want (%q, %v)", host, gotHost, gotHostOK, wantHost, wantHostOK)
		}

		gotCert, gotCertOK := r.GetCertVerify(host)
		wantRawCert, wantCertOK := referenceLookup(r.CertVerify, host)
		var wantCert CertPolicy
		if wantCertOK {
			wantCert, _ = ParseCertPolicy(wantRawCert)
		}
		if gotCertOK != wantCertOK || !reflect.DeepEqual(gotCert, wantCert) {
			t.Fatalf("GetCertVerify(%q) = (%+v, %v), want (%+v, %v)", host, gotCert, gotCertOK, wantCert, wantCertOK)
		}

		gotExp := r.GetIgnoreExpiry(host)
		wantExp, wantExpOK := referenceLookup(r.IgnoreExpiry, host)
		if gotExp != (wantExpOK && wantExp) {
			t.Fatalf("GetIgnoreExpiry(%q) = %v, want %v", host, gotExp, wantExpOK && wantExp)
		}
	}
}

func TestRules_PrecedenceAndNormalizationEdgeCases(t *testing.T) {
	r := NewRules()
	r.AlterHostname["*.example.com"] = "dot-example"
	r.AlterHostname["*example.com"] = "any-example"
	r.AlterHostname["*.sub.example.com"] = "dot-sub"
	r.AlterHostname["*x.example.com"] = "any-x"
	r.AlterHostname["exact.sub.example.com"] = "exact"
	r.AlterHostname["*.a.com"] = "dot-a"   // len 7, '.' (46) < 'b' (98)
	r.AlterHostname["*ba.com"] = "any-ba"  // len 7
	r.AlterHostname["*-a.com"] = "dash-a"  // len 7, '-' (45) < '.' (46)

	r.CertVerify["*.example.com"] = []interface{}{"front.example.net", "*.front.example.net"}
	r.CertVerify["strict.example.com"] = "strict"
	r.CertVerify["off.example.com"] = false

	r.Hosts["*.Example.ORG."] = "1.2.3.4"
	r.IgnoreExpiry["*.expired.test"] = true

	r.Init()

	cases := []struct {
		host    string
		wantAlt string
		wantOK  bool
	}{
		{"exact.sub.example.com", "exact", true},
		{"EXACT.SUB.EXAMPLE.COM.", "exact", true},
		{" exact.sub.example.com. ", "exact", true},
		{"a.sub.example.com", "dot-sub", true},
		{"sub.example.com", "dot-sub", true},
		{"fx.example.com", "any-x", true}, // len("*x.example.com")=14 > len("*.example.com")=13
		{"www.example.com", "dot-example", true},
		{"myexample.com", "any-example", true},
		{"example.com", "dot-example", true}, // len("*.example.com")=13 > len("*example.com")=12
		{"b-a.com", "dash-a", true},
		{"other.invalid", "", false},
		{"", "", false},
		{".", "", false},
	}

	for _, tc := range cases {
		got, ok := r.GetAlterHostname(tc.host)
		if got != tc.wantAlt || ok != tc.wantOK {
			t.Errorf("GetAlterHostname(%q) = (%q, %v), want (%q, %v)", tc.host, got, ok, tc.wantAlt, tc.wantOK)
		}
	}

	// Equal-length tie-breaking check: *ba.com vs *.a.com on "ba.com" (wait: "ba.com" doesn't match *.a.com,
	// whereas "x.a.com" matches *.a.com only; "-a.com" matches *-a.com; test both deterministic sort and lookup).
	if got, ok := r.GetAlterHostname("foo-a.com"); !ok || got != "dash-a" {
		t.Errorf("GetAlterHostname(foo-a.com) = (%q, %v), want (dash-a, true)", got, ok)
	}
	if got, ok := r.GetAlterHostname("xba.com"); !ok || got != "any-ba" {
		t.Errorf("GetAlterHostname(xba.com) = (%q, %v), want (any-ba, true)", got, ok)
	}
	if got, ok := r.GetAlterHostname("sub.a.com"); !ok || got != "dot-a" {
		t.Errorf("GetAlterHostname(sub.a.com) = (%q, %v), want (dot-a, true)", got, ok)
	}

	// Verify getSortedKeys deterministic tie-break.
	keys := getSortedKeys(map[string]int{"*ba.com": 1, "*.a.com": 2, "*-a.com": 3})
	wantOrder := []string{"*-a.com", "*.a.com", "*ba.com"}
	if !reflect.DeepEqual(keys, wantOrder) {
		t.Errorf("getSortedKeys() = %v, want %v", keys, wantOrder)
	}

	// Pre-parsed CertPolicy
	pol, ok := r.GetCertVerify("WWW.EXAMPLE.COM.")
	if !ok || !pol.Enabled || pol.Strict || !reflect.DeepEqual(pol.Allowed, []string{"front.example.net", "*.front.example.net"}) {
		t.Errorf("GetCertVerify(WWW.EXAMPLE.COM.) = (%+v, %v)", pol, ok)
	}
	polStrict, ok := r.GetCertVerify("strict.example.com")
	if !ok || !polStrict.Enabled || !polStrict.Strict {
		t.Errorf("GetCertVerify(strict.example.com) = (%+v, %v)", polStrict, ok)
	}
	polOff, ok := r.GetCertVerify("off.example.com")
	if !ok || polOff.Enabled {
		t.Errorf("GetCertVerify(off.example.com) = (%+v, %v)", polOff, ok)
	}

	// Normalized pattern key in Hosts
	if ip, ok := r.GetHost("sub.example.org."); !ok || ip != "1.2.3.4" {
		t.Errorf("GetHost(sub.example.org.) = (%q, %v), want (1.2.3.4, true)", ip, ok)
	}
	if !r.GetIgnoreExpiry("WWW.EXPIRED.TEST.") {
		t.Error("GetIgnoreExpiry(WWW.EXPIRED.TEST.) = false, want true")
	}
}

func TestRules_UninitializedExactFallback(t *testing.T) {
	r := NewRules()
	r.Hosts["example.com"] = "127.0.0.1"
	r.AlterHostname["example.com"] = "alt.example.com"
	r.CertVerify["example.com"] = false
	r.IgnoreExpiry["example.com"] = true

	if ip, ok := r.GetHost("EXAMPLE.COM."); !ok || ip != "127.0.0.1" {
		t.Fatalf("GetHost uninitialized = (%q, %v)", ip, ok)
	}
	if alt, ok := r.GetAlterHostname("example.com"); !ok || alt != "alt.example.com" {
		t.Fatalf("GetAlterHostname uninitialized = (%q, %v)", alt, ok)
	}
	if pol, ok := r.GetCertVerify("example.com"); !ok || pol.Enabled {
		t.Fatalf("GetCertVerify uninitialized = (%+v, %v)", pol, ok)
	}
	if !r.GetIgnoreExpiry("example.com") {
		t.Fatal("GetIgnoreExpiry uninitialized = false")
	}
}

func TestRules_ConcurrentLoadAndLookupRace(t *testing.T) {
	const workers = 12
	const iters = 200
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				r, err := LoadRules()
				if err != nil {
					t.Errorf("LoadRules() error = %v", err)
					return
				}
				_, _ = r.GetAlterHostname("www.youtube.com.")
				_, _ = r.GetCertVerify("www.youtube.com")
				_, _ = r.GetHost("upload.github.com")
				_ = r.GetIgnoreExpiry("example.com")
			}
		}(w)
	}
	wg.Wait()
}
