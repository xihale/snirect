<div align="center">

<img src="assets/logo/snirect_banner_dark.svg#gh-dark-mode-only" alt="Snirect" width="480">
<img src="assets/logo/snirect_banner_light.svg#gh-light-mode-only" alt="Snirect" width="480">

**按规则改写 TLS SNI 的本地代理**

桌面端 CLI（Linux / macOS / Windows）+ Android 应用，Go 实现

[![CI](https://github.com/xihale/snirect/actions/workflows/ci.yml/badge.svg)](https://github.com/xihale/snirect/actions/workflows/ci.yml)
[![Release](https://github.com/xihale/snirect/actions/workflows/release.yml/badge.svg)](https://github.com/xihale/snirect/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

</div>

---

## 这是什么

Snirect 是一个本地 HTTP/HTTPS 代理。对 TLS 流量，它在握手阶段读出 ClientHello 里的 SNI，按内置规则改写后转发，用于应对基于 SNI 的过滤；可选用本机生成的根 CA 做 MITM 解密。

功能：

- **SNI 改写**：规则支持 `example.com` / `*.example.com` / `*example.com` 三种写法；改写目标为空即剥掉 SNI
- **证书策略**：`CertVerify` 按域名放行证书、`IgnoreExpiry` 忽略过期证书、`Hosts` 把域名指到固定 IP 绕过 DNS
- **MITM**：本机生成根 CA 并签发叶证书（带缓存），系统需信任该 CA 才能解密；Firefox 用独立的 NSS 库，单独安装
- **DNS**：UDP / TCP / DoT / DoH 上游，域名上游可用 IP bootstrap 解析，ECS 支持 auto 模式；IP 优选提供 fastest 模式（v4/v6 都测延迟取最快，结果缓存）
- **系统代理**：设置 PAC；`proxy env` 打印终端用的 `http_proxy` 命令，配合 `eval` 使用
- **tunnel hook**：站点级修正逻辑挂在转发路径外，目前只有 GitHub codeload 重定向修正一处

## 快速开始

预编译包在 [Releases](https://github.com/xihale/snirect/releases)，或自己编译：

```bash
make build
```

运行：

```bash
./dist/snirect -s           # 前台运行，-s 同时设系统代理
./dist/snirect cert install # 要解密 HTTPS 就先装根 CA
./dist/snirect cert firefox install # Firefox 另走 NSS 库，需要再执行这条
```

装成后台服务（systemd user / launchd / Windows Service）：

```bash
./dist/snirect install
./dist/snirect status       # 服务 / 系统代理 / CA / 端口占用情况
```

完整命令表见 [docs/cli.md](docs/cli.md)。

## Android

仓库同时包含 Android 应用（`android/`）。Go 核心经 gomobile 编译成 `core.aar`，Kotlin 层用 `VpnService`（TUN）+ [gVisor netstack](https://gvisor.dev) 用户态协议栈接住 TUN 流量，为每条 TCP 流合成 CONNECT 请求发给环回上的 Go 代理；出站 socket 经 `VpnService.protect()` 绕开 TUN 防止环路。支持按应用白/黑名单过滤流量。

数据链路细节见 [android/bindings/README.md](android/bindings/README.md)，构建见 [docs/build.md](docs/build.md)。

## 平台支持

| 平台 | 架构 | 系统代理 | CA 安装 |
| :--- | :--- | :--- | :--- |
| Linux | amd64 / arm64 | PAC (gsettings / kwriteconfig5) | trust (p11-kit)，回退发行版目录；Firefox 走 NSS |
| macOS | amd64 / arm64 | 系统网络设置 | Keychain (`security add-trusted-cert`) |
| Windows | amd64 / arm64 | 注册表 PAC | 系统证书库 |
| Android | arm64 / x86_64 | VpnService (TUN)，应用白/黑名单 | 用户证书导入引导 |

## 配置

配置文件位于 `~/.config/snirect/config.toml`（Windows 为 `%APPDATA%\snirect\config.toml`），所有项都有内置默认值，注释掉即回落默认。详见 [docs/config.md](docs/config.md) 与 `internal/config/defaults.go` 中的完整示例。

分流规则静态编进二进制（`internal/rules/builtin_rules.go`），其中 SNI 改写与 Hosts 表同步自 [Cealing-Host](https://github.com/SpaceTimee/Cealing-Host) 项目并做了适配，运行时不联网拉取。

## 目录结构

```text
├── cmd/snirect/        CLI 入口
├── internal/
│   ├── cert/           根 CA 管理、叶证书签发与缓存
│   ├── cli/            cobra 命令：install/cert/proxy/config/update…
│   ├── color/          终端着色
│   ├── config/         TOML 加载、默认值、样例配置
│   ├── dns/            DoH/DoT/plain 解析器、bootstrap、IP 优选
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
make crossAll # 交叉编译 linux/darwin/windows × amd64/arm64 共 6 个平台
make debug    # Android Debug 构建
```

发布流程是 git tag + GitHub Actions（CLI 走 `v*`，Android 走 `android-v*`，两条线独立发版），见 [docs/build.md](docs/build.md)。

## 致谢

- [Cealing-Host](https://github.com/SpaceTimee/Cealing-Host)：内置 SNI 改写与 Hosts 规则来源
- [gVisor](https://gvisor.dev)：Android 端用户态 TCP/IP 协议栈
- [gomobile](https://pkg.go.dev/golang.org/x/mobile/cmd/gomobile)：Go ↔ Android 绑定

## 许可

[MIT](LICENSE) © 2026 xihale
