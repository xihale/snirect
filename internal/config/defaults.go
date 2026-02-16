package config

import _ "embed"

import ruleslib "github.com/xihale/snirect-shared/rules"

//go:generate go run tools/gendefaults/main.go .

//go:embed config.default.toml
var DefaultConfigTOML string

// FetchedRulesTOML contains rules fetched from upstream (Cealing-Host).
var FetchedRulesTOML = ruleslib.FetchedRulesTOML

// UserRulesTOML contains the default user rules template.
var UserRulesTOML = ruleslib.UserRulesTOML

//go:embed config.toml
var SampleConfigTOML string

//go:embed pac
var DefaultPAC string
