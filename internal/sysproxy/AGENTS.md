# internal/sysproxy Package

System proxy configuration via PAC URL and Firefox certificate handling.

## OVERVIEW
Abstracts platform-specific proxy settings: modifying system proxy preferences (or environment) and installing/removing root CA from browser trust stores, with special handling for Firefox.

## WHERE TO LOOK
| File | Platform | Purpose |
|------|----------|---------|
| sysproxy.go | all | Main interfaces and common functions |
| sysproxy_linux.go | Linux | Uses `gsettings`/`dconf` (GNOME) or environment variables |
| sysproxy_darwin.go | macOS | Uses `networksetup` CLI |
| sysproxy_windows.go | Windows | Registry edits and WinHTTP APIs; uses unsafe pointers |
| firefox.go | all | Firefox cert DB operations (certutil, NSS) |

## CONVENTIONS
- Build tags (`//go:build`) separate OS implementations.
- Commands executed via `exec.Command`; output logged.
- On Windows, uses unsafe pointers for FFI (isolated to this package).
- Firefox handling locates cert DB, imports CA using `certutil` or direct NSS calls.
- All operations return user-friendly errors.
- PAC URL is provided to system settings; proxy bypass list includes localhost.

## ANTI-PATTERNS
- NEVER use unsafe pointers outside platform-specific files (already scoped).
- Global: InsecureSkipVerify must be paired with manual verification (not applicable here).
- See root for additional anti-patterns.

## TESTING
Manual testing on each OS:
```bash
snirect set-proxy
snirect unset-proxy
snirect install-cert
snirect firefox-cert
```