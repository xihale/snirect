package config

import (
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func BenchmarkLoadConfig_Runtime(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var cfg Config
		if err := toml.Unmarshal([]byte(DefaultConfigTOML), &cfg); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLoadConfig_Pregenerated(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		cfg := PreparsedDefaultConfig
		_ = cfg
	}
}

func BenchmarkLoadRules_Runtime(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var rules Rules
		if err := toml.Unmarshal([]byte(DefaultRulesTOML), &rules); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLoadRules_Pregenerated(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rules := PreparsedDefaultRules
		_ = rules
	}
}
