#!/bin/sh
# Ensures initial AP state by: (1) remove all AP entries and commit, (2) add
# canonical APs (default_radio0, default_radio1) and commit, (3) verify UCI,
# (4) optionally apply via rpcd (session + ubus apply) so rollback is possible.
# Does not remove or modify STA (client) configs. Does NOT run "wifi" or "wifi up".
# If apply is used, host must call uci confirm within timeout. See AGENTS.md.
# Runs on the OpenWRT device (ash).
#
# Optional environment (set by setup-local.sh):
#   SETUP_WIFI_SSID — base SSID: non-5g radio uses this; 5g uses ${SETUP_WIFI_SSID}-5G
#   SETUP_WIFI_KEY  — WPA2 key for every AP (default remains travelrouter if unset)

set -e

log() { echo "[setup-wireless] $*" >&2; }

# Defaults matching backend/internal/services/ap_health.go
DEFAULT_COUNTRY="US"
DEFAULT_CHANNEL="auto"
DEFAULT_SSID_24="OpenWrt-Travel"
DEFAULT_SSID_5G="OpenWrt-Travel-5G"
DEFAULT_KEY="travelrouter"

# Optional overrides from setup-local.sh (env on device):
# SETUP_WIFI_SSID — base name: 2.4 GHz AP uses this, 5 GHz uses "${SETUP_WIFI_SSID}-5G"
# SETUP_WIFI_KEY    — WPA key shared by all APs (if unset, DEFAULT_KEY stays)
if [ -n "${SETUP_WIFI_SSID:-}" ]; then
  DEFAULT_SSID_24="$SETUP_WIFI_SSID"
  DEFAULT_SSID_5G="${SETUP_WIFI_SSID}-5G"
fi
if [ -n "${SETUP_WIFI_KEY:-}" ]; then
  DEFAULT_KEY="$SETUP_WIFI_KEY"
fi

# --- Phase 1: Remove all AP (mode=ap) entries, keep STA and wifi-device ---
log "Phase 1: finding AP sections to remove..."
_ap_sections=""
_tmp="/tmp/setup-wireless-$$"
uci show wireless 2>/dev/null >"$_tmp" || true
while read -r line; do
  case "$line" in
    wireless.*=wifi-iface)
      key="${line%=*}"
      sec="${key#wireless.}"
      mode=$(uci -q get "wireless.${sec}.mode" 2>/dev/null)
      if [ "$mode" = "ap" ]; then
        _ap_sections="${_ap_sections} ${sec}"
      fi
      ;;
  esac
done <"$_tmp"
rm -f "$_tmp"

if [ -z "$_ap_sections" ]; then
  log "Phase 1: no AP sections to remove."
else
  log "Phase 1: removing AP sections:$_ap_sections"
  for sec in $_ap_sections; do
    [ -z "$sec" ] && continue
    uci delete "wireless.${sec}"
  done
  uci commit wireless
  log "Phase 1: committed (no wifi command — apply via LuCI or reboot)."
fi

# --- Phase 2: Set radio defaults and add canonical APs ---
log "Phase 2: adding canonical APs (default_radio0, default_radio1)..."
for r in radio0 radio1; do
  if ! uci -q get "wireless.${r}.type" >/dev/null 2>&1; then
    continue
  fi
  uci set "wireless.${r}.country=${DEFAULT_COUNTRY}"
  uci set "wireless.${r}.channel=auto"
  uci set "wireless.${r}.disabled=0"

  sec="default_${r}"
  if ! uci -q get "wireless.${sec}.device" >/dev/null 2>&1; then
    uci add wireless wifi-iface 2>/dev/null
    uci rename "wireless.@wifi-iface[-1]=${sec}" 2>/dev/null
  fi
  uci set "wireless.${sec}.device=${r}"
  uci set "wireless.${sec}.mode=ap"
  uci set "wireless.${sec}.network=lan"
  uci set "wireless.${sec}.disabled=0"
  band=$(uci -q get "wireless.${r}.band" 2>/dev/null || true)
  if [ "$band" = "5g" ]; then
    uci set "wireless.${sec}.ssid=${DEFAULT_SSID_5G}"
  else
    uci set "wireless.${sec}.ssid=${DEFAULT_SSID_24}"
  fi
  uci set "wireless.${sec}.encryption=psk2"
  uci set "wireless.${sec}.key=${DEFAULT_KEY}"
done

log "Phase 2: committing wireless..."
uci commit wireless
log "Phase 2: wireless config (apply via LuCI Save & Apply or reboot):"
uci show wireless >&2
log "Phase 2: done. Not running 'wifi' (ath11k soft-brick risk, no rollback)."

# --- Phase 3: Verify UCI still has the AP entries (no wifi status; config may be discarded on reboot) ---
log "Phase 3: verifying AP entries in UCI..."
_missing=""
for sec in default_radio0 default_radio1; do
  if ! uci -q get "wireless.${sec}.device" >/dev/null 2>&1; then
    _missing="${_missing} ${sec}(no section)"
  elif [ "$(uci -q get "wireless.${sec}.mode" 2>/dev/null)" != "ap" ]; then
    _missing="${_missing} ${sec}(mode!=ap)"
  elif [ -z "$(uci -q get "wireless.${sec}.ssid" 2>/dev/null)" ]; then
    _missing="${_missing} ${sec}(no ssid)"
  fi
done
if [ -n "$_missing" ]; then
  log "ERROR: required AP entries missing or invalid:$_missing"
  exit 1
fi
log "Phase 3: UCI has default_radio0 and default_radio1 with mode=ap and ssid set."

# --- Phase 4: Apply with rollback (same as LuCI Save & Apply) ---
# Use ubus -S for single-line JSON so sed can extract SID. Session login (root, empty pass) works over SSH.
_apply_timeout=30
_sid="${UCI_APPLY_SID:-}"
if [ -z "$_sid" ]; then
  _out=$(ubus -S call session login '{"username":"root","password":""}' 2>/dev/null) || true
  if [ -n "$_out" ]; then
    _sid=$(echo "$_out" | sed 's/.*"ubus_rpc_session":"\([^"]*\)".*/\1/')
  fi
fi
if [ -z "$_sid" ]; then
  log "Phase 4: no session (use LuCI or reboot to apply). Apply via LuCI Save & Apply or reboot."
else
  _savedir="/var/run/rpcd/uci-${_sid}"
  mkdir -p "$_savedir"
  cp /etc/config/wireless /etc/config/network /etc/config/system "$_savedir/" 2>/dev/null || true
  if ubus call uci apply "{\"rollback\":true,\"timeout\":$_apply_timeout,\"ubus_rpc_session\":\"$_sid\"}" 2>/dev/null; then
    log "Phase 4: apply started (rollback in ${_apply_timeout}s if not confirmed)."
    echo "UCI_APPLY_SESSION=$_sid"
  else
    log "Phase 4: ubus apply failed. Apply via LuCI Save & Apply or reboot."
  fi
fi
log "Done."
