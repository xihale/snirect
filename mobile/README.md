# Snirect Mobile Core (Go Gomobile 绑定层)

`mobile` 是 Snirect 面向 Android 系统的 Go 语言胶水模块。它通过 [gomobile](https://pkg.go.dev/golang.org/x/mobile/cmd/gomobile) 编译生成 `core.aar`，将平台无关的 Go 核心代理能力与 Android 系统的 `VpnService`（TUN 虚拟网卡）对接。

---

## 1. 核心架构与数据链路

在 Android 系统中，应用层发出的并非代理请求，而是直连目的 IP 的原始 TCP 报文。`mobile` 负责完成数据平面的协议适配：

```ascii
[Android App 流量]
       │
       ▼ (TUN 文件描述符 / IP 报文)
[gVisor netstack] (用户态协议栈，终结 TCP 连接，获取原始目的 IP:Port)
       │
       ▼ (TCP 流转发)
[relayToProxy] ──> 合成 "CONNECT <orig-dst-ip>:<port> HTTP/1.1"
       │
       ▼ (127.0.0.1 环回连接)
[Snirect Proxy]
       │
       ├──> [ClientHello Peek] 嗅探真实 SNI 扩展并匹配分流规则
       ├──> [AlterHostname / CertVerify] 动态 MITM 或直连穿透
       │
       ▼ (通过 protect() 绕过 TUN 的 Outbound Socket)
[目标远端服务器]
```

---

## 2. 核心技术机制

### 2.1 合成 CONNECT 转发 (`netstack.go`)
- **机制**：TUN 捕获的 TCP 流通过 gVisor forwarder 获取目的地址（`r.ID().LocalAddress` 与 `LocalPort`）。
- **适配**：在将流注入本地 `ProxyServer` 时，先自动写入合成的 `CONNECT <ip>:<port> HTTP/1.1\r\n\r\n`，使所有透明流量平滑进入通用代理的 MITM / SNI 重写处理流水线。

### 2.2 DNS 劫持与 AAAA 拦截过滤
- **UDP:53 拦截**：TUN 内发往 53 端口的 DNS 请求直接转交 Go 解析器，使用配置的安全 DoH/DoT 上游解析。
- **AAAA 拦截机制**：当 `IPv6 = false` 时，对 AAAA 查询直接响应空 `NOERROR`（表示域名存在但无 IPv6 记录），强制应用回退至 IPv4，避免因运营商 IPv6 路由黑洞导致的连接假死。

### 2.3 套接字逃逸 (`protect()`)
- 所有由 Go 核心主动向外发起的出站 TCP 连接和 DNS 查询套接字，必须经过 `EngineCallbacks.Protect(fd)` 处理，调用 Android `VpnService.protect(fd)` 排除在 TUN 路由之外，杜绝流量自环。

### 2.4 健壮的生命周期契约 (`lib.go`)
- **非阻塞启动**：`StartEngine` 立即返回，所有繁重的密钥生成、规则加载及网络监听全在后台协程执行。
- **代际安全 (Epoch)**：内部维护代际计数器，彻底解决快速点击切换时的 Start/Stop 竞态与双启动泄漏。
- **显式状态回调**：
  - `OnEngineStarted()`：所有组件就绪，代理端口开始正常服务。
  - `OnEngineError(reason)`：遇到启动异常或运行时致命故障（如 TUN 异常断开）时触发，并自动完成资源回收。
  - `OnEngineStopped(reason)`：显式调用 `StopEngine` 后的终态回调。

---

## 3. 导出符号约束

- 严禁在 `mobile-core` 中定义多余的非必要导出结构体（如业务 Rule 模型）。
- 所有与 Kotlin 交互的配置通过 JSON 字符串统一传递，保持 JNI 边界轻量稳定。
