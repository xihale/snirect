package config

import _ "embed"

//go:generate go run tools/gendefaults/main.go .

//go:embed config.default.toml
var DefaultConfigTOML string

//go:embed rules.default.toml
var DefaultRulesTOML string

//go:embed config.toml
var SampleConfigTOML string

//go:embed rules.toml
var SampleRulesTOML string

//go:embed pac
var DefaultPAC string
