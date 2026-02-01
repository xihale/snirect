# Snirect

**Snirect** is a transparent proxy designed to bypass SNI-based censorship (SNI RST). It is the Go implementation of [Accesser (Python)](https://github.com/URenko/Accesser).

**Cross-Platform Support:** Linux, macOS, and Windows

## Quick Start

### 1. Install

#### Linux / macOS
**From Release:**
```bash
# Linux
chmod +x snirect-linux-amd64
./snirect-linux-amd64 install

# macOS (might require sudo for /usr/local/bin)
chmod +x snirect-darwin-arm64
./snirect-darwin-arm64 install
```

**From Source:**
```bash
git clone https://github.com/xihale/snirect.git
cd snirect
make install
```

#### Windows
**From Release:**
```powershell
# Run in PowerShell as Administrator
.\snirect-windows-amd64.exe install
```

**What `install` does:**
- **Linux:** Copies binary to `~/.local/bin`, sets up systemd service, installs Root CA
- **macOS:** Installs to `/usr/local/bin`, sets up launchd service, installs Root CA to Keychain
- **Windows:** Installs to `%LOCALAPPDATA%\Programs\snirect`, creates scheduled task, installs Root CA

### 2. Configure Proxy

#### System-wide (Recommended)
```bash
# Linux / macOS
snirect set-proxy

# Windows (PowerShell)
snirect.exe set-proxy
```

#### Current Terminal Only
```bash
# Linux / macOS
eval $(snirect proxy-env)

# Windows (Command Prompt)
FOR /F %i IN ('snirect.exe proxy-env') DO %i

# Windows (PowerShell)
& snirect.exe proxy-env | Invoke-Expression
```

## Usage

### The Recommended Way (Service)

#### Linux (systemd)
```bash
systemctl --user start|stop|status|restart snirect
journalctl --user -u snirect -f  # View logs
```

#### macOS (launchd)
```bash
launchctl start|stop com.snirect.proxy
launchctl list | grep snirect  # Check status
tail -f ~/Library/Logs/snirect.log  # View logs
```

#### Windows (Task Scheduler)
```powershell
# Start manually
snirect.exe

# Check scheduled task
schtasks /Query /TN Snirect
```

### The Direct Way
Run directly in your terminal:
```bash
snirect           # Run with config defaults
snirect --set-proxy  # Run and temporarily auto-set system proxy
```

## Commands

| Command | Description |
| :--- | :--- |
| `snirect install` | Install binary, service, and CA certificate |
| `snirect uninstall` | Full cleanup (binary, service, config) |
| `snirect set-proxy` | Enable system-wide PAC proxy |
| `snirect unset-proxy` | Disable system-wide PAC proxy |
| `snirect install-cert` | Manual Root CA installation |
| `snirect proxy-env` | Print proxy environment variables |
| `snirect completion [bash\|zsh\|fish\|powershell] -i` | Install shell completions |

## Credits
Inspired by [Accesser (Python)](https://github.com/URenko/Accesser) by URenko.
