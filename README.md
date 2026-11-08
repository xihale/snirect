<div align="center">

<img src="assets/logo/snirect_banner_dark.svg#gh-dark-mode-only" alt="Snirect" width="480">
<img src="assets/logo/snirect_banner_light.svg#gh-light-mode-only" alt="Snirect" width="480">

**按规则改写 TLS SNI 的本地代理**

[![CI](https://github.com/xihale/snirect/actions/workflows/ci.yml/badge.svg)](https://github.com/xihale/snirect/actions/workflows/ci.yml)
[![Release](https://github.com/xihale/snirect/actions/workflows/release.yml/badge.svg)](https://github.com/xihale/snirect/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

</div>

---

Snirect 是一个本地 HTTP/HTTPS 代理，用于在 TLS 握手阶段按内置规则改写或剥离 ClientHello 中的 SNI，并对需要的站点使用本机生成的根 CA 做 MITM 中转。内置规则同步自 [Cealing-Host](https://github.com/SpaceTimee/Cealing-Host) 并做了适配，编译期静态写入，运行时不联网拉取。

支持 Linux、macOS、Windows 桌面端 CLI 与 Android 客户端（基于 `VpnService` + gVisor netstack）。

## 快速开始

从 [Releases](https://github.com/xihale/snirect/releases) 下载对应平台二进制，或本地执行 `make build` 编译。

```bash
# 安装根证书（首次使用必须，需要管理员/sudo 权限）
snirect cert install
snirect cert firefox install  # Firefox 用户需额外执行

# 前台运行并设置系统 PAC 代理
snirect -s
```

注册为开机自启后台服务（systemd user / launchd / Windows Service）：

```bash
snirect install
snirect status
```

## 文档

- [命令行用法与上游分流导出（dae / clash）](docs/cli.md)
- [配置文件与规则格式](docs/config.md)
- [证书信任（Steam / Firefox / Python / Node / JVM / Android）](docs/cert.md)
- [编译、交叉构建与发版](docs/build.md)
- [Android 端架构与 gomobile 绑定层](android/bindings/README.md)

## 致谢

- [Cealing-Host](https://github.com/SpaceTimee/Cealing-Host)：上游 SNI 改写与 Hosts 规则
- [gVisor](https://gvisor.dev)：Android 端用户态协议栈

## 许可

[MIT](LICENSE) © 2026 xihale
