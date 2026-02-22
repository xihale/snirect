# internal/app Package

Installation, uninstallation, and service management across platforms.

## OVERVIEW
Handles installing Snirect as a system service, uninstalling, and managing certificate trust. Platform-specific logic is isolated via build tags.

## WHERE TO LOOK
| File | Platform | Purpose |
|------|----------|---------|
| install.go | all | Common install logic (copy binary, create dirs) |
| install_linux.go | Linux | Systemd service installation |
| install_darwin.go | macOS | Launchd plist installation |
| install_windows.go | Windows | Task Scheduler registration |
| uninstall.go | all | Common uninstall logic |
| uninstall_*.go | platform-specific | Removal of service and files |
| cert.go | all | Root CA installation to system trust store |

## CONVENTIONS
- Build tags (`//go:build`) select OS-specific files at compile time.
- Uses `sysproxy` to configure system proxy during install.
- On Windows, elevated privileges required; uses task scheduler.
- Binary installed to `~/.local/bin` or `/usr/local/bin` on Unix; `%APPDATA%` on Windows.
- Service runs silently in background after installation.
- Logging via `logger` for user feedback.

## ANTI-PATTERNS
None specific beyond global anti-patterns. Ensure privileged operations are properly gated.

## TESTING
Manual install/uninstall on each platform:
```bash
snirect install
snirect uninstall
```