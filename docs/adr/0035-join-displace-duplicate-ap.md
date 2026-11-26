# 0035 Join displaces a duplicate AP when no radio is free

Status: accepted

Extends the WiFi Join layout rules in [invariants](../invariants.md) §10 and §2 following [0034](0034-first-install-lan-and-wifi.md).

## Context

[ADR 0034](0034-first-install-lan-and-wifi.md) (`install.sh --wifi` and `POST /api/v1/wifi/setup`) enables the main access point (same SSID, default `Travo`, bridged into `lan`) on every radio that carries no uplink STA. On a dual-radio router such as the GL-AXT1800 (`radio0` = 5 GHz, `radio1` = 2.4 GHz), both radios then host an enabled main AP.

Joining an upstream WiFi (`POST /api/v1/wifi/join`) subsequently failed with 409 `wifi_no_free_radio`:

- `pickRadio` requires a band-compatible radio with no enabled AP: none exists.
- `reconcileRadio` moves enabled APs to another radio, and refuses because the other radio already carries an enabled AP of the same role.

## Options considered

1. **AP+STA with channel-following on the STA radio.** Rejected: committing AP and STA on the same radio crashes ath11k, and an upstream drop takes the local AP down ([device-facts](../device-facts.md) §2–§3, [invariants](../invariants.md) §10).
2. **Disable any AP on the chosen radio unconditionally.** Rejected: an AP with no duplicate on another radio (for example a unique SSID or an unmovable guest AP) would vanish, stranding clients or causing avoidable lockouts.
3. **Displace a radio whose enabled APs can each move or be disabled because a twin stays up.** Chosen.

## Decision

Add a third radio-selection step in Join after `pickRadio` and `reconcileRadio`:

- **Eligibility.** A band-compatible, enabled radio `R` is a displace candidate when every enabled AP on `R` can either move to another radio under the existing reconcile rules or be disabled (`wireless.<section>.disabled=1`) because it has a **twin**: another enabled AP section on a different, enabled radio (`wifi-device` not disabled and live radio not disabled) with the same non-empty SSID and the same role (main vs guest). If any enabled AP on `R` has neither option, `R` is not a candidate. If no candidate exists, Join returns 409 `wifi_no_free_radio` with no write.
- **Deterministic choice.** Classify the caller once before radio selection. Among candidates, prefer a radio that does not host the caller's AP (mapping `lockout.Caller.AP` through `network.wireless status` interfaces to its radio); break ties by radio name ascending (`radio0` first). Ethernet and unknown callers pick by radio name ascending.
- **Staging order.** Inside the single transaction, disable enabled uplink STAs, disable displaced APs, move any movable APs, stage the STA writes, and enable the STA last. No intermediate staged state has an enabled AP and an enabled STA on one radio.
- **Lockout.** Displaced and moved APs count as disabled in `after` for `lockout.Check`. Because a non-guest twin stays enabled, a WiFi caller on a displaced main AP is allowed (clients roam to the twin); unacknowledged risky layouts still return 409 `wifi_lockout_risk`.
- **Probe and rollback.** The join probe proves each displaced AP is absent from `network.wireless status` and every remaining enabled AP (including the twin and any moved AP) is up individually, alongside fresh STA carrier and a kernel default route via the STA. Rollback restores the original configs unchanged.

## Consequences

- While the uplink STA holds the displaced radio, that band's AP (5 GHz or 2.4 GHz) stays off; clients use the twin on the other band.
- When a dual-band upstream is joined from a WiFi client, the client's radio is spared so the operator's session stays connected through the confirm window.
- Automatically re-enabling a displaced AP when the uplink is later removed or switched to another radio is out of scope and left for a follow-up.
- Hardware behaviour (confirm and rollback on a dual-AP AXT1800) is verified via the playbook step in [testing.md](../testing.md).
