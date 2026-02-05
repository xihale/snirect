# Snirect

**Snirect** 是一个跨平台的透明 HTTP/HTTPS 代理工具，旨在通过修改 SNI（服务器名称指示）来绕过基于 SNI 的审查和封锁（如 SNI RST）。它是 [Accesser (Python)](https://github.com/URenko/Accesser) 的 Go 语言实现。

**支持平台：** Linux · macOS · Windows

## 特性

- **跨平台支持**：原生支持 Windows、macOS 和 Linux。
- **透明代理**：通过 PAC 自动分流，无需手动配置每个应用。
- **SNI 修改**：有效绕过 SNI 封锁，直连目标服务器。
- **一键集成**：提供简单的安装命令，支持注册为系统后台服务。
- **浏览器友好**：内置 Firefox 专用证书安装工具，自动处理系统证书信任。
- **轻量级依赖**：默认不包含 QUIC 模块，显著减小二进制体积。

---

## 编译与开发

本项目支持通过 Go Build Tags 来控制功能模块：

| 版本 | 编译命令 | 说明 |
| :--- | :--- | :--- |
| **标准版 (默认)** | `go build ./cmd/snirect` | 极致轻量 (~8MB)。包含核心代理、系统服务管理、CA 证书（含 Firefox）自动安装。 |
| **完整版** | `go build -tags quic ./cmd/snirect` | 包含所有功能，增加 **QUIC (DoQ/H3)** 支持。体积约 11MB。 |

> **提示**：可以使用 `snirect version` 或 `snirect status` 查看当前二进制文件开启的功能模块。

---

## 快速开始

下载对应平台的二进制文件后，即可直接运行，无需安装：

### Linux / macOS

```bash
chmod +x snirect-linux-amd64
# 启动代理并自动开启系统代理
./snirect-linux-amd64 -s
```

### Windows

```powershell
# 启动代理并自动开启系统代理
.\snirect-windows-amd64.exe -s
```

**注意：** 首次运行会提示安装 CA 证书，请务必确认安装。安装后需要**重启浏览器**才能正常访问 HTTPS 网站。

---

## 系统安装 (可选)

如果你希望 Snirect 能够随系统启动或作为后台服务运行，可以使用以下方式：

### Linux / macOS

```bash
# 安装到系统路径 (~/.local/bin 或 /usr/local/bin) 并注册后台服务
./snirect install
```

### Windows (方式一：注册为计划任务)

```powershell
# 以管理员身份运行：安装并注册为后台计划任务
.\snirect.exe install
```

### Windows (方式二：移动到启动目录)

1. 按下 `Win + R` 键，输入 `shell:startup` 并回车，打开“启动”文件夹。
2. 将 `snirect.exe` 文件的快捷方式（或程序本身）移动到该文件夹中。

**说明**：
- **静默运行**：手动双击打开时会显示窗口；若在**系统启动 3 分钟内**运行（即开机自启场景）或通过计划任务启动，程序将自动隐藏窗口进入后台运行。
- **系统代理**：程序启动后会自动配置系统代理（由 `config.toml` 中的 `set_proxy` 控制，默认为 `true`）。

---

## 常用命令


| 命令                   | 简写 | 说明                               |
| :--------------------- | :--- | :--------------------------------- |
| `snirect -s`           | -    | **启动代理并自动启用系统代理**     |
| `snirect status`       | -    | 查看当前代理、证书和服务的运行状态 |
| `snirect install`      | `i`  | 安装二进制文件并注册后台服务/任务  |
| `snirect uninstall`    | `rm` | 完整卸载（包含二进制、服务 and 配置） |
| `snirect install-cert` | `ic` | 仅安装根 CA 证书到系统信任库       |
| `snirect firefox-cert` | -    | 将 CA 证书安装到 Firefox 系浏览器  |

---

## 进阶指南

<details>
<summary>点击展开查看更多配置与使用细节</summary>

### 代理设置

Snirect 主要通过 PAC (Proxy Auto-Config) 进行系统级代理设置：

- **全局启用**: `snirect set-proxy`
- **全局禁用**: `snirect unset-proxy`
- **仅当前终端生效**:
  - Linux/macOS: `eval $(snirect proxy-env)`
  - Windows PowerShell: `& snirect.exe proxy-env | Invoke-Expression`

### 证书管理 (HTTPS 必选)

由于 HTTPS 代理需要解密 SNI，浏览器必须信任 Snirect 的根证书：

- **Chrome/Edge/Brave/Safari**: 使用系统信任库，运行 `snirect install-cert` 即可.
- **Firefox 系列**: Firefox 使用独立的证书库，请运行 `snirect firefox-cert` 进行自动安装。

### 配置与规则

- **配置文件路径**:
  - Linux/macOS: `~/.config/snirect/config.toml`
  - Windows: `%APPDATA%\snirect\config.toml`
- **分流规则**: `rules.toml` 决定了哪些域名需要通过 Snirect 修改 SNI。默认规则同步自 [Cealing-Host](https://github.com/SpaceTimee/Cealing-Host)。

</details>

---

## 故障排除

| 问题                 | 解决方法                                                               |
| :------------------- | :--------------------------------------------------------------------- |
| 浏览器提示“证书无效” | 运行 `snirect install-cert` (及 `firefox-cert`)，并参考下方证书说明    |
| 提示端口被占用       | 在 `config.toml` 中修改 `server.port` (默认为 7654)                    |
| 某些网站仍无法访问   | 运行 `snirect status` 检查状态，或尝试 `snirect reset-config` 重置规则 |

### 证书相关问题

如果浏览器访问 HTTPS 网站时提示“连接不安全”、“您的连接不是私密连接”或证书无效：

1.  **彻底重启浏览器**：有些浏览器（如 Chrome）在关闭窗口后仍有后台进程运行。请确保**完全杀死**所有浏览器进程后再重启。
2.  **刷新证书缓存**：在系统证书管理器中检查是否已存在 "Snirect Root CA"，如果存在但仍报错，尝试卸载并重新运行 `snirect install-cert`。
3.  **手动导入证书**：如果自动安装失败，你可以手动获取证书并导入浏览器：
    *   在代理运行状态下，通过浏览器或 `curl` 访问 `http://localhost:7654/CERT/root.crt` 下载证书。
    *   **Chrome/Edge**: 进入设置 -> 隐私和安全性 -> 安全 -> 管理证书 -> 授权中心 -> 导入。
    *   **Firefox**: 进入设置 -> 隐私与安全 -> 查看证书 -> 证书颁发机构 -> 导入（务必勾选“信任由此证书颁发机构来标识网站”）。

---

## 开发与致谢

- **灵感来源**: [Accesser (Python)](https://github.com/URenko/Accesser) by URenko.
- **数据来源**: [Cealing-Host](https://github.com/SpaceTimee/Cealing-Host).
- **License**: MIT
