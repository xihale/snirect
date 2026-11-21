package proxy

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
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
	peekBufCap         = 2048
)

var peekBufPool = sync.Pool{
	New: func() any {
		b := make([]byte, 0, peekBufCap)
		return &b
	},
}

func growPeekBuf(buf []byte, extra int) []byte {
	need := len(buf) + extra
	if cap(buf) >= need {
		return buf
	}
	newCap := cap(buf) * 2
	if newCap < need {
		newCap = need
	}
	nb := make([]byte, len(buf), newCap)
	copy(nb, buf)
	return nb
}

func releaseOrAttachPoolBuf(bp *[]byte, raw []byte) *[]byte {
	if bp == nil {
		return nil
	}
	if cap(raw) == peekBufCap {
		*bp = raw[:0]
		return bp
	}
	// raw grew onto a new backing array; return the original 2 KiB buffer now.
	*bp = (*bp)[:0]
	peekBufPool.Put(bp)
	return nil
}

// peekClientHelloSNI reads the client's ClientHello from conn and returns a
// replaying wrapper plus the SNI it carries ("" when the ClientHello has no
// server_name extension). The wrapper hands back every peeked byte before
// reading further from conn, so both the MITM and the direct-tunnel paths see
// an untouched stream.
func peekClientHelloSNI(conn net.Conn) (net.Conn, string, error) {
	bp := peekBufPool.Get().(*[]byte)
	raw := (*bp)[:0]

	var msg []byte
	var msgType byte
	var want int
	var totalPayload int
	hasFirstRec := false

	fail := func(err error) (net.Conn, string, error) {
		return &replayConn{
			Conn:    conn,
			head:    raw,
			poolBuf: releaseOrAttachPoolBuf(bp, raw),
		}, "", err
	}

	for {
		if totalPayload >= maxPeekBytes {
			return fail(errors.New("client hello too large"))
		}

		hdrOff := len(raw)
		raw = growPeekBuf(raw, tlsRecordHeaderLen)
		n, err := io.ReadFull(conn, raw[hdrOff:hdrOff+tlsRecordHeaderLen])
		raw = raw[:hdrOff+n]
		if err != nil {
			return fail(fmt.Errorf("read record header: %w", err))
		}
		if raw[hdrOff] != 22 { // not a handshake record
			return fail(fmt.Errorf("not a handshake record: type %d", raw[hdrOff]))
		}
		recLen := int(raw[hdrOff+3])<<8 | int(raw[hdrOff+4])
		if recLen == 0 || recLen > maxPeekBytes || totalPayload+recLen > maxPeekBytes {
			return fail(fmt.Errorf("bad record length %d", recLen))
		}

		bodyOff := len(raw)
		raw = growPeekBuf(raw, recLen)
		n, err = io.ReadFull(conn, raw[bodyOff:bodyOff+recLen])
		raw = raw[:bodyOff+n]
		if err != nil {
			return fail(fmt.Errorf("read record body: %w", err))
		}
		totalPayload += recLen
		rec := raw[bodyOff : bodyOff+recLen]

		if !hasFirstRec {
			hasFirstRec = true
			msgType = rec[0]
			if recLen >= 4 {
				want = 4 + (int(rec[1])<<16 | int(rec[2])<<8 | int(rec[3]))
				if want > maxPeekBytes {
					return fail(fmt.Errorf("handshake message too large: %d", want))
				}
				if recLen >= want {
					if msgType != 1 {
						return fail(fmt.Errorf("not a ClientHello: handshake type %d", msgType))
					}
					sni := parseSNI(rec[4:want])
					return &replayConn{
						Conn:    conn,
						head:    raw,
						poolBuf: releaseOrAttachPoolBuf(bp, raw),
					}, sni, nil
				}
				msg = make([]byte, recLen, want)
				copy(msg, rec)
				continue
			}
			msg = append(msg, rec...)
			continue
		}

		msg = append(msg, rec...)
		if want == 0 && len(msg) >= 4 {
			want = 4 + (int(msg[1])<<16 | int(msg[2])<<8 | int(msg[3]))
			if want > maxPeekBytes {
				return fail(fmt.Errorf("handshake message too large: %d", want))
			}
		}
		if want > 0 && len(msg) >= want {
			break
		}
	}

	if msgType != 1 {
		return fail(fmt.Errorf("not a ClientHello: handshake type %d", msgType))
	}
	sni := parseSNI(msg[4:want])
	return &replayConn{
		Conn:    conn,
		head:    raw,
		poolBuf: releaseOrAttachPoolBuf(bp, raw),
	}, sni, nil
}

// parseSNI walks a ClientHello body for the server_name extension.
func parseSNI(body []byte) string {
	// version(2) random(32) session_id_len(1)
	if len(body) < 35 {
		return ""
	}
	p := body[34:]
	sidLen := int(p[0])
	if len(p) < 1+sidLen {
		return ""
	}
	p = p[1+sidLen:]

	// cipher_suites(2+)
	if len(p) < 2 {
		return ""
	}
	csLen := int(p[0])<<8 | int(p[1])
	if len(p) < 2+csLen {
		return ""
	}
	p = p[2+csLen:]

	// compression_methods(1+)
	if len(p) < 1 {
		return ""
	}
	compLen := int(p[0])
	if len(p) < 1+compLen {
		return ""
	}
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
	p = p[:extLen]

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
			if listLen > len(e) {
				return ""
			}
			e = e[:listLen]
			for len(e) >= 3 {
				nameType := e[0]
				nameLen := int(e[1])<<8 | int(e[2])
				if len(e) < 3+nameLen {
					return ""
				}
				if nameType == 0 { // host_name
					return string(e[3 : 3+nameLen])
				}
				e = e[3+nameLen:]
			}
			return ""
		}
		p = p[4+elen:]
	}
	return ""
}

// replayConn is a net.Conn that serves the peeked bytes first, then the
// wrapped conn. Everything but Read delegates straight through.
type replayConn struct {
	net.Conn
	head    []byte
	poolBuf *[]byte
}

func (c *replayConn) Read(p []byte) (int, error) {
	if len(c.head) > 0 {
		n := copy(p, c.head)
		c.head = c.head[n:]
		if len(c.head) == 0 && c.poolBuf != nil {
			*c.poolBuf = (*c.poolBuf)[:0]
			peekBufPool.Put(c.poolBuf)
			c.poolBuf = nil
			c.head = nil
		}
		return n, nil
	}
	return c.Conn.Read(p)
}
