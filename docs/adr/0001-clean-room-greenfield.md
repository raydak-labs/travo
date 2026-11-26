# 0001 Clean-room greenfield

Status: accepted

Built in parallel to the existing Travo repo. Reuses its lessons and captured device payloads, not its code. If it proves better, it replaces or moves into `travo`.

It was built as `travo-greenfield` (repo, binary, init script, UCI package) so both could coexist on one device while comparing.

Outcome (0.11.0): the rewrite replaced the old code. Its history was merged into `raydak-labs/travo`; the old code stays on the `legacy` branch. Everything is named `travo` now: `/etc/travo/` (state), `/etc/travo-guards/` (guards), defined once in `internal/app` (`RealPaths`).
