package proxy

import (
	"context"
	"net"
)

// tunnelHook lets site-specific MITM behaviors plug into the CONNECT
// pipeline without hardwiring host checks into handleTLS. Each hook
// declares which connections it owns and how they differ from the
// default blind pipe:
//
//   - match: whether this hook applies to the host/SNI pair (called on
//     every CONNECT, so keep it allocation-free).
//   - pinALPN: an ALPN list to offer upstream instead of mirroring the
//     client's offer. Return nil to keep the default behavior.
//   - interceptsH1: whether the established tunnel is served through an
//     HTTP/1.1 intercept loop instead of a plain byte pipe.
//
// Hooks are evaluated in registration order; the first match wins.
// P1 will move registration into the rules layer so new hooks can be
// declared by configuration rather than compiled in.
type tunnelHook interface {
	match(host, sni string) bool
	pinALPN() []string
	interceptsH1() bool
	serveH1(s *ProxyServer, client, remote net.Conn, host, clientAddr string, ctx context.Context)
}

// tunnelHooks is the ordered registry of built-in site hooks.
var tunnelHooks = []tunnelHook{
	githubHook{},
}

// tunnelHookFor returns the first hook matching host/sni, or nil when
// the connection should take the default pipe path.
func tunnelHookFor(host, sni string) tunnelHook {
	for _, h := range tunnelHooks {
		if h.match(host, sni) {
			return h
		}
	}
	return nil
}
