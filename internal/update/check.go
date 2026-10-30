package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	repoOwner       = "xihale"
	repoName        = "snirect"
	maxBody         = 8 << 20   // 8 MiB for JSON / checksums
	maxAssetBytes   = 200 << 20 // 200 MiB for a release binary
	releasesPerPage = 5         // releases fetched per /releases page
	// Same window the old single per_page=30 fetch covered; reaching it
	// means the caller's line has no stable release among the recent 30.
	maxReleaseScans = 30
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

// releaseTagPrefix is the tag line goos releases from. CLI and Android ship
// independently from the same repo: v* for desktop, android-v* for the app,
// so the two lines never force a joint release.
func releaseTagPrefix(goos string) string {
	if goos == "android" {
		return "android-v"
	}
	return "v"
}

// Check fetches the newest GitHub release of the caller's line (v* for CLI,
// android-v* for the app — /releases/latest would return whichever line
// published most recently) and compares it to current. goos/goarch select
// the desktop asset name (ignored for a missing asset: Check still succeeds
// so Android can report "newer" without a matching APK field). Draft and
// prerelease tags are skipped. The listing is paged in releasesPerPage-sized
// chunks so the common case stays a small single request.
func (c *Client) Check(ctx context.Context, current, goos, goarch string) (*Info, error) {
	if c.APIBase == "" {
		c.APIBase = defaultAPIBase
	}
	ua := "snirect/" + current + " (+https://github.com/" + repoOwner + "/" + repoName + ")"
	prefix := releaseTagPrefix(goos)
	rel, err := c.findRelease(ctx, ua, prefix)
	if err != nil {
		return nil, err
	}
	if rel == nil {
		return nil, fmt.Errorf("no stable release published for this line (%s*)", prefix)
	}

	// Report the app line as v1.5.0 (drop the android- discriminator) so the
	// version shown to the user matches the installed versionName.
	tag := rel.TagName
	if goos == "android" {
		tag = strings.TrimPrefix(tag, "android-")
	}

	assetName := AssetName(goos, goarch, tag)
	info := &Info{
		Current:   current,
		Latest:    tag,
		Newer:     Newer(tag, current),
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

// findRelease returns the newest stable release whose tag carries prefix.
// The two release lines interleave in one /releases listing, so the page
// head may hold none of the caller's line: fetch pages of releasesPerPage
// until a match, the end of the list, or the maxReleaseScans window.
func (c *Client) findRelease(ctx context.Context, ua, prefix string) (*ghRelease, error) {
	apiBase := strings.TrimRight(c.APIBase, "/")
	for page := 1; page*releasesPerPage <= maxReleaseScans; page++ {
		url := fmt.Sprintf("%s/repos/%s/%s/releases?per_page=%d&page=%d",
			apiBase, repoOwner, repoName, releasesPerPage, page)
		resp, err := c.get(ctx, url, ua)
		if err != nil {
			return nil, fmt.Errorf("check update: %w", err)
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBody))
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("check update: read body: %w", readErr)
		}
		if resp.StatusCode != http.StatusOK {
			msg := strings.TrimSpace(string(body))
			if msg == "" {
				msg = resp.Status
			}
			return nil, fmt.Errorf("github releases: %s", msg)
		}
		var rels []ghRelease
		if err := json.Unmarshal(body, &rels); err != nil {
			return nil, fmt.Errorf("check update: parse releases: %w", err)
		}
		for i := range rels {
			r := &rels[i]
			if r.Draft || r.Prerelease || r.TagName == "" {
				continue
			}
			if strings.HasPrefix(r.TagName, prefix) {
				return r, nil
			}
		}
		if len(rels) < releasesPerPage {
			break // short page: end of the list
		}
	}
	return nil, nil
}

// Check is a convenience wrapper around New().Check.
func Check(ctx context.Context, current, goos, goarch string) (*Info, error) {
	return New().Check(ctx, current, goos, goarch)
}

// androidABI maps a Go arch to the Android ABI segment of release asset
// names (release-android.yml builds arm64 and x86_64 APKs).
var androidABI = map[string]string{
	"arm64": "arm64",
	"amd64": "x86_64",
	"386":   "x86",
}

// AssetName is the release filename for a GOOS/GOARCH pair, matching
// Makefile crossAll / release-android.yml. Android tags carry an android-
// prefix that the asset name drops. Unknown Android arches get "" — Check
// then reports Newer without a downloadable asset.
func AssetName(goos, goarch, tag string) string {
	if goos == "android" {
		if abi := androidABI[goarch]; abi != "" {
			tag = strings.TrimPrefix(tag, "android-")
			return "snirect-android-" + abi + "-" + tag + ".apk"
		}
		return ""
	}
	ext := ""
	if goos == "windows" {
		ext = ".exe"
	}
	return "snirect-" + goos + "-" + goarch + ext
}
