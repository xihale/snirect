package container

import (
	"snirect/internal/config"
	"snirect/internal/dns"
	"snirect/internal/interfaces"
	"snirect/internal/proxy"
	"snirect/internal/upstream"
)

type Container struct {
	cfg      *config.Config
	rules    *config.Rules
	certMgr  interfaces.CertificateManager
	resolver interfaces.Resolver
	upstream *upstream.Client
	proxySrv *proxy.ProxyServer
}

func New(cfg *config.Config, rules *config.Rules) *Container {
	return &Container{cfg: cfg, rules: rules}
}

func (c *Container) GetConfig() *config.Config { return c.cfg }
func (c *Container) GetRules() *config.Rules   { return c.rules }

func (c *Container) GetCertManager() interfaces.CertificateManager {
	if c.certMgr == nil {
		panic("certificate manager not set - call SetCertManager first")
	}
	return c.certMgr
}

func (c *Container) SetCertManager(mgr interfaces.CertificateManager) { c.certMgr = mgr }

func (c *Container) GetResolver() interfaces.Resolver {
	if c.resolver == nil {
		c.resolver = dns.NewResolver(c.cfg, c.rules)
	}
	return c.resolver
}

func (c *Container) SetResolver(res interfaces.Resolver) { c.resolver = res }

func (c *Container) GetUpstreamClient() *upstream.Client {
	if c.upstream == nil {
		c.upstream = upstream.NewWithResolver(c.cfg, c.rules, c.GetResolver())
	}
	return c.upstream
}

func (c *Container) SetUpstreamClient(cli *upstream.Client) { c.upstream = cli }

func (c *Container) GetProxyServer() *proxy.ProxyServer {
	if c.proxySrv == nil {
		c.proxySrv = proxy.NewProxyServerWithResolver(c.cfg, c.rules, c.GetCertManager(), c.GetResolver())
	}
	return c.proxySrv
}

func (c *Container) SetProxyServer(srv *proxy.ProxyServer) { c.proxySrv = srv }

func (c *Container) Close() error {
	var err error
	if c.resolver != nil {
		if r, ok := c.resolver.(interface{ Close() error }); ok {
			err = r.Close()
		}
	}
	if c.certMgr != nil {
		if cerr := c.certMgr.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}
	return err
}
