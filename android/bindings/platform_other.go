//go:build !android

// Non-Android stubs for the platform seam. The real implementations live in
// platform_android.go (build-tagged to GOOS=android by filename). These stubs
// exist only so `go build ./...` on a host (linux/darwin) for tooling/editor
// support type-checks — StartEngine is never actually invoked off-Android
// (gomobile bind, the sole caller, runs with GOOS=android).

package core

import (
	"fmt"
	"net"
	"time"
)

// newTunFile is unreachable off-Android; returning an error lets host builds
// type-check. StartEngine would fail loudly at runtime if ever called here,
// but that path is impossible (gomobile is the only caller).
func newTunFile(fd int) (tunFile, error) {
	return nil, fmt.Errorf("newTunFile is android-only")
}

// bypassDialer returns a plain dialer off-Android (no VpnService.protect to
// call). Unreachable in practice; exists for host type-checking.
func bypassDialer(cb EngineCallbacks) *net.Dialer {
	return &net.Dialer{Timeout: 30 * time.Second}
}
