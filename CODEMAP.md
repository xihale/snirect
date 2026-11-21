# Snirect Codemap

> **Token Guard**: Discovery first (`csearch` MCP `search_code` or `rg -n`), then targeted `read_file` slices (`<= 120-180` lines around hit lines). Android UI copy: two-step `rg -l "<text>" android/app/src/main/res` -> `rg -n "<key>"` (`strings.xml` is excluded from `.csearch`).

## 1. Architecture & Directory Index

Snirect is a local TLS SNI-rewriting MITM proxy (built-in static rules synced from Cealing-Host, passthrough on miss) across **Desktop Go CLI** and **Android VPN (`gomobile` + gVisor netstack)**, plus a static landing site (`site/`).

| Module | Key Paths | Responsibility |
|---|---|---|
| `cmd/snirect` | `cmd/snirect/main.go` | CLI binary entry -> `cli.Execute()` |
| `internal/cli` | `internal/cli/{root,proxy,cert,cert_firefox,service_cmd,rules_cmd,config_cmd,status,update,lifecycle,completion}.go` | Cobra commands, proxy lifecycle, cert install, service/update UX |
| `internal/proxy` | `internal/proxy/{proxy,peek,verify,hook,github}.go` | ClientHello SNI peek, MITM relay / passthrough, cert verify & hooks |
| `internal/rules` | `internal/rules/{rules,builtin_rules,defaults,certpolicy,export}.go` | Built-in rules (`builtin_rules.go` 40KB), policy match, `dae`/`clash` export |
| `internal/cert` | `internal/cert/{cert_manager,tls}.go` | Local root CA & dynamic leaf certificate issuance |
| `internal/dns` | `internal/dns/{resolver_base,resolver_std}.go` | Shared DoH/DoT/bootstrap DNS resolution |
| `internal/config` | `internal/config/{config,defaults,loader}.go` | Config schema, defaults, file loading |
| `internal/sysproxy` | `internal/sysproxy/{sysproxy,status}_{linux,darwin,windows}.go` · `firefox.go` | OS PAC/system proxy & NSS/Firefox cert status |
| `internal/service` | `internal/service/{service,install,cert}.go` · `service_{linux,darwin,windows}.go` · `packaged_*.go` | User background daemon (`systemd --user`, `LaunchAgent`, Windows Run) |
| `internal/update` | `internal/update/{check,client,download,version}.go` | Self-update check & release download |
| `android/bindings` | `android/bindings/{lib,netstack,engine,update,log,log_bridge,platform_*,sockdiag_*}.go` | `gomobile bind` Go `core` pkg -> `core.aar` (gVisor TUN + synthetic `CONNECT`) |
| `android/app` | `android/app/src/main/kotlin/com/xihale/snirect/{MainActivity.kt,ktlib/*.kt,service/*.kt,ui/*/*.kt,data/*/*.kt,util/*.kt}` | Jetpack Compose app + `SnirectVpnService` + `SnirectCoreClient` |
| `site` | `site/{index.html,styles.css,script.js,DESIGN.md}` | Static website / landing page (branch site/v2) |

## 2. Key Entry Points & Platform Differences

- **Desktop CLI**: `cmd/snirect/main.go` -> `internal/cli/root.go` (`Execute`); runs `internal/proxy` directly on OS TCP listeners and configures OS PAC/CA trust via `internal/sysproxy` & `internal/service`.
- **Android VPN**: `MainActivity.kt` (`NavHost` routes: `home`, `help`, `settings`, `risk_settings`, `dns`, `app_whitelist`, `auto_excluded`, `logs`) + `service/SnirectVpnService.kt` -> `ktlib/SnirectCoreClient.kt` -> `android/bindings/lib.go` (`SetDataDir`, `StartEngine`, `StopEngine`, `GetCACertificate`, `CheckUpdate`).
  - **Android-only bridge**: `bindings/netstack.go` terminates TUN IP packets in gVisor and synthesizes `CONNECT <dst-ip>:<port>` with `X-Snirect-Src` / `X-Snirect-Uid` (`sockdiag_netlink.go` + `LookupUID`) to loopback `proxy.ProxyServer`. Outbound sockets use `bypassDialer` -> `EngineCallbacks.Protect(fd)`. Cert-pinning rejections trigger `OnCertRejected` for auto-exclusion (`AutoExcludeScreen.kt`).
  - **Desktop-only**: OS PAC/system proxy (`internal/sysproxy`), Firefox NSS integration (`cert_firefox.go`), background user daemon (`internal/service`), rule export (`rules export`).
- **Large files (>25KB — read in slices)**: `internal/rules/builtin_rules.go`, `android/app/.../ui/screens/HelpScreen.kt`, `android/app/.../service/SnirectVpnService.kt`.

## 3. Build, Lint & Test Commands (1-Line Cheatsheet)

- **Go CLI & tests**: `make build` (`dist/snirect`) · `make test` (`go test -v -race ./...`) · `make lint` (`GOTOOLCHAIN=go1.26.2 golangci-lint run --timeout=5m`).
- **Android AAR & APK**: `make core` (`gomobile bind` -> `android/app/libs/core.aar`) · `make debug` (`core` + Debug APK) · `make release` (`arm64` Release APK).
- **Android Gradle directly**: `cd android && ./gradlew assembleDebug` (or `lintDebug` / `testDebugUnitTest`).
