# internal/cmd Package

Cobra-based CLI command definitions for the Snirect binary.

## OVERVIEW
All user-facing commands are implemented here, organized as separate files and registered with the root command. The default action (no subcommand) starts the proxy.

## WHERE TO LOOK
| File | Command | Purpose |
|------|---------|---------|
| root.go | root | Defines root command and global flags (`--set-proxy`, `--log-level`) |
| run.go | (default) | Bootstraps the application: loads config, CA, starts proxy, sets system proxy |
| install.go | install | Installs binary to system path and registers as background service |
| uninstall.go | uninstall | Removes binary, service, and configuration |
| status.go | status | Shows proxy, certificate, and service status |
| version.go | version | Prints version information |
| update.go | update | Checks and applies updates for rules and binary |
| setup.go | setup | Initial setup and configuration wizard |
| service.go | service | Start/stop/restart service control |
| firefox.go | firefox-cert | Installs CA certificate into Firefox |
| shell_config.go | proxy-env | Prints shell commands to set proxy environment variables |
| env.go | env | Displays proxy-related environment variables |
| fetch_rules.go | fetch-rules | Syncs community rules from Cealing-Host |
| inspect.go | inspect | Shows effective configuration and rules |
| verify.go | verify | Tests certificate and connection verification |
| completion.go | completion | Generates shell completion scripts |
| modules.go | modules | Lists enabled build modules |
| modules_quic.go | modules-quic | QUIC-specific module information |

## CONVENTIONS
- Each command lives in its own file with a `cobra.Command` struct.
- `Run` or `RunE` performs the command logic; errors logged and may exit.
- Global flags defined once in root.go; subcommands inherit.
- Use `logger` for user output and diagnostics.
- Commands typically call into internal packages (`proxy`, `app`, `config`, etc.).
- Follows project-wide Go conventions (camelCase, error wrapping).

## ANTI-PATTERNS
None specific; see root for global rules.

## TESTING
```bash
go test ./internal/cmd
```
Most commands are integration-focused; unit tests minimal.