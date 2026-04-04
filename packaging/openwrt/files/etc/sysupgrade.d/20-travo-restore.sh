#!/bin/sh
# Post-sysupgrade hook: restore travo configuration and reinstall if needed
# This runs after sysupgrade completes

set -e

TRAVO_CONFIG="/etc/config/travo"
TRAVO_AUTH="/etc/travo/auth.json"
BACKUP_DIR="/overlay/etc/travo-backup"

GITHUB_REPO="${GITHUB_REPO:-raydak-labs/travo}"
GITHUB_RAW_BASE="https://raw.githubusercontent.com/${GITHUB_REPO}/main"

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

# Restore version file
if [ -f "$BACKUP_DIR/version" ]; then
    cp "$BACKUP_DIR/version" /etc/travo/version 2>/dev/null || true
fi

# Restore uhttpd ports (run uci-defaults if exists)
if [ -f /etc/uci-defaults/99-travel-gui-ports ]; then
    sh /etc/uci-defaults/99-travel-gui-ports
    rm -f /etc/uci-defaults/99-travel-gui-ports
fi

# Determine architecture
ARCH="$(uname -m)"
case "$ARCH" in
    aarch64) DIST_ARCH="aarch64_cortex-a53" ;;
    x86_64|amd64) DIST_ARCH="x86_64" ;;
    *) DIST_ARCH="aarch64_cortex-a53" ;;
esac

# Get version
VERSION="$(cat "$BACKUP_DIR/version" 2>/dev/null || echo "latest")"

# Re-install travo if missing
if [ ! -x /usr/bin/travo ]; then
    # Try local binary backup first
    if [ -f "$BACKUP_DIR/travo-bin" ]; then
        cp "$BACKUP_DIR/travo-bin" /usr/bin/travo
        chmod +x /usr/bin/travo
    else
        # Download tarball from GitHub
        if [ "$VERSION" = "latest" ]; then
            VERSION=$(wget -qO- "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" 2>/dev/null | \
                sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | tr -d 'v') || VERSION="latest"
        fi

        TARFILE="travo_${VERSION}_${DIST_ARCH}.tar.gz"
        cd /tmp
        wget -q "https://github.com/${GITHUB_REPO}/releases/download/v${VERSION}/${TARFILE}" -O "${TARFILE}" 2>/dev/null && \
            tar -xzf "${TARFILE}" -C / 2>/dev/null && \
            rm -f "${TARFILE}" || true
    fi
fi

# Restore init.d script if missing
if [ ! -x /etc/init.d/travo ]; then
    if [ -f "$BACKUP_DIR/init.d-travo" ]; then
        cp "$BACKUP_DIR/init.d-travo" /etc/init.d/travo
        chmod +x /etc/init.d/travo
    else
        # Download init.d from GitHub
        wget -q "${GITHUB_RAW_BASE}/packaging/openwrt/files/etc/init.d/travo" -O /etc/init.d/travo 2>/dev/null && \
            chmod +x /etc/init.d/travo || true
    fi
fi

# Restore frontend assets if missing or outdated
if [ ! -d /www/travo ] || [ ! -f /www/travo/index.html ]; then
    mkdir -p /www/travo
    if [ -d "$BACKUP_DIR/www" ]; then
        cp -r "$BACKUP_DIR/www/"* /www/travo/ 2>/dev/null || true
    else
        # Extract from tarball if available
        if [ -f "/tmp/travo_${VERSION}_${DIST_ARCH}.tar.gz" ]; then
            tar -xzf "/tmp/travo_${VERSION}_${DIST_ARCH}.tar.gz" -C / www/travo 2>/dev/null || true
        fi
    fi
fi

# Enable and start travo
/etc/init.d/travo enable 2>/dev/null || true
/etc/init.d/travo start 2>/dev/null || true

exit 0