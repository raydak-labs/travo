#!/bin/sh
# Post-sysupgrade hook: restore travo configuration and reinstall if needed
# This runs after sysupgrade completes

set -e

TRAVO_CONFIG="/etc/config/travo"
TRAVO_AUTH="/etc/travo/auth.json"
BACKUP_DIR="/overlay/etc/travo-backup"

GITHUB_REPO="${GITHUB_REPO:-raydak-labs/travo}"

# Exit if no backup exists (fresh install case)
[ -d "$BACKUP_DIR" ] || exit 0

# Restore UCI config
if [ -f "$BACKUP_DIR/travo" ]; then
    cp "$BACKUP_DIR/travo" "$TRAVO_CONFIG" 2>/dev/null || true
    chmod 600 "$TRAVO_CONFIG" 2>/dev/null || true
fi

# Restore auth config
mkdir -p /etc/travo
chmod 700 /etc/travo
if [ -f "$BACKUP_DIR/auth.json" ]; then
    cp "$BACKUP_DIR/auth.json" "$TRAVO_AUTH" 2>/dev/null || true
    chmod 600 "$TRAVO_AUTH" 2>/dev/null || true
fi

# Restore uhttpd ports
if [ -f /etc/uci-defaults/99-travel-gui-ports ]; then
    sh /etc/uci-defaults/99-travel-gui-ports
    rm -f /etc/uci-defaults/99-travel-gui-ports
fi

# Re-install travo binary if missing
if [ ! -x /usr/bin/travo ]; then
    ARCH="$(uname -m)"
    case "$ARCH" in
        aarch64) IPK_ARCH="aarch64_cortex-a53" ;;
        x86_64|amd64) IPK_ARCH="x86_64" ;;
        *) IPK_ARCH="aarch64_cortex-a53" ;;
    esac

    # Try to use local IPK from backup first
    if [ -f "$BACKUP_DIR/travo.ipk" ]; then
        opkg install "$BACKUP_DIR/travo.ipk" 2>/dev/null || true
    else
        # Get version from backup or use latest
        VERSION="$(cat "$BACKUP_DIR/version" 2>/dev/null || echo "latest")"

        if [ "$VERSION" = "latest" ]; then
            VERSION=$(wget -qO- "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" 2>/dev/null | \
                sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | tr -d 'v')
        fi

        # Download IPK
        cd /tmp
        wget -q "https://github.com/${GITHUB_REPO}/releases/download/v${VERSION}/travo_${VERSION}_${IPK_ARCH}.ipk" \
            -O travo.ipk 2>/dev/null && \
            opkg install travo.ipk 2>/dev/null && \
            rm -f travo.ipk
    fi
fi

# Enable and start travo
/etc/init.d/travo enable 2>/dev/null || true
/etc/init.d/travo start 2>/dev/null || true

exit 0