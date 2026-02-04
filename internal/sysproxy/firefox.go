package sysproxy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"snirect/internal/logger"
)

// InstallFirefoxCert installs the CA certificate to all Firefox-based browser profiles
// Supports: Firefox, Zen Browser, Waterfox, LibreWolf, Floorp
func InstallFirefoxCert(certPath string) error {
	if _, err := exec.LookPath("certutil"); err != nil {
		return fmt.Errorf("certutil not found. Install nss-tools (Debian/Ubuntu) or nss (Fedora/Arch)")
	}

	profiles, err := findFirefoxProfiles()
	if err != nil {
		return err
	}

	if len(profiles) == 0 {
		return fmt.Errorf("no Firefox profiles found")
	}

	installed := 0
	for _, profile := range profiles {
		if err := installCertToFirefoxProfile(certPath, profile); err != nil {
			logger.Warn("Failed to install cert to Firefox profile %s: %v", filepath.Base(profile), err)
		} else {
			installed++
			logger.Info("✓ 证书已安装到 Firefox profile: %s", filepath.Base(profile))
		}
	}

	if installed == 0 {
		return fmt.Errorf("failed to install certificate to any Firefox profile")
	}

	logger.Info("成功安装证书到 %d 个 Firefox profile(s)", installed)
	return nil
}

// UninstallFirefoxCert removes the CA certificate from all Firefox profiles
func UninstallFirefoxCert() error {
	profiles, err := findFirefoxProfiles()
	if err != nil {
		return err
	}

	if len(profiles) == 0 {
		logger.Info("No Firefox profiles found")
		return nil
	}

	removed := 0
	for _, profile := range profiles {
		cmd := exec.Command("certutil", "-D", "-n", "Snirect Root CA", "-d", "sql:"+profile)
		if err := cmd.Run(); err == nil {
			removed++
			logger.Info("✓ 证书已从 Firefox profile 移除: %s", filepath.Base(profile))
		}
	}

	if removed > 0 {
		logger.Info("成功从 %d 个 Firefox profile(s) 移除证书", removed)
	}
	return nil
}

// CheckFirefoxCert checks if the CA is installed in any Firefox profile
func CheckFirefoxCert() (bool, error) {
	profiles, err := findFirefoxProfiles()
	if err != nil {
		return false, err
	}

	if len(profiles) == 0 {
		return false, nil
	}

	for _, profile := range profiles {
		cmd := exec.Command("certutil", "-L", "-d", "sql:"+profile, "-n", "Snirect Root CA")
		if err := cmd.Run(); err == nil {
			return true, nil
		}
	}

	return false, nil
}

func findFirefoxProfiles() ([]string, error) {
	var browserDirs []string

	switch runtime.GOOS {
	case "linux":
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		browserDirs = []string{
			filepath.Join(home, ".mozilla", "firefox"),
			filepath.Join(home, ".zen"),       // Zen Browser
			filepath.Join(home, ".waterfox"),  // Waterfox
			filepath.Join(home, ".librewolf"), // LibreWolf
			filepath.Join(home, ".floorp"),    // Floorp
		}
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		browserDirs = []string{
			filepath.Join(home, "Library", "Application Support", "Firefox", "Profiles"),
			filepath.Join(home, "Library", "Application Support", "Zen", "Profiles"),
			filepath.Join(home, "Library", "Application Support", "Waterfox", "Profiles"),
			filepath.Join(home, "Library", "Application Support", "LibreWolf", "Profiles"),
		}
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return nil, fmt.Errorf("APPDATA environment variable not set")
		}
		browserDirs = []string{
			filepath.Join(appData, "Mozilla", "Firefox", "Profiles"),
			filepath.Join(appData, "Zen", "Profiles"),
			filepath.Join(appData, "Waterfox", "Profiles"),
			filepath.Join(appData, "LibreWolf", "Profiles"),
		}
	default:
		return nil, fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	var profiles []string

	for _, browserDir := range browserDirs {
		if _, err := os.Stat(browserDir); os.IsNotExist(err) {
			continue // Browser not installed
		}

		entries, err := os.ReadDir(browserDir)
		if err != nil {
			logger.Warn("Failed to read directory %s: %v", browserDir, err)
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			profilePath := filepath.Join(browserDir, entry.Name())
			// Check if cert9.db exists (modern Firefox/Zen/etc)
			if _, err := os.Stat(filepath.Join(profilePath, "cert9.db")); err == nil {
				profiles = append(profiles, profilePath)
			}
		}
	}

	return profiles, nil
}

func installCertToFirefoxProfile(certPath, profilePath string) error {
	// Remove old cert if exists
	exec.Command("certutil", "-D", "-n", "Snirect Root CA", "-d", "sql:"+profilePath).Run()

	// Install new cert with trust flags
	// C,, = Trusted CA for SSL
	cmd := exec.Command("certutil", "-A", "-n", "Snirect Root CA", "-t", "C,,", "-i", certPath, "-d", "sql:"+profilePath)
	return cmd.Run()
}
