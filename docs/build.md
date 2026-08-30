# 编译与发版

## 桌面端 CLI

需要 Go 1.25.5+、Make。

```bash
make build         # 当前平台 → dist/snirect
make crossAll      # 6 目标平台交叉编译 (linux/darwin/windows × amd64/arm64)
make upx           # UPX 压缩 dist/ 下二进制（可选，需自装 upx）
make checksum      # 生成 dist/checksums.txt SHA256
make lint          # golangci-lint
make test          # 单元测试
```

版本号由 `scripts/git_version.sh` 从 git tag 推导（无 tag 时为 `0.0.0-dev`），经 `-ldflags -X` 注入 `internal/cli.Version`。

## Android

前置：JDK 17+、Android SDK（API 21+）、Go + gomobile（`go install golang.org/x/mobile/cmd/gobind@latest` 后由 Makefile 自动 `gomobile init`）。

```bash
make core          # 编译 Go runtime AAR → android/app/libs/core.aar
                   # 可用 ARCH=android/<abi> 指定单 ABI，默认全 ABI
make app           # 编译 APK（BUILD_TYPE=Debug 默认 arm64；ABI=x86_64 可覆盖）
make debug         # 一键 Debug：全 ABI core.aar + Debug APK
make release       # 一键 Release：arm64 core.aar + Release APK（需签名配置）
make install       # 安装 APK 到连接的设备
make run           # 安装并启动
```

Debug 包 applicationId 带 `.debug` 后缀，可与 Release 共存。签名配置读 `android/keystore.properties`（gitignored，Release 构建需自行提供；CI 用 debug 签名兜底）。

## 发布

发布只走 **git tag + GitHub Actions**，CLI 与 Android 两条线各自独立发版：

- CLI：推送 `v*`（例如 `v1.5.0`）→ [release-cli.yml](../.github/workflows/release-cli.yml)：交叉编译 6 个平台二进制、生成 SHA256 校验和，挂到 Releases，并同步 AUR（`snirect` / `snirect-bin`）。UPX 压缩默认关：本地 `make release-cli UPX=1`，或手动触发 workflow 勾选 upx
- Android：推送 `android-v*`（例如 `android-v0.4.0`）→ [release-android.yml](../.github/workflows/release-android.yml)：构建签名 APK（arm64 + x86_64）挂到 Releases
- [ci.yml](../.github/workflows/ci.yml)：非 tag 推送与 PR 跑 lint + 单元测试

版本号各自从本系列的 tag 推导（`scripts/git_version.sh 'v*'` / `'android-v*'`），互不影响。两端的更新检查按 tag 前缀各取各的最新 Release。

已安装用户可用 `snirect update` 自更新，下载后校验 SHA256 再覆盖。
