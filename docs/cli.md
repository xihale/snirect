# 命令行用法

```bash
snirect run -s              # 前台运行并设置系统 PAC 代理（与 snirect -s 等价）
snirect service install -s  # 注册并启动用户后台服务（自动设系统 PAC）
snirect status              # 查看端口、服务、CA、系统代理状态
snirect update --check      # 检查新版本
```

## 命令一览

| 命令 | 作用 |
| :--- | :--- |
| `snirect [run] [-s] [-p PORT] [-c FILE] [-l LEVEL] [-v]` | 前台运行代理（别名 `serve`；不带子命令直接运行 `snirect` 等价）。`-s` 设系统 PAC 代理，`-p` 临时指定端口，`-c` 指定配置文件，`-l` 设日志级别（`debug`/`info`/`warn`/`error`），`-v` 等价 `-l debug` |
| `snirect status` | 查看配置、端口占用、CA 信任、系统代理与后台服务状态 |
| `snirect service install [-s]` | 注册并启动当前用户的后台自启服务（Linux `systemd --user` / macOS `LaunchAgent` / Windows `HKCU\...\Run`） |
| `snirect service uninstall` | 停止并移除后台服务（保留二进制与配置文件） |
| `snirect service start \| stop \| restart \| status` | 启停、重启后台服务或查看服务状态 |
| `snirect cert install [--firefox]` | 生成并安装根 CA 到系统信任库（系统有 `certutil` 时顺带写入 Chrome/Firefox NSS，或显式传 `--firefox`） |
| `snirect cert uninstall [--firefox]` | 从系统信任库（及 Chrome/Firefox NSS）移除根 CA |
| `snirect cert status` | 查看系统与 Firefox 证书信任状态 |
| `snirect proxy on \| off \| status` | 手动开/关系统 PAC 代理，或查看代理状态 |
| `snirect proxy env` | 输出当前终端代理环境变量（Linux/macOS：`eval $(snirect proxy env)`；Windows CMD：`FOR /F %i IN ('snirect proxy env') DO %i`） |
| `snirect config show \| path \| reset` | 查看生效配置、打印 `config.toml` 路径、重置默认配置与 `pac`（保留证书） |
| `snirect rules list` | 打印内置规则表 |
| `snirect rules match <host>` | 测试单个域名命中的 IP、SNI 改写与证书策略 |
| `snirect rules export dae \| clash [--policy NAME]` | 导出内置域名列表给 `dae` 或 `clash`，默认策略名 `snirect` |
| `snirect update [--check]` | 检查或原地更新当前二进制 |
| `snirect completion <bash\|zsh\|fish\|powershell> [-i]` | 输出或安装 Shell 自动补全脚本 |
| `snirect version` | 打印版本号（或 `snirect --version`） |

## 自更新说明

`snirect update` 使用与代理相同的 `Hosts` IP + 空 SNI 路径直连 GitHub Releases，校验 SHA-256 后原地替换当前可执行文件；若检测到后台服务正在运行会自动重启。如当前二进制由 Arch Linux `pacman`（AUR）安装，命令会拒绝覆盖并提示改用 `paru` / `yay` 升级。

关于证书信任与客户端排除，见 [证书信任](cert.md)；关于规则匹配与 `dae` / `clash` 分流导出，见 [内置规则与分流导出](rules.md)。
