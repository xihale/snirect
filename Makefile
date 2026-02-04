BINARY_NAME=snirect
BUILD_DIR=dist
CMD_PATH=./cmd/snirect

.PHONY: all build release upx checksum install uninstall clean cross-all clean-dist

all: build

# Standard build
build:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_PATH)

# Optimized build for release (smaller binary)
release:
	@mkdir -p $(BUILD_DIR)
	go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_PATH)

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

# Clean only dist folder
clean-dist:
	rm -rf $(BUILD_DIR)

# Simplified Cross-platform builds
cross-all: clean
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_PATH) & \
	GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(CMD_PATH) & \
	GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_PATH) & \
	GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(CMD_PATH) & \
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(CMD_PATH) & \
	GOOS=windows GOARCH=arm64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-arm64.exe $(CMD_PATH) & \
	wait

# Generate checksums
checksum:
	cd $(BUILD_DIR) && (sha256sum * > checksums.txt 2>/dev/null || shasum -a 256 * > checksums.txt 2>/dev/null || true)
