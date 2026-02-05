BINARY_NAME=snirect
BUILD_DIR=dist
CMD_PATH=./cmd/snirect
# Try to get version from git tag, fallback to "0.0.0-dev"
VERSION:=$(shell git describe --tags --always --dirty 2>/dev/null || echo "0.0.0-dev")
TAGS?=

LDFLAGS=-s -w -X 'snirect/internal/cmd.Version=$(VERSION)'

.PHONY: all build release full upx checksum install uninstall clean cross-all clean-dist

all: build

# Standard build
build: generate-completions
	@mkdir -p $(BUILD_DIR)
	go build -tags "$(TAGS)" -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_PATH)

# Optimized build for release (smaller binary)
release: generate-completions
	@mkdir -p $(BUILD_DIR)
	go build -tags "$(TAGS)" -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_PATH)

# Full feature build (includes QUIC)
full:
	$(MAKE) release TAGS="quic"

# Generate completion scripts for embedding
generate-completions:
	@rm -rf internal/cmd/completions
	@mkdir -p internal/cmd/completions
	@go run -tags "$(TAGS)" $(CMD_PATH) completion bash > internal/cmd/completions/bash 2>/dev/null || true
	@go run -tags "$(TAGS)" $(CMD_PATH) completion zsh > internal/cmd/completions/zsh 2>/dev/null || true
	@go run -tags "$(TAGS)" $(CMD_PATH) completion fish > internal/cmd/completions/fish 2>/dev/null || true
	@go run -tags "$(TAGS)" $(CMD_PATH) completion powershell > internal/cmd/completions/powershell 2>/dev/null || true

# Compress binaries with UPX (multi-threaded, skip unsupported windows-arm64)
upx:
	@ls $(BUILD_DIR)/$(BINARY_NAME)-* 2>/dev/null | xargs -I {} -P 0 sh -c 'upx "{}" >/dev/null 2>&1 || true'

# Build and run the internal install logic
install: build
	./$(BUILD_DIR)/$(BINARY_NAME) install

# Run the internal uninstall logic
uninstall: build
	./$(BUILD_DIR)/$(BINARY_NAME) uninstall

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)
	rm -f log.txt
	rm -rf internal/cmd/completions

# Clean only dist folder
clean-dist:
	rm -rf $(BUILD_DIR)

# Simplified Cross-platform builds
cross-all: clean generate-completions
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build -tags "$(TAGS)" -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_PATH) & \
	GOOS=linux GOARCH=arm64 go build -tags "$(TAGS)" -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(CMD_PATH) & \
	GOOS=darwin GOARCH=amd64 go build -tags "$(TAGS)" -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_PATH) & \
	GOOS=darwin GOARCH=arm64 go build -tags "$(TAGS)" -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(CMD_PATH) & \
	GOOS=windows GOARCH=amd64 go build -tags "$(TAGS)" -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(CMD_PATH) & \
	GOOS=windows GOARCH=arm64 go build -tags "$(TAGS)" -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-arm64.exe $(CMD_PATH) & \
	wait

# Generate checksums
checksum:
	cd $(BUILD_DIR) && (sha256sum * > checksums.txt 2>/dev/null || shasum -a 256 * > checksums.txt 2>/dev/null || true)

