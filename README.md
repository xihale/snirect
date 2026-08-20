# Snirect

跨平台 HTTP/HTTPS 代理：按规则改 TLS SNI，必要时用本机根 CA 做 MITM。提供桌面端 CLI 与 Android 应用。

预编译包：[Releases](https://github.com/xihale/snirect/releases)。

```bash
make build
./dist/snirect -s          # 前台运行，顺带设系统代理
./dist/snirect cert install
```

HTTPS 站点必须先装根 CA（Firefox 再加一条 `cert firefox install`）。私钥只在本机。

- 命令与后台服务 → [docs/cli.md](docs/cli.md)
- 配置、DNS、规则 → [docs/config.md](docs/config.md)
- 编译与发版 → [docs/build.md](docs/build.md)
- 规范守则 → [AGENTS.md](AGENTS.md)
