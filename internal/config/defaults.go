package config

import _ "embed"

import ruleslib "github.com/xihale/snirect-shared/rules"

//go:generate go run tools/gendefaults/main.go .

//go:embed config.default.toml
var DefaultConfigTOML string

var DefaultRulesTOML = ruleslib.FetchedRulesTOML

//go:embed config.toml
var SampleConfigTOML string

var SampleRulesTOML = ruleslib.UserRulesTOML

//go:embed pac
var DefaultPAC string
