# Snirect Android Bindings（gomobile 绑定层）

`android/bindings`（Go package `core`）是 Snirect 面向 Android 的胶水层，通过 `gomobile bind` 编译为 `core.aar`，将平台无关的核心代理（`internal/proxy`、`internal/dns`、`internal/cert`、`internal/rules`、`internal/update`）与 Android `VpnService`（TUN 虚拟网卡）对接。

## 1. 数据链路

Android VPN 捕获的是直连目的 IP 的原始 IP 报文，而非 HTTP 代理请求。`core` 在本地完成协议桥接：

```text
[Android App 流量]
       │
       ▼ (VpnService TUN FD，dup 后交由 Go 独占关闭)
[gVisor netstack] (10.0.0.2/24，终结 TCP/UDP，保留原始目的 IP:Port)
       │
       ├── UDP :53 ──> [handleDNS]
       │                 ├─ IPv6=false 时 AAAA 直接返回空 NOERROR
       │                 ├─ 命中 Hosts 规则时直接返回静态 A 记录
       │                 └─ 其余走共享 dns.Resolver（经 protect() 出站，失败重试一次否则 SERVFAIL）
       │
       └── TCP ──────> [relayToProxy] 合成 "CONNECT <dst-ip>:<port> HTTP/1.1"
                         │             附带 X-Snirect-Src 与（443 端口）X-Snirect-Uid
                         ▼ (127.0.0.1:<auto> 环回)
                       [proxy.ProxyServer]
                         ├─ ClientHello Peek 提取 SNI 并匹配内置规则
                         ├─ 命中规则：按 AlterHostname / CertVerify 执行 MITM
                         ├─ 未命中：直接透传；客户端拒绝叶证书则触发 OnCertRejected
                         │
                         ▼ (bypassDialer → EngineCallbacks.Protect(fd))
                       [远端服务器]
```

## 2. 关键机制

- **合成 CONNECT 转发（`netstack.go`）**：TUN 捕获的 TCP 流从 gVisor forwarder 获取原始目的地址后，向本地 `ProxyServer` 发送合成的 `CONNECT <ip>:<port> HTTP/1.1`，复用与桌面端完全一致的 SNI 嗅探、MITM 与证书校验流水线。
- **套接字逃逸（`platform_android.go`）**：所有由 Go 核心发起的出站 TCP 连接与加密 DNS 查询都使用 `bypassDialer`，在建立连接前调用 `EngineCallbacks.Protect(fd)` 触发 `VpnService.protect(fd)`，避免流量回环进 TUN。
- **按流应用归属与证书固定检测（`sockdiag_netlink.go` / `lib.go`）**：对 `:443` 流优先通过内核 `NETLINK_SOCK_DIAG` 查询套接字 UID，查不到时回退 `EngineCallbacks.LookupUID`；若客户端在收到叶证书后中止握手，代理通过 `OnCertRejected` 上报 `{uid, src, host, err}` JSON，供 Kotlin 侧自动排除对应应用。
- **TUN FD 生命周期安全（`platform_android.go`）**：`newTunFile` 先 `syscall.Dup(fd)` 再交由 `os.NewFile` 接管，保证 Go 与 Kotlin 各自安全关闭互不干扰，避免重连时 double-close 引发崩溃。

## 3. 导出接口与生命周期契约

Kotlin 侧通过 `SnirectCoreClient` 调用的导出符号（`lib.go` / `update.go`）：

- `SetDataDir(path string)`：设置 CA 证书与私钥存放目录（`ca.crt` / `ca.key`），需在启动或导出证书前调用。
- `StartEngine(fd int, configStr string, cb EngineCallbacks)`：同步解析 JSON 配置后立即返回，在后台协程完成 CA 初始化、代理监听与 netstack 启动；内部维护 `epoch` 代际计数器消除连续启停竞态。
- `StopEngine()`：同步幂等停止当前引擎、关闭 netstack/代理/解析器/CA，并触发终态回调。
- `GetCACertificate() []byte`：返回根 CA PEM（引擎未运行时按需冷加载并在读取后清理）。
- `CheckUpdate(current string) (*UpdateInfo, error)`：通过代理同款的 Hosts-IP + 空 SNI 路径检查 Android 最新 Release。

`EngineCallbacks` 回调契约：

- `OnEngineStarted()`：CA、DNS 解析器、本地代理与 netstack 全部就绪后触发一次。
- `OnEngineError(reason string)`：启动失败（含同步 JSON 解析失败）或运行时致命异常时触发一次并完成资源清理，之后不再触发 `OnEngineStopped`。
- `OnEngineStopped(reason string)`：显式调用 `StopEngine()` 且确实存在运行/启动中引擎时触发一次。
- `OnStatusChanged(status string)`：异步日志流回调（覆盖 `core` 与共享 `internal/logger`）。
- `Protect(fd int) bool` / `LookupUID(...) int32` / `OnCertRejected(payload string)`：出站保护、UID 兜底查询与证书固定遥测。

导出边界约束：避免在 `core` 中直接导出复杂业务结构体；与 Kotlin 间的配置及遥测数据通过简单参数或 JSON 字符串传递。
