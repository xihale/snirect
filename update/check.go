package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const (
	repoOwner     = "xihale"
	repoName      = "snirect"
	maxBody       = 8 << 20   // 8 MiB for JSON / checksums
	maxAssetBytes = 200 << 20 // 200 MiB for a release binary
)

// Info is the result of checking GitHub Releases for a newer build.
type Info struct {
	Current      string
	Latest       string
	Newer        bool
	URL          string
	Notes        string
	AssetName    string
	AssetURL     string
	ChecksumsURL string
}

type ghRelease struct {
	TagName    string `json:"tag_name"`
	HTMLURL    string `json:"html_url"`
	Body       string `json:"body"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	Assets     []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// Check fetches the latest GitHub release and compares it to current.
// goos/goarch select the desktop asset name (ignored for a missing asset:
// Check still succeeds so Android can report "newer" without a matching APK
// field). Draft and prerelease tags are skipped.
func (c *Client) Check(ctx context.Context, current, goos, goarch string) (*Info, error) {
	if c.APIBase == "" {
		c.APIBase = defaultAPIBase
	}
	ua := "snirect/" + current + " (+https://github.com/" + repoOwner + "/" + repoName + ")"
	url := strings.TrimRight(c.APIBase, "/") + "/repos/" + repoOwner + "/" + repoName + "/releases/latest"
	resp, err := c.get(ctx, url, ua)
	if err != nil {
		return nil, fmt.Errorf("check update: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, fmt.Errorf("check update: read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = resp.Status
		}
		return nil, fmt.Errorf("github releases: %s", msg)
	}
	var rel ghRelease
	if err := json.Unmarshal(body, &rel); err != nil {
		return nil, fmt.Errorf("check update: parse release: %w", err)
	}
	if rel.Draft || rel.Prerelease || rel.TagName == "" {
		return nil, fmt.Errorf("no stable release published")
	}

	assetName := AssetName(goos, goarch, rel.TagName)
	info := &Info{
		Current:   current,
		Latest:    rel.TagName,
		Newer:     Newer(rel.TagName, current),
		URL:       rel.HTMLURL,
		Notes:     rel.Body,
		AssetName: assetName,
	}
	for _, a := range rel.Assets {
		switch a.Name {
		case assetName:
			info.AssetURL = a.BrowserDownloadURL
		case "checksums.txt":
			info.ChecksumsURL = a.BrowserDownloadURL
		}
	}
	return info, nil
}

// Check is a convenience wrapper around New().Check.
func Check(ctx context.Context, current, goos, goarch string) (*Info, error) {
	return New().Check(ctx, current, goos, goarch)
}

// AssetName is the release filename for a GOOS/GOARCH pair, matching
// Makefile crossAll / release.yml.
func AssetName(goos, goarch, tag string) string {
	if goos == "android" {
		return "snirect-android-arm64-" + tag + ".apk"
	}
	ext := ""
	if goos == "windows" {
		ext = ".exe"
	}
	return "snirect-" + goos + "-" + goarch + ext
}

// Download fetches info.AssetURL into dest, verifying SHA-256 against
// checksums.txt from the same release. Refuses to install if either the
// checksum file or the matching line is missing.
func (c *Client) Download(ctx context.Context, info *Info, dest string) (err error) {
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

	h := sha256.New()
	var n int64
	n, err = io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, maxAssetBytes+1))
	if err != nil {
		err = fmt.Errorf("download %s: %w", info.AssetName, err)
		return err
	}
	if n > maxAssetBytes {
		err = fmt.Errorf("download %s: exceeds %d bytes", info.AssetName, maxAssetBytes)
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, want) {
		err = fmt.Errorf("sha256 mismatch for %s: got %s want %s", info.AssetName, got, want)
		return err
	}
	return nil
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
