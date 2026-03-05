# Deployment, Packaging, AdGuard & CI/CD Plan

## Summary

Fix the broken deployment pipeline end-to-end: backend binary flags, AdGuard Home integration, one-liner install script, GitHub Actions CI/CD, and packaging correctness.

---

## Phase 1: Fix Backend Binary (CLI flags, version, JWT persistence)

**Goal:** The init.d script passes `--port`, `--static-dir`, `--password` as CLI flags, but the Go binary only reads env vars. Fix so both work, with CLI flags taking precedence.

### 1.1 Add `Version` variable to `main.go`

**File:** `backend/cmd/server/main.go`

**Change:** Add a package-level `Version` variable that `ldflags` can inject:

```go
// Near the top of main.go, after imports:
var Version = "dev"
```

Add version logging in `main()`:

```go
func main() {
    log.Printf("openwrt-travel-gui %s", Version)
    cfg := config.LoadConfig()
    // ... rest unchanged
}
```

### 1.2 Add CLI flag parsing to `config.go`

**File:** `backend/internal/config/config.go`

**Change:** Add a new function `LoadConfig()` that reads defaults → env vars → CLI flags (later source overrides earlier):

```go
import (
    "flag"
    // ... existing imports
)

// LoadConfig reads configuration from defaults, then env vars, then CLI flags.
// Later sources override earlier ones.
func LoadConfig() Config {
    cfg := LoadConfigFromEnv()

    // Define CLI flags with env-loaded values as defaults
    port := flag.Int("port", cfg.Port, "Server port")
    password := flag.String("password", cfg.Password, "Admin password")
    staticDir := flag.String("static-dir", cfg.StaticDir, "Path to static frontend files")
    jwtSecret := flag.String("jwt-secret", cfg.JWTSecret, "JWT signing secret")
    mockMode := flag.Bool("mock", cfg.MockMode, "Enable mock mode")
    corsOrigins := flag.String("cors-origins", cfg.CorsOrigins, "CORS allowed origins")

    flag.Parse()

    cfg.Port = *port
    cfg.Password = *password
    cfg.StaticDir = *staticDir
    cfg.JWTSecret = *jwtSecret
    cfg.MockMode = *mockMode
    cfg.CorsOrigins = *corsOrigins

    return cfg
}
```

Keep `LoadConfigFromEnv()` as-is (for test compatibility and docker usage).

### 1.3 Update `main.go` to use `LoadConfig()`

**File:** `backend/cmd/server/main.go`

**Change:** Replace `config.LoadConfigFromEnv()` with `config.LoadConfig()`:

```go
func main() {
    log.Printf("openwrt-travel-gui %s", Version)
    cfg := config.LoadConfig()
    // ... rest stays the same
}
```

### 1.4 Add `jwt_secret` to UCI config with persistence

**File:** `packaging/openwrt/files/etc/config/openwrt-travel-gui`

**Change:** Add `jwt_secret` option:

```
config travel_gui 'main'
	option enabled '1'
	option port '80'
	option password 'admin'
	option jwt_secret ''
```

### 1.5 Fix init.d script to pass `--jwt-secret` and generate on first boot

**File:** `packaging/openwrt/files/etc/init.d/openwrt-travel-gui`

**Change:** Read `jwt_secret` from UCI, generate one on first boot if empty, and pass all flags:

```sh
#!/bin/sh /etc/rc.common
# openwrt-travel-gui init script (procd)

START=99
STOP=10

USE_PROCD=1
PROG=/usr/bin/openwrt-travel-gui

generate_secret() {
    head -c 32 /dev/urandom | hexdump -e '32/1 "%02x"' 2>/dev/null || \
    cat /proc/sys/kernel/random/uuid | tr -d '-'
}

start_service() {
    local enabled port password jwt_secret

    config_load openwrt-travel-gui
    config_get enabled main enabled '1'
    config_get port main port '80'
    config_get password main password 'admin'
    config_get jwt_secret main jwt_secret ''

    [ "$enabled" = "0" ] && return 0

    # Generate and persist JWT secret on first run
    if [ -z "$jwt_secret" ]; then
        jwt_secret=$(generate_secret)
        uci set openwrt-travel-gui.main.jwt_secret="$jwt_secret"
        uci commit openwrt-travel-gui
    fi

    procd_open_instance
    procd_set_param command "$PROG"
    procd_append_param command --port "$port"
    procd_append_param command --static-dir /www/openwrt-travel-gui
    procd_append_param command --password "$password"
    procd_append_param command --jwt-secret "$jwt_secret"
    procd_set_param respawn
    procd_set_param stdout 1
    procd_set_param stderr 1
    procd_close_instance
}
```

### 1.6 Add tests for `LoadConfig()`

**File:** `backend/internal/config/config_test.go`

**Change:** Add test cases for CLI flag parsing. Note: `flag.Parse()` uses `os.Args`, so test via `flag.CommandLine` reset pattern:

```go
func TestLoadConfig_CLIOverridesEnv(t *testing.T) {
    // Reset flags for test isolation
    os.Args = []string{"cmd", "--port", "9999", "--password", "clipass"}
    flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

    // Set env vars that should be overridden
    os.Setenv("PORT", "1111")
    os.Setenv("PASSWORD", "envpass")
    defer func() {
        os.Unsetenv("PORT")
        os.Unsetenv("PASSWORD")
    }()

    cfg := LoadConfig()
    if cfg.Port != 9999 {
        t.Errorf("expected CLI port 9999, got %d", cfg.Port)
    }
    if cfg.Password != "clipass" {
        t.Errorf("expected CLI password 'clipass', got %q", cfg.Password)
    }
}
```

### 1.7 Update `build.sh` ldflags

**File:** `scripts/build.sh`

**Change:** Fix ldflags to inject into the correct package path:

```bash
# Current (broken — main.Version doesn't exist yet as a var):
-ldflags="-s -w -X main.Version=${VERSION}"

# This is already correct once we add `var Version` in main.go.
# No change needed to build.sh — just confirming the ldflags target is `main.Version`.
```

No change to `build.sh` needed — the `-X main.Version=${VERSION}` already targets the right variable once Phase 1.1 adds it.

---

## Phase 2: AdGuard Home Integration

**Goal:** Provide scripts to install/configure AdGuard Home alongside the Travel GUI. AdGuard UI on port 3000, DNS on port 53.

### 2.1 Create AdGuard Home install helper script

**File:** `scripts/install-adguard.sh` (new)

This script runs on the OpenWRT device:

```sh
#!/bin/sh
# Install and configure AdGuard Home on OpenWRT
set -e

ADGUARD_VERSION="${ADGUARD_VERSION:-v0.107.52}"
ARCH="${1:-linux_arm64}"

echo "Installing AdGuard Home ${ADGUARD_VERSION} (${ARCH})..."

# Download
cd /tmp
wget -q "https://github.com/AdguardTeam/AdGuardHome/releases/download/${ADGUARD_VERSION}/AdGuardHome_${ARCH}.tar.gz" \
    -O adguardhome.tar.gz
tar xzf adguardhome.tar.gz
mv AdGuardHome/AdGuardHome /usr/bin/AdGuardHome
chmod +x /usr/bin/AdGuardHome
rm -rf /tmp/AdGuardHome /tmp/adguardhome.tar.gz

# Create working directory
mkdir -p /etc/adguardhome

# Create initial config
cat > /etc/adguardhome/AdGuardHome.yaml << 'EOF'
http:
  pprof:
    port: 6060
    enabled: false
  address: 0.0.0.0:3000
  session_ttl: 720h
dns:
  bind_hosts:
    - 0.0.0.0
  port: 53
  upstream_dns:
    - https://dns.cloudflare.com/dns-query
    - https://dns.google/dns-query
  bootstrap_dns:
    - 1.1.1.1
    - 8.8.8.8
  filtering_enabled: true
  protection_enabled: true
filters:
  - enabled: true
    url: https://adguardteam.github.io/HostlistsRegistry/assets/filter_1.txt
    name: AdGuard DNS filter
    id: 1
  - enabled: true
    url: https://adguardteam.github.io/HostlistsRegistry/assets/filter_2.txt
    name: AdAway Default Blocklist
    id: 2
user_rules: []
schema_version: 28
EOF

# Create procd init script
cat > /etc/init.d/adguardhome << 'INITEOF'
#!/bin/sh /etc/rc.common
START=95
STOP=15
USE_PROCD=1

start_service() {
    procd_open_instance
    procd_set_param command /usr/bin/AdGuardHome -c /etc/adguardhome/AdGuardHome.yaml -w /etc/adguardhome --no-check-update
    procd_set_param respawn
    procd_set_param stdout 1
    procd_set_param stderr 1
    procd_close_instance
}
INITEOF
chmod +x /etc/init.d/adguardhome

# Disable dnsmasq DNS (let AdGuard handle port 53)
# Keep dnsmasq for DHCP only
uci set dhcp.@dnsmasq[0].port='0'
uci commit dhcp
/etc/init.d/dnsmasq restart

# Point the router's own DNS resolution to AdGuard
uci set network.lan.dns='127.0.0.1'
uci commit network

# Enable and start
/etc/init.d/adguardhome enable
/etc/init.d/adguardhome start

echo "✓ AdGuard Home installed"
echo "  Web UI: http://<router_ip>:3000"
echo "  DNS:    port 53"
```

### 2.2 Create AdGuard removal script

**File:** `scripts/remove-adguard.sh` (new)

```sh
#!/bin/sh
# Remove AdGuard Home and restore dnsmasq
set -e

/etc/init.d/adguardhome stop 2>/dev/null || true
/etc/init.d/adguardhome disable 2>/dev/null || true

rm -f /usr/bin/AdGuardHome
rm -f /etc/init.d/adguardhome
rm -rf /etc/adguardhome

# Restore dnsmasq as DNS server
uci set dhcp.@dnsmasq[0].port='53'
uci commit dhcp
uci delete network.lan.dns 2>/dev/null || true
uci commit network
/etc/init.d/dnsmasq restart

echo "✓ AdGuard Home removed, dnsmasq restored"
```

### 2.3 Add AdGuard proxy endpoint in Travel GUI (optional, future)

**Note:** This is stretch/future — the Travel GUI could proxy `/adguard/` to `localhost:3000` so users have a single entry point. Not in scope for this plan but noted as enhancement.

---

## Phase 3: Install Script (curl-able one-liner)

**Goal:** `wget -O- https://github.com/.../install.sh | sh` on a fresh OpenWRT router.

### 3.1 Create the install script

**File:** `scripts/install.sh` (new)

```sh
#!/bin/sh
# One-liner installer for OpenWRT Travel GUI
# Usage: wget -O- https://raw.githubusercontent.com/<owner>/openwrt-travel-gui/main/scripts/install.sh | sh
#   or: sh install.sh [options]
#
# Options:
#   --no-adguard     Skip AdGuard Home installation
#   --password PASS  Set admin password (default: admin)
#   --version VER    Install specific version (default: latest)
set -e

# --- Defaults ---
INSTALL_ADGUARD=1
ADMIN_PASSWORD="admin"
VERSION="latest"
ARCH="aarch64_cortex-a53"
GITHUB_REPO="<owner>/openwrt-travel-gui"  # TODO: set real repo

# --- Parse arguments ---
while [ $# -gt 0 ]; do
    case "$1" in
        --no-adguard)  INSTALL_ADGUARD=0; shift ;;
        --password)    ADMIN_PASSWORD="$2"; shift 2 ;;
        --version)     VERSION="$2"; shift 2 ;;
        --arch)        ARCH="$2"; shift 2 ;;
        *)             echo "Unknown option: $1"; exit 1 ;;
    esac
done

echo "========================================="
echo " OpenWRT Travel GUI Installer"
echo "========================================="
echo ""

# --- Step 1: Detect architecture ---
detect_arch() {
    local machine
    machine=$(uname -m)
    case "$machine" in
        aarch64) ARCH="aarch64_cortex-a53" ;;
        x86_64)  ARCH="x86_64" ;;
        armv7l)  ARCH="arm_cortex-a7_neon-vfpv4" ;;
        *)       echo "Warning: Unknown arch '$machine', using $ARCH" ;;
    esac
    echo "→ Detected architecture: $ARCH"
}
detect_arch

# --- Step 2: Find latest release ---
if [ "$VERSION" = "latest" ]; then
    echo "→ Finding latest release..."
    VERSION=$(wget -qO- "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" | \
        jsonfilter -e '@.tag_name' 2>/dev/null || \
        wget -qO- "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" | \
        sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p')
    if [ -z "$VERSION" ]; then
        echo "Error: Could not determine latest version"
        exit 1
    fi
fi
echo "→ Installing version: $VERSION"

# --- Step 3: Download and install .ipk ---
IPK_URL="https://github.com/${GITHUB_REPO}/releases/download/${VERSION}/openwrt-travel-gui_${VERSION#v}_${ARCH}.ipk"
echo "→ Downloading $IPK_URL ..."
cd /tmp
wget -q "$IPK_URL" -O openwrt-travel-gui.ipk || {
    echo "Error: Failed to download .ipk from $IPK_URL"
    exit 1
}

echo "→ Installing package..."
opkg install /tmp/openwrt-travel-gui.ipk
rm -f /tmp/openwrt-travel-gui.ipk

# --- Step 4: Set password ---
if [ "$ADMIN_PASSWORD" != "admin" ]; then
    uci set openwrt-travel-gui.main.password="$ADMIN_PASSWORD"
    uci commit openwrt-travel-gui
fi

# --- Step 5: Install AdGuard Home ---
if [ "$INSTALL_ADGUARD" = "1" ]; then
    echo ""
    echo "→ Installing AdGuard Home..."
    # Determine AdGuard arch
    case "$(uname -m)" in
        aarch64) AG_ARCH="linux_arm64" ;;
        x86_64)  AG_ARCH="linux_amd64" ;;
        armv7l)  AG_ARCH="linux_armv7" ;;
        *)       AG_ARCH="linux_arm64" ;;
    esac

    # Download the AdGuard install script from the same release
    wget -qO- "https://raw.githubusercontent.com/${GITHUB_REPO}/${VERSION}/scripts/install-adguard.sh" | sh -s "$AG_ARCH"
fi

# --- Step 6: Restart service ---
echo "→ Restarting service..."
/etc/init.d/openwrt-travel-gui restart

# --- Done ---
ROUTER_IP=$(uci get network.lan.ipaddr 2>/dev/null || echo "192.168.8.1")
echo ""
echo "========================================="
echo " Installation Complete!"
echo "========================================="
echo ""
echo " Travel GUI:  http://${ROUTER_IP}/"
echo " Password:    ${ADMIN_PASSWORD}"
echo " LuCI:        http://${ROUTER_IP}:8080/"
if [ "$INSTALL_ADGUARD" = "1" ]; then
echo " AdGuard:     http://${ROUTER_IP}:3000/"
fi
echo ""
echo " Change password:"
echo "   uci set openwrt-travel-gui.main.password='newpass'"
echo "   uci commit openwrt-travel-gui"
echo "   /etc/init.d/openwrt-travel-gui restart"
echo ""
```

---

## Phase 4: GitHub Actions CI/CD

**Goal:** Automated build/test on PR, release with binary + .ipk assets on tag push.

### 4.1 CI Pipeline (build + test)

**File:** `.github/workflows/ci.yml` (new)

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test-backend:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - name: Run Go tests
        working-directory: backend
        run: go test -race -cover ./...
      - name: Go vet
        working-directory: backend
        run: go vet ./...

  test-frontend:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: pnpm/action-setup@v4
        with:
          version: 9
      - uses: actions/setup-node@v4
        with:
          node-version: 20
          cache: pnpm
      - run: pnpm install --frozen-lockfile
      - name: Run shared tests
        run: cd shared && pnpm test
      - name: Run frontend tests
        run: cd frontend && pnpm test
      - name: Lint
        run: pnpm lint

  build:
    runs-on: ubuntu-latest
    needs: [test-backend, test-frontend]
    steps:
      - uses: actions/checkout@v4
      - uses: pnpm/action-setup@v4
        with:
          version: 9
      - uses: actions/setup-node@v4
        with:
          node-version: 20
          cache: pnpm
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - run: pnpm install --frozen-lockfile
      - name: Build production binary (aarch64)
        run: ./scripts/build.sh arm64 linux
      - name: Verify binary
        run: file dist/openwrt-travel-gui
      - name: Upload binary artifact
        uses: actions/upload-artifact@v4
        with:
          name: openwrt-travel-gui-aarch64
          path: dist/openwrt-travel-gui
```

### 4.2 Release Pipeline (on tag)

**File:** `.github/workflows/release.yml` (new)

```yaml
name: Release

on:
  push:
    tags: ['v*']

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        include:
          - goarch: arm64
            ipk_arch: aarch64_cortex-a53
            suffix: aarch64
          - goarch: amd64
            ipk_arch: x86_64
            suffix: x86_64

    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0  # for git describe

      - uses: pnpm/action-setup@v4
        with:
          version: 9
      - uses: actions/setup-node@v4
        with:
          node-version: 20
          cache: pnpm
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - run: pnpm install --frozen-lockfile

      - name: Build binary (${{ matrix.suffix }})
        run: ./scripts/build.sh ${{ matrix.goarch }} linux

      - name: Rename binary
        run: mv dist/openwrt-travel-gui dist/openwrt-travel-gui-${{ matrix.suffix }}

      - name: Package .ipk
        run: |
          mv dist/openwrt-travel-gui-${{ matrix.suffix }} dist/openwrt-travel-gui
          ./scripts/package-ipk.sh ${{ matrix.ipk_arch }}
          mv dist/openwrt-travel-gui dist/openwrt-travel-gui-${{ matrix.suffix }}

      - name: Upload artifacts
        uses: actions/upload-artifact@v4
        with:
          name: release-${{ matrix.suffix }}
          path: |
            dist/openwrt-travel-gui-${{ matrix.suffix }}
            dist/*.ipk

  create-release:
    runs-on: ubuntu-latest
    needs: release
    steps:
      - uses: actions/checkout@v4

      - name: Download all artifacts
        uses: actions/download-artifact@v4
        with:
          path: release-assets
          merge-multiple: true

      - name: Create GitHub Release
        uses: softprops/action-gh-release@v2
        with:
          generate_release_notes: true
          files: |
            release-assets/*
            scripts/install.sh
            scripts/install-adguard.sh
```

---

## Phase 5: Fix Packaging Issues

**Goal:** Correct arch handling, clean up packaging scripts, update docs.

### 5.1 Fix `PKGARCH` in OpenWRT SDK Makefile

**File:** `packaging/openwrt/Makefile`

**Change:** Remove `PKGARCH:=all` (or set dynamically) since the package contains a compiled binary:

```makefile
define Package/openwrt-travel-gui
  SECTION:=luci
  CATEGORY:=LuCI
  TITLE:=Modern web UI for OpenWRT travel routers
  DEPENDS:=
endef
```

Remove the `PKGARCH:=all` line entirely. The OpenWRT build system will set the correct architecture automatically when building through the SDK.

### 5.2 Fix `package-ipk.sh` to use correct Architecture field

**File:** `scripts/package-ipk.sh`

The script already accepts arch as arg and writes it to the control file — this is correct. No change needed for the architecture field.

**Change:** Add a `Conffiles` field to the control file and improve the prerm script to not fail on missing UCI:

In control file generation, add after `Depends: libc`:
```
Conffiles:
 /etc/config/openwrt-travel-gui
```

But actually this is already handled by the separate `conffiles` file. So no change needed.

### 5.3 Update `deployment.md`

**File:** `docs/deployment.md`

**Change:** Add one-liner install section at the top, AdGuard section, and CI/CD release links:

Add after the "## Target Devices" section:

```markdown
## Quick Install (Recommended)

SSH into your OpenWRT router and run:

\```bash
wget -O- https://raw.githubusercontent.com/<owner>/openwrt-travel-gui/main/scripts/install.sh | sh
\```

Options:
\```bash
# Custom password, skip AdGuard
wget -O- .../install.sh | sh -s -- --password mypass --no-adguard

# Specific version
wget -O- .../install.sh | sh -s -- --version v1.0.0
\```

## What Gets Installed

| Component        | Port | Purpose                       |
| ---------------- | ---- | ----------------------------- |
| Travel GUI       | 80   | Main web interface            |
| LuCI             | 8080 | OpenWRT native UI (relocated) |
| AdGuard Home UI  | 3000 | DNS filtering management      |
| AdGuard Home DNS | 53   | DNS server (replaces dnsmasq) |

## AdGuard Home

AdGuard Home is installed automatically unless `--no-adguard` is passed.

To install AdGuard separately:
\```bash
sh /tmp/install-adguard.sh linux_arm64
\```

To remove:
\```bash
sh /tmp/remove-adguard.sh
\```
```

### 5.4 Update `Makefile` (root) with new targets

**File:** `Makefile`

**Change:** Add convenience targets:

```makefile
# Build for multiple architectures
build-all:
	./scripts/build.sh arm64 linux
	mv dist/openwrt-travel-gui dist/openwrt-travel-gui-aarch64
	./scripts/build.sh amd64 linux
	mv dist/openwrt-travel-gui dist/openwrt-travel-gui-x86_64

# Package all architectures
package-all: build-all
	mv dist/openwrt-travel-gui-aarch64 dist/openwrt-travel-gui
	./scripts/package-ipk.sh aarch64_cortex-a53
	mv dist/openwrt-travel-gui dist/openwrt-travel-gui-aarch64
	mv dist/openwrt-travel-gui-x86_64 dist/openwrt-travel-gui
	./scripts/package-ipk.sh x86_64
	mv dist/openwrt-travel-gui dist/openwrt-travel-gui-x86_64
```

---

## File Change Summary

### New Files
| File                            | Phase | Description                               |
| ------------------------------- | ----- | ----------------------------------------- |
| `scripts/install.sh`            | 3     | Curl-able one-liner installer for OpenWRT |
| `scripts/install-adguard.sh`    | 2     | AdGuard Home install & configure script   |
| `scripts/remove-adguard.sh`     | 2     | AdGuard Home removal script               |
| `.github/workflows/ci.yml`      | 4     | CI pipeline (test on push/PR)             |
| `.github/workflows/release.yml` | 4     | Release pipeline (on tag push)            |

### Modified Files
| File                                                    | Phase | Changes                                                    |
| ------------------------------------------------------- | ----- | ---------------------------------------------------------- |
| `backend/cmd/server/main.go`                            | 1     | Add `Version` var, use `LoadConfig()`, log version         |
| `backend/internal/config/config.go`                     | 1     | Add `LoadConfig()` with CLI flag parsing                   |
| `backend/internal/config/config_test.go`                | 1     | Add tests for `LoadConfig()` with CLI flags                |
| `packaging/openwrt/files/etc/init.d/openwrt-travel-gui` | 1     | Pass env vars OR fix flag names, add JWT secret generation |
| `packaging/openwrt/files/etc/config/openwrt-travel-gui` | 1     | Add `jwt_secret` option                                    |
| `packaging/openwrt/Makefile`                            | 5     | Remove `PKGARCH:=all`                                      |
| `docs/deployment.md`                                    | 5     | Add quick install, AdGuard, and release docs               |
| `Makefile`                                              | 5     | Add `build-all` and `package-all` targets                  |

---

## Implementation Order & Dependencies

```
Phase 1 (BLOCKER — do first)
  ├── 1.1 Add Version var             (no deps)
  ├── 1.2 Add LoadConfig() with flags (no deps)
  ├── 1.3 Update main.go              (depends on 1.1, 1.2)
  ├── 1.4 UCI config jwt_secret       (no deps)
  ├── 1.5 Fix init.d script           (depends on 1.2, 1.4)
  ├── 1.6 Tests                       (depends on 1.2)
  └── 1.7 Verify build.sh ldflags     (depends on 1.1)

Phase 2 (independent of Phase 1)
  ├── 2.1 install-adguard.sh          (no deps)
  └── 2.2 remove-adguard.sh           (no deps)

Phase 3 (depends on Phase 1 + 2)
  └── 3.1 install.sh                  (depends on working .ipk + adguard script)

Phase 4 (depends on Phase 1)
  ├── 4.1 ci.yml                      (depends on tests passing)
  └── 4.2 release.yml                 (depends on build.sh + package-ipk.sh working)

Phase 5 (depends on all above)
  ├── 5.1 Fix PKGARCH                 (no deps)
  ├── 5.2 Verify package-ipk.sh       (no deps)
  ├── 5.3 Update deployment.md        (depends on all features existing)
  └── 5.4 Makefile targets            (no deps)
```

## Testing Checklist

- [ ] `cd backend && go test ./...` — all pass (especially new config tests)
- [ ] `make build-prod` — binary built with correct version string
- [ ] `file dist/openwrt-travel-gui` — shows aarch64 ELF binary
- [ ] `dist/openwrt-travel-gui --port 8888 --password test` — starts on port 8888
- [ ] `PORT=7777 dist/openwrt-travel-gui --port 8888` — starts on 8888 (CLI wins)
- [ ] `make package` — produces arch-specific .ipk
- [ ] `.ipk` control file shows correct architecture (not `all`)
- [ ] init.d script generates jwt_secret on first boot
- [ ] init.d script reuses jwt_secret on restart
- [ ] `install-adguard.sh` installs AdGuard, dnsmasq port set to 0
- [ ] `remove-adguard.sh` removes AdGuard, dnsmasq restored to port 53
- [ ] GitHub Actions CI runs on push to main
- [ ] GitHub Actions release creates assets on tag push
