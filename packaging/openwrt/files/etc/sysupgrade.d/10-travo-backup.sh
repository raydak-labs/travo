#!/bin/sh
# Pre-sysupgrade hook: save travo configuration and binary to /overlay
# This runs before the sysupgrade process

TRAVO_CONFIG="/etc/config/travo"
TRAVO_AUTH="/etc/travo/auth.json"
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

# Backup current IPK if available (from opkg)
if [ -f /usr/lib/opkg/info/travo.control ]; then
    for ipk in /tmp/opkg-lists/*/travo*.ipk /var/opkg-lists/*/travo*.ipk; do
        [ -f "$ipk" ] && cp "$ipk" "$BACKUP_DIR/travo.ipk" && break
    done
fi

# Store version from opkg info or default
if [ -f /usr/lib/opkg/info/travo.control ]; then
    VERSION="$(sed -n 's/^Version: //p' /usr/lib/opkg/info/travo.control 2>/dev/null || echo "latest")"
else
    VERSION="latest"
fi
echo "$VERSION" > "$BACKUP_DIR/version"

exit 0