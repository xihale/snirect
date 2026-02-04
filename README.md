# Snirect

**Snirect** is a transparent HTTP/HTTPS proxy designed to bypass SNI-based censorship (SNI RST). Go implementation of [Accesser (Python)](https://github.com/URenko/Accesser).

**Cross-Platform:** Linux · macOS · Windows

## 数据源

Domain rules and configuration data are sourced from [Cealing-Host](https://github.com/SpaceTimee/Cealing-Host).

---

## 快速开始

Just want to get started? Run these commands:

### Linux / macOS
```bash
# One-time setup
./snirect install

# Start proxy and enable system proxy
snirect -s
```

### Windows (PowerShell as Administrator)
```powershell
# One-time setup
.\snirect.exe install

# Start proxy and enable system proxy
snirect.exe -s
```

That's it! Your system is now using Snirect to bypass SNI-based blocking.

**To stop:** Press `Ctrl+C` to stop the proxy, and your system proxy will be automatically cleared.

---

## 命令参考

| Quick Command | What it does |
|:--|:--|
| `snirect -s` | **Start proxy + enable system proxy** (simplest way) |
| `snirect status` | Check if everything is working |
| `snirect install` | Install binary and service |
| `snirect uninstall` | Complete removal |

---

## 进阶使用

<details>
<summary>点击展开进阶主题</summary>

### 安装选项

#### Option 1: From Release (Recommended)

**Linux:**
```bash
chmod +x snirect-linux-amd64
./snirect-linux-amd64 install
```

**macOS:**
```bash
chmod +x snirect-darwin-arm64
./snirect-darwin-arm64 install
```

**Windows (PowerShell as Administrator):**
```powershell
.\snirect-windows-amd64.exe install
```

#### Option 2: From Source
```bash
git clone https://github.com/xihale/snirect.git
cd snirect
make install
```

**What `install` does:**
- **Linux:** Copies to `~/.local/bin`, creates systemd user service
- **macOS:** Copies to `/usr/local/bin`, creates launchd service  
- **Windows:** Copies to `%LOCALAPPDATA%\Programs\snirect`, creates scheduled task

**注意:** 首次运行 (`snirect -s`) 会自动安装 CA 证书，也可以手动运行 `snirect install-cert`。安装证书后，你 **必须重启** 浏览器（如 Chrome, Firefox）或相关应用，代理才能正常生效。

### Running Methods

#### Method 1: Service (Recommended for daily use)

**Linux (systemd):**
```bash
systemctl --user start snirect    # Start
systemctl --user stop snirect     # Stop
systemctl --user status snirect   # Check status
journalctl --user -u snirect -f   # View logs
```

**macOS (launchd):**
```bash
launchctl start com.snirect.proxy
launchctl stop com.snirect.proxy
tail -f ~/Library/Logs/snirect.log
```

**Windows (Task Scheduler):**
```powershell
schtasks /Run /TN Snirect
schtasks /End /TN Snirect
```

#### Method 2: Direct (For testing or temporary use)

```bash
snirect              # Run with defaults
snirect -s           # Run and auto-set system proxy
snirect --help       # See all options
```

### Proxy Configuration

#### System-wide (Persistent)
```bash
snirect set-proxy     # Enable
snirect unset-proxy   # Disable
```

#### Current Terminal Only (Temporary)
```bash
# Linux / macOS
eval $(snirect proxy-env)

# Windows CMD
FOR /F %i IN ('snirect.exe proxy-env') DO %i

# Windows PowerShell
& snirect.exe proxy-env | Invoke-Expression
```

### Certificate Management

```bash
snirect cert-status      # Check if CA is installed
snirect install-cert     # Install CA certificate
snirect uninstall-cert   # Remove CA certificate
snirect firefox-cert     # Install CA to Firefox (recommended for Firefox users)
```

**Firefox 用户注意**: Firefox 使用独立证书存储，运行 `snirect install-cert` 后仍可能显示证书警告。
请使用 `snirect firefox-cert` 安装证书到 Firefox。

### All Available Commands

| Command | Aliases | Description |
|:--|:--|:--|
| `install` | `i`, `setup` | Install binary and service |
| `uninstall` | `rm`, `remove` | Full system cleanup |
| `status` | — | Check proxy/CA/service status |
| `set-proxy` | `sp` | Enable system proxy |
| `unset-proxy` | `up` | Disable system proxy |
| `install-cert` | `ic`, `install-ca` | Install root CA |
| `uninstall-cert` | `uc`, `uninstall-ca` | Remove root CA |
| `cert-status` | `cs`, `ca-status` | Check CA installation |
| `proxy-env` | — | Print shell proxy settings |
| `reset-config` | — | Reset config to defaults |
| `completion` | — | Shell completion scripts |
| `env` | — | Check system environment |

### Configuration

Config file location:
- **Linux/macOS:** `~/.config/snirect/config.toml`
- **Windows:** `%APPDATA%\snirect\config.toml`

Key options:
- `check_hostname`: Certificate verification (default: `false` for compatibility)
- `ipv6`: Enable IPv6 support (default: `true`)
- `importca`: Auto-install CA cert - `"auto"`, `"always"`, or `"never"`
- `server.port`: Proxy port (default: `7654`)

### Rules

Snirect uses rules to determine which domains need SNI modification. Default rules are integrated from [Cealing-Host](https://github.com/SpaceTimee/Cealing-Host).

**Rule files:**
- `~/.config/snirect/rules.toml` — Domain rules
- `~/.config/snirect/config.toml` — DNS configuration


### 安全注意事项

Some rules (Google/YouTube) use third-party public proxy IPs that require `check_hostname = false`. This has MITM risks. For better security:

1. Use your own trusted proxy IPs
2. Monitor the GGC IP updates
3. Consider contributing verified IPs

</details>

---

## 故障排除

| Issue | Solution |
|:--|:--|
| "Certificate warnings in browser" | 运行 `snirect install-cert` 并重启浏览器 |
| "tls: unknown certificate" | CA 证书安装失败或缓存未刷新。请尝试重启应用，或检查系统证书管理器中是否存在相应证书。 |
| "Port already in use" | Change `server.port` in config.toml |
| "Proxy not working" | Run `snirect status` to check |
| "Can't access some sites" | Check `rules.toml` |

### 浏览器证书安装（重要）

**注意：不同浏览器使用不同的证书存储机制**

运行 `snirect install-cert` 后：
- **Chrome/Edge/Brave/Safari** 会自动信任证书（使用系统证书存储）
- **Firefox 系浏览器**（Firefox、Zen Browser、Waterfox、LibreWolf、Floorp）需要手动安装证书

#### Firefox 系浏览器证书安装

Firefox 系浏览器使用独立的 NSS 证书数据库，不读取系统信任库。即使系统证书已安装，浏览器仍会显示证书警告。

**方法 1：使用内置命令（推荐）**
```bash
# 自动安装到所有 Firefox 系浏览器
snirect firefox-cert

# 检查是否已安装
snirect firefox-cert --check

# 从浏览器移除证书
snirect firefox-cert --remove
```

**方法 2：GUI 手动导入**

1. 打开 Firefox 设置：`about:preferences#privacy`
2. 滚动到底部，点击 **"查看证书"**
3. 选择 **"证书颁发机构"** 标签
4. 点击 **"导入"**
5. 选择证书文件：
   - Linux/macOS: `~/.config/snirect/certs/root.crt`
   - Windows: `%APPDATA%\snirect\certs\root.crt`
6. 勾选 **"信任由此证书颁发机构来标识网站"**
7. 点击 "确定"
8. 重启浏览器

**故障排除**

如果遇到 `SEC_ERROR_REUSED_ISSUER_AND_SERIAL` 错误（证书序列号冲突）：

```bash
# 先删除旧证书
snirect firefox-cert --remove

# 重新安装
snirect firefox-cert
```

#### Chrome/Edge/Brave 证书安装

这些浏览器通常使用系统证书存储，运行 `snirect install-cert` 后会自动信任。

如需手动导入：
- **Chrome:** 访问 `chrome://settings/certificates` → "授权中心" → "导入"
- **Edge:** 访问 `edge://settings/privacy/manageCertificates`
- **Brave:** 访问 `brave://settings/certificates`

#### 验证证书安装

**测试证书是否生效：**
```bash
# 1. 启动代理
snirect -s

# 2. 在浏览器中访问 https://www.google.com
#    点击地址栏左侧的锁图标 → "连接是安全的" → "证书有效"
#    应该显示：颁发者 "Snirect Root CA"
```

**检查 Firefox 证书列表：**
1. 访问 `about:preferences#privacy`
2. 点击 "查看证书" → "证书颁发机构"
3. 搜索 "Snirect"，应该看到 "Snirect Root CA"

**检查 Chrome 证书列表：**
1. 访问 `chrome://settings/certificates`
2. 选择 "授权中心" 标签
3. 搜索 "Snirect"，应该看到 "Snirect Root CA"

---

## Credits

Inspired by [Accesser (Python)](https://github.com/URenko/Accesser) by URenko.
