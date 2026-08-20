package rules

import (
	"testing"
)

func TestNewRules(t *testing.T) {
	r := NewRules()
	if r == nil {
		t.Fatal("NewRules() returned nil")
	}
	if r.AlterHostname == nil || r.CertVerify == nil || r.Hosts == nil {
		t.Error("NewRules() didn't initialize maps")
	}
}

func TestParseCertPolicy(t *testing.T) {
	tests := []struct {
		name        string
		input       interface{}
		wantOK      bool
		wantEnabled bool
		wantAllow   int
	}{
		{name: "bool", input: true, wantOK: true, wantEnabled: true, wantAllow: 0},
		{name: "disabled skips hostname only", input: false, wantOK: true, wantEnabled: false, wantAllow: 0},
		{name: "strict", input: "strict", wantOK: true, wantEnabled: true, wantAllow: 0},
		{name: "string allow", input: "healthdatanexus.ai", wantOK: true, wantEnabled: true, wantAllow: 1},
		{name: "array allow", input: []interface{}{"a.com", "b.com"}, wantOK: true, wantEnabled: true, wantAllow: 2},
		{name: "invalid", input: 123, wantOK: false, wantEnabled: false, wantAllow: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := ParseCertPolicy(tt.input)
			ok := err == nil
			if ok != tt.wantOK {
				t.Fatalf("ParseCertPolicy() ok=%v want=%v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if p.Enabled != tt.wantEnabled {
				t.Fatalf("ParseCertPolicy() enabled=%v want=%v", p.Enabled, tt.wantEnabled)
			}
			if len(p.Allowed) != tt.wantAllow {
				t.Fatalf("ParseCertPolicy() allow=%d want=%d", len(p.Allowed), tt.wantAllow)
			}
		})
	}
}

func TestLoadRules_ParseOnly(t *testing.T) {
	r, err := LoadRules()
	if err != nil {
		t.Fatalf("LoadRules() error = %v", err)
	}
	if r.AlterHostname == nil || r.CertVerify == nil || r.Hosts == nil {
		t.Fatalf("LoadRules() should initialize all rule maps")
	}
}

func TestLoadRules_RealHostsPresent(t *testing.T) {
	r, err := LoadRules()
	if err != nil {
		t.Fatalf("LoadRules() error = %v", err)
	}

	if got, ok := r.GetHost("store.steampowered.com"); !ok || got == "" {
		t.Fatalf("LoadRules() should have real host for store.steampowered.com, got %q, ok=%v", got, ok)
	}
}

// TestLoadRules_FrontedHostsHaveAllowlist is the regression guard for the
// domain-fronting cases surfaced (and fixed) by `snirect doctor`. Each entry
// below rewrites SNI to a front domain whose cert does NOT name the original
// host; without a CertVerify allowlist the handshake is rejected — exactly the
// pixiv failure pattern. The expected Allowed set comes from the live leaf-cert
// probe done during the doctor investigation.
//
// This is a positive-presence assertion rather than a full scan because the
// predicate "this rewrite needs an allowlist" is not statically decidable:
// `*netflix.com → netflix.com` rewrites SNI but the cert still names the host,
// while `*pixiv.net → pixivision.net` does not. doctor --check-config is the
// runtime equivalent that decides via live probe; this test just locks in the
// cases we already confirmed.
func TestLoadRules_FrontedHostsHaveAllowlist(t *testing.T) {
	r, err := LoadRules()
	if err != nil {
		t.Fatalf("LoadRules() error = %v", err)
	}

	// Each case: a concrete host that the CertVerify must cover, plus the
	// expected front-cert allow domains. `target` is optional — when non-empty it
	// also asserts the AlterHostname SNI, so a silent rule regression is caught.
	cases := []struct {
		host   string
		target string
		allow  []string
	}{
		// A class — confirmed front certs via insecure probe.
		{"www.pixiv.net", "pixivision.net", []string{"*.pixivision.net", "pixivision.net"}},
		{"www.fanbox.cc", "pixivision.net", []string{"*.pixivision.net", "pixivision.net"}},
		{"www.nicovideo.jp", "", []string{"*.cloudfront.net", "cloudfront.net"}},
		{"www.audiomack.com", "", []string{"*.cloudfront.net", "cloudfront.net"}},
		{"www.twitch.tv", "", []string{"*.twitch.tv", "twitch.tv", "*.cloudfront.net", "cloudfront.net"}},
		{"www.bbc.co.uk", "", []string{"*.cdn.cyberarena.at", "cdn.cyberarena.at"}},
		{"www.bbci.co.uk", "", []string{"*.cdn.cyberarena.at", "cdn.cyberarena.at"}},
		{"www.cdn-telegram.org", "", []string{"*.cdn-telegram.org", "cdn-telegram.org", "*.telegram.org", "telegram.org"}},
		{"www.t.me", "", []string{"*.telegram.org", "telegram.org"}},
		{"www.instagr.am", "", []string{"*.instagram.com", "*.cdninstagram.com", "*.igsonar.com", "cdninstagram.com", "igsonar.com", "instagram.com"}},
		{"www.ig.me", "", []string{"*.instagram.com", "*.cdninstagram.com", "*.igsonar.com", "cdninstagram.com", "igsonar.com", "instagram.com"}},
		{"www.mega.io", "", []string{"*.static.mega.co.nz", "static.mega.co.nz"}},
		{"www.ok.ru", "", []string{"*.ok.ru", "ok.ru"}},
		{"www.z-lib.help", "", []string{"*.go-to-library.sk", "go-to-library.sk"}},
		{"www.z-library.sk", "", []string{"*.go-to-library.sk", "go-to-library.sk"}},
		{"www.proton.me", "", []string{"*.proton.me", "proton.me", "*.pr.tn", "pr.tn"}},
		{"api.fanbox.cc", "api.fanbox.cc", []string{"*.fanbox.cc", "fanbox.cc"}},

		// B class — stale allowlist corrected: youtube/ggpht/ytimg used to point at
		// healthdatanexus.ai (a dead front); the real Google front cert is *.google.cn.
		// ggpht/ytimg have dotted/exact AlterHostname keys (*.ggpht.com, i.ytimg.com)
		// but undotted CertVerify wildcards (*ggpht.com / *ytimg.com) that match both
		// bare and one-level subdomains, so we assert against a representative host.
		{"www.youtube.com", "g.cn", []string{"*.google.cn", "google.cn"}},
		{"www.ggpht.com", "g.cn", []string{"*.google.cn", "google.cn"}},
		{"i.ytimg.com", "g.cn", []string{"*.google.cn", "google.cn"}},
	}

	for _, tc := range cases {
		t.Run(tc.host, func(t *testing.T) {
			if tc.target != "" {
				gotTarget, ok := r.GetAlterHostname(tc.host)
				if !ok {
					t.Fatalf("no AlterHostname rule for %s", tc.host)
				}
				if gotTarget != tc.target {
					t.Fatalf("AlterHostname(%s) = %q, want %q", tc.host, gotTarget, tc.target)
				}
			}

			policy, ok := r.GetCertVerify(tc.host)
			if !ok {
				t.Fatalf("no CertVerify allowlist for %s — handshake will fail "+
					"because the front cert does not name %s", tc.host, tc.host)
			}
			if !policy.Enabled {
				t.Fatalf("CertVerify(%s) is disabled, want enabled with allowlist", tc.host)
			}
			if len(policy.Allowed) == 0 {
				t.Fatalf("CertVerify(%s) has empty Allowed — front cert will be rejected", tc.host)
			}
			want := map[string]bool{}
			for _, a := range tc.allow {
				want[a] = true
			}
			for _, a := range policy.Allowed {
				delete(want, a)
			}
			if len(want) != 0 {
				t.Fatalf("CertVerify(%s) missing expected allow domains %v, got %v",
					tc.host, want, policy.Allowed)
			}
		})
	}
}

func TestMerge_RebuildsIndexes(t *testing.T) {
	base := NewRules()
	base.AlterHostname["example.com"] = "target.com"
	base.Init()

	override := NewRules()
	override.Hosts["api.example.com"] = "1.2.3.4"
	override.CertVerify["*.example.com"] = "strict"
	override.Init()

	base.Merge(override)

	if got, ok := base.GetAlterHostname("example.com"); !ok || got != "target.com" {
		t.Fatalf("merge should retain existing alter hostname, got %q ok=%v", got, ok)
	}
	if got, ok := base.GetHost("api.example.com"); !ok || got != "1.2.3.4" {
		t.Fatalf("merge should expose merged host, got %q ok=%v", got, ok)
	}
	if got, ok := base.GetCertVerify("foo.example.com"); !ok || !got.Enabled || !got.Strict {
		t.Fatalf("merge should rebuild cert verify indexes, got %+v ok=%v", got, ok)
	}
}
