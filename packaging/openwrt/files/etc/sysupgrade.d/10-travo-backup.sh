#!/bin/sh
# Pre-sysupgrade hook: save travo configuration and binary to /overlay
# This runs before the sysupgrade process

TRAVO_CONFIG="/etc/config/travo"
TRAVO_AUTH="/etc/travo/auth.json"
TRAVO_VERSION="/etc/travo/version"
BACKUP_DIR="/overlay/etc/travo-backup"

[ -d "$BACKUP_DIR" ] || mkdir -p "$BACKUP_DIR"

# Backup UCI config
if [ -f "$TRAVO_CONFIG" ]; then
    cp "$TRAVO_CONFIG" "$BACKUP_DIR/travo"
fi

# Backup auth config
if [ -f "$TRAVO_AUTH" ]; then
    cp "$TRAVO_AUTH" "$BACKUP_DIR/auth.json"
fi

# Backup travo binary (for tarball installs without opkg)
if [ -x /usr/bin/travo ]; then
    cp /usr/bin/travo "$BACKUP_DIR/travo-bin"
fi

# Backup init.d script
if [ -x /etc/init.d/travo ]; then
    cp /etc/init.d/travo "$BACKUP_DIR/init.d-travo"
fi

# Backup frontend assets
if [ -d /www/travo ]; then
    mkdir -p "$BACKUP_DIR/www"
    cp -r /www/travo/* "$BACKUP_DIR/www/" 2>/dev/null || true
fi

# Store version
if [ -f "$TRAVO_VERSION" ]; then
    cp "$TRAVO_VERSION" "$BACKUP_DIR/version"
elif [ -x /usr/bin/travo ]; then
    # Try to get version from binary (if it supports --version)
    /usr/bin/travo --version 2>/dev/null | head -1 > "$BACKUP_DIR/version" || echo "latest" > "$BACKUP_DIR/version"
else
    echo "latest" > "$BACKUP_DIR/version"
fi

exit 0