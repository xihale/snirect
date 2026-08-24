package rules

import (
	"slices"
	"strings"
	"testing"
)

func TestExportDomains(t *testing.T) {
	r := NewRules()
	r.AlterHostname = map[string]string{
		"*github.com":     "",  // suffix
		"api.github.com":  "",  // exact, covered by *github.com
		"*nyaa.si":        "",  // suffix
		"*sukebei.nyaa.si": "", // suffix nested under *nyaa.si
	}
	r.CertVerify = map[string]interface{}{
		"gemini.gstatic.com": false, // union across maps: covered by *gstatic.com below
	}
	r.Hosts = map[string]string{
		"*gstatic.com": "1.2.3.4",
		"*.t.me":       "1.2.3.4",
		"t.me":         "1.2.3.4", // exact, covered by *.t.me
	}
	r.IgnoreExpiry = map[string]bool{
		"*cdn-telegram.org":     true,
		"cdn1.cdn-telegram.org": true, // covered by *cdn-telegram.org
	}
	r.Init()

	suffixes, exacts := r.ExportDomains()

	wantSuffixes := []string{"cdn-telegram.org", "github.com", "gstatic.com", "nyaa.si", "t.me"}
	if !slices.Equal(suffixes, wantSuffixes) {
		t.Errorf("ExportDomains() suffixes = %v, want %v", suffixes, wantSuffixes)
	}
	// Nothing survives on the exact side: every exact key is covered by a suffix.
	if len(exacts) != 0 {
		t.Errorf("ExportDomains() exacts = %v, want empty", exacts)
	}
}

func TestExportDomains_KeepsUncoveredExacts(t *testing.T) {
	// The docker.io family has no suffix rule — snirect only knows specific
	// vhosts. Those must stay exact so the export never routes hosts
	// snirect has no rule for.
	r := NewRules()
	r.Hosts = map[string]string{
		"docker.io":       "1.2.3.4",
		"registry-1.docker.io": "1.2.3.4",
	}
	r.Init()

	suffixes, exacts := r.ExportDomains()

	if len(suffixes) != 0 {
		t.Errorf("ExportDomains() suffixes = %v, want empty", suffixes)
	}
	want := []string{"docker.io", "registry-1.docker.io"}
	if !slices.Equal(exacts, want) {
		t.Errorf("ExportDomains() exacts = %v, want %v", exacts, want)
	}
}

func TestExportDomains_BuiltinSmoke(t *testing.T) {
	r, err := LoadRules()
	if err != nil {
		t.Fatalf("LoadRules() error = %v", err)
	}
	suffixes, exacts := r.ExportDomains()

	if len(suffixes) < 100 || len(exacts) < 10 {
		t.Fatalf("ExportDomains() = %d suffixes, %d exacts; builtin set should be much larger", len(suffixes), len(exacts))
	}

	// docker.io has no suffix rule in the builtin set; its vhost pins must
	// survive minimization as exact entries.
	if !slices.Contains(exacts, "docker.io") {
		t.Errorf("ExportDomains() exacts missing uncovered builtin host docker.io")
	}

	// Invariants after minimization: no duplicates, no exact covered by any
	// suffix, no suffix nested under another, both lists sorted.
	seen := map[string]bool{}
	for _, e := range exacts {
		if seen[e] {
			t.Errorf("duplicate exact %q", e)
		}
		seen[e] = true
		for _, s := range suffixes {
			if e == s || strings.HasSuffix(e, "."+s) {
				t.Errorf("exact %q still covered by suffix %q", e, s)
			}
		}
	}
	for i, s := range suffixes {
		if seen[s] {
			t.Errorf("duplicate suffix %q", s)
		}
		seen[s] = true
		if i > 0 && suffixes[i-1] >= s {
			t.Errorf("suffixes not sorted at %d: %v", i, suffixes)
		}
		for j, w := range suffixes {
			if j != i && strings.HasSuffix(s, "."+w) {
				t.Errorf("suffix %q still nested under %q", s, w)
			}
		}
	}
	if !slices.IsSorted(exacts) {
		t.Errorf("exacts not sorted: %v", exacts)
	}
}

func TestFormatDAE(t *testing.T) {
	// Format* render verbatim; ordering is ExportDomains' job.
	got := FormatDAE([]string{"a.com", "b.com"}, []string{"x.org"}, "mygroup")
	want := "domain(\n" +
		"    suffix: a.com\n" +
		"    suffix: b.com\n" +
		"    full: x.org\n" +
		") -> mygroup\n"
	if got != want {
		t.Errorf("FormatDAE() =\n%q\nwant\n%q", got, want)
	}
}

func TestFormatClash(t *testing.T) {
	got := FormatClash([]string{"a.com", "b.com"}, []string{"x.org"}, "PROXY")
	want := "- DOMAIN-SUFFIX,a.com,PROXY\n" +
		"- DOMAIN-SUFFIX,b.com,PROXY\n" +
		"- DOMAIN,x.org,PROXY\n"
	if got != want {
		t.Errorf("FormatClash() =\n%q\nwant\n%q", got, want)
	}
}
