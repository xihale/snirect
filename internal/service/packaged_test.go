//go:build linux

package service

import (
	"os/exec"
	"testing"
)

func TestOwningPackage(t *testing.T) {
	if _, err := exec.LookPath("pacman"); err != nil {
		t.Skip("pacman not available on this host")
	}

	if _, ok := owningPackage("/usr/bin/pacman"); !ok {
		t.Fatal("pacman should own its own binary")
	}

	if _, ok := owningPackage(t.TempDir() + "/definitely-not-owned"); ok {
		t.Fatal("a path no package owns must not report an owner")
	}
}
