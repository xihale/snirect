//go:build linux || android

package core

import (
	"encoding/binary"
	"errors"
	"net"
	"os"
	"testing"

	"golang.org/x/sys/unix"
)

func TestBuildInetDiagRequestIPv4(t *testing.T) {
	req := buildInetDiagRequest(unix.AF_INET,
		net.IPv4(10, 0, 0, 1).To4(), 0x1234,
		net.IPv4(1, 2, 3, 4).To4(), 443)

	if got := binary.LittleEndian.Uint16(req[4:6]); got != sockDiagType {
		t.Fatalf("nlmsg type: got %d, want %d", got, sockDiagType)
	}
	if got := binary.LittleEndian.Uint16(req[6:8]); got != unix.NLM_F_REQUEST {
		t.Fatalf("nlmsg flags: got %d, want %d", got, unix.NLM_F_REQUEST)
	}
	if got := int(req[16]); got != unix.AF_INET {
		t.Fatalf("sdiag_family: got %d, want %d", got, unix.AF_INET)
	}
	if got := int(req[17]); got != unix.IPPROTO_TCP {
		t.Fatalf("sdiag_protocol: got %d, want %d", got, unix.IPPROTO_TCP)
	}
	if got := binary.LittleEndian.Uint32(req[20:24]); got != tcpfAll {
		t.Fatalf("idiag_states: got %#x, want %#x", got, tcpfAll)
	}
	id := req[24:]
	if got := binary.BigEndian.Uint16(id[0:2]); got != 0x1234 {
		t.Fatalf("idiag_sport: got %#x, want 0x1234", got)
	}
	if got := binary.BigEndian.Uint16(id[2:4]); got != 443 {
		t.Fatalf("idiag_dport: got %#x, want 443", got)
	}
	if string(id[4:8]) != string(net.IPv4(10, 0, 0, 1).To4()) {
		t.Fatalf("idiag_src: got %v", id[4:20])
	}
	if string(id[20:24]) != string(net.IPv4(1, 2, 3, 4).To4()) {
		t.Fatalf("idiag_dst: got %v", id[20:36])
	}
	for i := 36; i < 44; i++ {
		if id[i] != 0xff {
			t.Fatalf("idiag_cookie not NOCOOKIE at %d: %#x", i, id[i])
		}
	}
}

func TestBuildInetDiagRequestIPv6(t *testing.T) {
	src := net.ParseIP("fd00::1").To16()
	dst := net.ParseIP("2001:db8::1").To16()
	req := buildInetDiagRequest(unix.AF_INET6, src, 50000, dst, 8443)
	if got := int(req[16]); got != unix.AF_INET6 {
		t.Fatalf("sdiag_family: got %d, want %d", got, unix.AF_INET6)
	}
	id := req[24:]
	if got := binary.BigEndian.Uint16(id[0:2]); got != 50000 {
		t.Fatalf("idiag_sport: got %d", got)
	}
	if string(id[4:20]) != string(src) {
		t.Fatalf("idiag_src: got %v", id[4:20])
	}
	if string(id[20:36]) != string(dst) {
		t.Fatalf("idiag_dst: got %v", id[20:36])
	}
}

func diagReply(nlType uint16, srcPort, dstPort uint16, uid uint32) []byte {
	msg := make([]byte, 16+4+48+20)
	binary.LittleEndian.PutUint32(msg[0:4], uint32(len(msg)))
	binary.LittleEndian.PutUint16(msg[4:6], nlType)
	id := msg[20:]
	binary.BigEndian.PutUint16(id[0:2], srcPort)
	binary.BigEndian.PutUint16(id[2:4], dstPort)
	binary.LittleEndian.PutUint32(msg[16+4+48+12:], uid)
	return msg
}

func TestParseInetDiagUID(t *testing.T) {
	if got, err := parseInetDiagUID(diagReply(sockDiagType, 0x1234, 443, 10123), 0x1234, 443); err != nil || got != 10123 {
		t.Fatalf("uid: got (%d, %v), want (10123, nil)", got, err)
	}
	// Port mismatch in the reply id must be rejected, not trusted.
	if got, err := parseInetDiagUID(diagReply(sockDiagType, 0x9999, 443, 10123), 0x1234, 443); err != nil || got != 0 {
		t.Fatalf("port mismatch: got (%d, %v), want (0, nil)", got, err)
	}
	// ENOENT-style NLMSG_ERROR (socket gone) is a clean zero without error.
	makeErrMsg := func(errno unix.Errno) []byte {
		errMsg := make([]byte, 20)
		binary.LittleEndian.PutUint32(errMsg[0:4], 20)
		binary.LittleEndian.PutUint16(errMsg[4:6], unix.NLMSG_ERROR)
		neg := -int32(errno)
		binary.LittleEndian.PutUint32(errMsg[16:20], uint32(neg))
		return errMsg
	}
	if got, err := parseInetDiagUID(makeErrMsg(unix.ENOENT), 0x1234, 443); err != nil || got != 0 {
		t.Fatalf("NLMSG_ERROR ENOENT: got (%d, %v), want (0, nil)", got, err)
	}

	// Permission/unsupported errors must return the errno and latch sockDiagBroken.
	prevBroken := sockDiagBroken.Load()
	defer sockDiagBroken.Store(prevBroken)
	for _, tc := range []unix.Errno{unix.EACCES, unix.EPERM, unix.EOPNOTSUPP} {
		sockDiagBroken.Store(false)
		got, err := parseInetDiagUID(makeErrMsg(tc), 0x1234, 443)
		if got != 0 || !errors.Is(err, tc) {
			t.Fatalf("NLMSG_ERROR %v: got (%d, %v), want (0, %v)", tc, got, err, tc)
		}
		latchSockDiagIfFatal(err)
		if !sockDiagBroken.Load() {
			t.Fatalf("NLMSG_ERROR %v must latch sockDiagBroken", tc)
		}
	}
	sockDiagBroken.Store(false)
	_, enoentErr := parseInetDiagUID(makeErrMsg(unix.ENOENT), 0x1234, 443)
	latchSockDiagIfFatal(enoentErr)
	if sockDiagBroken.Load() {
		t.Fatal("NLMSG_ERROR ENOENT must NOT latch sockDiagBroken")
	}

	// uid 0 = kernel / TIME_WAIT, not attributable.
	if got, err := parseInetDiagUID(diagReply(sockDiagType, 0x1234, 443, 0), 0x1234, 443); err != nil || got != 0 {
		t.Fatalf("uid 0: got (%d, %v), want (0, nil)", got, err)
	}
	// Truncated garbage never yields a uid.
	if got, err := parseInetDiagUID([]byte{1, 2, 3}, 0x1234, 443); err != nil || got != 0 {
		t.Fatalf("truncated: got (%d, %v), want (0, nil)", got, err)
	}
}

func TestBuildInetDiagRequestIntoClearsReusedAddrBytes(t *testing.T) {
	var buf [sockDiagReqTotalLen]byte
	src6 := net.ParseIP("fd00::1234").To16()
	dst6 := net.ParseIP("2001:db8::5678").To16()
	buildInetDiagRequestInto(buf[:], unix.AF_INET6, src6, 50000, dst6, 8443)

	src4 := net.IPv4(10, 0, 0, 1).To4()
	dst4 := net.IPv4(1, 2, 3, 4).To4()
	req := buildInetDiagRequestInto(buf[:], unix.AF_INET, src4, 0x1234, dst4, 443)
	id := req[24:]
	for i := 8; i < 20; i++ {
		if id[i] != 0 {
			t.Fatalf("stale v6 src byte at id[%d]=%#x", i, id[i])
		}
	}
	for i := 24; i < 36; i++ {
		if id[i] != 0 {
			t.Fatalf("stale v6 dst byte at id[%d]=%#x", i, id[i])
		}
	}
}

// TestUIDOfTCPFlowLive exercises the real netlink query against a loopback
// connection: the socket belongs to this test process, so the kernel must
// report our own uid. Skipped when the environment denies the diag socket
// (sandboxed CI) — the denial itself is the degraded mode the code handles.
func TestUIDOfTCPFlowLive(t *testing.T) {
	if os.Getuid() == 0 {
		// Root-owned sockets report uid 0, which the parser deliberately
		// treats as unattributable (kernel/TIME_WAIT entries own no app uid).
		// True root and `unshare -Urn` (mapped root) both land here.
		t.Skip("running as uid 0: uid 0 is by design unattributable")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("listen: %v", err)
	}
	defer ln.Close()
	type result struct {
		conn net.Conn
		addr net.Addr
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		c, err := ln.Accept()
		ch <- result{c, ln.Addr(), err}
	}()
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Skipf("dial: %v", err)
	}
	defer conn.Close()
	res := <-ch
	if res.err != nil {
		t.Skipf("accept: %v", res.err)
	}
	defer res.conn.Close()

	remote := conn.RemoteAddr().(*net.TCPAddr)
	local := conn.LocalAddr().(*net.TCPAddr)
	uid := uidOfTCPFlow(local.IP, uint16(local.Port), remote.IP, uint16(remote.Port))
	if uid == 0 {
		if sockDiagBroken.Load() {
			t.Skip("netlink diag denied in this environment (degraded mode)")
		}
		t.Fatalf("uidOfTCPFlow returned 0 for a live socket (uid=%d)", os.Getuid())
	}
	if uid != os.Getuid() {
		t.Fatalf("uid: got %d, want %d", uid, os.Getuid())
	}
}

// TestUIDOfTCPFlowUnattributable: a tuple with no kernel socket resolves to
// 0 without latching degraded mode.
func TestUIDOfTCPFlowUnattributable(t *testing.T) {
	if sockDiagBroken.Load() {
		t.Skip("netlink diag denied in this environment")
	}
	sockDiagMu.Lock()
	prevFD := sockDiagFD
	sockDiagFD = -1 // force a fresh socket even if a prior test left one
	sockDiagMu.Unlock()
	defer func() {
		sockDiagMu.Lock()
		sockDiagFD = prevFD
		sockDiagMu.Unlock()
	}()

	got := uidOfTCPFlow(net.IPv4(192, 0, 2, 10), 55555, net.IPv4(192, 0, 2, 20), 443)
	if got != 0 {
		t.Fatalf("uid for dead tuple: got %d, want 0", got)
	}
	if sockDiagBroken.Load() {
		t.Fatal("ENOENT lookup must not latch degraded mode")
	}
}

func BenchmarkSockDiag_BuildAndParse(b *testing.B) {
	src := net.IPv4(10, 0, 0, 1).To4()
	dst := net.IPv4(1, 2, 3, 4).To4()
	reply := diagReply(sockDiagType, 0x1234, 443, 10123)
	var reqBuf [sockDiagReqTotalLen]byte

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := buildInetDiagRequestInto(reqBuf[:], unix.AF_INET, src, 0x1234, dst, 443)
		if len(req) == 0 {
			b.Fatal("empty request")
		}
		if uid, err := parseInetDiagUID(reply, 0x1234, 443); err != nil || uid != 10123 {
			b.Fatalf("uid: got (%d, %v), want (10123, nil)", uid, err)
		}
	}
}
