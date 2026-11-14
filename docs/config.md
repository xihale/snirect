# 配置文件与 PAC

配置文件 `config.toml` 与 PAC 文件默认位于应用配置目录，可通过 `snirect config path` 查看实际路径：

- **Linux**：`~/.config/snirect/`（受 `$XDG_CONFIG_HOME` 影响）
- **macOS**：`~/Library/Application Support/snirect/`
- **Windows**：`%APPDATA%\snirect\`

修改后需重启进程或服务。未填写或被注释的字段使用编译期默认值，可通过 `snirect config show` 查看当前生效配置，或 `snirect config reset` 恢复默认。

## `config.toml`

```toml
check_hostname = "loose"       # "off" / "loose" / "strict"
# check_hostname_allow = []    # 非空时取代上面的模式：证书 DNS 名命中任一模式即放行
set_proxy = false              # 启动时自动设置系统 PAC 代理
ca_install = "auto"            # "auto" / "always" / "never"
ipv6 = false                   # 默认关闭，避免不可达的 IPv6 拖慢连接
ecs = ""                       # EDNS Client Subnet："" / "auto" / CIDR（如 "218.102.0.0/24"）

[server]
address = "127.0.0.1"          # 绑定地址，"0.0.0.0" 可供局域网使用
port = 7654                    # 监听端口（0 为自动分配）
pac_host = "127.0.0.1"         # 生成 PAC 脚本时填入的代理主机名（局域网共享时改为本机 IP）
buffer_size = 65536            # 隧道拷贝缓冲区字节数（范围 4096 ~ 1048576）

[DNS]
# 支持 DoH (https://)、DoT (tls://)、UDP/TCP (ip:port)
nameserver = [
    "https://dnschina1.soraharu.com/dns-query",
    "tls://223.5.5.5"
]
# 用于解析加密 DNS 上游自身域名的引导 DNS，必须为 IP 字面量
bootstrap_dns = ["tls://223.5.5.5"]

[preference]
mode = "standard"              # IPv6 开启时的选址策略："standard" / "ipv6" / "ipv4"

[timeout]
dial = 30                      # 连接远程服务器超时（秒）
dns = 5                        # DNS 查询超时（秒）

[limit]
max_connections = 0            # 最大并发连接数（0 为不限制）
dns_cache_size = 10000         # DNS 缓存最大条目数

[log]
loglevel = "INFO"              # "DEBUG" / "INFO" / "WARN" / "ERROR"
logfile = ""                   # 留空输出到控制台；相对路径相对于配置目录解析
```

### 字段说明

- **证书校验（`check_hostname` / `check_hostname_allow`）**：仅作用于被规则或站点钩子拦截（MITM）的域名；无规则站点直接原样透传 TLS，不受此配置影响。`check_hostname_allow` 非空时整体替代 `check_hostname`（域名模式语法见 [内置规则与分流导出](rules.md)）。
- **引导 DNS（`bootstrap_dns`）**：当 `DNS.nameserver` 包含域名形式的 DoH/DoT 地址时，使用 `bootstrap_dns` 解析上游域名，因此 `bootstrap_dns` 必须填写 IP 字面量（如 `tls://223.5.5.5`、`1.1.1.1:53`）。

## PAC 脚本（`pac`）

配置目录下的 `pac` 文件在开启系统代理时通过 `http://<host>:<port>/pac/` 下发给系统。文件内可使用以下占位符：

- `{{host}}`：替换为 `server.pac_host`
- `{{port}}`：替换为实际监听端口

将不需要走代理或使用证书固定（Certificate Pinning，如网银、部分桌面客户端）的域名从 `pac` 中排除，即可让对应流量直连而不经过本地代理。
