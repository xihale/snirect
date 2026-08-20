# 命令

```bash
./dist/snirect -s          # 前台跑代理。-s 同时设系统代理
./dist/snirect install     # 注册后台服务
./dist/snirect status
./dist/snirect uninstall
```

| 命令 | 作用 |
| :--- | :--- |
| `snirect [-s]` | 前台跑代理。`-s` 同时设系统代理 |
| `snirect status` | 服务、系统代理、CA、当前配置 |
| `snirect install` | 装二进制并注册后台服务（Linux systemd user / macOS launchd / Windows Service） |
| `snirect uninstall` | 卸服务、配置和系统代理 |
| `snirect cert install` | 把根 CA 装进系统信任库 |
| `snirect cert remove` | 卸系统 CA |
| `snirect cert firefox install` | 写进本机所有 Firefox `cert9.db` |
| `snirect proxy set` / `unset` | 手动开/关系统 PAC 代理 |
| `snirect proxy env` | 打出终端代理变量，`eval $(snirect proxy env)` |
| `snirect config reset` | 重置 `config.toml`（证书保留） |
| `snirect version` | 打印构建版本 |

证书固定的客户端（网银等）不信任用户 CA，用 PAC / 规则排除。
