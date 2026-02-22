package proxy

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"time"

	"snirect/internal/logger"
)

// connectContext holds shared data during the connection state machine.
type connectContext struct {
	clientConn    net.Conn
	tlsClientConn *tls.Conn
	remoteConn    net.Conn
	host          string
	port          string
	clientHello   string
	targetSNI     string
	parentCtx     context.Context
}

// connectState represents one step in the connection state machine.
type connectState func(ctx *connectContext) (connectState, error)

// cleanupConnect closes all connections that are still open.
func (ps *ProxyServer) cleanupConnect(ctx *connectContext) {
	if ctx.tlsClientConn != nil {
		ctx.tlsClientConn.Close()
	}
	if ctx.remoteConn != nil {
		ctx.remoteConn.Close()
	}
	if ctx.clientConn != nil {
		ctx.clientConn.Close()
	}
}

// extractClientIP parses the client IP from a remote address.
func extractClientIP(remoteAddr net.Addr) net.IP {
	clientIP, _, _ := net.SplitHostPort(remoteAddr.String())
	return net.ParseIP(clientIP)
}

// ========== State Methods ==========

// stateClientTLS performs TLS handshake with the client to extract SNI.
func (ps *ProxyServer) stateClientTLS(ctx *connectContext) (connectState, error) {
	tlsClientConn, clientHello, err := ps.handshakeClient(ctx.clientConn, ctx.host)
	if err != nil {
		return nil, fmt.Errorf("TLS handshake with client failed: %w", err)
	}
	ctx.tlsClientConn = tlsClientConn
	ctx.clientHello = clientHello
	return ps.stateDetermineSNI, nil
}

// stateDetermineSNI determines what SNI to use for the remote connection.
func (ps *ProxyServer) stateDetermineSNI(ctx *connectContext) (connectState, error) {
	targetSNI, ok := ps.Rules.GetAlterHostname(ctx.clientHello)
	if !ok {
		targetSNI = ctx.clientHello
	}
	if targetSNI == "" {
		logger.Debug("Stripping SNI for %s", ctx.host)
	} else if targetSNI != ctx.clientHello {
		logger.Debug("Replacing SNI for %s with %s", ctx.host, targetSNI)
	}
	ctx.targetSNI = targetSNI
	return ps.stateRemoteDial, nil
}

// stateRemoteDial connects to the remote server.
func (ps *ProxyServer) stateRemoteDial(ctx *connectContext) (connectState, error) {
	// Resolve IP
	clientIP := extractClientIP(ctx.clientConn.RemoteAddr())
	remoteIP, err := ps.Resolver.Resolve(ctx.parentCtx, ctx.host, clientIP)
	if err != nil {
		ps.Resolver.Invalidate(ctx.host)
		return nil, fmt.Errorf("DNS resolution failed: %w", err)
	}
	remoteAddr := net.JoinHostPort(remoteIP, ctx.port)

	// Dial TCP
	timeout := time.Duration(ps.Config.Timeout.Dial) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	dialer := &net.Dialer{Timeout: timeout}
	netConn, err := dialer.DialContext(ctx.parentCtx, "tcp", remoteAddr)
	if err != nil {
		ps.Resolver.Invalidate(ctx.host)
		return nil, fmt.Errorf("dial failed to %s: %w", remoteAddr, err)
	}

	// TLS handshake
	remoteConn := tls.Client(netConn, &tls.Config{
		ServerName:         ctx.targetSNI,
		InsecureSkipVerify: true,
	})
	if err := remoteConn.Handshake(); err != nil {
		netConn.Close()
		return nil, fmt.Errorf("remote handshake failed: %w", err)
	}

	ctx.remoteConn = remoteConn
	return ps.stateVerifyCert, nil
}

// stateVerifyCert verifies the remote server's certificate.
func (ps *ProxyServer) stateVerifyCert(ctx *connectContext) (connectState, error) {
	// verifyServerCert expects *tls.Conn
	remoteTLS, ok := ctx.remoteConn.(*tls.Conn)
	if !ok {
		return nil, errors.New("remote connection is not TLS")
	}
	if !ps.verifyServerCert(remoteTLS, ctx.host, ctx.targetSNI) {
		// verifyServerCert already logged details
		return nil, errors.New("certificate verification failed")
	}
	return ps.stateTunnel, nil
}

// stateTunnel pipes data between client and remote and terminates the state machine.
func (ps *ProxyServer) stateTunnel(ctx *connectContext) (connectState, error) {
	ps.tunnel(ctx.clientConn, ctx.remoteConn)
	// Mark connections as cleaned to avoid double-close
	ctx.clientConn = nil
	ctx.remoteConn = nil
	if ctx.tlsClientConn != nil {
		ctx.tlsClientConn.Close()
		ctx.tlsClientConn = nil
	}
	return nil, nil
}

// stateDirectDial bypasses MITM and connects client directly to remote.
func (ps *ProxyServer) stateDirectDial(ctx *connectContext) (connectState, error) {
	ps.directTunnel(ctx.parentCtx, ctx.clientConn, ctx.host, ctx.port)
	// directTunnel closes clientConn via tunnel; mark as nil to avoid double-close
	ctx.clientConn = nil
	return nil, nil
}
