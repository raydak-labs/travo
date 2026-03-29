#!/bin/sh
# install-adguard.sh — Install and configure AdGuard Home on OpenWRT
# Usage: install-adguard.sh [--version VERSION]
#
# Safe to run multiple times (idempotent).

set -euo pipefail

ADGUARD_VERSION="0.107.54"
ADGUARD_DIR="/opt/AdGuardHome"
ADGUARD_BIN="${ADGUARD_DIR}/AdGuardHome"
ADGUARD_CONF="${ADGUARD_DIR}/AdGuardHome.yaml"
INIT_SCRIPT="/etc/init.d/adguardhome"

# ---------- helpers ----------
log()  { printf '[adguard-install] %s\n' "$*"; }
die()  { printf '[adguard-install] ERROR: %s\n' "$*" >&2; exit 1; }

# ---------- parse flags ----------
while [ $# -gt 0 ]; do
    case "$1" in
        --version)
            shift
            [ $# -gt 0 ] || die "--version requires an argument"
            ADGUARD_VERSION="$1"
            ;;
        -h|--help)
            echo "Usage: install-adguard.sh [--version VERSION]"
            exit 0
            ;;
        *)
            die "Unknown option: $1"
            ;;
    esac
    shift
done

# ---------- detect architecture ----------
detect_arch() {
    local machine
    machine="$(uname -m)"
    case "$machine" in
        aarch64)        echo "arm64"   ;;
        armv7l|armv6l)  echo "armv7"   ;;
        x86_64|amd64)   echo "amd64"   ;;
        i386|i686)      echo "386"     ;;
        mips)           echo "mipsle_softfloat" ;;
        mipsel)         echo "mipsle_softfloat" ;;
        *)              die "Unsupported architecture: $machine" ;;
    esac
}

# ---------- download & extract ----------
install_binary() {
    local arch="$1"
    local tarball="AdGuardHome_linux_${arch}.tar.gz"
    local url="https://github.com/AdguardTeam/AdGuardHome/releases/download/v${ADGUARD_VERSION}/${tarball}"

    if [ -x "$ADGUARD_BIN" ]; then
        local current_version
        current_version="$("$ADGUARD_BIN" --version 2>/dev/null || true)"
        if echo "$current_version" | grep -q "$ADGUARD_VERSION"; then
            log "AdGuard Home v${ADGUARD_VERSION} already installed — skipping download"
            return 0
        fi
        log "Upgrading AdGuard Home to v${ADGUARD_VERSION}..."
    else
        log "Installing AdGuard Home v${ADGUARD_VERSION} (${arch})..."
    fi

    local tmpdir
    tmpdir="$(mktemp -d)"
    trap "rm -rf '$tmpdir'" EXIT

    log "Downloading ${url} ..."
    if command -v curl >/dev/null 2>&1; then
        curl -fSL -o "${tmpdir}/${tarball}" "$url"
    elif command -v wget >/dev/null 2>&1; then
        wget -q -O "${tmpdir}/${tarball}" "$url"
    else
        die "Neither curl nor wget found — cannot download"
    fi

    log "Extracting to ${ADGUARD_DIR} ..."
    mkdir -p "$ADGUARD_DIR"
    tar -xzf "${tmpdir}/${tarball}" -C "${tmpdir}"
    cp "${tmpdir}/AdGuardHome/AdGuardHome" "$ADGUARD_BIN"
    chmod 0755 "$ADGUARD_BIN"

    rm -rf "$tmpdir"
    trap - EXIT
    log "Binary installed at ${ADGUARD_BIN}"
}

# ---------- initial config ----------
write_config() {
    if [ -f "$ADGUARD_CONF" ]; then
        log "Config already exists at ${ADGUARD_CONF} — skipping"
        return 0
    fi

    log "Writing initial config to ${ADGUARD_CONF} ..."
    local default_cfg="/etc/travo/adguardhome.yaml"
    if [ -f "$default_cfg" ]; then
        cp "$default_cfg" "$ADGUARD_CONF"
    else
        # Fallback: download from repo (standalone install without tarball)
        wget -qO "$ADGUARD_CONF" \
            "https://raw.githubusercontent.com/raydak-labs/travo/main/packaging/adguard/AdGuardHome.yaml" \
            || die "Failed to download AdGuard Home default config"
    fi
    log "Config written"
}

# ---------- point dnsmasq upstream at AdGuard (port 5353) ----------
# AdGuard listens on port 5353. dnsmasq stays on port 53 (serving DHCP clients)
# but forwards all DNS queries upstream to AdGuard on 127.0.0.1#5353.
configure_dnsmasq_upstream() {
    local current
    current="$(uci -q get dhcp.@dnsmasq[0].server 2>/dev/null || true)"
    if echo "$current" | grep -q "127.0.0.1#5353"; then
        log "dnsmasq upstream already points to AdGuard — skipping"
        return 0
    fi
    log "Configuring dnsmasq to forward DNS to AdGuard on 127.0.0.1#5353 ..."
    uci set dhcp.@dnsmasq[0].noresolv='1'
    uci -q delete dhcp.@dnsmasq[0].server 2>/dev/null || true
    uci add_list dhcp.@dnsmasq[0].server='127.0.0.1#5353'
    uci commit dhcp
    /etc/init.d/dnsmasq restart
    log "dnsmasq forwarding to AdGuard"
}

# ---------- procd init script ----------
install_init_script() {
    if [ -f "$INIT_SCRIPT" ]; then
        log "Init script already exists — overwriting"
    fi
    log "Creating procd init script at ${INIT_SCRIPT} ..."
    cat > "$INIT_SCRIPT" <<'INITEOF'
#!/bin/sh /etc/rc.common

START=99
STOP=10
USE_PROCD=1

start_service() {
    procd_open_instance
    procd_set_param command /opt/AdGuardHome/AdGuardHome \
        -c /opt/AdGuardHome/AdGuardHome.yaml \
        -w /opt/AdGuardHome \
        --no-check-update
    procd_set_param respawn ${respawn_threshold:-3600} ${respawn_timeout:-5} ${respawn_retry:-5}
    procd_set_param stdout 1
    procd_set_param stderr 1
    procd_set_param pidfile /var/run/adguardhome.pid
    procd_close_instance
}

stop_service() {
    :
}

service_triggers() {
    procd_add_reload_trigger "adguardhome"
}
INITEOF
    chmod 0755 "$INIT_SCRIPT"
    log "Init script installed"
}

# ---------- enable & start ----------
enable_and_start() {
    log "Enabling adguardhome service ..."
    "$INIT_SCRIPT" enable
    log "Starting adguardhome service ..."
    "$INIT_SCRIPT" start
    log "AdGuard Home is running — web UI at http://<router-ip>:3000"
    log "Default credentials: admin / password  (change immediately via Settings → AdGuard Password)"
}

# ---------- configure router DNS ----------
configure_network_dns() {
    local current_peerdns
    current_peerdns="$(uci -q get network.wan.peerdns 2>/dev/null || echo '1')"
    local current_dns
    current_dns="$(uci -q get network.wan.dns 2>/dev/null || echo '')"

    if [ "$current_peerdns" = "0" ] && [ "$current_dns" = "127.0.0.1" ]; then
        log "Network DNS already configured — skipping"
        return 0
    fi
    log "Setting router DNS to 127.0.0.1 (through AdGuard) ..."
    uci set network.wan.peerdns='0'
    uci set network.wan.dns='127.0.0.1'
    uci commit network
    log "Network DNS configured"
}

# ---------- main ----------
main() {
    log "=== AdGuard Home Installer for OpenWRT ==="
    log "Version: ${ADGUARD_VERSION}"

    local arch
    arch="$(detect_arch)"
    log "Detected architecture: ${arch}"

    install_binary "$arch"
    write_config
    install_init_script
    configure_dnsmasq_upstream
    configure_network_dns
    enable_and_start

    log "=== Installation complete ==="
}

main
