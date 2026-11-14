# 编译与发版

## 桌面端 CLI

前置：Go 1.25.5+、Make。

```bash
make build         # 当前平台 → dist/snirect
make crossAll      # 6 平台交叉编译 (linux/darwin/windows × amd64/arm64)
make upx           # UPX 压缩 dist/ 下二进制（可选，需自装 upx）
make checksum      # 生成 dist/checksums.txt SHA256
make release-cli   # 清理 dist → 交叉编译 → 生成校验和（UPX=1 时启用压缩）
make test          # 单元测试（go test -v -race ./...）
make lint          # golangci-lint
```

版本号由 `scripts/git_version.sh` 从最近的 `v*` tag 推导（无 tag 时为 `0.0.0-dev`），经 `-ldflags -X` 注入 `internal/cli.Version`。

## Android

前置：JDK 17+、Android SDK（`compileSdk` / `targetSdk` 36，`minSdk` 26）、NDK `27.2.12479018`、Go 与 `gomobile`：

```bash
go install golang.org/x/mobile/cmd/gomobile@latest
gomobile init
```

常用构建目标：

```bash
make core          # 编译 Go runtime AAR → android/app/libs/core.aar（默认全 ABI；可用 ARCH=android/arm64 指定单 ABI）
make app           # 编译 APK（BUILD_TYPE=Debug 默认双 ABI arm64-v8a + x86_64；BUILD_TYPE=Release 默认 arm64-v8a，可用 ABI=x86_64 覆盖）
make debug         # 一键 Debug：全 ABI core.aar + Debug APK
make release       # 一键 Release：arm64 core.aar + Release APK
make install       # 安装 APK 到连接的设备
make run           # 安装并启动
```

Debug 包 `applicationId` 带 `.debug` 后缀，可与 Release 共存。Release 签名配置读取 `android/keystore.properties`（gitignored）；该文件缺失时生成未签名的 Release APK。

## CI 与发版流程

发版只走 **git tag + GitHub Actions**，桌面端 CLI 与 Android 各自独立：

- **CLI**：推送 `v*`（如 `v0.5.0`）→ [release-cli.yml](../.github/workflows/release-cli.yml)：运行测试、交叉编译 6 平台二进制、生成 `checksums.txt` 发布到 GitHub Releases，并同步更新 AUR（`snirect` / `snirect-bin`）。默认关闭 UPX，手动触发 workflow 时可勾选启用。
- **Android**：推送 `android-v*`（如 `android-v0.4.1`）→ [release-android.yml](../.github/workflows/release-android.yml)：分别构建 `arm64` 与 `x86_64` Release APK（配置了仓库签名 Secrets 时自动签名）并生成校验和挂到 Releases。
- **CI**：[ci.yml](../.github/workflows/ci.yml) 在非 `v*` tag 推送与 PR 时运行 `golangci-lint`、单元测试、交叉编译及 `android/bindings` 编译检查。

两端各自使用 `scripts/git_version.sh 'v*'` 与 `scripts/git_version.sh 'android-v*'` 推导版本号，互不干扰。
