# Architecture decision records

ADRs record decisions and stay as written. A later decision that changes one is a new ADR, and the old one's status line links it.

| ADR | Decision |
|---|---|
| [0001](0001-clean-room-greenfield.md) | Clean-room rewrite beside old Travo: reuse lessons, not code |
| [0002](0002-single-go-daemon-with-ports.md) | One Go daemon; all OpenWrt access behind `Device` / `PackageManager` ports |
| [0003](0003-api-and-client-generation.md) | Huma generates OpenAPI 3.1, hey-api generates the TypeScript client (validation narrowed by 0031) |
| [0004](0004-frontend-stack.md) | Svelte 5 SPA with hash routing, Paraglide `en` only |
| [0005](0005-transactions-and-watchdog.md) | Own transaction layer with an out-of-process rollback watchdog |
| [0006](0006-adguard-package-and-topology.md) | AdGuard Home package on 127.0.0.1:5353 behind dnsmasq, fail-open, memory caps |
| [0007](0007-distribution-and-self-update.md) | Release tarball, install script, signed self-update |
| [0008](0008-auth-and-trust-boundary.md) | Sessions, API tokens and the trust boundary |
| [0009](0009-persistence.md) | UCI as source of truth, atomic JSON state, no KV store |
| [0010](0010-dns-layer-stack.md) | One ordered DNS layer stack |
| [0011](0011-lockout-classification.md) | Classify the caller's path to prevent lockout |
| [0012](0012-os-support-and-package-managers.md) | OpenWrt 25.12 with apk supported, opkg best effort |
| [0013](0013-shadcn-components.md) | Copy only the shadcn-svelte components in use |
| [0014](0014-capped-uplink-reconnect.md) | Uplink reconnect capped at 5 `ifup wwan`, never `wifi up` |
| [0015](0015-no-native-apk-yet.md) | No native `.apk` yet |
| [0016](0016-self-signed-https.md) | Self-signed HTTPS beside plain HTTP |
| [0017](0017-tailscale.md) | Tailscale through the OpenWrt package |
| [0018](0018-mwan3-failover.md) | Multi-WAN failover with mwan3 and `track_ip` |
| [0019](0019-usb-tether-uplink.md) | USB tethering as an uplink |
| [0020](0020-offline-adguard-bundle.md) | AdGuard Home apk shipped as a release asset for offline install |
| [0021](0021-adguard-expert-topology.md) | Opt-in expert topology: AdGuard Home on :53, dnsmasq on :5354 |
| [0022](0022-captive-probe-semantics.md) | Captive portal probe semantics |
| [0023](0023-captive-auto-accept.md) | Automatic click-through for captive portals |
| [0024](0024-daemon-hooks-for-cron-and-buttons.md) | Cron jobs and button scripts call the daemon over a hook socket |
| 0025 | Not used |
| [0026](0026-stats-history.md) | Persistent stats history |
| [0027](0027-wifi-schedule.md) | Scheduled access point on/off via cron |
| [0028](0028-lan-address-change.md) | LAN address change confirmed from the new address |
| [0029](0029-slide-switch-self-confirm.md) | Slide switch actions confirm themselves |
| [0030](0030-firmware-guard-completion.md) | A finished firmware upgrade releases its guard |
| [0031](0031-runtime-validation-only-where-needed.md) | Zod Mini or Valibot only when a UI boundary outside the generated client needs runtime validation |
| [0032](0032-self-update-forward-compatibility.md) | Self-update accepts signed additions; CI stages each release with the previous one |
