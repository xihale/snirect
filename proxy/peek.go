package proxy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
)

// Transparent (address-only) flows arrive as "CONNECT <ip>:443", so the
// hostname rules can only be keyed on the ClientHello SNI. crypto/tls offers
// no way to bail out of tls.Server mid-handshake back into a raw pipe, so the
// decision has to happen before it: peekClientHelloSNI reads the first
// handshake message, extracts the SNI, and returns a conn that replays the
// consumed bytes — whichever way the decision goes, the stream is intact.

const (
	maxPeekBytes       = 64 << 10 // generous cap: records are ≤16KB each
	tlsRecordHeaderLen = 5
)

// peekClientHelloSNI reads the client's ClientHello from conn and returns a
// replaying wrapper plus the SNI it carries ("" when the ClientHello has no
// server_name extension). The wrapper hands back every peeked byte before
// reading further from conn, so both the MITM and the direct-tunnel paths see
// an untouched stream.
func peekClientHelloSNI(conn net.Conn) (net.Conn, string, error) {
	var buf bytes.Buffer
	hello, err := readClientHello(io.TeeReader(conn, &buf))
	if err != nil {
		return &replayConn{Conn: conn, head: buf.Bytes()}, "", err
	}
	return &replayConn{Conn: conn, head: buf.Bytes()}, hello.sni(), nil
}

// clientHello holds the parsed fields the routing decision needs.
type clientHello struct {
	serverName string
}

func (h *clientHello) sni() string { return h.serverName }

// readClientHello assembles the first TLS handshake message from r and parses
// the server_name extension out of it. It reads exactly what it needs — the
// caller relies on the tee'd side carrying every consumed byte.
func readClientHello(r io.Reader) (*clientHello, error) {
	// 1. Records: assemble the first handshake message (type 0x01). A
	//    ClientHello normally fits in one record, but may span several.
	var msg []byte
	var msgType byte
	var want int // remaining bytes of the handshake message; 0 = not known yet
	for len(msg) < want || want == 0 {
		if len(msg) >= maxPeekBytes {
			return nil, errors.New("client hello too large")
		}
		header := make([]byte, tlsRecordHeaderLen)
		if _, err := io.ReadFull(r, header); err != nil {
			return nil, fmt.Errorf("read record header: %w", err)
		}
		if header[0] != 22 { // not a handshake record
			return nil, fmt.Errorf("not a handshake record: type %d", header[0])
		}
		recLen := int(header[3])<<8 | int(header[4])
		if recLen == 0 || recLen > maxPeekBytes {
			return nil, fmt.Errorf("bad record length %d", recLen)
		}
		rec := make([]byte, recLen)
		if _, err := io.ReadFull(r, rec); err != nil {
			return nil, fmt.Errorf("read record body: %w", err)
		}
		if len(msg) == 0 {
			msgType = rec[0]
		}
		msg = append(msg, rec...)
		// First fragment: read the 4-byte handshake header for the total size.
		if want == 0 && len(msg) >= 4 {
			want = int(msg[1])<<16 | int(msg[2])<<8 | int(msg[3])
			want += 4 // handshake header
			if want > maxPeekBytes {
				return nil, fmt.Errorf("handshake message too large: %d", want)
			}
		}
	}
	if msgType != 1 {
		return nil, fmt.Errorf("not a ClientHello: handshake type %d", msgType)
	}
	hello := &clientHello{}
	hello.serverName = parseSNI(msg[4:]) // skip the handshake header
	return hello, nil
}

// parseSNI walks a ClientHello body for the server_name extension.
func parseSNI(body []byte) string {
	// version(2) random(32) session_id
	if len(body) < 35 {
		return ""
	}
	p := body[34:]
	if len(p) < 1 {
		return ""
	}
	sidLen := int(p[0])
	p = p[1+sidLen:]
	// cipher_suites(2+)
	if len(p) < 2 {
		return ""
	}
	csLen := int(p[0])<<8 | int(p[1])
	p = p[2+csLen:]
	// compression_methods(1+)
	if len(p) < 1 {
		return ""
	}
	compLen := int(p[0])
	p = p[1+compLen:]
	// extensions(2+)
	if len(p) < 2 {
		return ""
	}
	extLen := int(p[0])<<8 | int(p[1])
	p = p[2:]
	if extLen > len(p) {
		return ""
	}
	for len(p) >= 4 {
		etype := int(p[0])<<8 | int(p[1])
		elen := int(p[2])<<8 | int(p[3])
		if len(p) < 4+elen {
			return ""
		}
		ext := p[4 : 4+elen]
		if etype == 0 { // server_name
			// server_name_list: list_len(2) then entries: type(1) len(2) name
			if len(ext) < 2 {
				return ""
			}
			listLen := int(ext[0])<<8 | int(ext[1])
			e := ext[2:]
			if listLen > len(e) || len(e) < 3 {
				return ""
			}
			if e[0] != 0 { // host_name
				return ""
			}
			nameLen := int(e[1])<<8 | int(e[2])
			if len(e) < 3+nameLen {
				return ""
			}
			return string(e[3 : 3+nameLen])
		}
		p = p[4+elen:]
	}
	return ""
}

// replayConn is a net.Conn that serves the peeked bytes first, then the
// wrapped conn. Everything but Read delegates straight through.
type replayConn struct {
	net.Conn
	head []byte
}

func (c *replayConn) Read(p []byte) (int, error) {
	if len(c.head) > 0 {
		n := copy(p, c.head)
		c.head = c.head[n:]
		return n, nil
	}
	return c.Conn.Read(p)
}
