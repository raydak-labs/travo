# Deployment Guide

## Quick Install (recommended)

SSH into your OpenWRT router and run:

```sh
wget -O- https://raw.githubusercontent.com/raydak-labs/hackathon-202603-ui-openwrt/main/scripts/install.sh | sh
```

Or with options:

```sh
# Install a specific version with a custom password, non-interactive
wget -O- https://raw.githubusercontent.com/raydak-labs/hackathon-202603-ui-openwrt/main/scripts/install.sh | \
  sh -s -- --yes --version 1.0.0 --password mysecret

# Skip AdGuard Home installation
wget -O- https://raw.githubusercontent.com/raydak-labs/hackathon-202603-ui-openwrt/main/scripts/install.sh | \
  sh -s -- --no-adguard
```

## What Gets Installed

| Component    | Port          | Description                       |
| ------------ | ------------- | --------------------------------- |
| Travel GUI   | 80 (HTTP)     | Modern web dashboard              |
| AdGuard Home | 3000 (Web UI) | DNS ad-blocker, DNS on port 53    |
| LuCI (moved) | 8080 (HTTP)   | Original OpenWRT UI, still usable |

The install script:
1. Downloads the `.ipk` package for your architecture from GitHub Releases
2. Installs it via `opkg`
3. Moves LuCI (uhttpd) to port 8080/8443
4. Downloads and installs AdGuard Home
5. Sets an admin password (default: `admin`)
6. Starts the Travel GUI service

## Manual Installation from .ipk

Download the `.ipk` for your architecture from
[GitHub Releases](https://github.com/raydak-labs/hackathon-202603-ui-openwrt/releases):

```sh
# On the router
cd /tmp
wget https://github.com/raydak-labs/hackathon-202603-ui-openwrt/releases/download/v1.0.0/travo_1.0.0_aarch64_cortex-a53.ipk
opkg install travo_1.0.0_aarch64_cortex-a53.ipk
```

After installing the `.ipk` manually, you still need to:
- Move LuCI to port 8080 (or run the uci-defaults script)
- Optionally install AdGuard Home with `scripts/install-adguard.sh`

## Configuration

UCI config is at `/etc/config/travo`:

```
config travel_gui 'main'
    option enabled '1'
    option port '80'
    option password 'admin'
    option jwt_secret ''
```

### Change password

```sh
uci set travo.main.password='newpassword'
uci commit travo
/etc/init.d/travo restart
```

### Change port

```sh
uci set travo.main.port='8888'
uci commit travo
/etc/init.d/travo restart
```

### Disable the service

```sh
/etc/init.d/travo stop
/etc/init.d/travo disable
```

## AdGuard Home

AdGuard Home is installed to `/opt/AdGuardHome` and runs as an init.d service.

- **Web UI:** `http://<router_ip>:3000`
- **DNS:** listens on port 53 (replaces dnsmasq as the primary DNS)
- **Config:** `/opt/AdGuardHome/AdGuardHome.yaml`

To customize AdGuard Home, use its web UI at port 3000 or edit the YAML config
directly and restart:

```sh
/etc/init.d/adguardhome restart
```

## Uninstall

Run the uninstall script on the router:

```sh
wget -O- https://raw.githubusercontent.com/raydak-labs/hackathon-202603-ui-openwrt/main/scripts/install.sh | sh -s -- --uninstall
```

Or if you have the repo cloned:

```sh
sh scripts/uninstall.sh
```

This will:
1. Stop and remove the Travel GUI service
2. Remove AdGuard Home (if installed)
3. Restore LuCI to ports 80/443
4. Clean up config files

## Building from Source

### Prerequisites

- Node.js >= 20, pnpm >= 9
- Go >= 1.23
- Git

### Build

```sh
# Install dependencies
pnpm install
cd backend && go mod tidy && cd ..

# Cross-compile for aarch64 (default)
make build-prod

# Or specify architecture
GOARCH=amd64 bash scripts/build.sh

# Build for both architectures
make build-all
```

Output: `dist/travo`

### Create .ipk package

```sh
# Default (aarch64_cortex-a53)
make package

# Specify architecture
ARCH=x86_64 bash scripts/package-ipk.sh

# Build packages for both architectures
make package-all
```

Output: `dist/travo_<version>_<arch>.ipk`

### Deploy to router

```sh
make deploy ROUTER_IP=192.168.8.1
```

## Supported Devices

| Device                       | Architecture | Status    |
| ---------------------------- | ------------ | --------- |
| GL.iNet Beryl AX (MT3000)    | aarch64      | Tested    |
| GL.iNet Slate AXT1800        | aarch64      | Tested    |
| Any OpenWRT 23.05+ (aarch64) | aarch64      | Supported |
| Any OpenWRT 23.05+ (x86_64)  | x86_64       | Supported |

## Troubleshooting

| Issue                            | Solution                                                                                       |
| -------------------------------- | ---------------------------------------------------------------------------------------------- |
| Binary won't start               | Check arch: `file /usr/bin/travo` — must match your device                        |
| Port 80 conflict                 | Verify uhttpd moved: `uci get uhttpd.main.listen_http` should show `0.0.0.0:8080`              |
| Can't reach LuCI                 | Access via `http://<router_ip>:8080` after install                                             |
| Service not running              | Check logs: `logread -e travo`, restart: `/etc/init.d/travo restart` |
| Permission denied                | `chmod +x /usr/bin/travo`                                                         |
| AdGuard not blocking ads         | Verify DNS: `nslookup ads.example.com <router_ip>` — should return 0.0.0.0                     |
| AdGuard UI not accessible        | Check service: `/etc/init.d/adguardhome status`, restart if needed                             |
| Insufficient storage             | Need ~20 MB free; check with `df -h /`                                                         |
| Install script fails to download | Verify internet: `ping github.com`; try specifying `--version` explicitly                      |
