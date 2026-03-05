# Deployment Guide

## Target Devices

- GL.iNet Beryl AX (MT3000) — aarch64
- GL.iNet Slate AXT1800 — aarch64

## Prerequisites

- SSH access to your router (`ssh root@<router_ip>`)
- Build tools: Node.js 20+, pnpm 9+, Go 1.23+

## Build from Source

```bash
# Cross-compile for aarch64 (default)
make build-prod

# Or specify architecture
./scripts/build.sh arm64 linux
```

Output: `dist/openwrt-travel-gui`

## Create Package

```bash
# Build .ipk for default arch (aarch64_cortex-a53)
make package

# Or specify architecture
./scripts/package-ipk.sh aarch64_cortex-a53
```

Output: `dist/openwrt-travel-gui_<version>_<arch>.ipk`

## Deploy

```bash
# One-command deploy
make deploy ROUTER_IP=192.168.8.1

# Or manually
./scripts/deploy.sh 192.168.8.1
```

This will:
1. Copy the `.ipk` to the router
2. Install via `opkg`
3. Move LuCI to port 8080/8443
4. Start the travel GUI on port 80

## Manual Install

```bash
scp dist/openwrt-travel-gui_*.ipk root@192.168.8.1:/tmp/
ssh root@192.168.8.1 "opkg install /tmp/openwrt-travel-gui_*.ipk"
```

## Uninstalling

```bash
ssh root@192.168.8.1 "opkg remove openwrt-travel-gui"
```

This restores LuCI to port 80/443 automatically.

## Updating

```bash
# Build new version, then:
make deploy ROUTER_IP=192.168.8.1
# opkg handles the upgrade
```

## Configuration

UCI config at `/etc/config/openwrt-travel-gui`:

```
config travel_gui 'main'
    option enabled '1'
    option port '80'
    option password 'admin'
```

Change password:
```bash
uci set openwrt-travel-gui.main.password='newpassword'
uci commit openwrt-travel-gui
/etc/init.d/openwrt-travel-gui restart
```

## Troubleshooting

| Issue               | Solution                                                                           |
| ------------------- | ---------------------------------------------------------------------------------- |
| Binary won't start  | Check arch: `file /usr/bin/openwrt-travel-gui` — must be aarch64                   |
| Port 80 conflict    | Ensure uhttpd moved: `uci get uhttpd.main.listen_http` should be `0.0.0.0:8080`    |
| Can't reach LuCI    | Access via `http://<router_ip>:8080` after install                                 |
| Service not running | `logread -e openwrt-travel-gui` for logs, `/etc/init.d/openwrt-travel-gui restart` |
| Permission denied   | Ensure binary is executable: `chmod +x /usr/bin/openwrt-travel-gui`                |
