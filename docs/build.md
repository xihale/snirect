# 编译与发版

需要 Go 1.25.5+、Make。

```bash
make build         # 当前平台 → dist/snirect
make crossAll      # 6 目标平台交叉编译 (linux/darwin/windows × amd64/arm64)
make lint          # golangci-lint
make test          # 单元测试
make core          # 编译 Android core.aar
make app           # 编译 Android APK
```

发布只走 **git tag + GitHub Actions**。推送 `v*`（例如 `v1.5.0`）后，CI 交叉编译 6 个平台并构建 Android APK 挂到 [Releases](https://github.com/xihale/snirect/releases)。
