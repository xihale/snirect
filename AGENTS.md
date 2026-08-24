# Snirect Monorepo 开发与约束规范 (AGENTS.md)

本文档是 AI 助手与开发人员在 **Snirect** 单体仓库 (Monorepo) 中执行任务的操作守则。

---

## 1. 仓库与模块布局

项目统一收敛为 **`github.com/xihale/snirect`** 单一 Monorepo，不再划分多独立仓库或发布中间 Maven 库：

| 目录/文件 | 角色与定位 | 构建方式 |
| :--- | :--- | :--- |
| **`cmd/snirect/`** | 桌面端 CLI 命令行入口 (`snirect`)。 | `make build` / `go build` |
| **`proxy/`** | HTTP/HTTPS 代理服务器、TLS 拦截与上游证书验证（含 `CertificateManager`/`Resolver` 接口与 `VerifyCert`）。 | 原生 Go |
| **`service/`** | 桌面端服务生命周期管理（安装/卸载/系统服务，基于 kardianos/service）。 | 原生 Go |
| **`cert/`**, **`dns/`**, **`rules/`**, **`sysproxy/`** | 通用 Go 核心引擎逻辑（证书、DNS、规则与系统代理集成）。证书策略类型 (`CertPolicy`/`ParseCertPolicy`) 定义于 `rules/certpolicy.go`。 | 原生 Go |
| **`config/`** | 配置定义与加载器。默认配置与 PAC 模板直接以 Go 原生字符串编写于 `config/defaults.go`，无需外部 `.toml` 资源文件。 | 原生 Go |
| **`color/`** | logger 与 CLI 共享的 ANSI 颜色/样式转义序列（含 `NO_COLOR` 检测）。 | 原生 Go |
| **`logger/`** | 结构化日志 (`slog`) 分模块 logger。 | 原生 Go |
| **`mobile/`** | Android gomobile 绑定层 (`package core`)，生成 `core.aar`。 | `make core` (`gomobile bind`) |
| **`android/`** | Android App 宿主工程 (`app` 模块)。直接依赖本地 `core.aar`，Kotlin 桥接代码位于 `com.xihale.snirect.ktlib`。 | `make app` / Gradle |
| **`scripts/`** | 开发辅助脚本（`git_version.sh` 版本号生成、`automate_test.sh` 冒烟测试）。 | `bash` |
| **`Makefile`** | 根目录统一构建入口，替代以往的 Mage 系统。 | `make <target>` |

---

## 2. 构建与开发指令 (Makefile 统一入口)

所有构建操作均在**工作区根目录**下执行：

### 2.1 桌面端 CLI 与核心开发

```bash
make build         # 编译当前宿主架构二进制至 dist/snirect
make crossAll      # 6 目标平台交叉编译 (linux/darwin/windows × amd64/arm64)
make upx           # 使用 UPX 压缩 dist/ 下二进制
make checksum      # 生成 SHA256 校验和文件 dist/checksums.txt
make release-cli   # 完整 CLI 构建发布流程 (clean -> crossAll -> upx -> checksum)
make test          # 执行全部 Go 单元测试 (含 -race)
make lint          # 运行 golangci-lint
```

### 2.2 Android 开发

```bash
make core          # 编译 gomobile 产物至 android/app/libs/core.aar (全 ABI)
make debug         # 一键构建 Debug 版本 (编译 core.aar + assembleDebug)
make release       # 一键构建 Release 版本 (编译 arm64 core.aar + assembleRelease)
make app           # 单独触发 Gradle 编译 (支持 BUILD_TYPE=Debug/Release)
make install       # 编译并安装到连接的 adb 设备
make run           # 编译、安装并启动 Android App
```

### 2.3 清理

```bash
make clean         # 清理 dist/、core.aar 以及 Android build 产物
```

---

## 3. 架构与开发规范

1. **统一单模块**：Go 模块根路径为 `github.com/xihale/snirect`，所有内部包引用统一使用该前缀。
2. **零外部配置资源文件**：默认配置直接硬编码在 `config/defaults.go` 中的 `SampleConfigTOML` 和 `DefaultPAC`，严禁重新引入运行时必须读取的外部 `default.toml` 文件。
3. **原生 Makefile 管理**：项目已移除 Mage 依赖，严禁重新引入 `magefile.go` 或类似构建工具。
4. **Android 产物归属**：gomobile 产物唯一生成到 `android/app/libs/core.aar`，由 Android App 直接通过 `fileTree(dir: 'libs')` 消费，不再进行本地 Maven 发布发布流程。
5. **规则维护**：内置静态规则维护于 `rules/builtin_rules.go`，由 `LoadRules()` 在初始化时自动对 key 建立有序索引。
