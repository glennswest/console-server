#!/bin/bash
set -e

BINARY_NAME="ipmiserial"
IMAGE_NAME="ipmiserial"
TAR_NAME="${IMAGE_NAME}.tar"

echo "=== Building ipmiserial ==="

# Build the Go binary for arm64
echo "Building binary for arm64..."
GOOS=linux GOARCH=arm64 go build -o ${BINARY_NAME} .

# Build the container image
echo "Building container image..."
podman build --platform linux/arm64 -t ${IMAGE_NAME}:latest .

# Save as tarball
echo "Saving image as tarball..."
podman save ${IMAGE_NAME}:latest -o ${TAR_NAME}

# Cleanup binary (it's in the image now)
rm -f ${BINARY_NAME}

echo ""
echo "Build complete: ${TAR_NAME}"
echo "Run ./deploy.sh to deploy to rose1"
