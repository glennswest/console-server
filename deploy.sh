#!/bin/bash
set -e

TAR_NAME="console-server.tar"
ROSE_HOST="admin@rose1.gw.lo"
ROSE_TARBALL_PATH="raid1/tarballs"

echo "=== Deploying Console Server to rose1 ==="

# Check if tarball exists
if [ ! -f "${TAR_NAME}" ]; then
    echo "Error: ${TAR_NAME} not found. Run ./build.sh first."
    exit 1
fi

# Upload tarball to rose1
echo "Uploading ${TAR_NAME} to rose1..."
rsync -av ${TAR_NAME} ${ROSE_HOST}:${ROSE_TARBALL_PATH}/

# Run mkpod deployment
echo "Running mkpod deployment..."
cd ../mkpod
source venv/bin/activate
python3 deploy_console.py --redeploy

echo ""
echo "Deployment complete!"
echo "Console server should be running at http://console.g11.lo"
