# 0014 Capped uplink auto-reconnect

Status: accepted

Supersedes the `wifi up` exception in [invariants](../invariants.md) §1.

## Context

Old Travo ran `wifi up` from cron every minute with no uplink, which flapped the
radios (device-facts §8). Invariants §1 left room for a "bounded, documented
`wifi up` exception" for auto-reconnect. ath11k treats `wifi` commands as a crash
risk (device-facts §3), so `wifi*` stays forbidden in every daemon path.

Measured on the AXT1800 (OpenWrt 25.12.6, 2026-11-17, `SPIKES.md`):

- With no action, wpa_supplicant keeps scanning after the upstream disappears and
  re-associates within 2 to 4 s of it returning. DHCP follows within the same
  2 s sample. This covers the common hotel or phone-hotspot dropout.
- wpa_supplicant does not leave an administratively `DISCONNECTED` state, or a
  stale runtime filter, by itself. `ifup wwan` recovered both within 6 to 8 s.
- `ifup wwan` is `ubus call network reload` plus interface down and up. It rebuilds
  the STA vif on its own radio and makes hostapd reload settings for every phy.
  The AP on the other radio kept carrier at every 2 s sample, and logread showed
  no ath11k error.

## Decision

- A lifecycle worker (`internal/wifireconnect`) ticks every 60 s. It acts only when
  UCI has exactly one enabled managed `wwan` STA, that STA is proven disconnected
  (no carrier or no observed SSID/BSSID), `wwan` has no IPv4 address, and this
  was true on the previous tick too. The extra tick gives wpa_supplicant a full
  interval to heal on its own. Unreadable state does nothing.
- The recovery is `ifup wwan` through the `device.UplinkReconnector` port. It writes
  no UCI and never runs `wifi`. Any guard present skips the attempt, and the
  attempt itself runs under the `wifi-reconnect-in-progress` guard. A failed
  `ifup` keeps that guard, so the feature stays off until a redeploy clears it.
- At most 5 attempts. The counter is persisted in
  `/etc/travo-greenfield/wifi-reconnect.json` (`{attempts, last_attempt}`, no
  credentials) BEFORE each attempt, so restarts, redeploys and a crash mid-attempt
  never reset it. Only an IPv4 address on `wwan` clears it. A torn file fails
  closed: the worker stays idle and the API answers 503.
- No automatic profile switching in 0.1.0. Switching would be a persisted wireless
  change without explicit confirmation, which [0005](0005-transactions-and-watchdog.md)
  forbids. When the cap is reached, `GET /api/v1/wifi/reconnect` reports
  `exhausted`. The WiFi page then offers the highest-priority saved uplink seen in
  the last scan, and the operator presses Select (the guarded Join).

## Consequences

- At most 5 network reloads per outage, one per two minutes. Then the radios stay
  untouched until an address returns.
- An STA left enabled for an upstream that never returns ends in `exhausted` and
  stays there until a Join gets an address. That is intended: the banner asks
  the operator to act.
- Automatic switching needs a new ADR that defines a confirm-free transaction policy.
