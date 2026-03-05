#!/bin/bash
# Create .ipk package for OpenWRT
# Usage: ./scripts/package-ipk.sh [architecture]
set -euo pipefail

VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "0.1.0")
ARCH=${1:-aarch64_cortex-a53}
PKG_NAME="openwrt-travel-gui"
BUILD_DIR="dist"
IPK_DIR="${BUILD_DIR}/ipk"

echo "Packaging ${PKG_NAME} v${VERSION} for ${ARCH}..."

# Verify binary exists
if [ ! -f "${BUILD_DIR}/openwrt-travel-gui" ]; then
    echo "Error: Binary not found at ${BUILD_DIR}/openwrt-travel-gui"
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
mkdir -p "${IPK_DIR}/data/www/openwrt-travel-gui"
mkdir -p "${IPK_DIR}/control"

# Copy binary
cp "${BUILD_DIR}/openwrt-travel-gui" "${IPK_DIR}/data/usr/bin/"

# Copy frontend assets
cp -r frontend/dist/* "${IPK_DIR}/data/www/openwrt-travel-gui/"

# Copy config files
cp packaging/openwrt/files/etc/init.d/openwrt-travel-gui "${IPK_DIR}/data/etc/init.d/"
cp packaging/openwrt/files/etc/config/openwrt-travel-gui "${IPK_DIR}/data/etc/config/"
cp packaging/openwrt/files/etc/uci-defaults/99-travel-gui-ports "${IPK_DIR}/data/etc/uci-defaults/"

# Create control file
cat > "${IPK_DIR}/control/control" << EOF
Package: ${PKG_NAME}
Version: ${VERSION}
Architecture: ${ARCH}
Description: Modern web UI for OpenWRT travel routers
Maintainer: openwrt-travel-gui contributors
Section: luci
Priority: optional
Depends: libc
EOF

# Create postinst
cat > "${IPK_DIR}/control/postinst" << 'EOF'
#!/bin/sh
chmod +x /usr/bin/openwrt-travel-gui
chmod +x /etc/init.d/openwrt-travel-gui
/etc/init.d/openwrt-travel-gui enable
# Apply UCI defaults
[ -f /etc/uci-defaults/99-travel-gui-ports ] && {
    sh /etc/uci-defaults/99-travel-gui-ports
    rm -f /etc/uci-defaults/99-travel-gui-ports
}
/etc/init.d/openwrt-travel-gui start
EOF
chmod +x "${IPK_DIR}/control/postinst"

# Create prerm
cat > "${IPK_DIR}/control/prerm" << 'EOF'
#!/bin/sh
/etc/init.d/openwrt-travel-gui stop 2>/dev/null
/etc/init.d/openwrt-travel-gui disable 2>/dev/null
# Restore uhttpd to default ports
uci set uhttpd.main.listen_http='0.0.0.0:80'
uci set uhttpd.main.listen_https='0.0.0.0:443'
uci commit uhttpd
/etc/init.d/uhttpd restart
EOF
chmod +x "${IPK_DIR}/control/prerm"

# Create conffiles
cat > "${IPK_DIR}/control/conffiles" << EOF
/etc/config/openwrt-travel-gui
EOF

# Build ipk (nested tar.gz archives)
cd "${IPK_DIR}"
echo "2.0" > debian-binary
(cd control && tar czf ../control.tar.gz .)
(cd data && tar czf ../data.tar.gz .)
tar czf "../../${PKG_NAME}_${VERSION}_${ARCH}.ipk" debian-binary control.tar.gz data.tar.gz
cd ../..

echo "✓ Package created: dist/${PKG_NAME}_${VERSION}_${ARCH}.ipk"
ls -lh "dist/${PKG_NAME}_${VERSION}_${ARCH}.ipk"
