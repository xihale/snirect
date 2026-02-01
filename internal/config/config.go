package config

type Config struct {
	CheckHostname interface{}  `toml:"check_hostname"` // bool or string "strict"
	SetProxy      bool         `toml:"setproxy"`
	ImportCA      bool         `toml:"importca"`
	IPv6          bool         `toml:"ipv6"`
	Log           LogConfig    `toml:"log"`
	Server        ServerConfig `toml:"server"`
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

// DefaultConfig returns a Config with default values
func DefaultConfig() Config {
	return Config{
		CheckHostname: true,
		SetProxy:      true,
		ImportCA:      true,
		IPv6:          false,
		Log: LogConfig{
			Level: "DEBUG",
			File:  "",
		},
		Server: ServerConfig{
			Address: "127.0.0.1",
			Port:    7654,
			PACHost: "127.0.0.1",
		},
	}
}
