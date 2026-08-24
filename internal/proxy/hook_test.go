package proxy

import "testing"

func TestTunnelHookForMatchesGitHubHosts(t *testing.T) {
	for _, c := range []struct{ host, sni string }{
		{"github.com", ""},
		{"github.com", "github.com"},
		{"codeload.github.com", "codeload.github.com"},
		{"example.com", "www.github.com"}, // SNI alone is enough
	} {
		if got := tunnelHookFor(c.host, c.sni); got == nil {
			t.Errorf("tunnelHookFor(%q, %q) = nil, want githubHook", c.host, c.sni)
		}
	}
}

func TestTunnelHookForNonMatchingHosts(t *testing.T) {
	for _, c := range []struct{ host, sni string }{
		{"example.com", ""},
		{"notgithub.com", "notgithub.com"}, // exact-match switch, no suffix hits
		{"", ""},                           // empty SNI falls back to host upstream
	} {
		if got := tunnelHookFor(c.host, c.sni); got != nil {
			t.Errorf("tunnelHookFor(%q, %q) = %T, want nil", c.host, c.sni, got)
		}
	}
}

func TestGithubHookContract(t *testing.T) {
	h := tunnelHookFor("github.com", "")
	if h == nil {
		t.Fatal("githubHook not registered")
	}
	alpn := h.pinALPN()
	if len(alpn) != 1 || alpn[0] != "http/1.1" {
		t.Errorf("pinALPN() = %v, want [http/1.1]", alpn)
	}
	if !h.interceptsH1() {
		t.Error("interceptsH1() = false, want true")
	}
}
