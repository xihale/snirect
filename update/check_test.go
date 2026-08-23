package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewer(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"v1.6.0", "v1.5.0", true},
		{"v1.5.0", "v1.5.0", false},
		{"v1.5.0", "v1.5.0-12-gabc", false},
		{"v1.6.0", "v1.5.0-12-gabc", true},
		{"v1.5.0", "0.0.0-dev", true},
		{"1.5.1", "v1.5.0", true},
		{"v1.4.9", "v1.5.0", false},
		{"v2.0.0", "v1.9.9", true},
		{"", "v1.0.0", false},
		{"v1.0.0", "", true},
		{"v1.0.0-beta", "0.9.0", true},
	}
	for _, tc := range cases {
		got := Newer(tc.latest, tc.current)
		if got != tc.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", tc.latest, tc.current, got, tc.want)
		}
	}
}

func TestAssetName(t *testing.T) {
	cases := []struct {
		goos, goarch, tag, want string
	}{
		{"linux", "amd64", "v1.5.0", "snirect-linux-amd64"},
		{"linux", "arm64", "v1.5.0", "snirect-linux-arm64"},
		{"darwin", "arm64", "v1.5.0", "snirect-darwin-arm64"},
		{"windows", "amd64", "v1.5.0", "snirect-windows-amd64.exe"},
		{"windows", "arm64", "v1.0.0", "snirect-windows-arm64.exe"},
		{"android", "arm64", "v1.5.0", "snirect-android-arm64-v1.5.0.apk"},
	}
	for _, tc := range cases {
		got := AssetName(tc.goos, tc.goarch, tc.tag)
		if got != tc.want {
			t.Errorf("AssetName(%q,%q,%q) = %q, want %q", tc.goos, tc.goarch, tc.tag, got, tc.want)
		}
	}
}

func TestParseChecksums(t *testing.T) {
	const hash = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	text := hash + "  snirect-linux-amd64\n" +
		strings.ToUpper(hash) + " *snirect-windows-amd64.exe\n"
	got, ok := parseChecksums(text, "snirect-linux-amd64")
	if !ok || got != hash {
		t.Errorf("linux: got %q %v", got, ok)
	}
	got, ok = parseChecksums(text, "snirect-windows-amd64.exe")
	if !ok || !strings.EqualFold(got, hash) {
		t.Errorf("windows: got %q %v", got, ok)
	}
	if _, ok := parseChecksums(text, "missing"); ok {
		t.Error("missing name should not match")
	}
}

func TestCheckAndDownload(t *testing.T) {
	payload := []byte("snirect-fake-binary")
	sum := sha256.Sum256(payload)
	hexSum := hex.EncodeToString(sum[:])

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/xihale/snirect/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("missing User-Agent")
		}
		fmt.Fprintf(w, `{
			"tag_name": "v1.5.0",
			"html_url": "http://example/releases/v1.5.0",
			"body": "notes",
			"assets": [
				{"name": "snirect-linux-amd64", "browser_download_url": "http://%s/snirect-linux-amd64"},
				{"name": "checksums.txt", "browser_download_url": "http://%s/checksums.txt"}
			]
		}`, r.Host, r.Host)
	})
	mux.HandleFunc("/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s  snirect-linux-amd64\n", hexSum)
	})
	mux.HandleFunc("/snirect-linux-amd64", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := &Client{APIBase: srv.URL, Do: srv.Client().Do}
	info, err := c.Check(context.Background(), "0.0.0-dev", "linux", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if !info.Newer || info.Latest != "v1.5.0" || info.AssetName != "snirect-linux-amd64" {
		t.Fatalf("unexpected info: %+v", info)
	}
	if info.AssetURL == "" || info.ChecksumsURL == "" {
		t.Fatalf("missing asset urls: %+v", info)
	}

	android, err := c.Check(context.Background(), "0.0.0-dev", "android", "arm64")
	if err != nil {
		t.Fatal(err)
	}
	if !android.Newer || android.AssetURL != "" {
		t.Fatalf("android check should report newer without a matching apk in this fixture: %+v", android)
	}

	dest := filepath.Join(t.TempDir(), "snirect")
	if err := c.Download(context.Background(), info, dest); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("downloaded %q", got)
	}
}

func TestDownloadRejectsBadHash(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "0000000000000000000000000000000000000000000000000000000000000000  snirect-linux-amd64\n")
	})
	mux.HandleFunc("/bin", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "hello")
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := &Client{Do: srv.Client().Do}
	info := &Info{
		Current:      "0.0.0-dev",
		AssetName:    "snirect-linux-amd64",
		AssetURL:     srv.URL + "/bin",
		ChecksumsURL: srv.URL + "/checksums.txt",
	}
	dest := filepath.Join(t.TempDir(), "snirect")
	if err := c.Download(context.Background(), info, dest); err == nil {
		t.Fatal("expected sha256 mismatch")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatal("failed download must not leave dest")
	}
}

func TestCheckSkipsDraft(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"tag_name":"v1.0.0","draft":true}`)
	}))
	defer srv.Close()
	c := &Client{APIBase: srv.URL, Do: srv.Client().Do}
	if _, err := c.Check(context.Background(), "0.0.0", "linux", "amd64"); err == nil {
		t.Fatal("expected error for draft")
	}
}
