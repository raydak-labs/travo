---
title: On-device verification
status: standing — run whenever a change touches wireless, DNS/VPN layering, guards or install
last-verified-against: 192.168.1.1 (GL.iNet GL-AXT1800, OpenWrt 25.12.3), branch
fix/deep-review-2026-10-04 @ 3634ea6
---

# On-device verification

Unit tests cannot settle whether the router still works. They can tell you a mock returns the right
bytes; they cannot tell you whether ath11k brought the access point up, whether dnsmasq ended up
pointing at a dead resolver, or whether a rollback restored what it was supposed to.

This is the standing playbook for those questions. It replaces the temporary punch list
`VERIFY-on-device-2026-10-04.md`, which existed only because the device was unreachable while
`fix/deep-review-2026-10-04` was reviewed and remediated. The device is reachable; the list is now
permanent, and its durable findings are recorded in §3.

**Do not run any suite here while the branch under test has a known defect in §2.** A suite run
against a build that is known to be broken tells you nothing except that the build is broken.

---

## 1. Prerequisites

| # | Requirement | Why |
| - | ----------- | --- |
| 1 | **An Ethernet console on the LAN.** Non-negotiable for §5. | Wireless/lockout tests take the AP down. On `192.168.1.1` the managing host reaches the router *through the router's own Wi-Fi uplink*, so a failed apply strands the operator with no path back but a power cycle. |
| 2 | A **second known-good AP** to fall back to, if Ethernet is impossible. | Same reason. |
| 3 | **A spare device** for §4 (fresh install). | `install.sh` overwrites the live install, the root password and AdGuard. Do not run it on the device you need. |
| 4 | Two real uplinks (Ethernet + WiFi, or WiFi + USB tether) for §7. | Multi-WAN cannot be validated on a single uplink. |
| 5 | `ssh root@192.168.1.1` working. | Every command below is written for `ssh`. `rg` is **not** on the device; use `grep`. |

`make deploy ROUTER_IP=192.168.1.1` (or `scripts/deploy-local.sh`) is the deploy step. It is also
the sanctioned retry path that clears crash-guard files.

---

## 2. Defects already found and fixed

Found on `fix/deep-review-2026-10-04` and since fixed. They are listed because every one of
them is a class of bug that a green unit suite demonstrably cannot catch, so a future change in the
same area deserves the same scepticism.

| # | Defect as found | Why the suite stayed green | Device check |
| - | --------------- | ------------------------- | ------------ |
| B1 | The confirm probe read a per-interface `up` field that netifd emits per RADIO only, so every wireless apply rolled back. | The test fixture invented the missing field. | B.1, B.2 |
| B2 | `ensureSTASectionForScan` picked the uplink radio by ranging a map. | Its test seeded a state that skipped the code path. | B.5 |
| B3 | The probe budget was 4 s, shorter than the ~10 s AP bring-up (§3.2). | Constants were asserted against themselves. | B.1 |
| B4 | `SetMode("repeater")` failed **open** with every enabled AP on the uplink radio. | Only the happy layout was covered. | B.7 |
| B5 | `SwitchSTAToRadio` had no same-radio check at all. | No test existed for it. | B.5 |
| B6 | The probe then **failed open** when one radio hosted two APs — reachable by enabling guest WiFi beside the main AP — disarming the rollback with a dead AP. | The fixture baked in coarse per-radio semantics. | B.6 |
| B7 | `Scan()` committed an uplink STA onto a radio that runs an AP. | Determinism had been fixed; the guard had not. | B.5 |
| B8 | `deploy-local.sh` cleared crash guards *before* restarting, so a failed deploy erased the only record that the device was mid-recovery. | Every claim about the shell scripts was a string grep. | D.2 |
| B9 | The probe accepted an uplink STA that **never associated**: a settled radio was taken as proof, and a configured-but-unassociated station reads `up:true, carrier:false`. The apply confirmed and cancelled the rollback that would have restored the working uplink. | `MockUbus` ignores the requested device name, so no test could give one interface `carrier:false` and another `carrier:true`. | B.9 |
| B10 | **The rpcd rollback window does not revert a wireless change.** `StartApply` copies the already-committed config into the rpcd session dir *before* calling `uci apply`, so the rollback target is the changed config. When the probe refuses to confirm, the config stays applied. | Nothing tested the window *expiring*; only the confirm path was covered. | **KNOWN BROKEN — B.10** |

B9 is fixed: an uplink STA is now proven only by `network.device status` answering
`present && up && carrier`, because carrier is the only signal that says the station associated.

B10 is **not** fixed and is the most serious item here. It is pre-existing rather than introduced by
this branch, and it means "the change is rolled back" describes intent, not observed behaviour, on
the wireless path.

---

## 3. Durable device facts

Measured on `192.168.1.1`, 2026-10-04. These are facts about this hardware and netifd, not about a
build. They stay true until the firmware changes, and several of them are the answer to questions
the codebase should not have to guess at again.

### 3.1 What netifd actually reports

- `up`, `pending`, `retry_setup_failed`, `autostart` exist **per radio only**.
- A per-interface entry carries exactly `section`, `config`, `ifname`, `vlans`, `stations`.
- There is no per-interface `up`, and no `config_path`, on netifd `2026.02.26-r1`.
- Verified stable across two boots.

Authoritative "is this really up" signals on this hardware, best first:

1. `ubus call network.device status '{"name":"<ifname>"}'` — `present`, `up`, `carrier`. The
`ifname` is already available from the wireless status, so this costs one extra call per interface.
2. Radio-level `up && !pending && !retry_setup_failed` from the payload the probe already fetches.
3. `/sys/class/net/<if>/operstate`, `carrier`, `flags`.

Do **not** use `stations[]` or `iwinfo assoclist` as a liveness signal: they are empty for a healthy
zero-client AP, and structurally always empty for a `mode=sta` interface (`phy0-sta0` had a live
peer and still reported `stations: []`).

### 3.2 An AP with no clients is genuinely up

`phy1-ap0` (SSID `Cappuxinno-Travel`, channel 1) was beaconing, bridged to `br-lan`, with
`network.device status` reporting `present/up/carrier` all true and a **completely empty**
assoclist. An ath11k AP with zero stations reports up. The original worry was unfounded; the real
defect is B1.

Bring-up latency from `logread`:

```sh
logread | grep -iE "ath11k|netifd" | grep -E "wifi-scripts: Starting|link is up|is now up"
```

- `phy1-ap0`: ~10 s from radio start to link up.
- `phy0-sta0`: ~4 s; the `wwan` interface reaches up at ~10 s once DHCP completes.

The uplink-STA proof therefore cannot rely on association alone — association plus
DHCP took ~10 s.

### 3.3 Device baseline at the time of writing

- Branch **not deployed**: `/usr/bin/travo` dated 2026-10-01, self-reports `v3.5.0`. The branch
  under test is much newer. Confirm provenance before drawing any conclusion (suite A).
- `/etc/trafo/` exists, mode `0750`, and was **empty** — no stuck guards. Correct per
  ADR 0003 §2.
- `/etc/travo/travo.db` is `0600` — ADR 0009's path and mode are right.
- dnsmasq config lives in a **named** section (`dhcp.cfg01411c`), not `@dnsmasq[0]`. Most docs write
  `@dnsmasq[0]`; tooling that assumes the anonymous index will be wrong on a real device.
- mwan3 is **not installed** on this device, so §6 cannot run here as-is.
- The uplink is 5 GHz (`sta0` on radio0); radio1 carries only the AP.

---

## 4. Suite A — provenance and the auth gate

**Run first. Everything else is meaningless against the wrong binary.**

```sh
ssh root@192.168.1.1 'ps w | grep travo | grep -v grep; stat -c %y /usr/bin/travo'
ssh root@192.168.1.1 'logread | grep -i travo | head -3'
```

The binary's mtime and self-reported version must match the branch under test. If they do not, stop:
you are about to measure the wrong build.

```sh
for p in /api/health /api/openapi.json /api/v1/system/info /API/v1/system/info /Api/V1/System/Info \
/api/V1/system/info /api/v1/Wifi/Radios; do
printf '%s %s\n' "$p" "$(curl -s -o /dev/null -w '%{http_code}' "http://192.168.1.1$p")"
done
```

Expected:

| Path | Expected | Why |
| ---- | -------- | --- |
| `/api/health` | 200 | public |
| `/api/openapi.json` | 200 | public by design (ADR 0006) |
| `/api/v1/system/info` | 401 | protected |
| `/API/v1/system/info` | **404** | case-sensitive routing; 200 here is the P0 bypass |
| `/Api/V1/System/Info` | **404** | same |
| `/api/V1/system/info` | 401 | lower-cased `v1` still matches no route |
| `/api/v1/Wifi/Radios` | 401 | protected, and carries SSID/key material |

> **Before deploying:** the binary currently on the device has no auth on the admin API. An
> unauthenticated `POST /API/v1/system/reboot` from the LAN returned 200 and rebooted this router
> during verification. Do not probe mutating endpoints against the old build — establish
> reachability with GET-only probes first.

Also confirm, in a browser, that live stats keep ticking (the WebSocket must open) and that the
pre-login clock-recovery path still responds with no token.

---

## 5. Suite B — wireless apply and rollback (ADR 0002 §5)

**Requires prerequisite §1.1.** Take the console over Ethernet first and confirm you can reach the
router with Wi-Fi off entirely.

| # | Action | Expected |
| - | ------ | -------- |
| B.1 | Over Wi-Fi, change an AP SSID. | Applied and **confirmed**; no "rolled back" toast. |
| B.2 | Same, with a deliberately broken config (11-character key, impossible channel). | Rolled back to the previous working config after ~30 s; the UI says so specifically. |
| B.3 | Switch to Client mode **while connected over Wi-Fi**. | **Rollback.** The AP returns. This is the lockout case the model exists for. |
| B.4 | Switch to Client mode over **Ethernet**. | Applies and confirms. |
| B.5 | Connect to an upstream twice, no band pinned. | Both succeed. |
| B.9 | Connect to an upstream that **does not exist**, or with a wrong password. | Must be **refused**: the uplink STA has no carrier, so it is not up. This is the only reliable way to make the uplink genuinely fail — an invalid channel is silently ignored and the access point stays up. |
| B.10 | After a refused confirm, wait out the 30 s window. | **KNOWN BROKEN:** the config is *not* reverted. Verify it yourself rather than trusting the rollback, and restore the working uplink by hand. |
| B.6 | Client mode with the uplink on 2.4 GHz, then enable guest Wi-Fi. | Guest AP lands on the radio that does **not** carry the STA: `uci show wireless` — guest's `device` vs the `wwan` STA's. Both APs may end up on one radio; the confirm must still prove **each** interface, not the radio. |
| B.7 | With every enabled AP on the uplink radio, switch to Travel/repeater mode. | Must be **refused**, matching `Connect`'s behaviour. See B4. |
| B.8 | Press Scan, or switch to Client mode, with no saved upstream on a stock layout where both radios run an AP. | The AP is moved off the uplink radio before the STA is enabled; `uci show wireless` must never show an enabled AP and an enabled STA on one `device`. See B7. |

Assertion for B.6:

```sh
uci show wireless | grep -A4 'wireless.guest'   # device = the non-STA radio
uci show wireless | grep 'network.*wwan'        # the STA's radio
```

---

## 6. Suite C — DNS layering with VPN and AdGuard

```sh
uci show dhcp | grep -E 'server|noresolv|port'   # note the section may be named, not @dnsmasq[0]
ls -la /tmp/resolv.conf.d /tmp/dnsmasq.d 2>/dev/null && cat /tmp/resolv.conf.d/* 2>/dev/null
```

| # | Action | Expected |
| - | ------ | -------- |
| C.1 | Enable AdGuard, note `server`/`noresolv`. | dnsmasq points at AdGuard. |
| C.2 | Enable the VPN, note again. | dnsmasq points at the VPN resolver, AdGuard still below it. |
| C.3 | Reboot with the tunnel down. | dnsmasq falls back to the layer below (AdGuard). It must **not** point at a dead VPN resolver, and must **not** become `server=127.0.0.1, noresolv=0` (pointing at itself). |
| C.4 | Disable the VPN. | Pre-AdGuard resolvers restored. |
| C.5 | Disable AdGuard. | dnsmasq back to its original value. |
| C.6 | VPN healthy, then yank the uplink Wi-Fi. | LAN DNS recovers to the pre-VPN resolvers within the debounce window, with no operator action, and the VPN still shows disabled. |
| C.7 | VPN healthy; reload the dashboard repeatedly. | LAN DNS keeps using the VPN. A single transient `wg show` failure must not delete the snapshot. |
| C.8 | Repeat C.1–C.5 **with a captive portal** in the middle. | Known composition gap: captive bypass and the layer stack are separate owners of `server`/`noresolv`. See §7. |

---

## 7. Suite D — guards, failover, installer

```sh
ls -la /etc/trafo/    # the RIGHT directory. Not /etc/travo.
```

| # | Action | Expected |
| - | ------ | -------- |
| D.1 | Hand-write an mwan3 interface Travo did not create, then save any failover setting. | The member **survives**. Needs mwan3 **and** its rpcd ACL — see §7.1. |
| D.1a | Save a failover config (this is what D.1 sets up). | Must answer 200, not `Permission denied`. |
| D.2 | Block a client twice, or make a commit fail. | The guard file **remains** after the failure. |
| D.3 | Make `uci commit firewall` fail (read-only overlay), then configure USB tethering. | Guard file still present; interface not half-created. |
| D.4 | Stuck guard present, then a scheduled toggle fires. | Behaviour is documented per ADR 0003 §1.4; a stuck guard disables the schedule until `deploy-local.sh` clears it. Confirm you can actually reach that retry path. |
| D.5 | On the **spare** device: run the published one-liner with `--password`. | Succeeds; health probe hits the **configured** port (80), not AdGuard's 3000. |
| D.6 | On the spare device: run it with no password, and with a password under 8 characters. | Both refuse **before** touching state — no `/etc/config/travo`, no LuCI port change. |

---

## 8. Suite E — AdGuard first-run wizard (accepted risk, verify it is as bad as documented)

On a fresh install, from another machine on the LAN, **before** the owner sets a password:

```sh
curl -s -o /dev/null -w '%{http_code}\n' http://<router>:3000
```

Expect the first-run setup wizard, completable. This is the accepted risk in ADR 0001 §2.5. Confirm
it still matches, then set the password from Travo -> System -> AdGuard Password.

### 8.1 Reinstalling over an existing AdGuard can break LAN DNS silently

Observed on `192.168.1.1` while running D.5: a fresh install put AdGuardHome **v0.107.54** in front
of a configuration written by a newer build. AdGuard then crash-looped on every start:

    [error] parsing configuration file: unknown current schema version 33

Consequence, and the reason this deserves its own section: **dnsmasq keeps
`server='127.0.0.1#5353'` with `noresolv=1`**, so once AdGuard is down every LAN client
loses DNS — while the uplink still holds its lease, the AP is up, and Travo reports health
`ok`. A cached answer resolves for a few minutes and hides it, so check with a name you have
not looked up before.

The config the init script actually reads is `/opt/AdGuardHome/AdGuardHome.yaml`, **not**
`/etc/adguardhome/adguardhome.yaml` (which the UCI `config_file` option names) — editing the
latter appears to do nothing.

Recovery, when the saved config carries nothing worth keeping (`users: []` means the wizard was
never completed, which is usually the case):

```sh
cp /opt/AdGuardHome/AdGuardHome.yaml /tmp/AdGuardHome.yaml.bak
mv /opt/AdGuardHome/AdGuardHome.yaml /opt/AdGuardHome/AdGuardHome.yaml.schema33
/etc/init.d/adguardhome restart          # now starts in first-run state

# Configure it so DNS binds, otherwise dnsmasq still forwards into nothing:
curl -s -X POST http://<router>:3000/control/install/configure \
  -H 'Content-Type: application/json' \
  -d '{"web":{"ip":"0.0.0.0","port":3000},"dns":{"ip":"0.0.0.0","port":5353},
       "username":"admin","password":"<password>"}'
```

Worth deciding separately: whether a reinstall should refuse to downgrade AdGuard rather than leave
the device with working routing and no DNS.

---

## 9. What one device cannot settle

### 9.1 mwan3 needs its rpcd ACL, not just the package

Verified on `192.168.1.1` (OpenWrt 25.12.3): installing the `mwan3` package is **not** enough to
make failover saving work. Travo stages the change through rpcd (`uci apply`), and rpcd grants
`uci` access **per config package**. A root session can read `wireless` and `system` but gets
`Permission denied` on `mwan3` until `luci-app-mwan3` is installed — its ACL file is what grants
it.

```sh
# Reproduce, and check the grant rather than assuming it:
apk add mwan3
PW=$(jsonfilter -i /etc/travo/rpcd-login.json -e '@.password')
LOGIN="{\"username\":\"root\",\"password\":\"$PW\"}"
SID=$(ubus call session login "$LOGIN" | jsonfilter -e '@.ubus_rpc_session')
ubus call uci get "{\"ubus_rpc_session\":\"$SID\",\"config\":\"mwan3\"}" >/dev/null \
  && echo "mwan3 ACL present" || echo "MISSING the mwan3 uci ACL"

apk add luci-app-mwan3    # this is what supplies it
```

A failure surfaces as `uci apply mwan3: ... (Permission denied)` and leaves
`/etc/trafo/failover-in-progress` in place, because the failover rollback uses the same mechanism.
Keeping the guard there is correct fail-safe behaviour (ADR 0003), but it means a stuck failover
guard can mean "missing ACL" rather than "interrupted operation". `deploy-local.sh` clears it.

Note the router may not be able to download `luci-app-mwan3` over its own uplink; fetch it on a LAN
host and `scp -O` it across, then `apk add --allow-untrusted`.

### 9.2 Installing mwan3 can blackhole all internet traffic

Observed on `192.168.1.1`: with no working `wan` interface, installing `mwan3` produced a config
whose catch-all rule routed `0.0.0.0/0` into a policy with no healthy member. Every policy then
read `unreachable` and **every outbound connection failed with "Network unreachable"** — the
router looked healthy (uplink up, lease held, WiFi up) while carrying no traffic.

```sh
mwan3 status | head -20     # every policy reading "unreachable" is this failure
ping -c2 1.1.1.1            # "Network unreachable" is the symptom
```

Recovery is to remove the stock catch-all rules so normal routing applies again:

```sh
uci -q delete mwan3.default_rule_v4
uci -q delete mwan3.default_rule_v6
uci commit mwan3
```

Then check Travo: disabling failover makes it remove its own rule (`travo_default_v4`) but leaves
its generated interfaces, members and policy behind, which are inert without a rule. Removing the
package (`apk del mwan3`) is the clean way to get back to a device with no mwan3 at all.

So before any mwan3 work on a device whose WAN is a WiFi uplink, check that a policy can actually
become healthy — and verify with a real outbound request, not with the uplink's own status.

- **Multi-WAN failover** with two real uplinks — see
  [`failover-verification.md`](./failover-verification.md).
- **The captive-portal composition** (C.8): the captive bypass and the DNS layer stack are two
  uncoordinated owners of the same two UCI options, so the ordering has to be exercised with a real
  portal and a real tunnel together.
- **Failback**: a higher-priority link taking back over after hold-down.
- **The fail-closed direction of any replacement confirm probe.** A correct signal is easy to
  observe on a healthy AP; proving it correctly *reports down* needs a deliberately downed
  interface, and no read-only observation can produce that. Get this wrong and a broken config
  confirms as good.
- **The firewall-restart path** in Block/Unblock needs a session you are willing to have dropped.

---

## 10. Recording a result

Update §3 only when a measurement actually changes. A suite run that found nothing belongs in the
branch's PR description or the task log, not here — this document is for what stays true about the
hardware and for how to re-run the checks.
