//go:build linux

package service

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// OwningPackage reports whether the running executable is owned by a system
// package manager (pacman on Arch, including AUR-built packages). When true,
// install/update must not copy or replace the binary: pacman manages it, and
// a copy in ~/.local/bin would shadow package upgrades with a stale binary.
func OwningPackage() (binPath, pkg string, ok bool) {
	exe, err := os.Executable()
	if err != nil {
		return "", "", false
	}
	if owner, owned := owningPackage(exe); owned {
		return exe, owner, true
	}
	return "", "", false
}

// owningPackage checks pacman ownership of a path; split out for tests.
func owningPackage(path string) (owner string, ok bool) {
	if _, err := exec.LookPath("pacman"); err != nil {
		return "", false
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	out, err := exec.Command("pacman", "-Qo", "--quiet", path).Output()
	if err != nil {
		// Not owned by any package (or pacman refused): treat as self-managed.
		return "", false
	}
	owner = strings.TrimSpace(string(out))
	return owner, owner != ""
}
