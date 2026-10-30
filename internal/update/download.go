package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

// A release asset is fetched through one pinned IP with an empty SNI, so the
// whole download rides a single TCP flow. That flow can land on a distant,
// lossy Fastly POP: measured from a CN uplink, 240 ms RTT / 4% segment loss /
// ~10 KB/s, while a fresh connection to the same IP moved the same 11 MiB at
// 30 MB/s. Such an attempt is not dead — bytes keep trickling — it is just
// hopeless, so it shows up as an indefinite frozen "downloading..." line.
// The guard below aborts it and the next attempt opens a new flow, which is a
// fresh draw on the POP lottery.
//
// Variables rather than constants so tests can shrink the windows.
var (
	downloadAttempts = 3
	stallTimeout     = 20 * time.Second // no byte for this long: attempt is dead
	rateWindow       = 8 * time.Second  // average-rate floor applies after this
	// 128 KiB/s is the average floor: the worst accepted attempt still moves an
	// 11 MiB asset in ~90 s, so three of them fit the CLI deadline, while the
	// measured bad flows (10-90 KiB/s) are rejected outright.
	minRate = int64(128 << 10)
)

// Download fetches info.AssetURL into dest, verifying SHA-256 against
// checksums.txt from the same release. Refuses to install if either the
// checksum file or the matching line is missing. Each attempt is guarded
// against stalls and crawl-speed flows; a failed attempt is retried on a
// fresh connection, and unused connections are dropped between attempts.
func (c *Client) Download(ctx context.Context, info *Info, dest string) error {
	if info == nil || info.AssetURL == "" {
		return fmt.Errorf("no release asset for this platform")
	}
	if info.ChecksumsURL == "" {
		return fmt.Errorf("release has no checksums.txt")
	}
	ua := "snirect/" + info.Current + " (+https://github.com/" + repoOwner + "/" + repoName + ")"

	want, err := c.fetchChecksum(ctx, info.ChecksumsURL, info.AssetName, ua)
	if err != nil {
		return err
	}

	for attempt := 1; ; attempt++ {
		err = c.downloadAttempt(ctx, info, dest, want, ua)
		if err == nil {
			return nil
		}
		if attempt >= downloadAttempts || ctx.Err() != nil {
			return err
		}
		c.progressf("\n  retry %d/%d after: %v\n", attempt, downloadAttempts-1, err)
		c.closeIdle()
	}
}

// downloadAttempt performs one guarded fetch of the asset into dest.
func (c *Client) downloadAttempt(parent context.Context, info *Info, dest, want, ua string) (err error) {
	ctx, cancel := context.WithCancelCause(parent)
	defer cancel(nil)

	resp, err := c.get(ctx, info.AssetURL, ua)
	if err != nil {
		return fmt.Errorf("download %s: %w", info.AssetName, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", info.AssetName, resp.Status)
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
		if err != nil {
			_ = os.Remove(dest)
		}
	}()

	st := &assetStream{out: c.Progress, total: resp.ContentLength, start: time.Now()}
	st.last.Store(st.start.UnixNano())
	stop, guardDone := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(guardDone)
		guardflow(ctx, cancel, st, stop)
	}()

	h := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(f, h), io.LimitReader(&countingReader{r: resp.Body, st: st}, maxAssetBytes+1))
	close(stop)
	<-guardDone
	st.finish()

	if cause := context.Cause(ctx); cause != nil && copyErr != nil {
		return fmt.Errorf("download %s: %w", info.AssetName, cause)
	}
	if copyErr != nil {
		return fmt.Errorf("download %s: %w", info.AssetName, copyErr)
	}
	if n > maxAssetBytes {
		return fmt.Errorf("download %s: exceeds %d bytes", info.AssetName, maxAssetBytes)
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("sha256 mismatch for %s: got %s want %s", info.AssetName, got, want)
	}
	return nil
}

// guardflow cancels the attempt when the stream stalls or crawls. It also
// renders progress, so a slow-but-healthy download is never a silent freeze.
func guardflow(ctx context.Context, cancel context.CancelCauseFunc, s *assetStream, stop <-chan struct{}) {
	t := time.NewTicker(250 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ctx.Done():
			return
		case now := <-t.C:
			n := s.done.Load()
			if now.Sub(time.Unix(0, s.last.Load())) > stallTimeout {
				cancel(fmt.Errorf("stalled: no data for %v", stallTimeout))
				return
			}
			if el := now.Sub(s.start); el >= rateWindow {
				if rate := float64(n) / el.Seconds(); rate < float64(minRate) {
					cancel(fmt.Errorf("too slow: %.0f KiB/s over %v, floor is %.0f KiB/s",
						rate/1024, el.Round(time.Second), float64(minRate)/1024))
					return
				}
			}
			s.show(now, n)
		}
	}
}

// assetStream is the progress state shared by the copy loop and guardflow.
type assetStream struct {
	out   io.Writer
	total int64
	start time.Time
	done  atomic.Int64
	last  atomic.Int64 // unix nanos of the last received byte
	shown atomic.Bool
}

type countingReader struct {
	r  io.Reader
	st *assetStream
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	if n > 0 {
		c.st.done.Add(int64(n))
		c.st.last.Store(time.Now().UnixNano())
	}
	return n, err
}

func (s *assetStream) show(now time.Time, done int64) {
	if s.out == nil {
		return
	}
	rate := float64(done) / 1024 / max(now.Sub(s.start).Seconds(), 0.001)
	if s.total > 0 {
		fmt.Fprintf(s.out, "\r  %5.1f%%  %6.1f/%.1f MiB  %.0f KiB/s",
			100*float64(done)/float64(s.total), mib(done), mib(s.total), rate)
	} else {
		fmt.Fprintf(s.out, "\r  %.1f MiB  %.0f KiB/s", mib(done), rate)
	}
	s.shown.Store(true)
}

// finish terminates the progress line so following output starts clean.
func (s *assetStream) finish() {
	if s.shown.Load() {
		fmt.Fprintln(s.out)
	}
}

func mib(n int64) float64 { return float64(n) / (1 << 20) }

func (c *Client) progressf(format string, args ...any) {
	if c.Progress == nil {
		return
	}
	fmt.Fprintf(c.Progress, format, args...)
}

// closeIdle drops pooled connections so a retry redials instead of reusing the
// flow that just proved to be bad.
func (c *Client) closeIdle() {
	if c.HTTP == nil {
		return
	}
	if t, ok := c.HTTP.Transport.(interface{ CloseIdleConnections() }); ok {
		t.CloseIdleConnections()
	}
}

func (c *Client) fetchChecksum(ctx context.Context, url, name, ua string) (string, error) {
	resp, err := c.get(ctx, url, ua)
	if err != nil {
		return "", fmt.Errorf("download checksums.txt: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return "", fmt.Errorf("download checksums.txt: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download checksums.txt: %s", resp.Status)
	}
	sum, ok := parseChecksums(string(body), name)
	if !ok {
		return "", fmt.Errorf("checksums.txt has no entry for %s", name)
	}
	return sum, nil
}

func parseChecksums(text, name string) (string, bool) {
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		file := strings.TrimPrefix(fields[1], "*")
		if file == name && len(fields[0]) == 64 {
			return fields[0], true
		}
	}
	return "", false
}
