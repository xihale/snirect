# 配置与规则

路径：

- Linux / macOS：`~/.config/snirect/config.toml`
- Windows：`%APPDATA%\snirect\config.toml`

改完要重启进程。注释掉的项用编译期默认值。完整字段说明在仓库里的 `internal/config/defaults.go` 中的 `SampleConfigTOML`。

```toml
check_hostname = true          # true / false / "strict" / 域名列表
set_proxy = true
ca_install = "auto"            # auto / always / never
ipv6 = false                   # 默认关，避免不可达的 v6

[server]
address = "127.0.0.1"
port = 7654

[DNS]
# DoH / DoT / IP:53。域名形式的上游必须另有 IP Bootstrap
nameserver = [
    "https://dnschina1.soraharu.com/dns-query",
    "tls://223.5.5.5"
]
# bootstrap_dns = ["tls://223.5.5.5"]

[log]
loglevel = "info"
```

`nameserver` 可以是域名（例如 DoH URL）。解析这些上游主机名用的是 `bootstrap_dns`，必须是 IP 字面量（`tls://223.5.5.5`、`1.1.1.1:53` 这类），不能再套一层域名。

## 规则

规则编在 `internal/rules/builtin_rules.go`，启动时 `LoadRules()` 自动给 key 建索引，不要手维护并行切片。

| 表 | 作用 |
| :--- | :--- |
| `AlterHostname` | 握手时改 SNI，空字符串表示剥掉 SNI |
| `CertVerify` | 伪装后服务器返回伪装域名证书时放行 |
| `Hosts` | 跳过 DNS，写死 IP |
| `IgnoreExpiry` | 放行已过期证书 |

匹配：

- `example.com`：精确
- `*.example.com`：自身及子域
- `*example.com`：后缀（含 `www.example.com`）

Google 这类强 SNI 校验域名，三张表要一起改。
