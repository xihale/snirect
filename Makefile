# Snirect Monorepo Makefile
# Multi-platform Go CLI & Android Application

# --- Version & Metadata ---
VERSION ?= $(shell bash git_version.sh 2>/dev/null || echo "0.0.0-dev")
MODULE := github.com/xihale/snirect
LDFLAGS := -s -w -X '$(MODULE)/cmd.Version=$(VERSION)'

# --- Paths & Tooling ---
BUILD_DIR := dist
BINARY_NAME := snirect
CMD_PATH := ./cmd/snirect
MOBILE_PKG := ./mobile
ANDROID_DIR := android
CORE_AAR := $(ANDROID_DIR)/app/libs/core.aar

# Android configs
ARCH ?= android
ANDROID_API ?= 21
BUILD_TYPE ?= Debug
PKG := com.xihale.snirect$(if $(filter Debug,$(BUILD_TYPE)),.debug)
MAIN := com.xihale.snirect.MainActivity
GRADLE := $(shell cd $(ANDROID_DIR) 2>/dev/null && ./gradlew --version >/dev/null 2>&1 && echo "./gradlew" || echo "gradle")

# Target platforms for cross-compilation
CROSS_TARGETS := \
	linux/amd64 \
	linux/arm64 \
	darwin/amd64 \
	darwin/arm64 \
	windows/amd64 \
	windows/arm64

.PHONY: all build snirect cross crossAll upx checksum release-cli \
        test lint core app debug release install run clean help

# Default target
all: build

help:
	@echo "Snirect 构建管理工具 (Monorepo)"
	@echo ""
	@echo "桌面端 CLI 指令:"
	@echo "  make build / snirect - 编译当前平台 CLI 二进制至 $(BUILD_DIR)/$(BINARY_NAME)"
	@echo "  make crossAll        - 交叉编译 6 平台二进制 (linux/darwin/windows x amd64/arm64)"
	@echo "  make upx             - 使用 UPX 压缩 $(BUILD_DIR)/ 下的二进制"
	@echo "  make checksum        - 生成 $(BUILD_DIR)/checksums.txt SHA256 校验和"
	@echo "  make release-cli     - 完整发布流程: 清理 -> 交叉编译 -> UPX 压缩 -> 生成校验和"
	@echo "  make test            - 运行全部 Go 单元测试"
	@echo "  make lint            - 运行 golangci-lint 代码检查"
	@echo ""
	@echo "Android 指令:"
	@echo "  make core            - 编译 Go runtime AAR (当前 ARCH=$(ARCH))"
	@echo "  make app             - 编译 Android APK (当前 BUILD_TYPE=$(BUILD_TYPE))"
	@echo "  make debug           - 一键编译 Debug 版 (全 ABI core.aar + Debug APK)"
	@echo "  make release         - 一键编译 Release 版 (arm64 core.aar + Release APK)"
	@echo "  make install         - 安装 APK 到连接的 Android 设备"
	@echo "  make run             - 安装并启动 Android 应用"
	@echo ""
	@echo "通用指令:"
	@echo "  make clean           - 清理所有构建产物 ($(BUILD_DIR), core.aar, Gradle cache)"

# --- CLI 构建 ---

build: snirect

snirect:
	@echo ">>> 正在构建 $(BINARY_NAME) [$(VERSION)]..."
	@mkdir -p $(BUILD_DIR)
	go build -trimpath -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_PATH)
	@echo "构建完成: $(BUILD_DIR)/$(BINARY_NAME)"

cross: crossAll

crossAll:
	@echo ">>> 正在交叉编译全部目标平台 [$(VERSION)]..."
	@mkdir -p $(BUILD_DIR)
	@for target in $(CROSS_TARGETS); do \
		os=$${target%/*}; \
		arch=$${target#*/}; \
		ext=""; \
		if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
		output="$(BUILD_DIR)/$(BINARY_NAME)-$$os-$$arch$$ext"; \
		echo "Building for $$os/$$arch -> $$output..."; \
		GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags="$(LDFLAGS)" -o "$$output" $(CMD_PATH) || exit 1; \
	done
	@echo "交叉编译完成."

upx:
	@if command -v upx >/dev/null 2>&1; then \
		echo ">>> 正在执行 UPX 压缩..."; \
		for f in $(BUILD_DIR)/$(BINARY_NAME)-* $(BUILD_DIR)/$(BINARY_NAME); do \
			if [ -f "$$f" ] && [ "$${f##*.}" != "txt" ]; then \
				echo "Compressing $$f..."; \
				upx --best --lzma "$$f" 2>/dev/null || true; \
			fi; \
		done; \
		echo "UPX 压缩完成."; \
	else \
		echo ">>> 未安装 upx，跳过压缩步骤"; \
	fi

checksum:
	@echo ">>> 正在生成 SHA256 校验和..."
	@cd $(BUILD_DIR) && \
	rm -f checksums.txt && \
	( sha256sum $(BINARY_NAME)* 2>/dev/null || shasum -a 256 $(BINARY_NAME)* 2>/dev/null ) > checksums.txt && \
	cat checksums.txt

release-cli: clean-dist crossAll upx checksum

clean-dist:
	@rm -rf $(BUILD_DIR)

# --- 测试与代码检查 ---

test:
	@echo ">>> 运行单元测试..."
	go test -v -race ./...

lint:
	@echo ">>> 运行 golangci-lint..."
	golangci-lint run --timeout=5m

# --- Android 构建 ---

core:
	@if [ "$(ARCH)" != "android" ]; then \
		echo ">>> 警告: ARCH=$(ARCH) 不是全 ABI —— 生成的 AAR 装到其他 ABI（如 x86_64 模拟器）会 UnsatisfiedLinkError"; \
	fi
	@echo ">>> 正在编译 Snirect Android Go core runtime [$(ARCH)]..."
	@mkdir -p $(dir $(CORE_AAR))
	gomobile bind -v -trimpath -ldflags="-s -w" -target=$(ARCH) -androidapi $(ANDROID_API) -o $(CORE_AAR) $(MOBILE_PKG)
	@ls -lh $(CORE_AAR)

app:
	@echo ">>> 正在构建 Android 应用 [$(BUILD_TYPE)]..."
	@cd $(ANDROID_DIR) && $(GRADLE) assemble$(BUILD_TYPE)
	@echo ">>> APK 路径:"
	@find $(ANDROID_DIR)/app/build/outputs/apk -name "*.apk" 2>/dev/null | xargs ls -lh 2>/dev/null || true

debug:
	@$(MAKE) core ARCH=android
	@$(MAKE) app BUILD_TYPE=Debug

release:
	@$(MAKE) core ARCH=android/arm64
	@$(MAKE) app BUILD_TYPE=Release

install:
	@echo ">>> 正在安装 [$(BUILD_TYPE)] 到设备..."
	@cd $(ANDROID_DIR) && $(GRADLE) install$(BUILD_TYPE) || ( \
		echo ">>> install 失败，尝试保留应用数据并重新安装..."; \
		adb uninstall -k $(PKG); \
		cd $(ANDROID_DIR) && $(GRADLE) install$(BUILD_TYPE); \
	)

run: install
	@adb shell am start -n $(PKG)/$(MAIN)

# --- 清理 ---

clean: clean-dist
	@echo ">>> 正在清理构建产物..."
	@rm -f $(CORE_AAR)
	@cd $(ANDROID_DIR) 2>/dev/null && $(GRADLE) clean 2>/dev/null || true
	@echo "清理完成."
