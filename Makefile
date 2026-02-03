BINARY_NAME=snirect
BUILD_DIR=dist
CMD_PATH=./cmd/snirect

.PHONY: all build release install uninstall clean test cross-all clean-dist update-rules

all: build

# Standard build
build:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_PATH)

# Optimized build for release (smaller binary)
release:
	@mkdir -p $(BUILD_DIR)
	go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_PATH)

# Build and run the internal install logic (Binary + Systemd)
install: build
	./$(BUILD_DIR)/$(BINARY_NAME) install

# Run the internal uninstall logic
uninstall: build
	./$(BUILD_DIR)/$(BINARY_NAME) uninstall

# Run tests
test:
	go test ./...

# Update rules from Cealing-Host
update-rules:
	cd scripts && go run update_rules.go

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)
	rm -f log.txt

# Clean only dist folder
clean-dist:
	rm -rf $(BUILD_DIR)

# Cross-platform builds (parallel by default)
cross-all:
	@mkdir -p $(BUILD_DIR)
	@$(MAKE) -j cross-linux-amd64 cross-linux-arm64 cross-darwin-amd64 cross-darwin-arm64 cross-windows-amd64 cross-windows-arm64

cross-linux-amd64:
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_PATH)

cross-linux-arm64:
	GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(CMD_PATH)

cross-darwin-amd64:
	GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_PATH)

cross-darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(CMD_PATH)

cross-windows-amd64:
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(CMD_PATH)

cross-windows-arm64:
	GOOS=windows GOARCH=arm64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-arm64.exe $(CMD_PATH)
