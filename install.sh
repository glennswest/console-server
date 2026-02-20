#!/bin/bash
set -e

TARGET_HOST="console.g11.lo"
TARGET_USER="root"
BINARY_NAME="ipmiserial"

echo "=== ipmiserial Install ==="

# Build the binary
echo "Building binary..."
GOOS=linux GOARCH=arm64 go build -o ${BINARY_NAME} .

# Copy files to target
echo "Copying files to ${TARGET_HOST}..."
scp ${BINARY_NAME} ${TARGET_USER}@${TARGET_HOST}:/usr/local/bin/
scp config.yaml.example ${TARGET_USER}@${TARGET_HOST}:/tmp/config.yaml.example

# Setup on target
echo "Setting up on ${TARGET_HOST}..."
ssh ${TARGET_USER}@${TARGET_HOST} << 'ENDSSH'
set -e

# Create directories
mkdir -p /etc/ipmiserial
mkdir -p /data/logs

# Install config if not exists
if [ ! -f /etc/ipmiserial/config.yaml ]; then
    cp /tmp/config.yaml.example /etc/ipmiserial/config.yaml
    echo "Config installed at /etc/ipmiserial/config.yaml"
fi

# Make binary executable
chmod +x /usr/local/bin/ipmiserial

# Install ipmitool if not present
if ! command -v ipmitool > /dev/null 2>&1; then
    echo "Installing ipmitool..."
    apk add --no-cache ipmitool
fi

# Stop existing instance if running
pkill -f ipmiserial || true
sleep 1

# Start the server
cd /etc/ipmiserial
nohup /usr/local/bin/ipmiserial > /var/log/ipmiserial.log 2>&1 &
sleep 2

echo ""
echo "=== Installation Complete ==="
ps aux | grep ipmiserial | grep -v grep || echo "Process not found"
echo ""
echo "Logs: /var/log/ipmiserial.log"
echo "Web UI: http://console.g11.lo:8080"
ENDSSH

# Cleanup local binary
rm -f ${BINARY_NAME}

echo "Done!"
