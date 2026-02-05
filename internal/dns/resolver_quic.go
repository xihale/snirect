//go:build quic

package dns

import (
	"log/slog"
	"snirect/internal/config"
	"snirect/internal/logger"
	"time"

	"github.com/AdguardTeam/dnsproxy/upstream"
	"github.com/miekg/dns"
)

type quicBackend struct {
	upstreams []upstream.Upstream
}

func (b *quicBackend) Exchange(m *dns.Msg) (*dns.Msg, string, error) {
	reply, u, err := upstream.ExchangeParallel(b.upstreams, m)
	if err != nil {
		return nil, "", err
	}
	return reply, u.Address(), nil
}

func newBackend(cfg *config.Config, rules *config.Rules) dnsBackend {
	// Completely silence library logs
	libLogger := slog.New(&discardHandler{})

	opts := &upstream.Options{
		Timeout: time.Duration(cfg.Timeout.DNS) * time.Second,
		Logger:  libLogger,
	}
	if opts.Timeout == 0 {
		opts.Timeout = 5 * time.Second
	}

	if len(cfg.DNS.BootstrapDNS) > 0 {
		var bootstrapResolvers []upstream.Resolver
		for _, bootAddr := range cfg.DNS.BootstrapDNS {
			bootRes, err := upstream.NewUpstreamResolver(bootAddr, &upstream.Options{
				Timeout: 3 * time.Second,
				Logger:  libLogger,
			})
			if err != nil {
				continue
			}
			bootstrapResolvers = append(bootstrapResolvers, bootRes)
		}

		if len(bootstrapResolvers) > 0 {
			opts.Bootstrap = upstream.ParallelResolver(bootstrapResolvers)
		}
	}

	var upstreams []upstream.Upstream
	for _, ns := range cfg.DNS.Nameserver {
		u, err := upstream.AddressToUpstream(ns, opts)
		if err != nil {
			logger.Warn("DNS: failed to create upstream %s: %v", ns, err)
			continue
		}
		upstreams = append(upstreams, u)
	}

	if len(upstreams) == 0 {
		return nil
	}

	return &quicBackend{upstreams: upstreams}
}
