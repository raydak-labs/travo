---
title: Deployment guide
description: Official install path (tarball), LuCI coexistence, AdGuard, build and package.
updated: 2026-04-13
tags: [docs, deployment, openwrt]
---

# Deployment Guide

Production installs use a **release `.tar.gz`** from GitHub Releases (see `scripts/install.sh`). LuCI stays available on alternate ports.

## Quick install (recommended)

On the router:

```sh
wget -O- https://raw.githubusercontent.com/raydak-labs/travo/main/scripts/install.sh | sh
```

Non-interactive examples:

```sh
wget -O- https://raw.githubusercontent.com/raydak-labs/travo/main/scripts/install.sh | \
  sh -s -- --yes --version 1.0.0 --password 'your-root-password'

wget -O- https://raw.githubusercontent.com/raydak-labs/travo/main/scripts/install.sh | \
  sh -s -- --no-adguard
```

The script downloads `travo_<version>_<arch>.tar.gz`, extracts to `/`, moves LuCI off port 80 when configured, optionally installs AdGuard, sets root password, and enables `travo`.

## What gets installed

| Piece | Ports / path | Notes |
| ----- | ------------ | ----- |
| Travo UI + API | `http://<router>:80` (default) | Static UI under `/www/travo`; binary `/usr/bin/travo` |
| LuCI (after move) | `http://<router>:8080`, HTTPS `8443` | `uhttpd` listen updated by uci-defaults |
| AdGuard Home (optional) | Web UI `3000`; DNS **`5353`** | dnsmasq on the router forwards LAN DNS to AdGuard (`127.0.0.1#5353` pattern); see bundled `packaging/adguard/AdGuardHome.yaml` |

JWT secret and sealed auth data live under `/etc/travo/` (created on first start).

## Manual install from a release tarball

Pick the asset matching `uname -m` / OpenWrt arch (installer and `scripts/install.sh` use the same naming). Example:

```sh
cd /tmp
wget "https://github.com/raydak-labs/travo/releases/download/v1.0.0/travo_1.0.0_aarch64_cortex-a53.tar.gz"
tar -xzf travo_1.0.0_aarch64_cortex-a53.tar.gz -C /
chmod +x /usr/bin/travo /etc/init.d/travo 2>/dev/null || true
# Run any pending uci-defaults (e.g. LuCI port move), then:
/etc/init.d/travo enable
/etc/init.d/travo start
```

If you skip the install script, run `scripts/install-adguard.sh` yourself when you want AdGuard.

## Configuration

- UCI: `/etc/config/travo`
- Auth: `/etc/travo/auth.json` (auto-generated)

### Password

Travo uses the **root** password (rpcd/LuCI). Change with `passwd root` or LuCI.

### Travo listen port

```sh
uci set travo.main.port='8888'
uci commit travo
/etc/init.d/travo restart
```

### Disable service

```sh
/etc/init.d/travo stop
/etc/init.d/travo disable
```

## AdGuard Home

- Binary/config: `/opt/AdGuardHome`
- UI: `http://<router>:3000`
- DNS listener: **5353** in the stock template (not raw `53` on WAN)

Restart: `/etc/init.d/adguardhome restart`

## Uninstall

```sh
wget -O- https://raw.githubusercontent.com/raydak-labs/travo/main/scripts/install.sh | sh -s -- --uninstall
```

Or from a clone: `sh scripts/uninstall.sh`

## Build and package (maintainers)

Prerequisites: Node 20+, pnpm 10+, Go 1.23+.

```sh
pnpm install
cd backend && go mod tidy && cd ..

# Cross-compile binary into dist/
make build-prod # default arch
make build-all         # aarch64 + amd64 artifacts

# Release tarball(s) for install.sh / GitHub Releases
make package           # default ARCH
ARCH=x86_64 bash scripts/package-tarball.sh
make package-all
```

Output: `dist/travo_<version>_<arch>.tar.gz`

## Deploy from dev machine

Fast iteration (binary + frontend assets over SSH):

```sh
./scripts/deploy-local.sh --ip 192.168.1.1
./scripts/deploy-local.sh --binary-only --ip 192.168.1.1
./scripts/deploy-local.sh --method release --ip 192.168.1.1
```

`make deploy` runs the same script with `DEPLOY_METHOD` (default `direct`) and `ROUTER_IP` — see `scripts/deploy-local.sh --help` and `Makefile`.

## Supported targets

| Target | Arch | Notes |
| ------ | ---- | ----- |
| GL.iNet Beryl AX (MT3000) | aarch64 | Lab tested |
| GL.iNet Slate AXT1800 | aarch64 | Lab tested |
| Generic OpenWrt 23.05+ | aarch64 / x86_64 | Use matching tarball arch |

## Troubleshooting

| Symptom | Check |
| ------- | ----- |
| Binary won’t run | `file /usr/bin/travo` matches CPU; `logread -e travo` |
| Port 80 busy | `uci get uhttpd.main.listen_http` should be `0.0.0.0:8080` after migrate |
| LuCI missing | `http://<router>:8080` |
| AdGuard DNS oddities | `netstat`/`ss` on `5353`; dnsmasq `server=` / forwards |
| Low flash | `df -h /` (~20 MB headroom minimum recommended) |
