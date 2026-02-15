package config

import _ "embed"

import ruleslib "github.com/xihale/snirect-shared/rules"

//go:generate go run tools/gendefaults/main.go .

//go:embed config.default.toml
var DefaultConfigTOML string

var DefaultRulesTOML = ruleslib.DefaultRulesTOML

//go:embed config.toml
var SampleConfigTOML string

// SampleRulesTOML is not available in the shared rules package; leave empty.
var SampleRulesTOML = ""

//go:embed pac
var DefaultPAC string
