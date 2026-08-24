<div align="center">

<img src="assets/logo/snirect_banner_dark.svg#gh-dark-mode-only" alt="Snirect" width="480">
<img src="assets/logo/snirect_banner_light.svg#gh-light-mode-only" alt="Snirect" width="480">

**按规则改写 TLS SNI 的跨平台代理**

桌面端 CLI（Linux / macOS / Windows）+ Android 应用，Go 实现

[![CI](https://github.com/xihale/snirect/actions/workflows/ci.yml/badge.svg)](https://github.com/xihale/snirect/actions/workflows/ci.yml)
[![Release](https://github.com/xihale/snirect/actions/workflows/release.yml/badge.svg)](https://github.com/xihale/snirect/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

</div>

---

## 这是什么

Snirect 是一个本地 HTTP/HTTPS 代理。对 TLS 流量，它在握手阶段嗅探 ClientHello 里的 SNI，按内置规则改写成伪装域名后转发，用来绕过基于 SNI 的封锁与干扰；需要时用本机生成的根 CA 做 MITM 解密。

核心能力：

- **SNI 改写**：握手时替换或剥掉 SNI，规则匹配 `example.com` / `*.example.com` / `*example.com` 三种粒度
- **证书策略**：`CertVerify` 放行伪装域名的证书、`IgnoreExpiry` 放行过期证书、`Hosts` 写死 IP 跳过 DNS
- **按需 MITM**：根 CA 私钥只存本机，签发叶证书带缓存；Firefox 单独走 NSS 库安装
- **加密 DNS**：DoH / DoT / UDP / TCP 上游 + IP bootstrap + EDNS Client Subnet + IPv6/IPv4 优选测速
- **系统代理**：一键设置 PAC，导出终端 `http_proxy` 环境变量
- **站点钩子**：可插拔的 tunnel hook（如 GitHub codeload 重定向修正），不硬编码进转发路径

## 快速开始

预编译包在 [Releases](https://github.com/xihale/snirect/releases)，或自己编译：

```bash
make build
```

运行：

```bash
./dist/snirect -s          # 前台运行，-s 顺带设系统代理
./dist/snirect cert install # HTTPS 解密必须先装根 CA
```

HTTPS 站点解密依赖本机信任 Snirect 根 CA。Firefox 有独立的证书库，需要再加一条：

```bash
./dist/snirect cert firefox install
```

装成后台服务（systemd user / launchd / Windows Service）：

```bash
./dist/snirect install
./dist/snirect status      # 服务、系统代理、CA、当前配置一览
```

完整命令表见 [docs/cli.md](docs/cli.md)。

## Android

仓库同时包含 Android 应用（`android/`），通过 gomobile 把 Go 核心编译成 `core.aar`，Kotlin 层用 `VpnService`（TUN）+ [gVisor netstack](https://gvisor.dev) 用户态协议栈抓取整机流量，合成 CONNECT 请求回灌给环回上的 Go 代理。出站 socket 经 `VpnService.protect()` 绕回 TUN 防止环路。

数据链路与实现细节见 [android/bindings/README.md](android/bindings/README.md)。构建方式见 [docs/build.md](docs/build.md)。

## 平台支持

| 平台 | 架构 | 系统代理 | CA 安装 |
| :--- | :--- | :--- | :--- |
| Linux | amd64 / arm64 | PAC (gsettings / KDE) | NSS + p11-kit |
| macOS | amd64 / arm64 | 系统网络设置 | Keychain |
| Windows | amd64 / arm64 | 注册表 PAC | 系统证书库 |
| Android | arm64 / x86_64 | VpnService 全量接管 | 用户证书导入引导 |

## 配置

配置文件位于 `~/.config/snirect/config.toml`（Windows 为 `%APPDATA%\snirect\config.toml`），全部项都有内置默认值，注释掉即回落默认。支持 DNS 上游、ECS、IP 优选、超时、并发限制等。详见 [docs/config.md](docs/config.md) 与 `internal/config/defaults.go` 中的完整示例。

分流规则静态编进二进制（`internal/rules/builtin_rules.go`），其中 SNI 改写与 Hosts 表同步自 [Cealing-Host](https://github.com/SpaceTimee/Cealing-Host) 项目并做了适配，无运行时拉取。

## 目录结构

```text
├── cmd/snirect/        CLI 入口
├── internal/
│   ├── cert/           根 CA 管理、叶证书签发与缓存
│   ├── cli/            cobra 命令：install/cert/proxy/config/update…
│   ├── config/         TOML 加载、默认值、样例配置
│   ├── dns/            DoH/DoT/plain 解析器、bootstrap、优选
│   ├── logger/         日志
│   ├── proxy/          代理核心：CONNECT、SNI 嗅探改写、tunnel hook
│   ├── rules/          内置分流规则与匹配索引
│   ├── service/        后台服务封装（kardianos/service）
│   ├── sysproxy/       各平台系统代理与 CA 安装实现
│   └── update/         GitHub Releases 自更新 + SHA256 校验
├── android/            Android 应用（Kotlin + Compose）
│   └── bindings/       gomobile 绑定层，产出 core.aar
├── docs/               命令、配置、构建文档
└── scripts/            CI 辅助脚本
```

## 开发

```bash
make test     # 单元测试
make lint     # golangci-lint
make crossAll # 6 平台交叉编译
make debug    # Android Debug 一键构建
```

发布流程是 git tag + GitHub Actions，细节见 [docs/build.md](docs/build.md)。

## 致谢

- [Cealing-Host](https://github.com/SpaceTimee/Cealing-Host)：内置 SNI 改写与 Hosts 规则来源
- [gVisor](https://gvisor.dev)：Android 端用户态 TCP/IP 协议栈
- [gomobile](https://pkg.go.dev/golang.org/x/mobile/cmd/gomobile)：Go ↔ Android 绑定

## 许可

[MIT](LICENSE) © 2026 xihale
