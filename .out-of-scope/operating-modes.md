# Operating modes (client, bridge, access point)

Travo runs as a travel router: a LAN with its own DHCP and firewall behind an uplink. There is no mode switch to "client only", bridge or plain access point.

## Why this is out of scope

Every flow assumes travel-router mode. A client or bridge mode changes who serves DHCP, how the firewall zones work, and what the lockout checks protect: in bridge mode the admin reaches the router through the uplink's network, which Travo's trust boundary refuses ([ADR 0008](../docs/adr/0008-auth-and-trust-boundary.md), [ADR 0011](../docs/adr/0011-lockout-classification.md)). Supporting it would mean a second set of rules for almost every page, so it would be its own product epic rather than a setting.

To run the router as a bridge or plain access point, configure it in LuCI.

## Prior requests

- #292 (from the blackdark-org/travo-greenfield#309 review): "Travel router" vs "Client only" mode selection, not planned.
