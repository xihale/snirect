BINARY_NAME=snirect
BUILD_DIR=.
CMD_PATH=./cmd/snirect

.PHONY: all build release install uninstall clean test

all: build

# Standard build
build:
	go build -o $(BINARY_NAME) $(CMD_PATH)

# Optimized build for release (smaller binary)
release:
	go build -ldflags="-s -w" -o $(BINARY_NAME) $(CMD_PATH)

# Build and run the internal install logic (Binary + Systemd)
install: build
	./$(BINARY_NAME) install

# Run the internal uninstall logic
uninstall: build
	./$(BINARY_NAME) uninstall

# Run tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -f log.txt
