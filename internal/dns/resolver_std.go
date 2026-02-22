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
	"sync"
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

// exchangeParallel sends the same DNS query to multiple upstreams concurrently.
// It returns the first successful reply, or the last error if all upstreams fail.
// The caller provides a context for cancellation and timeout control.
func exchangeParallel(ctx context.Context, m *dns.Msg, upstreams []stdUpstream) (*dns.Msg, string, error) {
	if len(upstreams) == 1 {
		reply, err := upstreams[0].Exchange(m)
		if err != nil {
			return nil, "", err
		}
		return reply, upstreams[0].Address(), nil
	}

	type result struct {
		reply *dns.Msg
		addr  string
		err   error
	}

	resCh := make(chan result, len(upstreams))
	var wg sync.WaitGroup
	wg.Add(len(upstreams))

	for _, u := range upstreams {
		go func(u stdUpstream) {
			defer wg.Done()
			reply, err := u.Exchange(m)
			select {
			case resCh <- result{reply: reply, addr: u.Address(), err: err}:
			case <-ctx.Done():
				// Context cancelled before we could send, skip this result
			}
		}(u)
	}

	// Helper goroutine to close channel when all senders are done
	go func() {
		wg.Wait()
		close(resCh)
	}()

	var lastErr error
	received := 0
	for received < len(upstreams) {
		select {
		case res, ok := <-resCh:
			if !ok {
				// Channel closed, no more results expected
				break
			}
			received++
			if res.err == nil && res.reply != nil {
				return res.reply, res.addr, nil
			}
			lastErr = res.err
		case <-ctx.Done():
			// Timeout or cancellation; return with whatever error we have
			if lastErr != nil {
				return nil, "", lastErr
			}
			return nil, "", ctx.Err()
		}
	}

	if lastErr != nil {
		return nil, "", lastErr
	}
	return nil, "", fmt.Errorf("all upstreams failed")
}

func (b *stdBackend) Exchange(m *dns.Msg) (*dns.Msg, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), b.timeout)
	defer cancel()

	reply, addr, err := exchangeParallel(ctx, m, b.upstreams)
	if err != nil {
		return nil, "", err
	}
	return reply, addr, nil
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
