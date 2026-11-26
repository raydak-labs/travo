# Settings pages for optional catalog services

The Services page installs and removes optional OpenWrt services (SQM, Cloudflare Tunnel, vnStat, Watchcat, speedtestcpp). For the ones with an init script (all but speedtestcpp) it also starts and stops them and sets whether they start at boot. Except for SQM, Travo does not configure catalog services: each card with a LuCI app links it (`:8080`); the catalog installs the app next to the package. speedtestcpp is a command-line client with neither.

## Why this is out of scope

Each of these packages except speedtestcpp ships a LuCI app that owns its UCI config; speedtestcpp has no LuCI app. A Travo settings page per service would repeat that UI and need its own config model, transaction rules and device fixtures for every package. The catalog stays one data table (`internal/catalog`), so adding a service costs one entry.

A service that travellers use daily and that can cut connectivity or directly governs travel-uplink quality gets its own feature instead, as WireGuard, Tailscale, AdGuard Home, and SQM did.

## Carve-outs

- **SQM** (#302): bufferbloat mitigation on variable hotel/cellular uplinks is an everyday travel-router setting. Travo exposes essential SQM queue settings (`Services ▸ SQM`: enable, interface, download/upload bandwidth, qdisc/script, link layer adaptation and overhead) while keeping a secondary link to LuCI (`admin/network/sqm`) for advanced options.

## Prior requests

- #268 built the catalog and left settings to LuCI.
- #302 carved out SQM for essential settings in Travo.
