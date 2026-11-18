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

## Consequences

- The 2 s live throughput (dashboard rate) stays memory-only; history is a coarse
  10 min view and needs no extra counter reads beyond one per bucket.
- A daemon restart starts a new delta chain: the first bucket after start has no
  uplink bytes.
- Points use the router clock. If NTP corrects the clock forward, the chart shows a gap;
  backward corrections drop samples until the clock passes the newest point.
