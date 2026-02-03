package config

import _ "embed"

//go:embed config.toml
var DefaultConfigTOML string

//go:embed rules.toml
var DefaultRulesTOML string

//go:embed pac
var DefaultPAC string
