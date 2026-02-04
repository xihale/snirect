#!/bin/bash
set -e

# Project configuration
BINARY_NAME="snirect"
BUILD_DIR="dist"
CMD_PATH="./cmd/snirect/main.go"

# Clean up
echo "Cleaning up..."
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"

# Platforms to build for (OS/ARCH)
PLATFORMS=(
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
    "windows/amd64"
    "windows/arm64"
)

# Build for each platform
for PLATFORM in "${PLATFORMS[@]}"; do
    IFS="/" read -r OS ARCH <<< "$PLATFORM"
    
    EXT=""
    if [ "$OS" == "windows" ]; then
        EXT=".exe"
    fi
    
    OUTPUT_NAME="${BINARY_NAME}-${OS}-${ARCH}${EXT}"
    echo "Building $OUTPUT_NAME..."
    
    # We use -ldflags="-s -w" for smaller binaries in release
    GOOS=$OS GOARCH=$ARCH go build -ldflags="-s -w" -o "${BUILD_DIR}/${OUTPUT_NAME}" "$CMD_PATH"
done

# Generate checksums
echo "Generating checksums..."
cd "$BUILD_DIR"
# Use sha256sum (standard on linux) or shasum -a 256 (standard on macOS)
if command -v sha256sum >/dev/null 2>&1; then
    sha256sum * > checksums.txt
else
    shasum -a 256 * > checksums.txt
fi
cd ..

echo "Build complete! Artifacts are in $BUILD_DIR/"
ls -lh "$BUILD_DIR"
