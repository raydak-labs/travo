# 0036 Drop frontend size limits

Status: accepted; removes the UI gzip limits of [0004](0004-frontend-stack.md)

## Context

[ADR 0004](0004-frontend-stack.md) enforced three gzip size limits on `web/dist` during `mise run check` (shell, each other `.js` chunk, and total).

Flash is not the constraint for the UI: roughly 0.3 MB of pre-gzipped frontend assets sits next to a ~12.8 MB arm64 Go binary and ~66 MB of free overlay on the target router (ADR 0004). Meanwhile, the limits kept forcing raises: the total went from 256 KB to 320 KB and the chunk limit from 16 KB to 24 KB to fit the shadcn Select and DropdownMenu (#330), and uPlot as planned in #293 would need another raise to 352 KB. Before this decision the total stood at 305 of 320 KB, so each wanted UI library turned into a budget negotiation.

## Decision

Remove all frontend size limits from `mise run check`. Replace `web/scripts/size-budget.mjs` with `web/scripts/size-report.mjs`, which continues to print shell, total, and per-chunk gzip sizes on every build without failing CI.

The Go binary size check (`mise run check:size`, ~12.8 MB arm64 budget in [invariants](../invariants.md) §15) stays in place, as does route-level code splitting so pages load on demand.

## Consequences

- `mise run check` no longer fails on frontend bundle sizes, while keeping per-file and total gzip sizes visible in build output so growth stays visible.
- Established UI libraries are evaluated against the ADRs rather than enforced byte limits.
- Binary size remains enforced on every commit.
