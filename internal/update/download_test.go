package update

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// shrinking the guard windows keeps the stall/crawl tests fast; the defaults
// they restore are what production runs with.
func withGuard(t *testing.T, attempts int, stall, window time.Duration, floor int64) {
	t.Helper()
	sa, ss, sw, sm := downloadAttempts, stallTimeout, rateWindow, minRate
	downloadAttempts, stallTimeout, rateWindow, minRate = attempts, stall, window, floor
	t.Cleanup(func() {
		downloadAttempts, stallTimeout, rateWindow, minRate = sa, ss, sw, sm
	})
}

func downloadServer(t *testing.T, payload []byte, asset func(w http.ResponseWriter, r *http.Request, attempt int), attempts *int, mu *sync.Mutex) *httptest.Server {
	t.Helper()
	sum := sha256.Sum256(payload)
	mux := http.NewServeMux()
	mux.HandleFunc("/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s  snirect-linux-amd64\n", hex.EncodeToString(sum[:]))
	})
	mux.HandleFunc("/asset", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		*attempts++
		n := *attempts
		mu.Unlock()
		asset(w, r, n)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func infoFor(srv *httptest.Server) *Info {
	return &Info{
		Current:      "v1.0.0",
		AssetName:    "snirect-linux-amd64",
		AssetURL:     srv.URL + "/asset",
		ChecksumsURL: srv.URL + "/checksums.txt",
	}
}

// A flow that opens, sends a few bytes and then goes silent must be abandoned
// and retried on a fresh connection instead of hanging the caller forever.
func TestDownloadRetriesStalledFlow(t *testing.T) {
	withGuard(t, 3, 300*time.Millisecond, time.Hour, 0)
	payload := []byte(strings.Repeat("snirect", 4096))
	var mu sync.Mutex
	attempts := 0
	srv := downloadServer(t, payload, func(w http.ResponseWriter, r *http.Request, attempt int) {
		if attempt == 1 {
			w.Header().Set("Content-Length", "1048576")
			_, _ = w.Write(payload[:16])
			w.(http.Flusher).Flush()
			<-r.Context().Done() // hold the flow open, send nothing else
			return
		}
		_, _ = w.Write(payload)
	}, &attempts, &mu)

	var progress strings.Builder
	c := &Client{HTTP: srv.Client(), Progress: &progress}
	dest := filepath.Join(t.TempDir(), "asset")

	start := time.Now()
	if err := c.Download(t.Context(), infoFor(srv), dest); err != nil {
		t.Fatalf("Download: %v", err)
	}
	if took := time.Since(start); took > 5*time.Second {
		t.Errorf("download took %v, stall guard did not fire", took)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Errorf("downloaded %d bytes, want %d", len(got), len(payload))
	}
	if attempts != 2 {
		t.Errorf("attempts = %d, want 2 (one stalled, one good)", attempts)
	}
	out := progress.String()
	if !strings.Contains(out, "retry 1/2") || !strings.Contains(out, "stalled") {
		t.Errorf("progress output missing stall notice:\n%s", out)
	}
	if !strings.Contains(out, "%") || !strings.Contains(out, "KiB/s") {
		t.Errorf("progress output missing percent/rate line:\n%s", out)
	}
}

// A flow that keeps trickling below the rate floor is not dead, but it would
// take tens of minutes: it must be dropped for a fresh connection.
func TestDownloadRejectsCrawlSpeed(t *testing.T) {
	withGuard(t, 1, time.Hour, 300*time.Millisecond, 1<<20)
	payload := []byte("unused")
	var mu sync.Mutex
	attempts := 0
	srv := downloadServer(t, payload, func(w http.ResponseWriter, r *http.Request, _ int) {
		w.Header().Set("Content-Length", "10485760")
		w.(http.Flusher).Flush()
		for {
			if _, err := w.Write(make([]byte, 1024)); err != nil {
				return
			}
			w.(http.Flusher).Flush()
			select {
			case <-r.Context().Done():
				return
			case <-time.After(50 * time.Millisecond):
			}
		}
	}, &attempts, &mu)

	c := &Client{HTTP: srv.Client()}
	err := c.Download(t.Context(), infoFor(srv), filepath.Join(t.TempDir(), "asset"))
	if err == nil || !strings.Contains(err.Error(), "too slow") {
		t.Fatalf("Download error = %v, want crawl-speed rejection", err)
	}
}

// Every attempt stalling must end in an error after the attempt budget, not in
// an infinite wait.
func TestDownloadGivesUpAfterAttempts(t *testing.T) {
	withGuard(t, 2, 200*time.Millisecond, time.Hour, 0)
	payload := []byte("unused")
	var mu sync.Mutex
	attempts := 0
	srv := downloadServer(t, payload, func(w http.ResponseWriter, r *http.Request, _ int) {
		w.Header().Set("Content-Length", "1048576")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}, &attempts, &mu)

	c := &Client{HTTP: srv.Client()}
	dest := filepath.Join(t.TempDir(), "asset")
	err := c.Download(t.Context(), infoFor(srv), dest)
	if err == nil || !strings.Contains(err.Error(), "stalled") {
		t.Fatalf("Download error = %v, want stall error", err)
	}
	if attempts != 2 {
		t.Errorf("attempts = %d, want 2", attempts)
	}
	if _, statErr := os.Stat(dest); statErr == nil {
		t.Error("partial download left on disk")
	}
}
