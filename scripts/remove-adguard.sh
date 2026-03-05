#!/bin/sh
# remove-adguard.sh — Cleanly remove AdGuard Home and restore dnsmasq DNS on OpenWRT
#
# Safe to run multiple times (idempotent).

set -euo pipefail

ADGUARD_DIR="/opt/AdGuardHome"
INIT_SCRIPT="/etc/init.d/adguardhome"

# ---------- helpers ----------
log()  { printf '[adguard-remove] %s\n' "$*"; }
die()  { printf '[adguard-remove] ERROR: %s\n' "$*" >&2; exit 1; }

# ---------- stop & disable service ----------
stop_service() {
    if [ -x "$INIT_SCRIPT" ]; then
        log "Stopping adguardhome service ..."
        "$INIT_SCRIPT" stop 2>/dev/null || true
        log "Disabling adguardhome service ..."
        "$INIT_SCRIPT" disable 2>/dev/null || true
    else
        log "Init script not found — nothing to stop"
    fi
}

# ---------- remove init script ----------
remove_init_script() {
    if [ -f "$INIT_SCRIPT" ]; then
        log "Removing init script ${INIT_SCRIPT} ..."
        rm -f "$INIT_SCRIPT"
    else
        log "Init script already removed — skipping"
    fi
}

# ---------- remove installation directory ----------
remove_files() {
    if [ -d "$ADGUARD_DIR" ]; then
        log "Removing ${ADGUARD_DIR} ..."
        rm -rf "$ADGUARD_DIR"
    else
        log "Installation directory already removed — skipping"
    fi
}

# ---------- restore dnsmasq DNS ----------
restore_dnsmasq() {
    local current_port
    current_port="$(uci -q get dhcp.@dnsmasq[0].port 2>/dev/null || echo '53')"
    if [ "$current_port" = "53" ]; then
        log "dnsmasq DNS already on port 53 — skipping"
        return 0
    fi
    log "Restoring dnsmasq DNS listener (port 53) ..."
    uci set dhcp.@dnsmasq[0].port='53'
    uci commit dhcp
    /etc/init.d/dnsmasq restart
    log "dnsmasq restored"
}

# ---------- restore network DNS ----------
restore_network_dns() {
    local current_peerdns
    current_peerdns="$(uci -q get network.wan.peerdns 2>/dev/null || echo '1')"
    if [ "$current_peerdns" = "1" ]; then
        log "Network DNS already using upstream — skipping"
        return 0
    fi
    log "Restoring network DNS to upstream (peerdns=1) ..."
    uci set network.wan.peerdns='1'
    uci -q delete network.wan.dns 2>/dev/null || true
    uci commit network
    log "Network DNS restored"
}

# ---------- main ----------
main() {
    log "=== AdGuard Home Removal for OpenWRT ==="

    stop_service
    remove_init_script
    remove_files
    restore_dnsmasq
    restore_network_dns

    log "=== Removal complete — dnsmasq is your DNS server again ==="
}

main
