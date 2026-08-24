//go:build !linux

package service

// OwningPackage is a no-op off Linux: package-manager detection targets
// pacman/AUR installs, which only exist there.
func OwningPackage() (binPath, pkg string, ok bool) { return "", "", false }
