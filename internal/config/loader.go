package config

import (
	"fmt"
	"os"
	"path/filepath"
	"snirect/internal/logger"

	"github.com/pelletier/go-toml/v2"
	ruleslib "github.com/xihale/snirect-shared/rules"
)

const autoMarker = "__AUTO__"

type Rules struct {
	*ruleslib.Rules
}

func LoadRules(path string) (*Rules, error) {
	baseRules, err := ruleslib.LoadRules()
	if err != nil {
		return nil, fmt.Errorf("failed to load fetched rules: %w", err)
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Rules{Rules: baseRules}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read rules file: %w", err)
	}

	userRules := ruleslib.NewRules()
	if err := userRules.FromTOML(data); err != nil {
		return nil, fmt.Errorf("failed to parse user rules: %w", err)
	}

	mergeRulesWithOverride(baseRules, userRules)
	return &Rules{Rules: baseRules}, nil
}

func mergeRulesWithOverride(base, user *ruleslib.Rules) {
	if base.Hosts == nil {
		base.Hosts = make(map[string]string)
	}
	if base.AlterHostname == nil {
		base.AlterHostname = make(map[string]string)
	}
	if base.CertVerify == nil {
		base.CertVerify = make(map[string]interface{})
	}

	if user.AlterHostname != nil {
		for k, v := range user.AlterHostname {
			if v == autoMarker {
				delete(base.AlterHostname, k)
			} else {
				base.AlterHostname[k] = v
			}
		}
	}
	if user.CertVerify != nil {
		for k, v := range user.CertVerify {
			if v == autoMarker {
				delete(base.CertVerify, k)
			} else {
				base.CertVerify[k] = v
			}
		}
	}
	if user.Hosts != nil {
		for k, v := range user.Hosts {
			if v == autoMarker {
				delete(base.Hosts, k)
			} else {
				base.Hosts[k] = v
			}
		}
	}

	base.Init()
}

// LoadConfig loads configuration from a file.
func LoadConfig(path string) (*Config, error) {
	cfg := PreparsedDefaultConfig
	if cfg.Log.File == "" {
		cfg.Log.File = GetDefaultLogPath()
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &cfg, nil
	}
	if err != nil {
		return &cfg, fmt.Errorf("failed to read config file: %w", err)
	}

	// Detect which sections/fields are present using pointer-based structs.
	var presence struct {
		DNS        *DNSConfigPresence        `toml:"DNS"`
		Timeout    *TimeoutConfigPresence    `toml:"timeout"`
		Limit      *LimitConfigPresence      `toml:"limit"`
		Log        *LogConfigPresence        `toml:"log"`
		Server     *ServerConfigPresence     `toml:"server"`
		Preference *PreferenceConfigPresence `toml:"preference"`
		Update     *UpdateConfigPresence     `toml:"update"`
	}
	if err := toml.Unmarshal(data, &presence); err == nil {
		// Presence parse succeeded, unmarshal actual config and then merge missing fields.
		if err := toml.Unmarshal(data, &cfg); err != nil {
			return &cfg, fmt.Errorf("failed to parse user config: %w", err)
		}
		def := PreparsedDefaultConfig
		mergeDNS(&cfg.DNS, presence.DNS, def.DNS)
		mergeTimeout(&cfg.Timeout, presence.Timeout, def.Timeout)
		mergeLimit(&cfg.Limit, presence.Limit, def.Limit)
		mergeLog(&cfg.Log, presence.Log, def.Log)
		mergeServer(&cfg.Server, presence.Server, def.Server)
		mergePreference(&cfg.Preference, presence.Preference, def.Preference)
		mergeUpdate(&cfg.Update, presence.Update, def.Update)
	} else {
		// Presence parse failed; try regular unmarshal only.
		if err := toml.Unmarshal(data, &cfg); err != nil {
			return &cfg, fmt.Errorf("failed to parse user config: %w", err)
		}
	}

	if cfg.Log.File == "" {
		cfg.Log.File = GetDefaultLogPath()
	}

	return &cfg, nil
}

// Presence structs use pointer fields to detect which config keys were set by user.
type DNSConfigPresence struct {
	Nameserver   *[]string `toml:"nameserver"`
	BootstrapDNS *[]string `toml:"bootstrap_dns"`
}
type TimeoutConfigPresence struct {
	Dial *int `toml:"dial"`
	DNS  *int `toml:"dns"`
}
type LimitConfigPresence struct {
	MaxConns     *int `toml:"max_connections"`
	DNSCacheSize *int `toml:"dns_cache_size"`
}
type LogConfigPresence struct {
	Level *string `toml:"loglevel"`
	File  *string `toml:"logfile"`
}
type ServerConfigPresence struct {
	Address *string `toml:"address"`
	Port    *int    `toml:"port"`
	PACHost *string `toml:"pac_host"`
}
type PreferenceConfigPresence struct {
	Mode          *string `toml:"mode"`
	EnableTesting *bool   `toml:"enable_testing"`
	TestTimeoutMs *int    `toml:"test_timeout_ms"`
	TestParallel  *bool   `toml:"test_parallel"`
	MaxTestIPs    *int    `toml:"max_test_ips"`
	CacheTTL      *int    `toml:"cache_ttl"`
	CacheSize     *int    `toml:"cache_size"`
}
type UpdateConfigPresence struct {
	AutoCheckUpdate         *bool   `toml:"auto_check_update"`
	AutoCheckRules          *bool   `toml:"auto_check_rules"`
	AutoUpdate              *bool   `toml:"auto_update"`
	AutoUpdateRules         *bool   `toml:"auto_update_rules"`
	CheckIntervalHours      *int    `toml:"check_interval_hours"`
	RulesCheckIntervalHours *int    `toml:"rules_check_interval_hours"`
	RulesURL                *string `toml:"rules_url"`
}

// merge functions restore default values for fields not set by user (nil pointers).
func mergeDNS(dst *DNSConfig, pres *DNSConfigPresence, def DNSConfig) {
	if pres == nil {
		return
	}
	if pres.Nameserver == nil {
		dst.Nameserver = def.Nameserver
	}
	if pres.BootstrapDNS == nil {
		dst.BootstrapDNS = def.BootstrapDNS
	}
}
func mergeTimeout(dst *TimeoutConfig, pres *TimeoutConfigPresence, def TimeoutConfig) {
	if pres == nil {
		return
	}
	if pres.Dial == nil {
		dst.Dial = def.Dial
	}
	if pres.DNS == nil {
		dst.DNS = def.DNS
	}
}
func mergeLimit(dst *LimitConfig, pres *LimitConfigPresence, def LimitConfig) {
	if pres == nil {
		return
	}
	if pres.MaxConns == nil {
		dst.MaxConns = def.MaxConns
	}
	if pres.DNSCacheSize == nil {
		dst.DNSCacheSize = def.DNSCacheSize
	}
}
func mergeLog(dst *LogConfig, pres *LogConfigPresence, def LogConfig) {
	if pres == nil {
		return
	}
	if pres.Level == nil {
		dst.Level = def.Level
	}
	if pres.File == nil {
		dst.File = def.File
	}
}
func mergeServer(dst *ServerConfig, pres *ServerConfigPresence, def ServerConfig) {
	if pres == nil {
		return
	}
	if pres.Address == nil {
		dst.Address = def.Address
	}
	if pres.Port == nil {
		dst.Port = def.Port
	}
	if pres.PACHost == nil {
		dst.PACHost = def.PACHost
	}
}
func mergePreference(dst *PreferenceConfig, pres *PreferenceConfigPresence, def PreferenceConfig) {
	if pres == nil {
		return
	}
	if pres.Mode == nil {
		dst.Mode = def.Mode
	}
	if pres.EnableTesting == nil {
		dst.EnableTesting = def.EnableTesting
	}
	if pres.TestTimeoutMs == nil {
		dst.TestTimeoutMs = def.TestTimeoutMs
	}
	if pres.TestParallel == nil {
		dst.TestParallel = def.TestParallel
	}
	if pres.MaxTestIPs == nil {
		dst.MaxTestIPs = def.MaxTestIPs
	}
	if pres.CacheTTL == nil {
		dst.CacheTTL = def.CacheTTL
	}
	if pres.CacheSize == nil {
		dst.CacheSize = def.CacheSize
	}
}
func mergeUpdate(dst *UpdateConfig, pres *UpdateConfigPresence, def UpdateConfig) {
	if pres == nil {
		return
	}
	if pres.AutoCheckUpdate == nil {
		dst.AutoCheckUpdate = def.AutoCheckUpdate
	}
	if pres.AutoCheckRules == nil {
		dst.AutoCheckRules = def.AutoCheckRules
	}
	if pres.AutoUpdate == nil {
		dst.AutoUpdate = def.AutoUpdate
	}
	if pres.AutoUpdateRules == nil {
		dst.AutoUpdateRules = def.AutoUpdateRules
	}
	if pres.CheckIntervalHours == nil {
		dst.CheckIntervalHours = def.CheckIntervalHours
	}
	if pres.RulesCheckIntervalHours == nil {
		dst.RulesCheckIntervalHours = def.RulesCheckIntervalHours
	}
	if pres.RulesURL == nil {
		dst.RulesURL = def.RulesURL
	}
}

// EnsureConfig ensures default config files exist.
func EnsureConfig(force bool) (string, error) {
	appDir, err := GetAppDataDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(appDir, 0700); err != nil {
		return "", err
	}

	if err := ensureFile(filepath.Join(appDir, "config.toml"), SampleConfigTOML, force); err != nil {
		return "", err
	}
	if err := ensureFile(filepath.Join(appDir, "rules.toml"), UserRulesTOML, force); err != nil {
		return "", err
	}
	if err := ensureFile(filepath.Join(appDir, "pac"), DefaultPAC, force); err != nil {
		return "", err
	}

	return appDir, nil
}

func ensureFile(path, content string, force bool) error {
	if _, err := os.Stat(path); os.IsNotExist(err) || force {
		if force {
			logger.Info("Overwriting file: %s", path)
		} else {
			logger.Info("Creating default file: %s", path)
		}
		return os.WriteFile(path, []byte(content), 0644)
	} else if err != nil {
		return err
	}
	return nil
}

// GetPath resolves a path relative to workdir.
func GetPath(workDir, filename string) string {
	return filepath.Join(workDir, filename)
}

// GetAppDataDir returns the application data directory.
func GetAppDataDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	appDir := filepath.Join(configDir, "snirect")
	return appDir, nil
}

// ToLocalCertPolicy converts shared CertPolicy to local CertPolicy.
func ToLocalCertPolicy(sharedPolicy ruleslib.CertPolicy) CertPolicy {
	enabled := !sharedPolicy.Verify
	strict := sharedPolicy.Verify
	allowed := sharedPolicy.Allow

	if allowed == nil {
		allowed = []string{}
	}

	return CertPolicy{
		Enabled: enabled,
		Strict:  strict,
		Allowed: allowed,
	}
}

// GetCertVerify returns the cert verification policy for a host.
func (r *Rules) GetCertVerify(host string) (CertPolicy, bool) {
	if r.Rules == nil {
		return CertPolicy{}, false
	}

	sharedPolicy, ok := r.Rules.GetCertVerify(host)
	if !ok {
		return CertPolicy{}, false
	}

	return ToLocalCertPolicy(sharedPolicy), true
}

// GetAlterHostname returns the target SNI for a host.
func (r *Rules) GetAlterHostname(host string) (string, bool) {
	if r.Rules == nil {
		return "", false
	}

	return r.Rules.GetAlterHostname(host)
}

// GetHost returns the mapped IP for a host.
func (r *Rules) GetHost(host string) (string, bool) {
	if r.Rules == nil {
		return "", false
	}

	return r.Rules.GetHost(host)
}
