package core

import (
	"fmt"
	"net"
	"os"
	"syscall"
	"time"
)

// newTunFile wraps a VpnService.establish() file descriptor for the gVisor
// netstack to read/write.
//
// os.NewFile TAKES OWNERSHIP of the fd it is given (Close + a GC finalizer
// both syscall.Close it). The host ParcelFileDescriptor also owns the original
// fd and closes it on teardown. Wrapping the host fd directly therefore
// double-closes: after disconnect the kernel may recycle that number for the
// next TUN, and the leftover *os.File finalizer then closes the new interface
// — a native crash on "disconnect, then start".
//
// Dup gives Go a private fd. The host still closes the original; we close the
// dup in netstackBridge.Close. The two closes are independent.
//
// The file is build-tagged to GOOS=android by filename. The off-Android stub
// in platform_other.go exists so host `go build ./...` type-checks;
// StartEngine is never actually invoked off-Android (gomobile bind is the only
// caller).
func newTunFile(fd int) (tunFile, error) {
	nfd, err := syscall.Dup(fd)
	if err != nil {
		return nil, fmt.Errorf("dup tun fd %d: %w", fd, err)
	}
	syscall.CloseOnExec(nfd)
	f := os.NewFile(uintptr(nfd), "tun-android")
	if f == nil {
		_ = syscall.Close(nfd)
		return nil, fmt.Errorf("invalid tun fd %d", nfd)
	}
	return f, nil
}

// bypassDialer returns a *net.Dialer whose outbound sockets are routed through
// VpnService.protect(fd) so they escape the VPN's own TUN (the Android analog
// of desktop Linux's SO_MARK + fwmark policy routing). Without this, the
// proxy's upstream dials and the resolver's DNS queries would be captured by
// the TUN default route and loop back on themselves.
//
// The single returned dialer is shared by the resolver (DNS upstreams) and
// injected into proxy.ProxyServer (upstream TCP dials).
func bypassDialer(cb EngineCallbacks) *net.Dialer {
	return &net.Dialer{
		Timeout: 30 * time.Second,
		Control: func(network, address string, c syscall.RawConn) error {
			// Guard nil callbacks: calling Protect on a nil interface would
			// panic inside the dial path and take down the process.
			if cb == nil {
				return fmt.Errorf("no engine callbacks: cannot protect socket out of the VPN")
			}
			var protectErr error
			controlErr := c.Control(func(fd uintptr) {
				if !cb.Protect(int(fd)) {
					protectErr = fmt.Errorf("VpnService.protect failed for fd %d", fd)
					return
				}
				// Through LogDebug, not log.Printf: stdout writes are
				// invisible on Android, and this fires on every outbound
				// socket (audit L3).
				LogDebug("VPN: protected socket fd %d", fd)
			})
			if controlErr != nil {
				return controlErr
			}
			return protectErr
		},
	}
}
