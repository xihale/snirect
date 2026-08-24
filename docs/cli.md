# 命令

```bash
./dist/snirect -s          # 前台跑代理。-s 同时设系统代理
./dist/snirect install     # 注册后台服务
./dist/snirect status
./dist/snirect uninstall
./dist/snirect update --check
```

| 命令 | 作用 |
| :--- | :--- |
| `snirect [-s]` | 前台跑代理。`-s` 同时设系统代理 |
| `snirect status` | 服务、系统代理、CA、当前配置 |
| `snirect install` | 装二进制并注册后台服务（Linux systemd user / macOS launchd / Windows Service）；二进制由包管理器（AUR/pacman）安装时只注册服务，不动二进制 |
| `snirect uninstall` | 卸服务、配置和系统代理；包管理器安装时只 disable 服务（包本身用 pacman 卸） |
| `snirect cert install` | 把根 CA 装进系统信任库 |
| `snirect cert remove` | 卸系统 CA |
| `snirect cert firefox install` | 写进本机所有 Firefox `cert9.db` |
| `snirect proxy set` / `unset` | 手动开/关系统 PAC 代理 |
| `snirect proxy env` | 打出终端代理变量，`eval $(snirect proxy env)` |
| `snirect config reset` | 重置 `config.toml`（证书保留） |
| `snirect rules dae` | 输出 dae 格式域名规则块（粘进 routing 段），`--policy` 指向策略组（默认 `snirect`） |
| `snirect rules clash` | 输出 clash/mihomo 规则行（粘进 rules 列表），`--policy` 同上 |
| `snirect version` | 打印构建版本 |
| `snirect update --check` | 查 GitHub Releases 是否有新版本（不下载） |
| `snirect update` | 下载、校验 SHA256、覆盖已安装二进制；已注册服务则重启；包管理器安装时拒绝自更新，提示走 pacman |

证书固定的客户端（网银等）不信任用户 CA，用 PAC / 规则排除。

上游用 dae/clash 分流时，把 snirect 认识的域名导出去：

```bash
snirect rules dae  >> my-dae.conf   # domain(...) -> snirect 块
snirect rules clash > snirect.yaml  # - DOMAIN-SUFFIX,... 行
```
