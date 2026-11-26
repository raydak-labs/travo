# Settings pages for optional catalog services

The Services page installs and removes optional OpenWrt services (SQM, Cloudflare Tunnel, vnStat, Watchcat, speedtestcpp). For the ones with an init script (all but speedtestcpp) it also starts and stops them and sets whether they start at boot. It does not configure them. Each card with a LuCI app links it (`:8080`); the catalog installs the app next to the package. speedtestcpp is a command-line client with neither.

## Why this is out of scope

Each of these packages except speedtestcpp ships a LuCI app that owns its UCI config; speedtestcpp has no LuCI app. A Travo settings page per service would repeat that UI and need its own config model, transaction rules and device fixtures for every package. The catalog stays one data table (`internal/catalog`), so adding a service costs one entry.

A service that travellers use daily and that can cut connectivity gets its own feature instead, as WireGuard, Tailscale and AdGuard Home did.

## Prior requests

- #268 built the catalog and left settings to LuCI.
