---
title: "ADR 0002: Wireless model, health, and LuCI-style UCI apply"
status: Accepted
date: 2026-05-14
updated: 2026-10-04
tags: [adr, wireless, wwan, repeater, uci, rpcd, openwrt]
---

# ADR 0002: Wireless model, health, and LuCI-style UCI apply

## Status

Accepted.

## Context

Travel-router behavior depends on predictable **STA/WWAN**, **repeater** radio layout, and safe application of `wireless` (and related) UCI. OpenWrt’s **LuCI** uses rpcd **`uci apply`** with rollback and explicit **`uci confirm`** so a bad wireless change can time out back to the previous config. ath11k/IPQ6018 hardware is sensitive to **`wifi reload`**. Travo must align with these constraints while exposing **health** and **reconcile** actions to the UI.

## Decision

### 1. STA / WWAN ownership

- At most **one enabled** `wifi-iface` with `mode=sta` bound to **`network=wwan`**.
- Saved upstream networks are ordered and persisted (`wifi-priorities.json` under `/etc/travo/`); only one profile is active at runtime where the product model requires it.
- Any code path that creates or enables a STA for upstream use must **`ensureWwanNetwork`** (or equivalent): `network=wwan` exists and participates in the **`wan` firewall zone**. Violations are **errors**, not soft warnings.
- **Validation** before staging apply: `validateWirelessConsistency` rejects multiple active WWAN STAs (`ErrMultipleActiveSTA`).

### 2. Repeater and multi-radio layout

- With **two or more radios**, repeater mode prefers **STA and downlink AP on different radios** to avoid same-channel coupling and upstream-driven local AP failure on ath11k.
- **`allow_ap_on_sta_radio`** in `repeater-options.json` (`/etc/travo/repeater-options.json`) is the explicit escape hatch for same-radio STA+AP.
- **Repeater reconciliation** is a first-class API concern: changing AP or repeater options must not silently leave a fragile layout when a safer split exists.
- **Atomic reconcile rule (MUST):** Any function that activates a STA or moves a STA to a different radio **must** call `reconcileRepeaterAPRadioLayout()` *before* `uci.Commit("wireless")` in the same call. Committing an AP+STA-on-same-radio state even transiently is enough to crash ath11k/IPQ6018. Staging the STA change, reconciling (which stages AP enable/disable changes), and then committing once is the required pattern. The health-check banner and its "Fix" button are a fallback only — they must never be the primary path for avoiding a crash.

### 3. AP credential model

- Default UX: **one shared SSID/password** across enabled downlink AP sections; per-radio enable remains visible; optional per-radio overrides may exist behind UI toggles.

### 4. Health and recovery API

- **`GET /api/v1/wifi/health`** reports invariant violations (e.g. fragile repeater layout) and drives warnings plus **reconcile** actions in the frontend.
- **Auto-reconnect** scripts use a **failure counter** and crash-guard file under `/etc/travo/` (see ADR 0003) so broken credentials are not retried forever.

### 5. LuCI-style apply / confirm (user-driven wireless changes)

- **`RealUCIApplyConfirm`** (`backend/internal/services/uci_apply.go`) implements rpcd session login, copies **`/etc/config/{wireless,network,system,firewall,dhcp}`** into the session tree, calls **`uci apply`** with **`rollback: true`** and **30s** timeout, then **`uci confirm`** only when invoked after success.
- **`WifiService.stageWirelessApply`**: validates consistency → **`StartApply(uciApplyConfigs)`** → returns a **token** (session id) and rollback timeout for the client.
- **`WifiService.ConfirmApply(token)`** calls **`Confirm`** after the browser proves reachability. The backend **must not** self-confirm immediately after `StartApply` without that proof (see `docs/architecture/overview.md` §3).

#### 5.0 The rollback is our own file-level snapshot, not rpcd's window

**Hardware finding (GL.iNet GL-AXT1800, OpenWrt 25.12.3).** Both rollback mechanisms
travo relied on restored **nothing**, because travo **commits the UCI change before either
of them runs**:

1. **rpcd's rollback window.** `StartApply` copies `/etc/config/<name>` into the session dir
   and *then* calls `uci apply` with `rollback:true`. rpcd snapshots at **apply** time, so its
   rollback target is the **already-changed** config: when the 30 s window expired without a
   confirm, rpcd faithfully restored the change it existed to undo. Observed on the device:
   after a refused confirm and the full window, the working uplink section was still disabled
   and the broken profile still enabled.
2. **`uci revert <config>`** (in `mutateUCI`) only discards **uncommitted** staged changes, so
   for any writer that already committed it is a no-op — which every wireless mutator does,
   before `stageWirelessApply`.

So the probe was right to refuse; there was simply nothing behind the refusal.

**The decision.** Travo owns the snapshot:

- **`UCIApplyConfirm.Snapshot(configs)`** copies the named `/etc/config/<file>`s into a
  **pending** directory under `/var/run/rpcd` **before the mutation commits**, together with a
  manifest recording each name as `present` or `absent` (an `absent` name is deleted on
  restore, so a config the mutation created does not survive it). **One pending snapshot is
  enough**: `mutateWireless` holds the wireless write lock across snapshot → mutate → apply, so
  no second snapshot can be taken in between. A queue would be dead code.
- **`StartApply`** keeps the rpcd session + `uci apply` flow — harmless, and still rpcd's
  **crash-time** rollback — and then keys the pending snapshot to the session id it returned.
- **`Confirm(sessionID)`** discards that session's snapshot. This is the success path, and a
  discarded snapshot must never be able to undo a *later*, unrelated change.
- **`Rollback(sessionID)`** copies the snapshot back over `/etc/config/<name>`, cancels rpcd's
  window (`uci confirm`, so the window cannot fire later and **overwrite the restore with the
  already-committed config**), then asks the device to act: **`ubus call uci reload_config`**
  (rpcd re-reads the config files) and **`ubus call network reload`** (netifd re-reads
  `/etc/config/network` **and** `wireless`). An empty `sessionID` means the pending snapshot.
  **A missing snapshot is an error, never a reported success** — a rollback that cannot find
  what to restore must say so.
- **`WifiService.mutateWireless`** snapshots inside the locks it already holds, and restores
  when `fn` fails. That closes the second gap: a mutation that **committed and then failed**
  (including a `stageWirelessApply` whose `StartApply` failed, leaving no session at all) was
  previously not rolled back in any way.
- **`WifiService.ConfirmApply`** calls **`Rollback(token)`** when `verifyAppliedWirelessUp`
  refuses. **The probe's verdict must be followed by the action it implies**; returning the
  error and leaving the restore to a window that restores the wrong config is what stranded
  the operator.
- **`ApplyAndConfirm`** (guarded internal flows) snapshots, applies, and on any failure rolls
  back by hand — there is no session left to time out, and `uci revert` cannot undo a commit.

**Recovery is the only place that reloads, and it is bounded.** `network reload` is the
explicit **bounded-recovery exception** to the "never run `wifi` / `wifi up`" rule — see
[ADR 0003](./0003-crash-guards-and-live-state.md) §3.1. `wifi`, `wifi up` and `wifi reload`
are **never** run by the rollback, on any path.

#### 5.1 What the confirm probe reads

`ConfirmApply` proves the apply on the **device**, before `applier.Confirm`, so a
change that cannot bring WiFi up reverts to the previous config. The proof
(`appliedWirelessUp`) answers, for every AP and uplink-STA section the applied
config enables:

1. netifd **lists** the section (it is present under a radio's `interfaces`), and
2. the owning radio is **up and settled** (`up` && !`pending` && !`retry_setup_failed`)
   **and it carries no other expected interface**, **or** the interface itself
   answers `ubus call network.device status {"name":"<ifname>"}` with
   `present` && `up` && `carrier`.

Both conditions are required: listing alone proves netifd created the section,
not that it works; the device call is the only per-interface liveness netifd
exposes and is the same object `NetworkService` already reads, so it adds no
dependency. A section that is not listed, or whose signals are false, is **down**:
the probe fails closed and the rollback stays armed.

**A radio flag is proof only when it is the only expected interface on that
radio.** netifd reports liveness per RADIO, so on a radio hosting two wanted
interfaces the first one to come up settles the radio — and reading it that way
proved the second one too. That is reachable in the product, not a theory:
`SetGuestWifi` excludes only the uplink radio, so on the captured layout (uplink
STA on radio0, main AP on radio1) the guest AP lands on radio1 **beside** the
enabled main AP. With the guest up and the main AP down, the probe confirmed,
cancelled rpcd's rollback and left the operator with no main SSID. So when a
radio carries more than one expected interface, each of them has to answer
`network.device status` for itself; the per-device answer is the authority. The
single-interface case is unchanged and still costs no extra ubus round-trip.

**netifd reports liveness per RADIO on this firmware, not per interface.**
OpenWrt 25.12.3 / netifd 2026.02.26-r1 emits `up`, `pending`,
`retry_setup_failed` and `autostart` on the radio entry, and a per-interface
entry carries exactly `{section, config, ifname, vlans, stations}` — there is no
per-interface `up` and no `config_path`. A test fixture that adds such a field
makes a probe that can never pass on hardware look green; the golden fixture in
`wifi_service_test.go` is the verbatim device payload for that reason.

A payload that lacks the keys the probe reads is **not** silently down, and it is
**not** allowed to abort the whole proof: an entry this build cannot read is
skipped and reported (`ErrWirelessUnverifiable`), while the radios that do read
are proven normally. One malformed radio the apply has nothing to do with used
to convert a transient netifd hiccup into a failure that was never retried. The
two outcomes are still distinguished — if a section that is still unproven
belongs to a radio this build could not read, the answer is
`ErrWirelessUnverifiable` ("I cannot read this"), otherwise `ErrWirelessNotUp`
("your access point did not come up"). Both keep the rollback armed; only one
sends the operator looking at their own config, and the other means "this build
cannot read netifd's answer", which is a different thing to go and look at.

A transport failure is folded into the same condition: when
`ubus call network.wireless status` itself fails (ubus socket gone, netifd
restarting the object, call timeout) Travo knows nothing about the change, so it
is `ErrWirelessUnverifiable` too — never `ErrWirelessNotUp`, which claimed a
rollback that had not happened yet. That subset is marked with
`errWirelessStatusUnread` and is retried inside the budget (netifd restarting is
the normal case and is readable a moment later), while a payload in an unknown
shape is reported on the first probe, because waiting cannot make it readable.

**The two conditions must stay distinguishable end to end, on the wire.**
`ErrWirelessUnverifiable` is the one case where the frontend must NOT claim a
rollback: rpcd's rollback window is still open and nothing has been reverted yet,
so the operator is told that Travo could not verify the change and that the
router will revert it by itself if the settings do not come up. In Go that means
`ErrWirelessUnverifiable`'s message must not start with `ErrWirelessNotUp`'s —
`wireless apply could not be verified:` versus `wireless apply not verified:` —
because the client matches on that substring (`isWirelessNotVerifiedError` /
`isWirelessStatusUnreadableError` in `frontend/src/lib/wifi-apply.ts`). A shared
prefix once made an unreadable payload throw `WifiApplyRolledBackError`, which
told the operator the router had already rolled back. The client re-checks both
substrings whenever either constant changes.

**Attempt and budget constants** (`wirelessConfirmAttempts` = 3,
`wirelessConfirmDelay` = 5 s, budget = 12 s): sized from what the device actually
takes. After `uci apply`, netifd restarts the radios and hostapd must finish ACS
before `phy1-ap0` links up, which the device log puts at **~10 s**; the first
probe is immediate, so the common "already up" case still answers in well under a
second.

**The published budget is the worst-case blocking time, not the sleep total.**
A probe blocks on its sleeps *and* on its ubus round-trips: one
`network.wireless status` per attempt plus one `network.device status` per
expected interface on an unsettled (or multi-interface) radio. Budgeting only the
sleeps — 8 probes x 2 s = "14 s" while the probe really blocked for ~30 s — is
what moved the client's last probe to 0.5 s before rpcd's rollback, turning a
clean revert into a confusing failure. The accounting is now explicit:

| term | value | why |
| --- | --- | --- |
| waits between retries | 2 x 5 s = 10 s | covers the measured ~10 s link-up |
| ubus calls per attempt | 1 status + up to 4 device = 5 | one fallback per expected interface; more than any layout this service produces |
| budget per ubus round-trip | 100 ms | measured on the device: wireless status ~4 ms, device status ~5 ms including the ubus client; 100 ms is a ~20x allowance for a busy netifd |
| **published budget** | **10 s + 3 x 0.5 s = 12 s** | `wirelessProbeBudgetSeconds()` |

Fewer, longer waits cover the same device latency as 8 x 2 s at a sixth of the
round-trip cost, which is the whole reason the attempt count dropped. The budget
is published as `apply.probe_budget_seconds`; the client subtracts it plus
**`PROBE_SAFETY_MARGIN_MS` (2 s, the same number the Go gate
`wirelessProbeSafetyMarginSeconds` uses)** from the 30 s rollback window and caps
that reservation at half the window: 12 + 2 = 14 s against the 15 s cap, so the
cap does **not** bind, and the last probe is answered ~2 s before rpcd rolls
back. If the cap ever binds, the reserved time is truncated below the blocking
time and the final probe would be answered after the rollback — the Go gate
(`TestConfirmApply_ProbesImmediatelyAndReportsProbeBudget`) fails on that, and the
same check on the client side asserts the headroom. A device with more than four
expected interfaces spends more than the published budget; it still fails closed,
and the client stops probing early rather than late.

**Ranked alternative liveness signals, and why they are not used:**

1. `stations[]` in the wireless status — rejected: verified empty for a healthy
   zero-client AP (`phy1-ap0`, up and beaconing), and structurally always empty
   for a `mode=sta` interface (`phy0-sta0` had a live peer and still reported
   `stations: []`).
2. `iwinfo assoclist` — rejected for the same reason: a correct AP with no clients
   is not a failed AP.
3. Hostapd control socket / `ubus call hostapd.<phy> get_status` — capable but adds
   a dependency per radio; the radio+device signals cover the same ground.
4. Ping/L3 reachability of the AP address — confuses "AP up" with "client can talk
   to it", and cannot speak for a client-mode apply with no AP at all.

Device-side verification steps and captured payloads — including the ubus
round-trip timings the budget is built from — live in
[`docs/tests/on-device-verification.md`](../tests/on-device-verification.md); this
section records the decision, not the procedure.

#### 5.2 The lockout guard: refuse unless acknowledged

**The guarantee.** A mutating wireless request that would leave the operator who
made it with no usable access point is **refused before anything is written**,
unless the request explicitly acknowledges it. It is enforced **server-side**;
the UI is a convenience, not the enforcement point.

**Found by on-device testing, not by a test.** On a GL.iNet GL-AXT1800
(OpenWrt 25.12.3) an operator joined the access point from an iPhone and switched
the WiFi mode to Client. The access point was removed, the iPhone was
disconnected and **could not rejoin**; only a separate Ethernet console brought
the radio back. Nothing in the test suite failed, because nothing in the test
suite asserted this: the mode switch had no reference to the caller's connection
method anywhere in the service or the API. The unit tests were green *because*
the behaviour was simply absent — which is the point worth keeping in this
record. A guard that is only reachable by a human stranding themselves on real
hardware is not a guard the suite can prove.

Note what did **not** cause it: the confirm probe behaved correctly throughout.
In Client mode there are no access points to prove and the uplink came up fine,
so it confirmed a config that was working exactly as written. **The probe is not
responsible for this property and must not be changed to take it over.**

**The rule, in one place** (`backend/internal/services/wifi_lockout.go`,
`WifiService.guardLockout` / `guardLockoutExcluding`) so it cannot drift
per endpoint:

> refuse when the caller is connected over WiFi (`wifi-client` or `wifi-ap`,
> classified by `NetworkService.GetConnectionMethod` — the same code the
> `GET /network/connection-method` endpoint uses) **and** the resulting config
> would have no enabled `mode=ap` wifi-iface left on any radio.

**How the caller's connection method is decided.** The classifier
(`classifyClientConnection`, `backend/internal/services/network_service.go`) maps
the caller's IP onto the interface it is reachable through using
`ubus call network.interface dump`, and then onto a connection method. Two
things about that are load-bearing and both were wrong for a long time.

*The dump shape.* netifd reports each IPv4 address as a **bare address plus a
separate integer netmask length** — `{"address": "192.168.1.1", "mask": 24}`.
The classifier instead matched an `ipv4-prefix` key carrying a CIDR string, a
key the device does not emit anywhere. Every interface therefore contributed no
prefixes, nothing matched, and **every real client — wired and wireless alike —
classified as `unknown`**. The unit tests were green throughout, because their
fixture fed the classifier the invented key: a test asserting a fiction the
device never produces. This is the same defect class as the original P0 on this
branch (a fixture inventing a field the system does not emit), now in a third
place. The parser now accepts the real `{address, mask}` form *and* a CIDR
string in either `ipv4-address` or `ipv4-prefix`, because netifd builds differ;
a malformed entry is skipped rather than emptying the whole list, since an empty
list is exactly what makes everything `unknown` again. The fixture is now the
verbatim captured payload in
`backend/internal/services/testdata/network_interface_dump.json`, with its
provenance in the file — not a hand-written map.

*The medium.* Fixing the prefix alone is **not sufficient**, and it would have
been a new bug. On this hardware `br-lan` carries wired and wireless clients in
the same /24, so a subnet match resolves every LAN client to `br-lan` — and the
pre-existing rule answered `wifi-ap` for `br-lan`. That would classify the
operator's wired console as WiFi and refuse them, the one false positive the
guard's own tests forbid. A client on the LAN bridge is therefore resolved to a
MAC through the **neighbour table** (`/proc/net/arp`, the same reader the client
list uses, so the two cannot disagree) and that MAC is checked against the
**access points' station lists** (`iw dev` → each AP interface → `station
dump`), which is the mechanism the health path already uses. Associated →
`wifi-ap`. Not associated → `ethernet`. The uplink STA still answers
`wifi-client` from the interface name alone, because there the router itself is
the WiFi client. Real identities from the test device: `192.168.1.2` /
`02:00:00:00:00:01` (wired) and `192.168.1.151` / `02:00:00:00:00:02` (the
iPhone, associated with `phy1-ap0`) — same bridge, same subnet.

**It fails closed on `unknown`.** The guard refuses for every method except a
proven `ethernet`. A caller the classifier cannot place — an IP outside every
interface prefix, a MAC with no neighbour entry, a bridge whose station dumps
could not be read — is **refused unless acknowledged**, not waved through. This
is a deliberate choice of direction, and it is the safety-relevant decision in
this section:

- A refusal costs the operator one acknowledgement they can read and undo. The
  dialog names the remedy and, as before, the acknowledged request proceeds.
- Allowing it is unrecoverable: a router with no reachable access point cannot
  explain itself, and the operator has no way back short of physical access.
  That is precisely the failure this guard was written for.

The cost is a false positive on the case that used to be free — a wired
operator whose neighbour entry has not aged in gets a dialog instead of a silent
apply. That is the right trade, and it is the direction the owner ruled out in
the opposite direction. `unknown` is still an honest answer from the
`GET /network/connection-method` endpoint; it is the *guard* that reads it as
unsafe. Nothing was weakened to make a test pass.

Why "no enabled AP left anywhere" and not "the caller's own AP goes away": it
is conservative in the safe direction, it cannot strand anyone it does not
refuse, and it does not fire when the operator is on the other radio and that
radio's AP stays up — the everyday "turn 5G off while I am on 2.4G". A narrower
rule would add a second thing to keep in agreement for no extra safety.

**Endpoints covered.** All five mutators that can remove an access point:

| endpoint | removes the AP when |
| --- | --- |
| `PUT /api/v1/wifi/mode` | mode `client` (Client removes every access point) |
| `PUT /api/v1/wifi/radio` | `enabled: false` |
| `PUT /api/v1/wifi/radios/{name}/role` | role `none` / `sta` on that radio |
| `PUT /api/v1/wifi/ap/{section}` | `enabled: false` on that section |
| `PUT /api/v1/wifi/guest` | `enabled: false` |

Anything that cannot strand the caller — an SSID change, a key change, switching
a radio off while the other radio's AP stays enabled — proceeds exactly as
before. Refusing those would make the product unusable.

**Refusal shape.** `ErrLockoutRefused` (the `ErrAPAndSTASameRadio` pattern) maps
to **409 Conflict** with a stable machine-readable `code: "wifi_lockout_risk"`
next to the message, so the frontend keys off the code and never off the message
text. The message names the remedy: you are connected over WiFi, this removes the
access point you are using, connect over Ethernet first.

**The opt-in and what "refused" means.** The request bodies accept
`acknowledge_lockout: true`; with it the change proceeds. Without it the guard
runs **before** the mutation starts, so a refused request writes **no UCI
change, no staged apply and no apply session** — the guard is not a rollback,
it is a refusal. Pinned by the tests that compare the whole `wireless`,
`network`, `dhcp`, `firewall` and `system` config against a pre-request dump and
assert the apply session count did not move.

**Client.** The 409 raises a dialog that cannot be accepted without ticking a box
(`frontend/src/components/wifi/wifi-lockout-dialog.tsx`); only then is the same
request re-sent with `acknowledge_lockout: true`. Wired in the mode card, the
per-radio access-point section and the radio-hardware disable path.

### 6. Scripts, packaging, and `wifi` commands

- **User-facing** wireless mutations go through the apply/confirm path above when `applier` is configured; they **must not** run **`wifi`**, **`wifi up`**, or **`wifi reload`** as part of apply (matches `docs/architecture/overview.md` §3).
- **Install / uci-defaults** flows write UCI only; the operator applies via LuCI **Save & Apply** or reboot.
- **`applyWireless`** may use **`ApplyAndConfirm`** for **internal, synchronous** guarded paths when `applier` is set; when `applier` is nil (e.g. some tests), a **`Reloader`** path may exist—production device wiring uses the real applier.

#### 6.1 The generated wireless toggle helper

`wifi` / `wifi up` / `wifi down` / `wifi reload` are still **forbidden** from scheduled
and hotplug paths — they are the classic ath11k/IPQ6018 driver-crash trigger, and a
timer- or button-driven `wifi down` has no rollback. Those paths therefore call a
**backend-generated helper** instead:

- **Path:** `/usr/libexec/travo-wireless-toggle.sh` (mode `0755`), generated and owned
  by the backend from `backend/internal/services/wifi_toggle_script.go`. It is
  regenerated by `writeWirelessToggleScript()` on every path that can schedule a
  toggle, so it is idempotent and cannot drift.
- **Interface:** `travo-wireless-toggle.sh up|down`. `up` sets
  `wireless.@wifi-device[*].disabled=0`; `down` sets it to `1`. Any other argument
  exits `2`.
- **Mechanism:** `uci set` → `uci commit wireless` → rpcd
  `ubus call session login` → stage `wireless` into `/var/run/rpcd/uci-<sid>` →
  `ubus call uci apply '{"ubus_rpc_session":"<sid>","rollback":true,"timeout":30}'`
  → `ubus call uci confirm '{"ubus_rpc_session":"<sid>"}'`. A missing apply session,
  a failed set/commit, or a failed confirm runs `uci revert wireless` and exits
  non-zero. This is the same rollback-then-confirm shape as §5, expressed in shell
  because the caller is cron/procd rather than a browser. It mirrors
  `RealUCIApplyConfirm.StartApply` / `Confirm` in `uci_apply.go` and
  `scripts/setup-wireless-ap.sh`.
- **Session-scoped apply, and why the login cannot be anonymous:** rpcd's
  `uci apply`/`uci confirm` are session-scoped. `rpc_uci_apply` returns
  `INVALID_ARGUMENT` unless the caller presents a `ubus_rpc_session`, and the bare
  `ubus` CLI injects none — so the helper must log in first and pass that id on
  **both** calls. It cannot log in with an empty password: stock
  `/etc/config/rpcd` ships `option password '$p$root'`, which makes rpcd verify
  against the real system root password, so an anonymous login succeeds only on a
  device whose root account has no password at all.
- **Login-argument file (operational constraint):** because the helper runs from
  cron and hotplug with no travo process to ask, the backend writes the whole
  `ubus call session login` argument — JSON-encoded by `encoding/json`, root-only
  `0600` — to **`/etc/travo/rpcd-login.json`** (`auth.RPCDLoginHelperPath`), beside
  `rpcd-login.sealed`. It is written on every successful login and refreshed at
  startup from the seal. The helper passes those bytes to `ubus` verbatim; it never
  escapes anything itself, because reimplementing JSON string escaping in `sed` is
  not portable (BSD sed and BusyBox sed disagree on `s/\\/\\\\/g`). The sealed blob
  cannot serve the helper directly — unsealing needs the `jwt_secret`, which lives
  in the same root-only `auth.json`. With no such file the helper falls back to an
  empty password, which is correct only on a device with no root password. **If
  this file goes missing, every scheduled and button-driven WiFi toggle silently
  becomes a no-op**; `/etc/trafo/wifi-toggle-in-progress` will not appear, because
  the helper fails before the first mutation.
- **Guard:** writes `/etc/trafo/wifi-toggle-in-progress` before the first mutation
  and removes it only after a confirmed apply. While the guard exists the helper logs
  and exits `0` without touching the radios (ADR 0003 §2).

**Both** toggle paths go through this helper, and neither runs `wifi up`/`wifi down`:

| Caller | Where it is generated | What it runs |
| ------ | --------------------- | ------------ |
| WiFi on/off **schedule** | `WifiService.SetWiFiSchedule` writes `/etc/crontabs/root` | `/usr/libexec/travo-wireless-toggle.sh up` / `… down` |
| **Hardware button** WiFi toggle | `SystemService.buildButtonHotplugScript` writes `/etc/hotplug.d/button/50-gui-button-actions` (`0755`, run as root by procd) | `iwinfo` presence check, then `/usr/libexec/travo-wireless-toggle.sh down` / `… up` |

The button's VPN toggle still uses `ifstatus`/`ifdown`/`ifup` on `wg0` — that is
netifd interface control, not a wireless driver reload, and is out of scope for this
section.

**The one remaining `wifi up` exception** is the bounded auto-reconnect path (§4,
`wifi_reconnect.go`): a cron minute-tick that runs `wifi up` behind
`autoreconnect-crash-guard` and the `MAX_FAIL=5` counter. It stays narrow, guarded,
and documented; nothing else may add a `wifi` invocation.

**Validation on the schedule path** is a precondition of generating cron at all: the
`on_time`/`off_time` values are formatted straight into a root crontab line, so
`services.ValidateHHMM` (`^([01]\d|2[0-3]):([0-5]\d)$`, no whitespace/newline) is
enforced at the HTTP handler **and** re-checked inside `SetWiFiSchedule`.

## Consequences

- New wireless features that touch `network`/`firewall`/`dhcp` must consider whether copied config set in **`uciApplyConfigs`** needs extending so rollback snapshots stay consistent.
- Same-radio repeater remains supported but must stay **opt-in** and **visible** in health API responses.

## References

- `backend/internal/services/wifi_service.go` — invariants, `stageWirelessApply`, `ConfirmApply`, confirm probe (§5.1), `uciApplyConfigs`
- `backend/internal/services/uci_apply.go` — rpcd apply/confirm, the file-level `Snapshot`/`Rollback` recovery path (§5.0)
- `backend/internal/services/wifi_toggle_script.go` — the generated toggle helper
- `backend/internal/services/wifi_reconnect.go` — schedule cron file, bounded `wifi up` exception
- `backend/internal/services/system_service.go` — button hotplug script
- `backend/internal/services/validate.go` — `ValidateHHMM`, `ValidateButtonName`
- `docs/architecture/overview.md` §2–3
- [ADR 0003](./0003-crash-guards-and-live-state.md) — guard contract

## Addendum: radio choice determinism and one definition of "the uplink radio"

**Which radio an operation picks must be deterministic.** `radioForNewSTA`
(`wifi_connect.go`) sorts the radios by name and prefers one with no enabled access
point; `ensureSTASectionForScan` (`wifi_scan.go`) uses the same chooser instead of
taking the first radio a map range turned up. `GetRadios` and `GetAPConfigs`
(`wifi_ap.go`) return their slices ordered by section/radio name, and the band
switcher (`band_switching_service.go`) sorts before it picks "the radio for this
band" or "the other radio". A randomised choice here is not cosmetic: it decides
whether the uplink shares a PHY with an access point, and on a device where two
radios report the same band it makes the band switcher oscillate.

**"The uplink radio" means one thing: the radio of any enabled `mode=sta`
wifi-iface.** A `network=wwan` STA wins when several radios qualify, but a
hand-written STA without `network=wwan` still occupies its PHY, so it counts for
every guard. The strict direction is deliberate — the constraint is physical, and
a loose definition let the health API report a radio as "both" while the same-radio
guards saw no uplink on it. `activeIfaces` is the single predicate the guards and
the role detection read.

**Every writer that enables a STA on a radio runs the same refusal.**
`rejectSTAOnAPRadio` refuses a radio that carries an enabled access point with
`ErrAPAndSTASameRadio` (409 via `respondWifiMutationError`); `SwitchSTAToRadio` uses
it before any write, because it applies immediately and
`reconcileRepeaterAPRadioLayout` does nothing outside repeater mode. Single-radio
hardware and `allow_ap_on_sta_radio` remain the documented exemptions. A refusal is
not a crash, so the band switcher removes its crash guard for one — otherwise one
refused tick would disable automatic band switching until somebody removed the file.

**Guest WiFi refuses a subnet it cannot own.** `192.168.2.0/24` is fixed, so
`SetGuestWifi` checks it against every configured `network` interface and returns
`ErrGuestSubnetOverlap` instead of creating a second interface on one network.
