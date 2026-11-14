<div align="center">

<img src="assets/logo/snirect_banner_dark.svg#gh-dark-mode-only" alt="Snirect" width="480">
<img src="assets/logo/snirect_banner_light.svg#gh-light-mode-only" alt="Snirect" width="480">

**按规则改写 TLS SNI 的本地代理**

[![CI](https://github.com/xihale/snirect/actions/workflows/ci.yml/badge.svg)](https://github.com/xihale/snirect/actions/workflows/ci.yml)
[![Release CLI](https://github.com/xihale/snirect/actions/workflows/release-cli.yml/badge.svg)](https://github.com/xihale/snirect/actions/workflows/release-cli.yml)
[![Release Android](https://github.com/xihale/snirect/actions/workflows/release-android.yml/badge.svg)](https://github.com/xihale/snirect/actions/workflows/release-android.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

</div>

---

Snirect 是一个本地 HTTP/HTTPS 代理，在 TLS 握手阶段按内置规则改写或剥离 ClientHello 中的 SNI，并对命中规则的站点使用本机根 CA 做 MITM 中转；未命中规则的站点保持 TLS 原样透传。内置规则同步自 [Cealing-Host](https://github.com/SpaceTimee/Cealing-Host) 并做了适配，编译期静态写入，运行时不联网拉取。

支持 Linux、macOS、Windows 桌面端 CLI 与 Android 客户端（基于 `VpnService` + gVisor netstack）。

## 安装与快速开始

从 [Releases](https://github.com/xihale/snirect/releases) 下载桌面端二进制或 Android APK；Arch Linux 可通过 AUR 安装 `snirect` / `snirect-bin`；也可本地执行 `make build` 编译。

```bash
# 1. 生成并安装根证书（系统存在 certutil 时自动写入 NSS 信任库）
snirect cert install

# 2. 前台运行并设置系统 PAC 代理
snirect run -s
```

注册为当前用户的后台自启服务（`systemd --user` / `LaunchAgent` / Windows `HKCU\...\Run`）：

```bash
snirect service install -s
snirect status
```

## 文档

### 使用文档

- [命令行用法](docs/cli.md)
- [配置文件与 PAC](docs/config.md)
- [证书信任（Steam / Firefox / 开发工具 / Android）](docs/cert.md)
- [内置规则与分流导出（dae / clash）](docs/rules.md)

### 开发文档

- [编译与发版](docs/build.md)
- [Android 绑定层架构（gomobile）](android/bindings/README.md)

## 致谢

- [Cealing-Host](https://github.com/SpaceTimee/Cealing-Host)：上游 SNI 改写与 Hosts 规则
- [gVisor](https://gvisor.dev)：Android 端用户态协议栈

## 许可

[MIT](LICENSE) © 2026 xihale
