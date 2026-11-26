# 0026 Persistent stats history

Status: accepted

Extends [0009](0009-persistence.md), which kept stats history in RAM until a store with a
retention rule existed (#40).

## Decision

- Store: a RAM ring in the daemon (`internal/statshistory`), one point per 10 min bucket,
  retention 7 days (at most 1008 points, about 150 KB of JSON). No KV store: the ring is
  one compact JSON file, `/etc/travo-greenfield/stats-history.json` (mode 0600, atomic
  replacement).
- Flash: never per sample. The ring is flushed at most once per 6 h when it changed and
  once on graceful shutdown (SIGTERM from procd, which also covers a reboot). Worst case
  about 4 writes of about 150 KB a day. A crash or power cut loses at most 6 h.
- A point carries the router wall clock (`at`), `uptime_seconds`, `clients` and the bytes
  each candidate uplink (`wan`, `wwan`, `usbtether`) moved during the bucket. Uplinks map to
  their `l3_device` through `network.interface dump` by name (invariants §7). A byte delta is
  kept only when the same device was seen in both samples, no counter went backwards and
  uptime did not reset; anything else records no value for that uplink. `clients` is the
  number of distinct MACs in `ip -j -4 neigh show` on the `lan` and `guest` devices, null when
  the table is unreadable. It counts recently seen neighbours, not associations.
- Wall time: samples are skipped while the clock is below the build-time floor
  (invariants §14) or not after the newest point (a rewind). Either restarts the delta
  chain, as does any failed reading, so one point never spans two buckets.
- Failure: a missing file is an empty history. A torn, foreign or unreadable file and a
  failed flush both set `persistence: degraded`: sampling continues in RAM, the file is
  never overwritten, and the UI shows it (invariants §5). `DELETE /api/v1/stats/history`
  is the explicit reset: it removes the file and resumes flushing.
- Read: `GET /api/v1/stats/history?hours=1..168`, and the Go `statshistory.Reader` for
  other pages.
- Data usage (#246) needs months, not 7 days. The same file and flush keep day totals:
  `days: [{date, uplinks}]`, one entry per local day with traffic, the sum of that day's
  point deltas (a bucket crossing midnight counts for the later day), retention 93 days
  (three billing months, about 100 bytes per uplink and day, under 30 KB). The router has no
  zoneinfo (device-facts §8), but it stores the POSIX TZ string in
  `system.@system[0].timezone`; `syssettings.Location` wraps it in a minimal TZif v2 blob
  (no transitions, the rule as footer) for `time.LoadLocationFromTZData`, so days, today
  and the billing period (from 00:00 local on its start day) follow the router's zone with
  DST. The zone is read at start and after every System settings change; unreadable or
  unusable counts in UTC. Stored days keep the zone they were counted in: only the day the
  zone changes mixes both, and a date west of the newest stored day counts for that day so
  days stay in order. (Amended by the timezone fix; until then days were UTC.) A point also carries `up`, the
  candidate uplinks up with a readable device at the sample, so the uptime log can tell
  "down" from "no delta" (a restarted chain). Both fields are additive in version 1: a file
  without `days` rebuilds them from its points, a point without `up` is unknown (null),
  and an older daemon ignores both. Reset clears them with the points.
- Uptime log: derived on read from the points: a boot where `uptime_seconds` went down
  (at `at - uptime_seconds`), an uplink up or down where `up` changed between two points
  with a known list. Times are accurate to one bucket; an outage inside one bucket is not
  seen. No event log is stored.
- Data budget: `data-budget.json` (ADR 0009 rules: atomic, 0600, unreadable fails closed
  and is never overwritten until `DELETE /api/v1/network/usage/budget`). It warns only,
  through the alerts service on read; an automatic cut-off would need its own ADR.

## Consequences

- The 2 s live throughput (dashboard rate) stays memory-only; history is a coarse
  10 min view and needs no extra counter reads beyond one per bucket.
- A daemon restart starts a new delta chain: the first bucket after start has no
  uplink bytes.
- Points use the router clock. If NTP corrects the clock forward, the chart shows a gap;
  backward corrections drop samples until the clock passes the newest point.
