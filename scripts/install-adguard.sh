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
    cat > "$ADGUARD_CONF" <<'YAML'
http:
  pprof:
    port: 6060
    enabled: false
  address: 0.0.0.0:3000
  session_ttl: 720h
users: []
auth_attempts: 5
block_auth_min: 15
http_proxy: ""
language: ""
theme: auto
dns:
  bind_hosts:
    - 0.0.0.0
  port: 53
  anonymize_client_ip: false
  ratelimit: 0
  ratelimit_subnet_len_ipv4: 24
  ratelimit_subnet_len_ipv6: 56
  ratelimit_whitelist: []
  refuse_any: true
  upstream_dns:
    - https://dns.cloudflare.com/dns-query
    - https://dns.google/dns-query
  upstream_dns_file: ""
  bootstrap_dns:
    - 1.1.1.1
    - 8.8.8.8
  fallback_dns: []
  upstream_mode: load_balance
  fastest_timeout: 1s
  allowed_clients: []
  disallowed_clients: []
  blocked_hosts:
    - version.bind
    - id.server
    - hostname.bind
  trusted_proxies:
    - 127.0.0.0/8
    - ::1/128
  cache_size: 4194304
  cache_ttl_min: 0
  cache_ttl_max: 0
  cache_optimistic: true
  bogus_nxdomain: []
  aaaa_disabled: false
  enable_dnssec: false
  edns_client_subnet:
    custom_ip: ""
    enabled: false
    use_custom: false
  max_goroutines: 300
  handle_ddr: true
  ipset: []
  ipset_file: ""
  bootstrap_prefer_ipv6: false
  upstream_timeout: 10s
  private_networks: []
  use_private_ptr_resolvers: true
  local_ptr_upstreams: []
  use_dns64: false
  dns64_prefixes: []
  serve_http3: false
  use_http3_upstreams: false
  serve_plain_dns: true
  hostsfile_enabled: true
tls:
  enabled: false
  server_name: ""
  force_https: false
  port_https: 443
  port_dns_over_tls: 853
  port_dns_over_quic: 853
  port_dnscrypt: 0
  dnscrypt_config_file: ""
  allow_unencrypted_doh: false
  certificate_chain: ""
  private_key: ""
  certificate_path: ""
  private_key_path: ""
  strict_sni_check: false
querylog:
  dir_path: ""
  ignored: []
  interval: 24h
  size_memory: 1000
  enabled: true
  file_enabled: true
statistics:
  dir_path: ""
  ignored: []
  interval: 168h
  enabled: true
filters:
  - enabled: true
    url: https://adguardteam.github.io/HostlistsRegistry/assets/filter_1.txt
    name: AdGuard DNS filter
    id: 1
  - enabled: true
    url: https://adguardteam.github.io/HostlistsRegistry/assets/filter_2.txt
    name: AdAway Default Blocklist
    id: 2
dhcp:
  enabled: false
  interface_name: ""
  local_domain_name: lan
  dhcpv4:
    gateway_ip: ""
    subnet_mask: ""
    range_start: ""
    range_end: ""
    lease_duration: 86400
    icmp_timeout_msec: 1000
    options: []
  dhcpv6:
    range_start: ""
    lease_duration: 86400
    ra_slaac_only: false
    ra_allow_slaac: false
filtering:
  blocking_ipv4: ""
  blocking_ipv6: ""
  blocked_services:
    schedule:
      time_zone: UTC
    ids: []
  protection_disabled_until: null
  safe_browsing_enabled: true
  safe_browsing_cache_size: 1048576
  safesearch_cache_size: 1048576
  parental_cache_size: 1048576
  parental_enabled: false
  safesearch:
    enabled: false
    bing: true
    duckduckgo: true
    ecosia: true
    google: true
    pixabay: true
    yandex: true
    youtube: true
  blocking_mode: default
  rewrites: []
clients:
  runtime_sources:
    whois: true
    arp: true
    rdns: true
    dhcp: true
    hosts: true
  persistent: []
log:
  file: ""
  max_backups: 0
  max_size: 100
  max_age: 3
  compress: false
  local_time: false
  verbose: false
os:
  group: ""
  user: ""
  rlimit_nofile: 0
schema_version: 29
YAML
    log "Config written"
}

# ---------- disable dnsmasq DNS (keep DHCP) ----------
disable_dnsmasq_dns() {
    local current_port
    current_port="$(uci -q get dhcp.@dnsmasq[0].port 2>/dev/null || echo '53')"
    if [ "$current_port" = "0" ]; then
        log "dnsmasq DNS already disabled — skipping"
        return 0
    fi
    log "Disabling dnsmasq DNS listener (setting port=0) ..."
    uci set dhcp.@dnsmasq[0].port='0'
    uci commit dhcp
    /etc/init.d/dnsmasq restart
    log "dnsmasq restarted (DHCP only)"
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
    disable_dnsmasq_dns
    configure_network_dns
    enable_and_start

    log "=== Installation complete ==="
}

main
