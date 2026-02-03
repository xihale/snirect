package config

type Config struct {
	CheckHostname interface{}  `toml:"check_hostname"` // bool or string "strict"
	SetProxy      bool         `toml:"setproxy"`
	ImportCA      string       `toml:"importca"` // "auto", "always", "never"
	IPv6          bool         `toml:"ipv6"`
	ECS           string       `toml:"ecs"` // CIDR or "auto"
	DNS           DNSConfig    `toml:"DNS"`
	Log           LogConfig    `toml:"log"`
	Server        ServerConfig `toml:"server"`
}

type DNSConfig struct {
	Nameserver   []string `toml:"nameserver"`
	BootstrapDNS []string `toml:"bootstrap_dns"`
}

type LogConfig struct {
	Level string `toml:"loglevel"`
	File  string `toml:"logfile"`
}

type ServerConfig struct {
	Address string `toml:"address"`
	Port    int    `toml:"port"`
	PACHost string `toml:"pac_host"`
}
