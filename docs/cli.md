# 命令

```bash
snirect run -s              # 前台跑代理，-s 同时设系统 PAC 代理（裸跑 snirect 等价）
snirect service install -s  # 注册并启动用户后台服务（自动设系统 PAC）
snirect status              # 查看进程端口、服务、CA、系统代理状态
snirect update --check      # 检查新版本
```

| 命令 | 作用 |
| :--- | :--- |
| `snirect [run] [-s] [-p PORT] [-c FILE]` | 前台运行代理。`-s` 设系统代理，`-p` 临时指定端口，`-c` 指定配置文件，`-v` 开启 debug 日志 |
| `snirect status` | 查看代理端口、后台服务、CA 信任、系统代理状态 |
| `snirect service install [-s]` | 注册并启动当前用户的后台自启服务（Linux `systemd --user` / macOS `LaunchAgent` / Windows `HKCU\...\Run` 无窗口后台进程） |
| `snirect service uninstall` | 停止并移除后台服务（不删二进制与配置文件） |
| `snirect service start \| stop \| restart \| status` | 启停、重启后台服务或查看服务状态 |
| `snirect cert install [--firefox]` | 安装根 CA 到系统信任库（本机有 `certutil` 时自动写入 Firefox NSS，或显式传 `--firefox`） |
| `snirect cert uninstall [--firefox]` | 从系统信任库（及 Firefox）移除根 CA |
| `snirect cert status` | 查看系统与 Firefox 证书信任状态 |
| `snirect proxy on \| off \| status` | 手动开/关系统 PAC 代理，或查看代理状态 |
| `snirect proxy env` | 输出终端代理环境变量，`eval $(snirect proxy env)` |
| `snirect config show \| path \| reset` | 查看当前生效配置、打印配置文件路径、重置默认配置 |
| `snirect rules list` | 打印内置规则表 |
| `snirect rules match <host>` | 测试单个域名命中的 IP、SNI 改写与证书策略 |
| `snirect rules export dae \| clash` | 导出内置域名规则给 dae 或 clash/mihomo，`--policy` 指定策略组（默认 `snirect`） |
| `snirect update [--check]` | 原地更新当前二进制（SHA256 校验，服务运行中则自动重启；包管理器安装时提示走 pacman） |
| `snirect version` | 打印版本（或 `snirect --version`） |

`snirect update` 下载走与代理相同的 Hosts-IP + 空 SNI 路径，整包落在一条 TCP 流上。每次尝试带守护：20 s 无字节判死、8 s 后均速低于 128 KiB/s 直接换新连接重试（最多 3 次），并输出实时进度。

证书固定的客户端（网银等）不信任用户 CA，用 PAC / 规则排除。Steam、uv、conda、Node、JVM 等自带证书库的工具如何配置，见 [docs/cert.md](cert.md)。

上游用 dae/clash 分流时，把 snirect 认识的域名导出去：

```bash
snirect rules export dae   >> my-dae.conf   # domain(...) -> snirect 块
snirect rules export clash > snirect.yaml   # - DOMAIN-SUFFIX,... 行
```
