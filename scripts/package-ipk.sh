#!/bin/bash
# Create .ipk package for OpenWRT
# Usage: ./scripts/package-ipk.sh [architecture]
# Override with VERSION/ARCH env vars or positional argument.
set -euo pipefail

VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "0.1.0")}"
# Strip leading 'v' from version if present
VERSION="${VERSION#v}"
ARCH="${ARCH:-${1:-aarch64_cortex-a53}}"
PKG_NAME="travo"
BUILD_DIR="dist"
IPK_DIR="${BUILD_DIR}/ipk"

echo "Packaging ${PKG_NAME} v${VERSION} for ${ARCH}..."

# Verify binary exists
if [ ! -f "${BUILD_DIR}/travo" ]; then
    echo "Error: Binary not found at ${BUILD_DIR}/travo"
    echo "Run scripts/build.sh first."
    exit 1
fi

# Verify frontend dist exists
if [ ! -d "frontend/dist" ]; then
    echo "Error: Frontend dist not found. Run scripts/build.sh first."
    exit 1
fi

# Create ipk directory structure
rm -rf "${IPK_DIR}"
mkdir -p "${IPK_DIR}/data/usr/bin"
mkdir -p "${IPK_DIR}/data/etc/init.d"
mkdir -p "${IPK_DIR}/data/etc/config"
mkdir -p "${IPK_DIR}/data/etc/uci-defaults"
mkdir -p "${IPK_DIR}/data/etc/sysupgrade.d"
mkdir -p "${IPK_DIR}/data/www/travo"
mkdir -p "${IPK_DIR}/control"

# Copy binary
cp "${BUILD_DIR}/travo" "${IPK_DIR}/data/usr/bin/"

# Copy frontend assets
cp -r frontend/dist/* "${IPK_DIR}/data/www/travo/"

# Copy config files
cp packaging/openwrt/files/etc/init.d/travo "${IPK_DIR}/data/etc/init.d/"
cp packaging/openwrt/files/etc/config/travo "${IPK_DIR}/data/etc/config/"
cp packaging/openwrt/files/etc/uci-defaults/99-travel-gui-ports "${IPK_DIR}/data/etc/uci-defaults/"
cp packaging/openwrt/files/etc/sysupgrade.d/10-travo-backup.sh "${IPK_DIR}/data/etc/sysupgrade.d/"
cp packaging/openwrt/files/etc/sysupgrade.d/20-travo-restore.sh "${IPK_DIR}/data/etc/sysupgrade.d/"

# Create control file
cat > "${IPK_DIR}/control/control" << EOF
Package: ${PKG_NAME}
Version: ${VERSION}
Architecture: ${ARCH}
Description: Modern web UI for OpenWRT travel routers
Maintainer: travo contributors
Section: luci
Priority: optional
Depends: libc
EOF

# Create postinst
cat > "${IPK_DIR}/control/postinst" << 'EOF'
#!/bin/sh
chmod +x /usr/bin/travo
chmod +x /etc/init.d/travo
/etc/init.d/travo enable
# Apply UCI defaults
[ -f /etc/uci-defaults/99-travel-gui-ports ] && {
    sh /etc/uci-defaults/99-travel-gui-ports
    rm -f /etc/uci-defaults/99-travel-gui-ports
}
/etc/init.d/travo start
uci set attendedsysupgrade.client.login_check_for_upgrades='1' 2>/dev/null && uci commit attendedsysupgrade 2>/dev/null || true
EOF
chmod +x "${IPK_DIR}/control/postinst"

# Create prerm
cat > "${IPK_DIR}/control/prerm" << 'EOF'
#!/bin/sh
/etc/init.d/travo stop 2>/dev/null
/etc/init.d/travo disable 2>/dev/null
# Restore uhttpd to default ports
uci set uhttpd.main.listen_http='0.0.0.0:80'
uci set uhttpd.main.listen_https='0.0.0.0:443'
uci commit uhttpd
/etc/init.d/uhttpd restart
EOF
chmod +x "${IPK_DIR}/control/prerm"

# Create conffiles
cat > "${IPK_DIR}/control/conffiles" << EOF
/etc/config/travo
EOF

# Build ipk (nested tar.gz archives)
cd "${IPK_DIR}"
echo "2.0" > debian-binary
(cd control && tar czf ../control.tar.gz .)
(cd data && tar czf ../data.tar.gz .)
# We are currently inside dist/ipk/, so ../ resolves to dist/.
tar czf "../${PKG_NAME}_${VERSION}_${ARCH}.ipk" debian-binary control.tar.gz data.tar.gz
cd ../..

echo "✓ Package created: dist/${PKG_NAME}_${VERSION}_${ARCH}.ipk"
ls -lh "dist/${PKG_NAME}_${VERSION}_${ARCH}.ipk"
