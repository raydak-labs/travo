---
title: "TEMPORARY — device verification for fix/deep-review-2026-10-04"
date: 2026-10-04
status: TEMPORARY — DELETE THIS FILE AND ITS COMMIT WHEN THE CHECKS BELOW HAVE BEEN RUN
---

# ⛔ TEMPORARY — delete this file and drop its commit once the checks below have run

**This document must not ship.** It exists only because `192.168.1.1` was unreachable during the
2026-10-04 review and remediation, so a large part of the work is verified by unit tests and reading
rather than by running it on real hardware. Nothing here is documentation of Travo; it is a punch
list for one person with the device.

**When every item is done, delete `docs/tests/VERIFY-on-device-2026-10-04.md` and remove the commit
that introduced it.** That commit is:

```
88b66d5 docs(temporary): on-device verification checklist for this remediation
```

Before merging, drop it from the branch:

```sh
# find it again if the SHA has moved
git log --format=%H -1 -- docs/tests/VERIFY-on-device-2026-10-04.md

# then, on the branch, either
git rebase -d <sha>          # remove it from history
# or, if the branch is already merged / you prefer not to rewrite it
git rm docs/tests/VERIFY-on-device-2026-10-04.md && git commit -m "docs: remove temporary device verification checklist"
```

Do not merge it to `main`. If some items are still open when the device is next available, keep the file and re-date it
rather than deleting it with work outstanding — a checklist that silently disappears is worse than one
that is out of date.

Source branch: `fix/deep-review-2026-10-04`. Full context:
[`2026-10-04-deep-code-review.md`](../../2026-10-04-deep-code-review.md).

---

## 0. The one thing that can invalidate the branch

**Does ath11k report an access point as `up` when it has zero associated stations?**

`ConfirmApply` now probes, server-side, that every access point the applied config enables is
actually up before it cancels rpcd's rollback window. The probe reads
`ubus call network.wireless status` and checks the interface's `up` flag.

If ath11k reports `up: false` for an AP with no client associated, then **every wireless
configuration on this hardware becomes unconfirmable**: the probe never passes, the confirm never
lands, rpcd rolls every change back after 30 s, and the wireless UI becomes useless. Every other
item below assumes the answer is "yes, an AP with no stations is up".

```sh
# With the router NOT serving any client on that AP (best done from a laptop
# connected by ETHERNET, not Wi-Fi, so nothing associates implicitly):
ubus call network.wireless status
```

Look at the AP interfaces: `up` should be `true` while no station is associated.

- **If `up: true` → proceed.** Nothing to change.
- **If `up: false` → stop and report.** The probe needs a different signal (a radio-level or
  config-level netifd field, or `iwinfo` on the radio rather than the interface). This is the single
  largest unknown in the branch and it was flagged in the review before the change was written.

Also measure how long an AP takes to come up after `uci apply`, because the probe budget is short:

```sh
# Start a stopwatch, then from the UI change an AP SSID and confirm.
# If a plain AP change regularly takes longer than ~4 s to report up, the probe
# budget is too tight and GOOD configs will be rolled back. Symptoms: the UI
# says "rolled back" for changes that actually look applied after a reboot.
```

---

## 1. Auth — prove the gate holds on the real binary (P0, highest priority)

Unit tests drive the production app in-process, but nothing has run the shipped binary.

```sh
# On the device, from a machine on the LAN:
curl -s -o /dev/null -w '%{http_code}\n' http://192.168.1.1/api/v1/system/info
# expect 401

curl -s -o /dev/null -w '%{http_code}\n' http://192.168.1.1/API/v1/system/info
# expect 404 -- NOT 200. If this returns 200 the bypass is back.

curl -s -o /dev/null -w '%{http_code}\n' http://192.168.1.1/Api/V1/System/Info
# expect 404

curl -s -o /dev/null -w '%{http_code}\n' http://192.168.1.1/api/health
# expect 200 (public)

curl -s -o /dev/null -w '%{http_code}\n' -X POST \
  http://192.168.1.1/api/v1/system/ssh-keys \
  -H 'Content-Type: application/json' -d '{"key":"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIProbe test"}'
# expect 401 and NO key added. Verify: cat /etc/dropbear/authorized_keys
```

Then with a valid token, confirm the public endpoints are still open — both of these broke during
this work:

```sh
# WebSocket must actually connect. In the browser console on the dashboard:
#   new WebSocket('ws://' + location.host + '/api/v1/ws?token=' + <token>)
# expect onopen to fire within a couple of seconds, NOT a 401 close.
# Simpler: open the dashboard, confirm live stats keep ticking.

# Pre-login clock recovery: set the clock to something implausible, then confirm
# you can still fix it from the login screen without a token.
```

---

## 2. Installer — every install used to end in `die` (P0)

```sh
# The documented one-liner, with the password, on a FRESH device:
wget -O- https://raw.githubusercontent.com/raydak-labs/travo/main/scripts/install.sh | \
  sh -s -- --password 'a-real-password-here'
# expect success, and a probe of the CONFIGURED port (80 by default, since
# LuCI is moved to 8080)

# The failure paths must refuse BEFORE changing anything:
wget -O- .../install.sh | sh
# expect: refuses, and `logread | grep travo` shows nothing new, /etc/config/travo
# untouched, LuCI still on 80. A refusal that leaves a half-install is the bug.

wget -O- .../install.sh | sh -s -- --password 'short'
# expect: refuses (minimum 8 characters)
```

---

## 3. Guest WiFi on a 2.4 GHz uplink (regression risk — this was made impossible)

The same-radio guard was extended to the guest network, which briefly made guest WiFi
**unavailable** on the most common layout. Verify the fallback works:

```sh
# Client/repeater mode with the uplink on 2.4 GHz. Enable the guest network.
# expect: SUCCEEDS, and the guest AP is on the OTHER radio.
uci show wireless | grep -A4 'wireless.guest'
# the `device` must be the radio that does NOT carry the wwan STA
uci show wireless | grep 'network.*wwan'
```

---

## 4. Wireless apply / rollback (the ADR 0002 §5 contract)

```sh
# 1. Connected over Wi-Fi, change the AP SSID.
#    expect: the change is applied and CONFIRMED (no "rolled back" toast).
# 2. Same, but with a DELIBERATELY BROKEN config -- 11-char key, or an
#    impossible channel.
#    expect: the change is rolled back to the previous working config after ~30 s,
#    and the UI says so rather than showing a generic failure.
# 3. Switch to Client mode while connected over Wi-Fi.
#    expect: ROLLBACK. The AP comes back. This is the lockout case the whole
#    model exists for.
# 4. Switch to Client mode while connected over ETHERNET.
#    expect: applies and confirms (Ethernet is unaffected).
# 5. Connect to an upstream twice in a row, no band pinned.
#    expect: both succeed. It used to fail ~40% of the time.
```

---

## 5. DNS layering (VPN + AdGuard)

```sh
# Enable AdGuard DNS, note the value of:   uci show dhcp.@dnsmasq[0].server
# Enable the VPN (DNS resolvers configured), note it again.
# Reboot with the tunnel down.
# expect: dnsmasq points at the layer below (AdGuard), NOT at a dead VPN resolver,
# and NOT at itself. End state "server=127.0.0.1, noresolv=0" is the bug.
# Then disable the VPN -> expect the pre-AdGuard resolvers restored.
# Then disable AdGuard -> expect dnsmasq back to its original value.

# VPN on, then yank the uplink (disconnect the upstream Wi-Fi).
# expect: LAN DNS recovers to the pre-VPN resolvers within the debounce window,
# WITHOUT the operator touching anything, and the VPN still shows disabled.

# VPN on, healthy tunnel. Just reload the dashboard repeatedly.
# expect: LAN DNS keeps using the VPN. It used to fall back on a single transient
# `wg show` failure and delete the snapshot.
```

---

## 6. Guards, failover, mwan3

```sh
# mwan3: hand-write a member that Travo did not create.
uci add mwan3 interface   # name it 'hotel', set proto dhcp, uci commit mwan3
# Save ANY failover setting in Travo.
# expect: 'hotel' SURVIVES. It used to be deleted silently.

# Crash guards: block a client, then block it again / fail a commit.
# expect: on any failure the guard file REMAINS.
ls -la /etc/trafo/       # the RIGHT directory. Not /etc/travo.

# USB tethering: make `uci commit firewall` fail (e.g. read-only overlay),
# then configure.
# expect: the guard file still exists, and the interface is not half-created.
```

---

## 7. AdGuard first-run wizard (decided risk — verify it is as bad as documented)

```sh
# Fresh install with AdGuard. From a DIFFERENT machine on the LAN, before the
# owner has set a password:
# open http://<router>:3000
# expect: the first-run setup wizard, and it is completable.
# THIS IS THE DOCUMENTED ACCEPTED RISK. Confirm it matches ADR 0001 §2.5 and
# then set the password from Travo -> System -> AdGuard Password.
```

---

## 8. Documentation gates are not a substitute for any of the above

`backend/internal/services/docs_*_test.go` now fails the build on guard-path, store-path, ADR-index,
plans-index, Go-pin and published-install-instruction drift. Those tests are unit tests; they say
nothing about whether the product works.

---

## Items that genuinely cannot be tested on one device

- **Multi-WAN failover** with two real uplinks (see
  [`failover-verification.md`](./failover-verification.md), still outstanding and still not a backlog
  item).
- **VPN DNS forwarding vs AdGuard across a reboot** needs both features enabled and a real tunnel.
- The **iptables/firewall restart** path in Block/Unblock needs a session you are willing to have
  dropped.
