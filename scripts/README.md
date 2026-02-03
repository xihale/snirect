# Update Rules Script

This script downloads the latest Cealing-Host rules and updates the default configuration.

## Usage

```bash
cd scripts
go run update_rules.go
```

## What it does

1. Downloads Cealing-Host.toml from the specified version
2. Parses the TOML configuration
3. Updates rules and PAC configuration:
   - `internal/config/rules.toml` (alter_hostname and hosts)
   - `internal/config/pac` (PAC domains)

## Configuration

Edit the constants in `update_rules.go` to update to a different version:

```go
const (
    CealingHostURL     = "https://github.com/SpaceTimee/Cealing-Host/releases/download/1.1.4.41/Cealing-Host.toml"
    CealingHostVersion = "1.1.4.41"
)
```
