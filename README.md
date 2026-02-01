# Snirect

**Snirect** is a transparent proxy designed to bypass SNI-based censorship (SNI RST). It is the Go implementation of [Accesser (Python)](https://github.com/URenko/Accesser).

## Quick Start

### 1. Install
**From Release:**
Download the binary, then run:
```bash
chmod +x snirect
./snirect install
```

**From Source:**
```bash
git clone https://github.com/xihale/snirect.git
cd snirect
make install
```
*`install` will copy the binary, setup a Systemd service, and install the Root CA.*

### 2. Configure Proxy
*   **System-wide:** `snirect set-proxy` (Uses PAC)
*   **Current Terminal:** `eval $(snirect proxy-env)`

## Usage

### The Recommended Way (Service)
Manage the background service via Systemd:
```bash
systemctl --user start|stop|status|restart snirect
journalctl --user -u snirect -f  # View logs
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
| `snirect install` | Install binary, service, and CA |
| `snirect uninstall` | Full cleanup (binary, service, config) |
| `snirect set-proxy` | Enable system-wide PAC proxy |
| `snirect unset-proxy` | Disable system-wide PAC proxy |
| `snirect install-cert` | Manual Root CA installation |
| `snirect completion -i` | Setup shell completions (bash/zsh/fish) |

## Credits
Inspired by [Accesser (Python)](https://github.com/URenko/Accesser) by URenko.
