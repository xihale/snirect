package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/xihale/snirect-shared/rules"
	"golang.org/x/mod/semver"
	"snirect/internal/config"
	"snirect/internal/logger"
	"snirect/internal/upstream"
)

// parsedVersion represents a version string broken into components.
type parsedVersion struct {
	base  string // base semver (without distance/dirty)
	dist  int    // commit distance from tag (0 if at tag)
	dirty bool   // true if uncommitted changes present
}

// parseVersion parses a version string (with optional 'v' prefix) into components.
// Version format: [v]BASE[-DIST][-dirty]
// where BASE is a valid semver (may include pre-release like -beta.5),
// DIST is a numeric distance, and dirty is a literal "-dirty" suffix.
func parseVersion(v string) parsedVersion {
	v = strings.TrimPrefix(v, "v")
	dirty := false
	if strings.HasSuffix(v, "-dirty") {
		dirty = true
		v = strings.TrimSuffix(v, "-dirty")
	}
	dist := 0
	if idx := strings.LastIndexByte(v, '-'); idx != -1 {
		suffix := v[idx+1:]
		if suffix != "" {
			if n, err := strconv.Atoi(suffix); err == nil {
				dist = n
				v = v[:idx]
			}
		}
	}
	return parsedVersion{base: v, dist: dist, dirty: dirty}
}

// isOlder returns true if version a is semantically older than b.
// Uses semver for base comparison; if base equal, compares distance (larger is newer)
// and dirty flag (clean > dirty).
func isOlder(a, b string) bool {
	pa := parseVersion(a)
	pb := parseVersion(b)

	// Compare base using semver.
	aSemver := "v" + pa.base
	bSemver := "v" + pb.base
	if cmp := semver.Compare(aSemver, bSemver); cmp != 0 {
		return cmp < 0
	}

	// Same base version: compare distance.
	if pa.dist != pb.dist {
		return pa.dist < pb.dist
	}

	// Same distance: clean (dirty=false) is newer than dirty (true).
	if pa.dirty == pb.dirty {
		return false
	}
	return pa.dirty
}

// Manager handles automatic updates for rules and binary.
type Manager struct {
	client   *upstream.Client
	rulesURL string
}

// NewManager creates a new update manager with given config and rules.
func NewManager(cfg *config.Config, rules *config.Rules) *Manager {
	return &Manager{
		client:   upstream.New(cfg, rules),
		rulesURL: cfg.Update.RulesURL,
	}
}

// CheckForUpdate checks if there's a new version available by querying GitHub releases API
// using the internal network stack (Fake SNI + IP logic).
func (m *Manager) CheckForUpdate(currentVersion string) (bool, string, error) {
	const apiURL = "https://api.github.com/repos/xihale/snirect/releases"

	ctx := context.Background()
	resp, err := m.client.Get(ctx, apiURL)
	if err != nil {
		return false, "", fmt.Errorf("failed to fetch release info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var releases []GitHubReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return false, "", fmt.Errorf("failed to decode release info: %w", err)
	}

	if len(releases) == 0 {
		return false, "", fmt.Errorf("no releases found")
	}

	latest := releases[0].TagName

	// Special case: development sentinel version never updates.
	if currentVersion == "0.0.0-dev" {
		return false, latest, nil
	}

	hasUpdate := isOlder(currentVersion, latest)
	return hasUpdate, latest, nil
}

// GitHubReleaseInfo represents the GitHub release information.
type GitHubReleaseInfo struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// FetchRules downloads and installs the latest rules.
// FetchRules downloads and installs the latest rules.
// It downloads content to memory, detects format (TOML or JSON), and writes rules.toml directly.
func (m *Manager) FetchRules(appDir string) error {
	if m.rulesURL == "" {
		return fmt.Errorf("rules URL not configured: please set update.rules_url in config.toml")
	}

	rulesPath := filepath.Join(appDir, "rules.toml")
	logger.Info("Fetching rules from: %s", m.rulesURL)

	// Download to memory (handles redirects)
	data, err := m.downloadToMemory(m.rulesURL)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	// Determine format and process
	tomlData, format, err := m.processRulesData(data, m.rulesURL)
	if err != nil {
		return fmt.Errorf("failed to process rules: %w", err)
	}

	// Write final rules.toml
	if err := os.WriteFile(rulesPath, tomlData, 0644); err != nil {
		return fmt.Errorf("failed to write rules: %w", err)
	}

	logger.Info("Rules updated (%s format)", format)
	if err := m.UpdateRulesCheckTimestamp(appDir); err != nil {
		logger.Warn("Failed to update rules check timestamp: %v", err)
	}

	return nil
}

// DownloadFile downloads a file from the given URL and saves it to destPath.
// It follows redirects and uses the internal network stack.
func (m *Manager) DownloadFile(destPath, url string) error {
	logger.Info("Downloading: %s", url)
	data, err := m.downloadToMemory(url)
	if err != nil {
		return fmt.Errorf("failed to download %s: %w", url, err)
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	if err := os.WriteFile(destPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	logger.Info("Downloaded: %s -> %s", url, destPath)
	return nil
}

// downloadToMemory downloads a URL following redirects and returns the content.
func (m *Manager) downloadToMemory(url string) ([]byte, error) {
	const maxRedirects = 10
	var depth int

	for {
		if depth > maxRedirects {
			return nil, fmt.Errorf("too many redirects (>%d)", maxRedirects)
		}

		ctx := context.Background()
		resp, err := m.client.Get(ctx, url)
		if err != nil {
			return nil, fmt.Errorf("failed to download %s: %w", url, err)
		}
		defer resp.Body.Close()

		// Handle redirect
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			location := resp.Header.Get("Location")
			if location == "" {
				return nil, fmt.Errorf("redirect status %d but no Location header", resp.StatusCode)
			}
			logger.Debug("Redirect: %s -> %s", url, location)
			url = location
			depth++
			continue
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("bad status: %s", resp.Status)
		}

		return io.ReadAll(resp.Body)
	}
}

// processRulesData determines format (from URL suffix and content) and returns TOML data.
func (m *Manager) processRulesData(data []byte, url string) ([]byte, string, error) {
	// Try TOML first if URL suggests it
	if strings.HasSuffix(url, ".toml") {
		r := rules.NewRules()
		if err := r.FromTOML(data); err == nil {
			tomlData, err := r.ToTOML()
			if err != nil {
				return nil, "", fmt.Errorf("failed to re-serialize TOML: %w", err)
			}
			return tomlData, "toml", nil
		}
		// If URL says .toml but parsing fails, fall through to JSON check
	}

	// Try JSON array format (legacy)
	var raw [][3]interface{}
	if err := json.Unmarshal(data, &raw); err == nil {
		r := rules.NewRules()
		for _, entry := range raw {
			if len(entry) != 3 {
				continue
			}
			hosts, ok := entry[0].([]interface{})
			if !ok {
				continue
			}
			sniVal := entry[1]
			var sni string
			if sniVal != nil {
				if s, ok := sniVal.(string); ok {
					sni = s
				}
			}
			ip, ok := entry[2].(string)
			if !ok {
				continue
			}
			for _, h := range hosts {
				hostStr, ok := h.(string)
				if !ok {
					continue
				}
				r.Hosts[hostStr] = ip
				r.AlterHostname[hostStr] = sni
			}
		}
		r.Init()
		tomlData, err := r.ToTOML()
		if err != nil {
			return nil, "", fmt.Errorf("failed to convert JSON to TOML: %w", err)
		}
		return tomlData, "json", nil
	}

	// Format not recognized
	return nil, "", fmt.Errorf("unrecognized rules format (expected TOML or JSON array)")
}

func (m *Manager) convertAndInstallRules(rawPath, installPath string) error {
	data, err := os.ReadFile(rawPath)
	if err != nil {
		return fmt.Errorf("failed to read raw rules: %w", err)
	}

	// Try to parse as TOML first (new format)
	r := rules.NewRules()
	if err := r.FromTOML(data); err == nil {
		// TOML parsed successfully, use it directly
		tomlData, err := r.ToTOML()
		if err != nil {
			return fmt.Errorf("failed to re-serialize TOML: %w", err)
		}
		if err := os.WriteFile(installPath, tomlData, 0644); err != nil {
			return fmt.Errorf("failed to write rules: %w", err)
		}
		logger.Info("Rules installed directly from TOML: %s", installPath)
		return nil
	}

	// Not TOML, try to parse as legacy JSON array format
	// Expected structure: [[["host1", "host2"], "sni", "ip"], ...]
	var raw [][3]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("failed to parse JSON array: %w", err)
	}

	r = rules.NewRules()
	for _, entry := range raw {
		if len(entry) != 3 {
			continue
		}

		// Element 0: array of host patterns
		hosts, ok := entry[0].([]interface{})
		if !ok {
			continue
		}

		// Element 1: SNI value (string or null)
		sniVal := entry[1]
		var sni string
		if sniVal != nil {
			if s, ok := sniVal.(string); ok {
				sni = s
			}
		}

		// Element 2: IP address (must be string)
		ip, ok := entry[2].(string)
		if !ok {
			continue
		}

		// Add to both Hosts and AlterHostname maps
		for _, h := range hosts {
			hostStr, ok := h.(string)
			if !ok {
				logger.Debug("Skipping host: not string: %T", h)
				continue
			}
			r.Hosts[hostStr] = ip
			// Always set AlterHostname, even if empty string (means strip SNI)
			r.AlterHostname[hostStr] = sni
		}
	}

	r.Init()

	tomlData, err := r.ToTOML()
	if err != nil {
		return fmt.Errorf("failed to convert to TOML: %w", err)
	}

	if err := os.WriteFile(installPath, tomlData, 0644); err != nil {
		return fmt.Errorf("failed to write rules: %w", err)
	}

	logger.Info("Rules converted from JSON array and installed: %s", installPath)
	return nil
}

const rulesURL = "https://github.com/SpaceTimee/Cealing-Host/releases/latest/download/Cealing-Host.toml"

// HasPendingUpdate checks if a self-update is pending.
func HasPendingUpdate(appDir string) bool {
	markFile := filepath.Join(appDir, ".self_update")
	_, err := os.Stat(markFile)
	return err == nil
}

// PerformSelfUpdate replaces the binary and restarts.
func PerformSelfUpdate(appDir string) error {
	if !HasPendingUpdate(appDir) {
		return fmt.Errorf("no pending update")
	}

	versionBytes, err := os.ReadFile(filepath.Join(appDir, ".self_update"))
	if err != nil {
		return fmt.Errorf("failed to read update version: %w", err)
	}

	mgr := NewManager(&config.Config{}, &config.Rules{})
	downloadPath := mgr.GetDownloadedBinaryPath(appDir, string(versionBytes))

	if _, err := os.Stat(downloadPath); err != nil {
		return fmt.Errorf("downloaded binary not found: %w", err)
	}

	currentPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	if runtime.GOOS == "windows" {
		logger.Warn("Windows self-update requires manual replacement")
		logger.Info("Copy %s to %s after closing the program", downloadPath, currentPath)
		os.Remove(filepath.Join(appDir, ".self_update"))
		return nil
	}

	if err := os.Rename(downloadPath, currentPath); err != nil {
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	os.Remove(filepath.Join(appDir, ".self_update"))

	logger.Info("Self-update complete. Restarting...")
	return execNewProcess(currentPath, os.Args)
}

func execNewProcess(executable string, args []string) error {
	proc, err := os.StartProcess(executable, args, &os.ProcAttr{
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
	})
	if err != nil {
		return fmt.Errorf("failed to start new process: %w", err)
	}
	return proc.Release()
}

// ============== 辅助函数（原 config/update.go 迁移） ==============

// GetTempDownloadDir returns the temporary download directory for updates.
func (m *Manager) GetTempDownloadDir(appDir string) string {
	return filepath.Join(appDir, "updates")
}

// GetSelfUpdateMarkFile returns the path to the self-update mark file.
func (m *Manager) GetSelfUpdateMarkFile(appDir string) string {
	return filepath.Join(appDir, ".self_update")
}

// GetDownloadedBinaryPath returns the expected path of the downloaded binary.
func (m *Manager) GetDownloadedBinaryPath(appDir, version string) string {
	assetName := fmt.Sprintf("snirect-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		assetName += ".exe"
	}
	return filepath.Join(m.GetTempDownloadDir(appDir), assetName)
}

// MarkForSelfUpdate creates a mark file indicating a self-update is pending.
func (m *Manager) MarkForSelfUpdate(appDir, version string) error {
	markFile := m.GetSelfUpdateMarkFile(appDir)
	return os.WriteFile(markFile, []byte(version), 0644)
}

// GetPendingUpdateVersion reads the pending update version from the mark file.
func (m *Manager) GetPendingUpdateVersion(appDir string) (string, error) {
	markFile := m.GetSelfUpdateMarkFile(appDir)
	data, err := os.ReadFile(markFile)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ClearSelfUpdateMark removes the self-update mark file.
func (m *Manager) ClearSelfUpdateMark(appDir string) error {
	markFile := m.GetSelfUpdateMarkFile(appDir)
	return os.Remove(markFile)
}

// UpdateCheckTimestamp updates the last check timestamp for program updates.
func (m *Manager) UpdateCheckTimestamp(appDir string) error {
	checkFile := filepath.Join(appDir, ".update_check")
	timestamp := []byte(time.Now().Format(time.RFC3339))
	return os.WriteFile(checkFile, timestamp, 0644)
}

// UpdateRulesCheckTimestamp updates the last check timestamp for rules updates.
func (m *Manager) UpdateRulesCheckTimestamp(appDir string) error {
	checkFile := filepath.Join(appDir, ".rules_check")
	timestamp := []byte(time.Now().Format(time.RFC3339))
	return os.WriteFile(checkFile, timestamp, 0644)
}
