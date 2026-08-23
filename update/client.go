package update

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/xihale/snirect/config"
	"github.com/xihale/snirect/proxy"
	"github.com/xihale/snirect/rules"
)

// Client talks to GitHub Releases using the same Hosts-IP + empty-SNI path
// the proxy uses, so a check works even when the proxy itself is not up.
type Client struct {
	Rules   *rules.Rules
	APIBase string
	HTTP    *http.Client
	// Do, if set, replaces HTTP.Do. Tests inject this to avoid the network.
	Do func(*http.Request) (*http.Response, error)
}

const defaultAPIBase = "https://api.github.com"

// New returns a Client that dials GitHub via built-in Hosts/SNI rules.
func New() *Client {
	r, err := rules.LoadRules()
	if err != nil || r == nil {
		r = rules.NewRules()
	}
	return &Client{
		Rules:   r,
		APIBase: defaultAPIBase,
		HTTP: &http.Client{
			Timeout:   0, // callers bound the whole op with context
			Transport: newTransport(r),
		},
	}
}

func newTransport(r *rules.Rules) *http.Transport {
	return &http.Transport{
		Proxy:             nil, // never inherit HTTP_PROXY (loop / chicken-egg)
		ForceAttemptHTTP2: false,
		DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialTLS(ctx, r, addr)
		},
	}
}

func dialTLS(ctx context.Context, r *rules.Rules, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	ip := host
	if mapped, ok := r.GetHost(host); ok && mapped != "" {
		ip = mapped
	}
	raw, err := (&net.Dialer{Timeout: 30 * time.Second}).DialContext(ctx, "tcp", net.JoinHostPort(ip, port))
	if err != nil {
		return nil, err
	}
	sni := host
	if alt, ok := r.GetAlterHostname(host); ok {
		sni = alt
	}
	tc := tls.Client(raw, &tls.Config{
		ServerName:         sni,
		InsecureSkipVerify: true,
		NextProtos:         []string{"http/1.1"},
	})
	if err := tc.HandshakeContext(ctx); err != nil {
		raw.Close()
		return nil, err
	}
	policy, ok := r.GetCertVerify(host)
	if !ok {
		policy, _ = rules.ParseCertPolicy(true)
	}
	if !proxy.VerifyCert(tc, host, sni, policy, config.PreparsedDefaultConfig.Security, r.GetIgnoreExpiry(host)) {
		tc.Close()
		return nil, fmt.Errorf("certificate rejected for %s", host)
	}
	return tc, nil
}

func (c *Client) roundTrip(req *http.Request) (*http.Response, error) {
	if c.Do != nil {
		return c.Do(req)
	}
	if c.HTTP == nil {
		return nil, fmt.Errorf("update client has no HTTP transport")
	}
	return c.HTTP.Do(req)
}

func (c *Client) get(ctx context.Context, rawURL, userAgent string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	if userAgent == "" {
		userAgent = "snirect"
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/vnd.github+json")
	return c.roundTrip(req)
}
