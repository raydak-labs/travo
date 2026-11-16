# 0009 Persistence without a KV store

Status: accepted

UCI is the source of truth (incl. `/etc/config/<name>` for Travo settings). Derived or small state: atomic JSON in `/etc/travo-greenfield/`. Guards in a separate `/etc/travo-greenfield-guards/`. No bbolt in 0.1: revocation uses the epoch file, stats history stays in RAM (no per-sample flash writes). History becomes a later task with a retention rule.

Saved uplink profiles (#72): `wifi-profiles.json` in the state directory owns the ordered preference list and its credentials (mode 0600, atomic replacement, one process-local lock for read/modify/write). UCI owns the live STA configuration. Saving, reordering or forgetting a profile never writes UCI; only selecting one does, through the guarded join transaction, and the outcome never rewrites the store. An absent file is an empty list; an unreadable or invalid file fails closed and is never overwritten by a read or mutation.
