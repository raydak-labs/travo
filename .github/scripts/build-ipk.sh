#!/bin/bash
# build-ipk.sh — CI-friendly .ipk package builder
#
# Required env vars or arguments:
#   VERSION  — package version (e.g. 1.0.0)
#   ARCH     — ipk architecture (e.g. aarch64_cortex-a53, x86_64)
#
# Expects:
#   dist/openwrt-travel-gui  — compiled binary
#   frontend/dist/            — built frontend assets
#   packaging/openwrt/files/  — config/init/uci-defaults files
set -euo pipefail

VERSION="${VERSION:?VERSION env var is required}"
ARCH="${ARCH:?ARCH env var is required}"
PKG_NAME="openwrt-travel-gui"
BUILD_DIR="dist"
IPK_DIR="${BUILD_DIR}/ipk"

echo "==> Packaging ${PKG_NAME} v${VERSION} for ${ARCH}"

# ------- Verify prerequisites -------
if [ ! -f "${BUILD_DIR}/openwrt-travel-gui" ]; then
    echo "Error: binary not found at ${BUILD_DIR}/openwrt-travel-gui" >&2
    exit 1
fi
if [ ! -d "frontend/dist" ]; then
    echo "Error: frontend/dist not found" >&2
    exit 1
fi

# ------- Create ipk directory structure -------
rm -rf "${IPK_DIR}"
mkdir -p "${IPK_DIR}/data/usr/bin"
mkdir -p "${IPK_DIR}/data/etc/init.d"
mkdir -p "${IPK_DIR}/data/etc/config"
mkdir -p "${IPK_DIR}/data/etc/uci-defaults"
mkdir -p "${IPK_DIR}/data/www/openwrt-travel-gui"
mkdir -p "${IPK_DIR}/control"

# ------- Populate data -------
cp "${BUILD_DIR}/openwrt-travel-gui" "${IPK_DIR}/data/usr/bin/"
cp -r frontend/dist/* "${IPK_DIR}/data/www/openwrt-travel-gui/"
cp packaging/openwrt/files/etc/init.d/openwrt-travel-gui  "${IPK_DIR}/data/etc/init.d/"
cp packaging/openwrt/files/etc/config/openwrt-travel-gui  "${IPK_DIR}/data/etc/config/"
cp packaging/openwrt/files/etc/uci-defaults/99-travel-gui-ports "${IPK_DIR}/data/etc/uci-defaults/"

# ------- Control metadata -------
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

cat > "${IPK_DIR}/control/postinst" << 'SCRIPT'
#!/bin/sh
chmod +x /usr/bin/openwrt-travel-gui
chmod +x /etc/init.d/openwrt-travel-gui
/etc/init.d/openwrt-travel-gui enable
[ -f /etc/uci-defaults/99-travel-gui-ports ] && {
    sh /etc/uci-defaults/99-travel-gui-ports
    rm -f /etc/uci-defaults/99-travel-gui-ports
}
/etc/init.d/openwrt-travel-gui start
SCRIPT
chmod +x "${IPK_DIR}/control/postinst"

cat > "${IPK_DIR}/control/prerm" << 'SCRIPT'
#!/bin/sh
/etc/init.d/openwrt-travel-gui stop 2>/dev/null
/etc/init.d/openwrt-travel-gui disable 2>/dev/null
uci set uhttpd.main.listen_http='0.0.0.0:80'
uci set uhttpd.main.listen_https='0.0.0.0:443'
uci commit uhttpd
/etc/init.d/uhttpd restart
SCRIPT
chmod +x "${IPK_DIR}/control/prerm"

cat > "${IPK_DIR}/control/conffiles" << EOF
/etc/config/openwrt-travel-gui
EOF

# ------- Assemble .ipk (nested tar.gz) -------
cd "${IPK_DIR}"
echo "2.0" > debian-binary
(cd control && tar czf ../control.tar.gz .)
(cd data    && tar czf ../data.tar.gz .)
tar czf "../${PKG_NAME}_${VERSION}_${ARCH}.ipk" debian-binary control.tar.gz data.tar.gz
cd ../..

echo "==> Package created: ${BUILD_DIR}/${PKG_NAME}_${VERSION}_${ARCH}.ipk"
ls -lh "${BUILD_DIR}/${PKG_NAME}_${VERSION}_${ARCH}.ipk"
