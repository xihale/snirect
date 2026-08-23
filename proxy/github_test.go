package proxy

import "testing"

func TestIsGitHubHTTPHost(t *testing.T) {
	for _, h := range []string{"github.com", "www.github.com", "codeload.github.com", "GITHUB.COM"} {
		if !isGitHubHTTPHost(h) {
			t.Errorf("isGitHubHTTPHost(%q) = false", h)
		}
	}
	for _, h := range []string{"api.github.com", "raw.githubusercontent.com", "example.com", ""} {
		if isGitHubHTTPHost(h) {
			t.Errorf("isGitHubHTTPHost(%q) = true", h)
		}
	}
}

func TestIsCodeloadBlobPath(t *testing.T) {
	yes := []string{
		"/owner/repo/tar.gz/refs/tags/v1.0",
		"/owner/repo/zip/refs/heads/main",
		"/owner/repo/legacy.tar.gz/deadbeef",
		"/a/b/tarball/main",
	}
	for _, p := range yes {
		if !isCodeloadBlobPath(p) {
			t.Errorf("isCodeloadBlobPath(%q) = false", p)
		}
	}
	no := []string{
		"/owner/repo",
		"/owner/repo/archive/v1.0/repo-v1.0.tar.gz",
		"/owner/repo/releases/download/v1.0/file.tar.xz",
		"/login",
		"/",
		"",
	}
	for _, p := range no {
		if isCodeloadBlobPath(p) {
			t.Errorf("isCodeloadBlobPath(%q) = true", p)
		}
	}
}

func TestInternableRedirect(t *testing.T) {
	cases := []struct {
		loc  string
		host string
		path string
		ok   bool
	}{
		{
			"https://codeload.github.com/owner/repo/tar.gz/refs/tags/v1.0",
			"codeload.github.com", "/owner/repo/tar.gz/refs/tags/v1.0", true,
		},
		{
			"https://github.com/owner/repo/tar.gz/refs/tags/v1.0",
			"codeload.github.com", "/owner/repo/tar.gz/refs/tags/v1.0", true,
		},
		{
			"https://objects.githubusercontent.com/blob/1?sig=1",
			"objects.githubusercontent.com", "/blob/1?sig=1", true,
		},
		{"https://github.com/owner/repo", "", "", false},
		{"https://github.com/login", "", "", false},
		{"/relative", "", "", false},
		{"", "", "", false},
	}
	for _, tc := range cases {
		host, path, ok := internableRedirect(tc.loc)
		if ok != tc.ok || host != tc.host || path != tc.path {
			t.Errorf("internableRedirect(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tc.loc, host, path, ok, tc.host, tc.path, tc.ok)
		}
	}
}

func TestInternableRequest(t *testing.T) {
	host, path, ok := internableRequest("github.com", "/owner/repo/archive/v1.0/repo-v1.0.tar.gz")
	if ok {
		t.Errorf("archive URL should 302 via github.com, not intern immediately, got %s %s", host, path)
	}
	host, path, ok = internableRequest("github.com", "/owner/repo/tar.gz/refs/tags/v1.0")
	if !ok || host != "codeload.github.com" || path != "/owner/repo/tar.gz/refs/tags/v1.0" {
		t.Errorf("broken 301 target: got (%q, %q, %v)", host, path, ok)
	}
	if _, _, ok := internableRequest("codeload.github.com", "/owner/repo/tar.gz/refs/tags/v1.0"); ok {
		t.Error("direct codeload blob should pass through the existing empty-SNI hop")
	}
	if _, _, ok := internableRequest("github.com", "/owner/repo"); ok {
		t.Error("repo HTML should not intern")
	}
}
