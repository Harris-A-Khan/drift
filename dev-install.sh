#!/bin/bash
set -e

cd "$(dirname "$0")"

VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
echo "Building drift ${VERSION}..."

# Clean stale build artifacts and Go build cache to ensure fresh compilation
rm -rf bin
go clean -cache 2>/dev/null || true

# Build
mkdir -p bin
go build -ldflags "-s -w -X main.version=${VERSION}" -o bin/drift ./cmd/drift

# Install
echo "Installing drift to /usr/local/bin..."
mkdir -p /usr/local/bin
cp bin/drift /usr/local/bin/drift

echo "Installed drift ${VERSION} successfully!"
drift --version
