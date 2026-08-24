package sysproxy

import (
	"fmt"
	"github.com/xihale/snirect/internal/logger"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
			logger.System().Warn("failed to install cert to Firefox profile", "profile", filepath.Base(profile), "error", err)
		} else {
			installed++
			logger.System().Info("certificate installed to Firefox profile", "profile", filepath.Base(profile))
		}
	}

	if installed == 0 {
		return fmt.Errorf("failed to install certificate to any Firefox profile")
	}

	logger.System().Info("certificate installed to Firefox profiles", "count", installed)
	return nil
}

// UninstallFirefoxCert removes the CA certificate from all Firefox profiles
func UninstallFirefoxCert() error {
	profiles, err := findFirefoxProfiles()
	if err != nil {
		return err
	}

	if len(profiles) == 0 {
		logger.System().Info("no Firefox profiles found")
		return nil
	}

	removed := 0
	for _, profile := range profiles {
		cmd := exec.Command("certutil", "-D", "-n", "Snirect Root CA", "-d", "sql:"+profile)
		if err := cmd.Run(); err == nil {
			removed++
			logger.System().Info("certificate removed from Firefox profile", "profile", filepath.Base(profile))
		}
	}

	if removed > 0 {
		logger.System().Info("certificate removed from Firefox profiles", "count", removed)
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

// InstallChromeCert installs the CA certificate to Chrome/Chromium's NSS database (~/.pki/nssdb).
func InstallChromeCert(certPath string) error {
	if _, err := exec.LookPath("certutil"); err != nil {
		return fmt.Errorf("certutil not found. Install nss-tools (Debian/Ubuntu) or nss (Fedora/Arch)")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	nssDir := filepath.Join(home, ".pki", "nssdb")

	// Check if NSS database exists
	if _, err := os.Stat(filepath.Join(nssDir, "cert9.db")); os.IsNotExist(err) {
		return fmt.Errorf("chrome NSS database not found at %s", nssDir)
	}

	// Remove old cert if exists
	_ = exec.Command("certutil", "-D", "-n", "Snirect Root CA", "-d", "sql:"+nssDir).Run()

	// Install new cert with trust flags: C,, = Trusted CA for SSL
	cmd := exec.Command("certutil", "-A", "-n", "Snirect Root CA", "-t", "C,,", "-i", certPath, "-d", "sql:"+nssDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install cert to Chrome NSS database: %w", err)
	}

	logger.System().Info("certificate installed to Chrome/Chromium NSS database")
	return nil
}

// UninstallChromeCert removes the CA certificate from Chrome/Chromium's NSS database.
func UninstallChromeCert() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	nssDir := filepath.Join(home, ".pki", "nssdb")

	if _, err := os.Stat(filepath.Join(nssDir, "cert9.db")); os.IsNotExist(err) {
		return
	}

	if _, err := exec.LookPath("certutil"); err != nil {
		return
	}

	cmd := exec.Command("certutil", "-D", "-n", "Snirect Root CA", "-d", "sql:"+nssDir)
	if err := cmd.Run(); err == nil {
		logger.System().Info("certificate removed from Chrome/Chromium NSS database")
	}
}

// findFirefoxProfiles locates Firefox (and Firefox-based) profile directories.
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
			logger.System().Warn("failed to read directory", "path", browserDir, "error", err)
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
	_ = exec.Command("certutil", "-D", "-n", "Snirect Root CA", "-d", "sql:"+profilePath).Run()

	// Install new cert with trust flags
	// C,, = Trusted CA for SSL
	cmd := exec.Command("certutil", "-A", "-n", "Snirect Root CA", "-t", "C,,", "-i", certPath, "-d", "sql:"+profilePath)
	return cmd.Run()
}
