//go:build !quic

package dns

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"snirect/internal/config"
	"snirect/internal/logger"
	"strings"
	"time"

	"github.com/miekg/dns"
)

type stdBackend struct {
	upstreams []stdUpstream
	timeout   time.Duration
}

type stdUpstream interface {
	Exchange(m *dns.Msg) (*dns.Msg, error)
	Address() string
}

func (b *stdBackend) Exchange(m *dns.Msg) (*dns.Msg, string, error) {
	if len(b.upstreams) == 1 {
		reply, err := b.upstreams[0].Exchange(m)
		if err != nil {
			return nil, "", err
		}
		return reply, b.upstreams[0].Address(), nil
	}

	type result struct {
		reply *dns.Msg
		addr  string
		err   error
	}

	resCh := make(chan result, len(b.upstreams))
	ctx, cancel := context.WithTimeout(context.Background(), b.timeout)
	defer cancel()

	for _, u := range b.upstreams {
		go func(u stdUpstream) {
			reply, err := u.Exchange(m)
			resCh <- result{reply: reply, addr: u.Address(), err: err}
		}(u)
	}

	var lastErr error
	for i := 0; i < len(b.upstreams); i++ {
		select {
		case res := <-resCh:
			if res.err == nil && res.reply != nil {
				return res.reply, res.addr, nil
			}
			lastErr = res.err
		case <-ctx.Done():
			if lastErr != nil {
				return nil, "", lastErr
			}
			return nil, "", ctx.Err()
		}
	}

	return nil, "", fmt.Errorf("all upstreams failed: %v", lastErr)
}

func newBackend(cfg *config.Config, rules *config.Rules) dnsBackend {
	timeout := time.Duration(cfg.Timeout.DNS) * time.Second
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	var upstreams []stdUpstream
	for _, ns := range cfg.DNS.Nameserver {
		u, err := parseUpstream(ns, timeout)
		if err != nil {
			logger.Warn("DNS: failed to parse upstream %s: %v", ns, err)
			continue
		}
		upstreams = append(upstreams, u)
	}

	if len(upstreams) == 0 {
		return nil
	}

	return &stdBackend{
		upstreams: upstreams,
		timeout:   timeout,
	}
}

func parseUpstream(addr string, timeout time.Duration) (stdUpstream, error) {
	if strings.HasPrefix(addr, "https://") {
		return &dohUpstream{addr: addr, client: &http.Client{Timeout: timeout}}, nil
	}
	if strings.HasPrefix(addr, "tls://") {
		return &dnsUpstream{addr: strings.TrimPrefix(addr, "tls://"), network: "tcp-tls", timeout: timeout}, nil
	}
	if strings.HasPrefix(addr, "tcp://") {
		return &dnsUpstream{addr: strings.TrimPrefix(addr, "tcp://"), network: "tcp", timeout: timeout}, nil
	}
	// Default to UDP
	hostPort := strings.TrimPrefix(addr, "udp://")
	if !strings.Contains(hostPort, ":") {
		hostPort += ":53"
	}
	return &dnsUpstream{addr: hostPort, network: "udp", timeout: timeout}, nil
}

type dnsUpstream struct {
	addr    string
	network string
	timeout time.Duration
}

func (u *dnsUpstream) Address() string { return u.addr }
func (u *dnsUpstream) Exchange(m *dns.Msg) (*dns.Msg, error) {
	client := &dns.Client{
		Net:     u.network,
		Timeout: u.timeout,
	}
	if u.network == "tcp-tls" {
		client.TLSConfig = &tls.Config{InsecureSkipVerify: false}
	}
	reply, _, err := client.Exchange(m, u.addr)
	return reply, err
}

type dohUpstream struct {
	addr   string
	client *http.Client
}

func (u *dohUpstream) Address() string { return u.addr }
func (u *dohUpstream) Exchange(m *dns.Msg) (*dns.Msg, error) {
	data, err := m.Pack()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", u.addr, strings.NewReader(string(data)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/dns-message")
	req.Header.Set("Accept", "application/dns-message")

	resp, err := u.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("doh status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	reply := new(dns.Msg)
	err = reply.Unpack(body)
	return reply, err
}
