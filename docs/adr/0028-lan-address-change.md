# 0028 LAN address change confirmed from the new address

Status: accepted

Amends [0005](0005-transactions-and-watchdog.md) (one confirm window for every
transaction) and the confirm rule of [invariants](../invariants.md) §1 for one change (#244).

## Context

Every other guarded change keeps the caller reachable at the same address, so the page that
started it confirms it. Moving `network.lan` to another subnet removes the address the
caller's browser talks to, the connected route to the caller and, after a renewal, the
caller's own lease. The PUT's answer usually never arrives: the reply leaves from an address
that no longer exists. The caller's device keeps its old lease until it renews (half the
12 h lease), so it cannot reach the new address without reconnecting. The session cookie is
host-only, so the new address needs a new login. The 60 s window does not cover reconnect
plus login.

## Decision

- `PUT /api/v1/network/lan {address}` edits only `network.lan` `ipaddr` (CIDR; a one-element
  `list ipaddr`, the 25.12 default, stays a list; a `netmask` is dropped), in ONE `network`
  transaction. Only a `proto static` lan with exactly one IPv4 address is edited (409
  `lan_foreign_config` otherwise).
- The address is a private IPv4 host address (10/8, 172.16/12, 192.168/16) of a /24 to /30
  subnet, not the network or broadcast address (400 `invalid_request`).
- Refused before any write (409), so the router never moves into a broken state:
  - `lan_subnet_overlap`: the subnet overlaps another interface, configured (`ipaddr`,
    WireGuard `addresses`) or live in netifd's dump (guest 192.168.2.0/24, wan, wwan,
    usbtether, wg0; invariants §10 keeps the guest subnet fixed).
  - `lan_dhcp_pool_outside`: an enabled `dhcp.lan` pool (`start`, `limit` are offsets)
    does not end inside the new subnet. Travo does not shift the pool; the user shrinks it.
  - `lan_dependents_outside`: a dhcp `host` reservation or `domain` name, or a firewall
    `redirect` `dest_ip`, lies outside the new subnet or equals the new router address.
    Same-subnet moves (a new host number) keep them valid.
- The transaction uses a per-transaction window of 180 s (`txn.BeginWindow`, never shorter
  than the manager's window), counted from arming as before.
- Probe: netifd's interface dump shows `lan` up with exactly the new address/prefix (netifd
  sets the address itself; the kernel-route rule of device-facts §1 is about routes).
- Confirm comes from the NEW address. After the transaction arms, the daemon refreshes its
  LAN trust set at once (source check and the `Host` allowlist read the live lan addresses),
  so a client in the new subnet with `Host: <new ip>` is admitted without waiting for the
  30 s refresh; the old address is gone from br-lan, so a request can only reach the API
  through the new one (or loopback). The daemon remembers the pending change in memory and
  `GET /api/v1/network/lan` returns `pending {id, address, remaining_seconds}`; the page
  opened on the new address adopts it and shows the usual confirm toast. The generic
  `POST /transactions/{id}/confirm` is used; no new confirm route.
- The old-address page waits up to 10 s for the PUT's answer. A refusal shows its message; a
  rolled-back answer says so; a `pending` answer, a lost connection or no answer turns the
  card into a countdown with the new URL (same scheme and port, `#/network`) and the
  instruction to reconnect the device and log in there. A lost answer counts window plus
  apply budget, since the arm time is unknown.

## Consequences

- Nobody confirms → the watchdog restores `network`, `reload_config` brings the old address
  back; the old-address page tells the user to reconnect and reload.
- A daemon restart inside the window keeps the watchdog (ADR 0005) but loses the in-memory
  pointer: the new-address page then shows no pending change and the change rolls back. The
  safe side.
- Tailscale's `advertise_lan` route is computed when its settings are saved; after a lan
  move the user saves them again. The self-signed certificate (ADR 0016) keeps the names
  it was generated with.
- dnsmasq follows the new subnet through procd's interface trigger; the DHCP pool keeps its
  offsets. Firewall zones reference `lan` by name and need no edit.
- Mock mode has a static interface dump, so a mock PUT rolls back after the probe.
