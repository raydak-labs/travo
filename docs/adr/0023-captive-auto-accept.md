# 0023 Captive portal auto-accept

Status: accepted

## Context

Many hotel portals are click-through: a terms checkbox, hidden session fields and a
Connect button. The user can do that from "Open hotel login" (#129), but the router
can do it too. Invariants §12 allow an auto-accept that walks portal steps, where only
step 0 may renew the wwan lease, guarded, with bounded waits. Old Travo had a walker
with `InsecureSkipVerify` and a regex form detector; we harvest only its lessons
(ADR 0001).

## Decision

- **User-triggered only.** `POST /api/v1/captive/auto-accept` runs one walk while the
  shared probe reports `captive` with a portal URL (else 409
  `captive_accept_not_captive`); one walk at a time (409 `captive_accept_busy`). No
  background trigger in 0.2: an unattended walk would submit forms the user never saw.
- **Steps** (`internal/captive/accept.go`). Step 0 fetches the portal URL. Steps 1 to 5
  each act on the last page (`PlanPage`, pure, `golang.org/x/net/html` tokenizer):
  - exactly one form and no form needing a person: submit it with its hidden fields,
    every checkbox ticked, the default radio, prefilled text and the first named submit
    button; selects are left out;
  - any form with a visible empty text, password, email, tel, number or search input,
    or a named textarea, or more than one candidate form: stop with `needs_user`;
  - no form but a meta refresh: follow it;
  - otherwise stop with `no_form`.
  After each follow or submit the probe is invalidated and re-read; `online` ends the
  walk. More than 5 steps, a transport error or a non-2xx final status: `failed`.
- **Only step 0 renews.** When the step-0 fetch fails at transport level (stale lease,
  device-facts §7), the walker asks the existing `Renewer` for availability. Only
  `available` renews (wwan carries the kernel default route and the probe is not
  online); a kept guard reads `blocked` and the renew is skipped. The `Renewer` holds
  `captive-dhcp-renew-in-progress` and bounds its wait to 10 s. Step 0 then retries the
  fetch once. Later steps never renew.
- **Bounds.** 30 s per walk, 5 s per request, 5 redirects per request, 256 KiB per body.
- **HTTP edge.** A fresh cookie jar per walk, no proxy, no keep-alive, TLS verified with
  the system roots (no `InsecureSkipVerify`). Only http(s) targets are followed.
- **Target policy** (`internal/captive/acceptguard.go`). A portal page must never make
  the router reach itself or the networks behind it. At walk start (and again after a
  step-0 renew, which may change the uplink address) the walker reads the kernel's
  interfaces and addresses (all of them: lan, guest, wwan, wan, wg, tailscale, lo) and
  names the uplink by the kernel default route's device. It refuses:
  - every address on every router interface, the uplink's own address included;
  - every subnet of every interface except the uplink (LAN, guest, VPN, loopback);
  - loopback, unspecified, link-local and multicast addresses;
  - any address the kernel would not route out of the uplink: a connected UDP socket
    (sends nothing) returns the source address the kernel picks, and it must be one of
    the uplink's own addresses. This catches static routes into the LAN, VPN routes and
    tailnet peers that are not on a directly attached subnet.
  It allows the uplink subnet (the hotel gateway or portal, e.g. `10.66.0.1` in the
  simulator) and anything routed out of the uplink (public addresses, the hotel's other
  private nets). The check runs in the dialer's `Control` hook on the address actually
  dialed, for every connection, so redirects and DNS answers that change between lookups
  (rebinding) cannot bypass it; literal-IP redirect targets are refused before dialing.
  A refused target ends the walk as `failed` and never triggers the step-0 renew.
  The kernel is read with Go's `net.Interfaces` and a UDP route lookup rather than a
  `Device` port: the daemon runs on the router, the kernel is the truth (netifd route
  state lies, device-facts §1), and both are passed in as functions so tests use
  verbatim captures (`ip-addr.json`, the guest interface status).
- **No router state changes** apart from the guarded renew: no UCI, no DNS. If the
  portal name does not resolve (AdGuard or DoH base), the walk fails at step 0 and the
  card still offers the DNS bypass (#130).
- **Result.** Every outcome is a 200 with `{outcome, steps[{kind, host, status}],
  renewed}`. Steps carry the host only, never paths or queries (portal tokens).

## Consequences

- Portals asking for a room number, name, voucher or payment stop at `needs_user`;
  the user opens the login page.
- A portal with an invalid or self-signed certificate fails the walk. The joint
  hardware session (docs/testing.md) records whether real portals need more.
- Portals that rely on JavaScript to submit or to build fields fail or stop at
  `no_form`; we do not run scripts.
- Unit tests use synthetic pages shaped like `scripts/portal-sim.py`; verbatim hotel
  captures from the joint session replace or extend them.
