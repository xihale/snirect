//go:build linux || android

package core

import (
	"encoding/binary"
	"errors"
	"net"
	"sync"
	"sync/atomic"

	"golang.org/x/sys/unix"
)

// Per-flow app attribution: resolve the UID that owns the captured app's TCP
// socket via the kernel's inet_diag netlink interface (NETLINK_SOCK_DIAG,
// SOCK_DIAG_BY_FAMILY exact query). Android 10+ denies untrusted apps read
// access to /proc/net, but the netlink_tcpdiag_socket SELinux permissions
// remain available — the same mechanism NetGuard/PCAPdroid use for unrooted
// per-app attribution.
//
// The lookup happens once, in the netstack relay goroutine right after the
// flow is accepted, while the app's socket is still alive. A uid of 0 means
// "unattributable" (kernel socket, TIME_WAIT leftover, lookup failure) and
// simply leaves the flow anonymous — cert-reject events then carry no app
// identity and the auto-exclusion feature stays passive.
//
// Degraded mode: if the netlink socket cannot even be created (a hard SELinux
// denial), sockDiagBroken latches and every later lookup short-circuits to 0
// instead of retrying a doomed syscall per flow. Transient errors (socket
// gone mid-lookup → ENOENT, timeout) are per-call, silent, non-latching.

const (
	sockDiagType = 20 // SOCK_DIAG_BY_FAMILY
	tcpfAll      = 0xfff
	// inet_diag_msg: family/state/timer/retrans (4) + inet_diag_sockid (48)
	// + expires/queue/wqueue/uid/inode (20) — uid sits after the first 12 of
	// those, i.e. offset 64 within the nlmsg payload (after the 16-byte
	// nlmsghdr).
	inetDiagMsgUIDOffset = 16 + 64
	nlmsgErrno           = unix.NLMSG_ERROR
)

var (
	sockDiagMu     sync.Mutex
	sockDiagFD     = -1
	sockDiagBroken atomic.Bool
)

// uidOfTCPFlow returns the owning UID of the TCP socket with the given
// 4-tuple, or 0 when the flow cannot be attributed. src is the app socket's
// TUN-side address:port (the kernel socket's local address:port), dst the
// original destination the packet was routed to (the kernel socket's peer).
func uidOfTCPFlow(srcIP net.IP, srcPort uint16, dstIP net.IP, dstPort uint16) int {
	if sockDiagBroken.Load() {
		return 0
	}
	family := uint8(unix.AF_INET)
	src := srcIP.To4()
	dst := dstIP.To4()
	if src == nil || dst == nil {
		family = unix.AF_INET6
		src, dst = srcIP.To16(), dstIP.To16()
	}
	if src == nil || dst == nil || srcPort == 0 {
		return 0
	}

	sockDiagMu.Lock()
	defer sockDiagMu.Unlock()

	resp, err := sockDiagQuery(family, src, srcPort, dst, dstPort)
	if err != nil {
		if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) {
			sockDiagBroken.Store(true)
			LogWarn("sockdiag: netlink denied (%v); app attribution disabled, cert-reject events stay anonymous", err)
		}
		return 0
	}
	return parseInetDiagUID(resp, srcPort, dstPort)
}

// sockDiagQuery sends one exact-match SOCK_DIAG_BY_FAMILY request over a
// (re)created netlink socket and returns the raw first reply. The mutex must
// be held.
func sockDiagQuery(family uint8, src net.IP, srcPort uint16, dst net.IP, dstPort uint16) ([]byte, error) {
	fd, err := sockDiagSocketLocked()
	if err != nil {
		return nil, err
	}
	req := buildInetDiagRequest(family, src, srcPort, dst, dstPort)
	sa := &unix.SockaddrNetlink{Family: unix.AF_NETLINK}
	if err := unix.Sendto(fd, req, 0, sa); err != nil {
		// A dead netlink socket self-heals on the next call; drop this one.
		_ = unix.Close(fd)
		sockDiagFD = -1
		return nil, err
	}
	buf := make([]byte, 1024)
	n, _, err := unix.Recvfrom(fd, buf, 0)
	if err != nil {
		_ = unix.Close(fd)
		sockDiagFD = -1
		return nil, err
	}
	return buf[:n], nil
}

// sockDiagSocketLocked lazily creates the netlink diag socket with a short
// receive timeout. The mutex must be held.
func sockDiagSocketLocked() (int, error) {
	if sockDiagFD >= 0 {
		return sockDiagFD, nil
	}
	fd, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_RAW, unix.NETLINK_SOCK_DIAG)
	if err != nil {
		return -1, err
	}
	if err := unix.Bind(fd, &unix.SockaddrNetlink{Family: unix.AF_NETLINK}); err != nil {
		_ = unix.Close(fd)
		return -1, err
	}
	tv := unix.Timeval{Usec: 300 * 1000} // 300ms: bounded, far above loopback latency
	if err := unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &tv); err != nil {
		_ = unix.Close(fd)
		return -1, err
	}
	sockDiagFD = fd
	return fd, nil
}

// buildInetDiagRequest serializes nlmsghdr + inet_diag_req_v2 with an exact
// inet_diag_sockid match. Ports go on the wire in network byte order; the
// 16-byte address slots hold the v4 address in the first 4 bytes (kernel
// inet_diag_msg_common_fill layout), v6 addresses verbatim.
func buildInetDiagRequest(family uint8, src net.IP, srcPort uint16, dst net.IP, dstPort uint16) []byte {
	const (
		nlmsghdrLen = 16
		// inet_diag_req_v2: family/proto/ext/pad (4) + idiag_states (4)
		// + inet_diag_sockid (48).
		reqLen = 4 + 4 + 48
	)
	msg := make([]byte, nlmsghdrLen+reqLen)
	binary.LittleEndian.PutUint32(msg[0:4], uint32(len(msg)))
	binary.LittleEndian.PutUint16(msg[4:6], sockDiagType)
	binary.LittleEndian.PutUint16(msg[6:8], unix.NLM_F_REQUEST)
	binary.LittleEndian.PutUint32(msg[8:12], 1) // seq
	binary.LittleEndian.PutUint32(msg[12:16], 0)

	body := msg[nlmsghdrLen:]
	body[0] = family
	body[1] = unix.IPPROTO_TCP
	body[2] = 0 // idiag_ext
	body[3] = 0 // pad
	binary.LittleEndian.PutUint32(body[4:8], tcpfAll)
	id := body[8:]
	binary.BigEndian.PutUint16(id[0:2], srcPort)
	binary.BigEndian.PutUint16(id[2:4], dstPort)
	copy(id[4:20], src)
	copy(id[20:36], dst)
	// idiag_if = 0, idiag_cookie = INET_DIAG_NOCOOKIE (all ones)
	for i := 36; i < 48; i++ {
		id[i] = 0xff
	}
	return msg
}

// parseInetDiagUID extracts the owning uid from a SOCK_DIAG_BY_FAMILY reply.
// An NLMSG_ERROR payload means "no such socket" (ENOENT — the app's socket
// already closed) and yields 0. The ports are verified so a stale or
// misaddressed reply is never trusted.
func parseInetDiagUID(resp []byte, srcPort, dstPort uint16) int {
	if len(resp) < 16 {
		return 0
	}
	nlLen := int(binary.LittleEndian.Uint32(resp[0:4]))
	nlType := binary.LittleEndian.Uint16(resp[4:6])
	if nlLen > len(resp) {
		return 0
	}
	switch nlType {
	case sockDiagType:
	case nlmsgErrno:
		return 0 // includes ENOENT: socket closed before the lookup
	default:
		return 0
	}
	msg := resp[:nlLen]
	if len(msg) < inetDiagMsgUIDOffset+4 {
		return 0
	}
	id := msg[16+4:]
	if binary.BigEndian.Uint16(id[0:2]) != srcPort || binary.BigEndian.Uint16(id[2:4]) != dstPort {
		return 0
	}
	uid := int(binary.LittleEndian.Uint32(msg[inetDiagMsgUIDOffset : inetDiagMsgUIDOffset+4]))
	if uid <= 0 {
		return 0 // kernel/TIME_WAIT entries own no app uid
	}
	return uid
}
