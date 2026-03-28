#!/bin/sh
# install.sh — Install Travo on a fresh OpenWRT system
#
# Usage:
#   wget -O- https://raw.githubusercontent.com/raydak-labs/travo/main/scripts/install.sh | sh
#   sh install.sh [OPTIONS]
#
# POSIX sh compatible — works with busybox ash on OpenWRT.

set -eu

# ============================================================
# Configuration — override these with environment variables
# ============================================================
GITHUB_REPO="${GITHUB_REPO:-raydak-labs/travo}"
GITHUB_RAW_BASE="https://raw.githubusercontent.com/${GITHUB_REPO}/main"
GITHUB_API_BASE="https://api.github.com/repos/${GITHUB_REPO}"
GITHUB_RELEASE_BASE="https://github.com/${GITHUB_REPO}/releases/download"
PKG_NAME="travo"
MIN_SPACE_KB=20480  # 20 MB

# ============================================================
# Defaults
# ============================================================
VERSION="latest"
PASSWORD=""
INSTALL_ADGUARD=1
MOVE_LUCI=1
UNINSTALL=0
YES=0
SHOW_HELP=0

# ============================================================
# Colors & output helpers
# ============================================================
if [ -t 1 ]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[1;33m'
    BLUE='\033[0;34m'
    BOLD='\033[1m'
    NC='\033[0m'
else
    RED=''
    GREEN=''
    YELLOW=''
    BLUE=''
    BOLD=''
    NC=''
fi

info()    { printf "${BLUE}[info]${NC} %s\n" "$*"; }
success() { printf "${GREEN}[ok]${NC} %s\n" "$*"; }
warn()    { printf "${YELLOW}[warn]${NC} %s\n" "$*" >&2; }
error()   { printf "${RED}[error]${NC} %s\n" "$*" >&2; }
die()     { error "$*"; exit 1; }

# ============================================================
# Cleanup trap
# ============================================================
CLEANUP_FILES=""

cleanup() {
    if [ -n "$CLEANUP_FILES" ]; then
        rm -f $CLEANUP_FILES 2>/dev/null || true
    fi
}
trap cleanup EXIT INT TERM

add_cleanup() {
    CLEANUP_FILES="$CLEANUP_FILES $1"
}

# ============================================================
# Parse arguments
# ============================================================
while [ $# -gt 0 ]; do
    case "$1" in
        --version)
            shift
            [ $# -gt 0 ] || die "--version requires an argument"
            VERSION="$1"
            ;;
        --password)
            shift
            [ $# -gt 0 ] || die "--password requires an argument"
            PASSWORD="$1"
            ;;
        --no-adguard)
            INSTALL_ADGUARD=0
            ;;
        --no-luci-move)
            MOVE_LUCI=0
            ;;
        --uninstall)
            UNINSTALL=1
            ;;
        --yes|-y)
            YES=1
            ;;
        --help|-h)
            SHOW_HELP=1
            ;;
        *)
            die "Unknown option: $1 (use --help for usage)"
            ;;
    esac
    shift
done

# ============================================================
# Usage
# ============================================================
show_help() {
    cat <<'EOF'
Travo — Installer

Usage: install.sh [OPTIONS]

Options:
  --version VERSION   Install a specific version (default: latest)
  --password PASSWORD Set the admin password (default: prompt or "admin")
  --no-adguard        Skip AdGuard Home installation
  --no-luci-move      Skip moving LuCI to port 8080
  --uninstall         Remove Travo and restore defaults
  --yes, -y           Skip confirmation prompts
  --help, -h          Show this help message

Examples:
  # Interactive install (latest version)
  sh install.sh

  # Non-interactive install with a specific version and password
  sh install.sh --yes --version 1.0.0 --password mysecret

  # Install without AdGuard Home
  sh install.sh --no-adguard

  # Piped install (non-interactive, uses defaults)
  wget -O- https://raw.githubusercontent.com/raydak-labs/travo/main/scripts/install.sh | sh

  # Uninstall everything
  sh install.sh --uninstall
EOF
    exit 0
}

[ "$SHOW_HELP" = 1 ] && show_help

# ============================================================
# Utility functions
# ============================================================

# Download a URL to a file; works with wget or curl
download() {
    _url="$1"
    _dest="$2"
    if command -v wget >/dev/null 2>&1; then
        wget -q -O "$_dest" "$_url" 2>/dev/null
    elif command -v curl >/dev/null 2>&1; then
        curl -fsSL -o "$_dest" "$_url"
    else
        die "Neither wget nor curl found — cannot download files"
    fi
}

# Download URL contents to stdout
download_stdout() {
    _url="$1"
    if command -v wget >/dev/null 2>&1; then
        wget -q -O- "$_url" 2>/dev/null
    elif command -v curl >/dev/null 2>&1; then
        curl -fsSL "$_url"
    else
        die "Neither wget nor curl found — cannot download files"
    fi
}

# Prompt for yes/no confirmation (defaults to yes)
confirm() {
    [ "$YES" = 1 ] && return 0
    # If stdin is not a terminal (piped), assume yes
    if [ ! -t 0 ]; then
        info "Non-interactive mode detected — proceeding automatically"
        return 0
    fi
    printf "%s [Y/n] " "$1"
    read -r _answer </dev/tty || _answer="y"
    case "$_answer" in
        [nN]*) return 1 ;;
        *) return 0 ;;
    esac
}

# Detect the router's LAN IP address
detect_lan_ip() {
    _ip=""
    if command -v uci >/dev/null 2>&1; then
        _ip="$(uci -q get network.lan.ipaddr 2>/dev/null || true)"
    fi
    if [ -z "$_ip" ]; then
        _ip="192.168.1.1"
    fi
    echo "$_ip"
}

# Detect machine architecture and map to .ipk architecture string
detect_arch() {
    _machine="$(uname -m)"
    case "$_machine" in
        aarch64)       echo "aarch64_cortex-a53" ;;
        x86_64|amd64)  echo "x86_64" ;;
        *)             die "Unsupported architecture: $_machine (supported: aarch64, x86_64)" ;;
    esac
}

# Detect machine architecture for display
detect_arch_short() {
    _machine="$(uname -m)"
    case "$_machine" in
        aarch64)       echo "aarch64" ;;
        x86_64|amd64)  echo "x86_64" ;;
        *)             echo "$_machine" ;;
    esac
}

# Resolve "latest" version from GitHub API
resolve_version() {
    if [ "$VERSION" = "latest" ]; then
        info "Fetching latest release version..."
        _json="$(download_stdout "${GITHUB_API_BASE}/releases/latest" 2>/dev/null || true)"
        if [ -n "$_json" ]; then
            # Parse tag_name from JSON without jq (POSIX-safe)
            _tag="$(echo "$_json" | tr ',' '\n' | while IFS= read -r _line; do
                case "$_line" in
                    *'"tag_name"'*)
                        # Extract value: "tag_name": "v1.0.0" or "tag_name":"v1.0.0"
                        echo "$_line" | sed 's/.*"tag_name"[[:space:]]*:[[:space:]]*"//;s/".*//'
                        break
                        ;;
                esac
            done)"
            if [ -n "$_tag" ]; then
                # Strip leading 'v' if present
                VERSION="$(echo "$_tag" | sed 's/^v//')"
                info "Latest version: ${VERSION}"
                return 0
            fi
        fi
        die "Could not determine latest version from GitHub. Use --version to specify one."
    fi
}

# ============================================================
# Pre-flight checks
# ============================================================
preflight_checks() {
    info "Running pre-flight checks..."

    # 1. Verify OpenWRT
    if [ ! -f /etc/openwrt_release ]; then
        die "This script must be run on OpenWRT (/etc/openwrt_release not found)"
    fi
    success "Running on OpenWRT"

    # 2. Verify architecture
    _arch_short="$(detect_arch_short)"
    case "$_arch_short" in
        aarch64|x86_64) ;;
        *) die "Unsupported architecture: $_arch_short (supported: aarch64, x86_64)" ;;
    esac
    success "Architecture: $_arch_short"

    # 3. Verify internet connectivity
    _connected=0
    if download_stdout "https://github.com" >/dev/null 2>&1; then
        _connected=1
    elif ping -c 1 -W 3 github.com >/dev/null 2>&1; then
        _connected=1
    fi
    if [ "$_connected" = 0 ]; then
        die "No internet connectivity — cannot reach github.com"
    fi
    success "Internet connectivity OK"

    # 4. Check available storage
    _avail_kb="$(df /tmp 2>/dev/null | tail -1 | awk '{print $4}')"
    if [ -n "$_avail_kb" ] && [ "$_avail_kb" -lt "$MIN_SPACE_KB" ] 2>/dev/null; then
        die "Insufficient storage: ${_avail_kb}KB available, need at least ${MIN_SPACE_KB}KB"
    fi
    success "Storage: ${_avail_kb:-unknown}KB available"

    # 5. Detect package manager (OpenWrt 25+ uses apk, older uses opkg)
    if command -v apk >/dev/null 2>&1; then
        PKG_MGR="apk"
    elif command -v opkg >/dev/null 2>&1; then
        PKG_MGR="opkg"
    else
        die "No package manager found (apk or opkg) — is this a standard OpenWRT installation?"
    fi
    success "Package manager: ${PKG_MGR}"
}

# ============================================================
# Install flow
# ============================================================
do_install() {
    preflight_checks

    _arch="$(detect_arch)"
    resolve_version

    # Show summary
    echo ""
    printf "${BOLD}=== Travo Installer ===${NC}\n"
    echo ""
    info "Version:       ${VERSION}"
    info "Architecture:  ${_arch}"
    info "AdGuard Home:  $([ "$INSTALL_ADGUARD" = 1 ] && echo 'yes' || echo 'skip')"
    info "Move LuCI:     $([ "$MOVE_LUCI" = 1 ] && echo 'yes (to port 8080)' || echo 'skip')"
    echo ""

    confirm "Proceed with installation?" || { info "Aborted."; exit 0; }

    # --- Step 1: Download .ipk ---
    _ipk_file="/tmp/${PKG_NAME}_${VERSION}_${_arch}.ipk"
    _ipk_url="${GITHUB_RELEASE_BASE}/v${VERSION}/${PKG_NAME}_${VERSION}_${_arch}.ipk"

    info "Downloading ${PKG_NAME} v${VERSION}..."
    download "$_ipk_url" "$_ipk_file" || die "Failed to download .ipk from ${_ipk_url}"
    add_cleanup "$_ipk_file"

    if [ ! -s "$_ipk_file" ]; then
        die "Downloaded .ipk file is empty — check version and architecture"
    fi
    success "Downloaded: $_ipk_file"

    # --- Step 2: Install .ipk ---
    info "Installing package..."
    if [ "$PKG_MGR" = "apk" ]; then
        apk add --allow-untrusted "$_ipk_file" || die "apk install failed"
    else
        opkg install "$_ipk_file" || die "opkg install failed"
    fi
    success "Package installed"

    # --- Step 3: Set password ---
    _pw="$PASSWORD"
    if [ -z "$_pw" ]; then
        if [ -t 0 ] && [ "$YES" = 0 ]; then
            printf "Enter admin password (default: admin): "
            read -r _pw </dev/tty || _pw=""
        fi
        [ -z "$_pw" ] && _pw="admin"
    fi
    uci set "${PKG_NAME}.main.password=${_pw}"
    uci commit "$PKG_NAME"
    success "Admin password configured"

    # --- Step 4: Move LuCI to port 8080 ---
    if [ "$MOVE_LUCI" = 1 ]; then
        _current_http="$(uci -q get uhttpd.main.listen_http 2>/dev/null || echo '')"
        case "$_current_http" in
            *8080*) info "LuCI already on port 8080 — skipping" ;;
            *)
                info "Moving LuCI to port 8080/8443..."
                uci set uhttpd.main.listen_http='0.0.0.0:8080'
                uci set uhttpd.main.listen_https='0.0.0.0:8443'
                uci commit uhttpd
                /etc/init.d/uhttpd restart 2>/dev/null || true
                success "LuCI moved to port 8080"
                ;;
        esac
    fi

    # --- Step 5: Install AdGuard Home ---
    if [ "$INSTALL_ADGUARD" = 1 ]; then
        info "Installing AdGuard Home..."
        _adguard_script="/tmp/install-adguard.sh"
        download "${GITHUB_RAW_BASE}/scripts/install-adguard.sh" "$_adguard_script" \
            || die "Failed to download install-adguard.sh"
        add_cleanup "$_adguard_script"
        chmod +x "$_adguard_script"
        sh "$_adguard_script" || die "AdGuard Home installation failed"
        success "AdGuard Home installed"
    fi

    # --- Step 6: Start travel GUI ---
    info "Enabling and starting ${PKG_NAME}..."
    /etc/init.d/"$PKG_NAME" enable 2>/dev/null || true
    /etc/init.d/"$PKG_NAME" start  2>/dev/null || true
    success "${PKG_NAME} is running"

    # --- Step 7: Print success ---
    _lan_ip="$(detect_lan_ip)"
    echo ""
    printf "${GREEN}${BOLD}"
    echo "============================================"
    echo "        Travo installed successfully!"
    echo "============================================"
    printf "${NC}\n"
    echo ""
    printf "  ${BLUE}Travo:${NC}         http://${_lan_ip}\n"
    if [ "$_pw" != "admin" ]; then
        printf "  ${BLUE}Password:${NC}      (as configured)\n"
    else
        printf "  ${YELLOW}Password:${NC}      admin ${YELLOW}(change it in settings!)${NC}\n"
    fi
    if [ "$INSTALL_ADGUARD" = 1 ]; then
        printf "  ${BLUE}AdGuard Home:${NC}  http://${_lan_ip}:3000\n"
    fi
    if [ "$MOVE_LUCI" = 1 ]; then
        printf "  ${BLUE}LuCI (legacy):${NC} http://${_lan_ip}:8080\n"
    fi
    echo ""
    info "Enjoy your travel router!"
    echo ""
}

# ============================================================
# Uninstall flow
# ============================================================
do_uninstall() {
    printf "${BOLD}=== Travo Uninstaller ===${NC}\n"
    echo ""

    confirm "Remove Travo and restore defaults?" || { info "Aborted."; exit 0; }

    # --- Step 1: Stop and disable travel-gui ---
    if [ -x "/etc/init.d/${PKG_NAME}" ]; then
        info "Stopping ${PKG_NAME}..."
        /etc/init.d/"$PKG_NAME" stop 2>/dev/null || true
        /etc/init.d/"$PKG_NAME" disable 2>/dev/null || true
        success "Service stopped and disabled"
    else
        info "${PKG_NAME} service not found — skipping"
    fi

    # --- Step 2: Remove AdGuard Home ---
    if [ -d "/opt/AdGuardHome" ] || [ -x "/etc/init.d/adguardhome" ]; then
        info "Removing AdGuard Home..."
        _remove_script="/tmp/remove-adguard.sh"
        download "${GITHUB_RAW_BASE}/scripts/remove-adguard.sh" "$_remove_script" 2>/dev/null || true
        if [ -s "$_remove_script" ]; then
            add_cleanup "$_remove_script"
            chmod +x "$_remove_script"
            sh "$_remove_script" || warn "AdGuard Home removal had errors (continuing)"
        else
            warn "Could not download remove-adguard.sh — skipping AdGuard removal"
        fi
    else
        info "AdGuard Home not installed — skipping"
    fi

    # --- Step 3: Remove package ---
    if command -v apk >/dev/null 2>&1 && apk info "$PKG_NAME" >/dev/null 2>&1; then
        info "Removing ${PKG_NAME} package..."
        apk del "$PKG_NAME" || warn "apk del had errors"
        success "Package removed"
    elif command -v opkg >/dev/null 2>&1 && opkg status "$PKG_NAME" 2>/dev/null | head -1 | grep -q "Package:"; then
        info "Removing ${PKG_NAME} package..."
        opkg remove "$PKG_NAME" || warn "opkg remove had errors"
        success "Package removed"
    else
        info "${PKG_NAME} package not installed — skipping"
    fi

    # --- Step 4: Restore uhttpd to default ports ---
    _current_http="$(uci -q get uhttpd.main.listen_http 2>/dev/null || echo '')"
    case "$_current_http" in
        *8080*)
            info "Restoring uhttpd to ports 80/443..."
            uci set uhttpd.main.listen_http='0.0.0.0:80'
            uci set uhttpd.main.listen_https='0.0.0.0:443'
            uci commit uhttpd
            /etc/init.d/uhttpd restart 2>/dev/null || true
            success "uhttpd restored to default ports"
            ;;
        *)
            info "uhttpd already on default ports — skipping"
            ;;
    esac

    # --- Step 5: Clean up config ---
    uci -q delete "$PKG_NAME" 2>/dev/null || true
    uci commit "$PKG_NAME" 2>/dev/null || true

    # Clean leftover files
    rm -rf /www/travo 2>/dev/null || true
    rm -f /etc/config/travo 2>/dev/null || true

    echo ""
    printf "${GREEN}${BOLD}"
    echo "============================================"
    echo " Travo removed successfully!"
    echo "============================================"
    printf "${NC}\n"
    echo ""
    info "LuCI is available at http://$(detect_lan_ip)"
    echo ""
}

# ============================================================
# Main
# ============================================================
if [ "$UNINSTALL" = 1 ]; then
    do_uninstall
else
    do_install
fi
