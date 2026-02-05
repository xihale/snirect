package cmd

import (
	"embed"
)

//go:embed completions/bash completions/zsh completions/fish completions/powershell
var completionsFS embed.FS
