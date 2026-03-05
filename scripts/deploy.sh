#!/bin/bash
# Deploy to OpenWRT device via SSH
# Usage: ./scripts/deploy.sh <router_ip> [ipk_path]
set -euo pipefail

ROUTER_IP=${1:?"Usage: deploy.sh <router_ip> [ipk_path]"}
IPK_PATH=${2:-$(ls -1t dist/*.ipk 2>/dev/null | head -1)}

if [ -z "$IPK_PATH" ] || [ ! -f "$IPK_PATH" ]; then
    echo "Error: No .ipk file found. Run scripts/build.sh && scripts/package-ipk.sh first."
    exit 1
fi

echo "Deploying ${IPK_PATH} to ${ROUTER_IP}..."
scp "$IPK_PATH" "root@${ROUTER_IP}:/tmp/"
IPK_FILE=$(basename "$IPK_PATH")
ssh "root@${ROUTER_IP}" "opkg install /tmp/${IPK_FILE} && rm /tmp/${IPK_FILE}"
echo "✓ Deployed successfully. Access at http://${ROUTER_IP}/"
echo "  LuCI moved to http://${ROUTER_IP}:8080/"
