# 0022 Captive probe semantics

Status: accepted

## Context

The shared connectivity probe (`internal/connectivity`, invariants §12) fetches
`http://connectivitycheck.gstatic.com/generate_204`. In 0.1.0 it read any `200` as a
captive portal that intercepts in place. Some transparent proxies and middleboxes
rewrite the endpoint's `204` to a `200` with an empty body. Those networks are online,
but the dashboard showed the "Hotel WiFi login" card with a login button that opened
an empty page: a false positive with a misleading action.

## Decision

Only the endpoint's expected answer means online:

| Reply | Result |
|---|---|
| `204` | online |
| `200` with an empty body (`Content-Length: 0`, or no byte before EOF) | online |
| `200` with any body | captive, portal URL = the probe URL |
| `3xx` with an absolute or relative `http(s)` `Location` | captive, portal URL = that location |
| anything else, timeout, transport error | offline |

The probe reads at most one byte of a `200` body to decide; it never parses the page.
Recognising portals by content is the auto-accept walker's job, not the probe's.

## Consequences

- A proxy that rewrites `204` to an empty `200` no longer raises the login card.
- A network that injects a block or error page as `200` still reads captive. That is the
  conservative side: the card offers the page and the DNS bypass, nothing changes by itself.
- The joint hardware session checks each row with `scripts/portal-sim.py`
  (docs/testing.md, "Captive portal joint session").
