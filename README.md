# Snirect

**Snirect** is a transparent HTTP/HTTPS proxy designed to bypass SNI-based censorship (SNI RST). Go implementation of [Accesser (Python)](https://github.com/URenko/Accesser).

**Cross-Platform:** Linux · macOS · Windows

---

## 🚀 Quick Start (Simple)

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

## 📋 Command Reference

| Quick Command | What it does |
|:--|:--|
| `snirect -s` | **Start proxy + enable system proxy** (simplest way) |
| `snirect status` | Check if everything is working |
| `snirect install` | Install binary and service |
| `snirect uninstall` | Complete removal |

---

## 🔧 Advanced Usage

<details>
<summary>Click to expand advanced topics</summary>

### Installation Options

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

**Note:** CA certificate is auto-installed on first run (`snirect -s`) or you can manually run `snirect install-cert`.

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
```

### All Available Commands

| Command | Aliases | Description |
|:--|:--|:--|
| `install` | `i`, `setup` | Install binary and service |
| `uninstall` | `rm`, `remove` | Full system cleanup |
| `status` | — | Check proxy/CA/service status |
| `set-proxy` | `sp` | Enable system proxy |
| `unset-proxy` | `up` | Disable system proxy |
| `install-cert` | `ic` | Install root CA |
| `uninstall-cert` | `uc` | Remove root CA |
| `cert-status` | `cs` | Check CA installation |
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

To update rules:
```bash
make update-rules
```

### ⚠️ Security Note

Some rules (Google/YouTube) use third-party public proxy IPs that require `check_hostname = false`. This has MITM risks. For better security:

1. Use your own trusted proxy IPs
2. Monitor the [TODO list](https://github.com/xihale/snirect/issues) for GGC IP updates
3. Consider contributing verified IPs

</details>

---

## 🛠️ Troubleshooting

| Issue | Solution |
|:--|:--|
| "Certificate warnings in browser" | Run `snirect install-cert` |
| "Port already in use" | Change `server.port` in config.toml |
| "Proxy not working" | Run `snirect status` to check |
| "Can't access some sites" | Check `rules.toml` or run `make update-rules` |

---

## Credits

Inspired by [Accesser (Python)](https://github.com/URenko/Accesser) by URenko.
