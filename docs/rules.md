# 内置规则与分流导出

Snirect 的改写规则在编译期静态编入二进制，运行时不联网拉取。**只有命中规则或内置站点钩子（如 GitHub 下载跳转处理）的 443 端口连接才会做 TLS MITM 中转**；其余连接直接做 TCP 透传，客户端看到的仍是远端原始证书。

## 查询与测试

```bash
snirect rules list               # 查看全部内置规则
snirect rules match github.com   # 检查单域名命中的 IP、SNI 策略与证书策略
```

## 上游分流导出（dae / clash）

如果本机或网关已有 `dae`、`clash` / `mihomo` 等分流工具，可以只把 Snirect 认识的域名转给 Snirect：

```bash
snirect rules export dae >> my-dae.conf        # 输出 domain(...) -> snirect 路由块
snirect rules export clash > snirect.yaml      # 输出 - DOMAIN-SUFFIX,... / DOMAIN,... 规则
```

默认导出指向名为 `snirect` 的策略组/出站，可用 `--policy <名称>` 覆盖。导出器会自动对所有规则表的域名去重和合并（被更宽后缀覆盖的子域会被省略）。

---

## 规则格式与维护（开发说明）

内置规则定义在 `internal/rules/builtin_rules.go`，同步自 [Cealing-Host](https://github.com/SpaceTimee/Cealing-Host) 并针对真实前置 IP 与证书做了校验适配。`rules.LoadRules()` 启动时会自动根据各 map 的键重建按长度降序排列的索引切片。

### 四张规则表

| 表名 | 作用 |
| :--- | :--- |
| `AlterHostname` | 握手时改写 ClientHello SNI；空字符串 `""` 表示剥离 SNI |
| `CertVerify` | 控制远端证书校验：`false` 跳过校验；`"strict"` 严格校验；域名或域名列表表示放行伪装后的指定证书 SAN/CN |
| `Hosts` | 跳过 DNS 解析，直接连接指定 IP |
| `IgnoreExpiry` | 放行已过期的远端证书 |

### 域名匹配语义（`rules.MatchHost`）

精确键优先；其余键按长度从长到短依次匹配（大小写不敏感，忽略末尾 `.`）：

- `example.com`：仅精确匹配 `example.com`
- `*.example.com`：匹配 `example.com` 本身及所有子域名（如 `a.example.com`）
- `*example.com`：后缀匹配（同样匹配 `example.com` 与 `www.example.com`，注意避免过短后缀误伤无关域名）

> 修改强 SNI 校验的域名家族（如 Google）时，通常需要联动检查 `AlterHostname`、`Hosts` 和 `CertVerify` 三张表。
