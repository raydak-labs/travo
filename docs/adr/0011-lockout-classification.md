# 0011 Caller classification for lockout protection

Status: accepted (payloads via spike S3)

Source IP → neighbour table (ARP) → MAC → AP station dumps (`hostapd.*` `get_clients`) → ethernet / wifi-ap / wifi-client / unknown, because `br-lan` mixes wired and WiFi clients. One pure function, table-driven tests on verbatim captured payloads. Unknown fails closed. Refusal: 409 `wifi_lockout_risk`, overridable only with `acknowledge_lockout: true`.

Implementation (`internal/lockout`): `Classify` and `Check` are pure over parsed facts. Caller on an AP keeps access if its own AP section stays enabled or any non-guest AP does; unknown and uplink-side callers need a non-guest AP; Ethernet is never blocked; `acknowledge_lockout` overrides. The parsers for the neighbour table, `hostapd.*` `get_clients` and `network.interface dump` live in `internal/device/facts` and are tested on the spike S3 captures; `internal/wifi` reads those facts live for AP edits (#74). If a station list cannot be read, the caller must pass `Unknown`, never `Ethernet`.
