//go:build linux

package sysproxy

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// isSystemProxySet reports whether a PAC/system proxy is currently active. It
// inspects the desktop environment's proxy backend so the answer is symmetric
// with SetPAC/ClearPAC (which also branch on desktop environment).
//
// Previously this only checked GNOME, so on KDE — where SetPAC writes
// kioslaverc — it always reported false and `snirect status` lied. Now it
// matches whichever backend set the proxy.
func isSystemProxySet() bool {
	switch getDesktopEnvironment() {
	case "gnome":
		return gnomeProxyActive()
	case "kde":
		return kdeProxyActive()
	}
	// Unknown DE: fall back to the GNOME check so we don't silently regress on
	// GNOME derivatives that getDesktopEnvironment() doesn't recognize as "gnome".
	return gnomeProxyActive()
}

// gnomeProxyActive is true when GNOME proxy mode is "auto" (PAC).
func gnomeProxyActive() bool {
	if !HasTool("gsettings") {
		return false
	}
	out, err := exec.Command("gsettings", "get", "org.gnome.system.proxy", "mode").Output()
	return err == nil && strings.Contains(string(out), "auto")
}

// kdeProxyActive is true when KDE kioslaverc ProxyType is set to 2 (PAC script),
// matching the value written by setKDEProxy. kreadconfig5 reads the live value.
func kdeProxyActive() bool {
	if !HasTool("kreadconfig5") {
		return false
	}
	out, err := exec.Command("kreadconfig5",
		"--file", "kioslaverc",
		"--group", "Proxy Settings",
		"--key", "ProxyType",
	).Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "2"
}

// kdeProxyConfigPath returns the path to the KDE kioslaverc file. Exported via a
// helper so tests can point KDE_HOME at a temp dir without depending on a real
// KDE install.
func kdeProxyConfigPath() string {
	if p := os.Getenv("XDG_CONFIG_HOME"); p != "" {
		return filepath.Join(p, "kioslaverc")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "kioslaverc")
}
