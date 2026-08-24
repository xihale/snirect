package proxy

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/xihale/snirect/internal/logger"
)

// GitHub archive / "Download ZIP" URLs 302 to codeload.github.com. The
// dedicated codeload vhost (real SNI) currently 301s back to
// github.com/<owner>/<repo>/tar.gz/... which 404s. Empty SNI to the Hosts IP
// for codeload still serves the blob. Terminating HTTP on these hosts lets us
// follow the first hop internally and never show the client the broken 301.

func isGitHubHTTPHost(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	switch host {
	case "github.com", "www.github.com", "codeload.github.com":
		return true
	}
	return false
}

func isCodeloadBlobPath(path string) bool {
	// /<owner>/<repo>/(tar.gz|zip|legacy.tar.gz|legacy.zip)/...
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) < 3 {
		return false
	}
	switch parts[2] {
	case "tar.gz", "zip", "legacy.tar.gz", "legacy.zip", "tarball", "zipball":
		return parts[0] != "" && parts[1] != ""
	}
	return false
}

// internableRedirect maps a GitHub blob redirect onto a host+path we can fetch
// with the SNI-stripped Hosts IP. Anything else (login, HTML pages) is left
// for the client to follow.
func internableRedirect(location string) (host, path string, ok bool) {
	if location == "" {
		return "", "", false
	}
	u, err := url.Parse(location)
	if err != nil || u.Host == "" {
		return "", "", false
	}
	h := strings.ToLower(u.Hostname())
	reqURI := u.RequestURI()
	switch {
	case h == "codeload.github.com":
		return "codeload.github.com", reqURI, true
	case (h == "github.com" || h == "www.github.com") && isCodeloadBlobPath(u.Path):
		return "codeload.github.com", reqURI, true
	case h == "githubusercontent.com" || strings.HasSuffix(h, ".githubusercontent.com"):
		return h, reqURI, true
	}
	return "", "", false
}

// githubHook adapts the GitHub archive/codeload workaround below to the
// tunnelHook registry: pin HTTP/1.1 upstream and serve the established
// tunnel through the H1 intercept loop so broken redirects can be
// intern-followed before the client ever sees them.
type githubHook struct{}

func (githubHook) match(host, sni string) bool { return isGitHubHTTPHost(host) || isGitHubHTTPHost(sni) }
func (githubHook) pinALPN() []string           { return []string{"http/1.1"} }
func (githubHook) interceptsH1() bool          { return true }

func (githubHook) serveH1(s *ProxyServer, client, remote net.Conn, host, clientAddr string, ctx context.Context) {
	s.serveGitHubH1(client, remote, host, clientAddr, ctx)
}

func (s *ProxyServer) serveGitHubH1(client, remote net.Conn, host, clientAddr string, ctx context.Context) {
	defer client.Close()
	defer remote.Close()

	cbr := bufio.NewReader(client)
	rbr := bufio.NewReader(remote)
	for {
		req, err := http.ReadRequest(cbr)
		if err != nil {
			return
		}
		resp, err := s.githubRoundTrip(ctx, req, remote, rbr, host, clientAddr)
		_ = req.Body.Close()
		if err != nil {
			logger.Upstream().Debug("github intern round-trip failed", "host", host, "path", req.URL.Path, "error", err)
			return
		}
		err = resp.Write(client)
		_ = resp.Body.Close()
		if err != nil {
			return
		}
		if req.Close || resp.Close {
			return
		}
	}
}

func (s *ProxyServer) githubRoundTrip(ctx context.Context, req *http.Request, remote net.Conn, rbr *bufio.Reader, host, clientAddr string) (*http.Response, error) {
	if internHost, path, ok := internableRequest(host, req.URL.RequestURI()); ok {
		return s.fetchIntern(ctx, clientAddr, req.Method, internHost, path, req.Header, 0)
	}

	if err := req.Write(remote); err != nil {
		return nil, err
	}
	resp, err := http.ReadResponse(rbr, req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		if internHost, path, ok := internableRedirect(resp.Header.Get("Location")); ok {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			return s.fetchIntern(ctx, clientAddr, req.Method, internHost, path, req.Header, 0)
		}
	}
	return resp, nil
}

func internableRequest(host, reqURI string) (internHost, path string, ok bool) {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	u, err := url.Parse(reqURI)
	if err != nil {
		return "", "", false
	}
	// Only rewrite the broken 301 target (github.com/<owner>/<repo>/tar.gz/...).
	// Direct CONNECT to codeload already uses the Hosts IP + empty SNI; pass it
	// through and intern-follow only if that hop still 301s.
	if (host == "github.com" || host == "www.github.com") && isCodeloadBlobPath(u.Path) {
		return "codeload.github.com", reqURI, true
	}
	return "", "", false
}

const maxInternRedirects = 4

func (s *ProxyServer) fetchIntern(ctx context.Context, clientAddr, method, host, path string, hdr http.Header, depth int) (*http.Response, error) {
	if depth > maxInternRedirects {
		return nil, errTooManyInternRedirects
	}
	targetSNI := s.determineSNI(host, host)
	rc, err := s.connectToRemote(ctx, host, "443", clientAddr, targetSNI, []string{"http/1.1"})
	if err != nil {
		return nil, err
	}
	if !s.verifyServerCert(rc, host, targetSNI) {
		rc.Close()
		return nil, errUpstreamCert
	}

	u, err := url.ParseRequestURI(path)
	if err != nil {
		u = &url.URL{Path: path}
	}
	out := &http.Request{
		Method:     method,
		URL:        &url.URL{Scheme: "https", Host: host, Path: u.Path, RawQuery: u.RawQuery},
		Host:       host,
		Header:     internRequestHeader(hdr),
		Close:      true,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
	}

	if err := out.Write(rc); err != nil {
		rc.Close()
		return nil, err
	}
	resp, err := http.ReadResponse(bufio.NewReader(rc), out)
	if err != nil {
		rc.Close()
		return nil, err
	}
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		if internHost, next, ok := internableRedirect(resp.Header.Get("Location")); ok {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			rc.Close()
			return s.fetchIntern(ctx, clientAddr, method, internHost, next, hdr, depth+1)
		}
	}
	// Body is tied to rc; the caller Close()s the body which closes the conn
	// via the response's underlying reader once drained. Pin rc to the body
	// so a short-read still releases the socket.
	resp.Body = &connBody{ReadCloser: resp.Body, c: rc}
	logger.Upstream().Debug("github intern fetch", "host", host, "path", path, "status", resp.StatusCode)
	return resp, nil
}

func internRequestHeader(src http.Header) http.Header {
	h := make(http.Header)
	if src == nil {
		return h
	}
	for _, k := range []string{"User-Agent", "Range", "Authorization", "If-None-Match", "If-Modified-Since", "Accept"} {
		if v := src.Values(k); len(v) > 0 {
			h[k] = append([]string(nil), v...)
		}
	}
	return h
}

type connBody struct {
	io.ReadCloser
	c net.Conn
}

func (b *connBody) Close() error {
	err := b.ReadCloser.Close()
	_ = b.c.Close()
	return err
}

var (
	errUpstreamCert           = errors.New("upstream certificate rejected")
	errTooManyInternRedirects = errors.New("too many intern redirects")
)
