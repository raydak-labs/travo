---
title: Primitive usage mop-up report
description: Select h-10, form button height, CardInset leftovers, Badge variants, docs.
updated: 2026-07-21
tags: [frontend, ux, mop-up]
worktree: /Users/marbaced/projects/raydak/travo/.worktrees/ui-consistency
branch: fix/ui-consistency
status: done
---

# Primitive usage mop-up report

Executed against audit [`_primitive-usage-audit.md`](_primitive-usage-audit.md) Top 10 + HIGH/MEDIUM tables.

## Done

| Item | Change |
|------|--------|
| **SelectTrigger** | Default `h-9` → `h-10`; dark border `gray-700` → `white/10` |
| **Form buttons** | Dropped `size="sm"` (and `h-7`/`h-8` mismatches) on labeled Input rows: WoL, MAC policy, port-forward, Tailscale auth, reserve-IP, AdGuard password Save/Cancel, MAC clone, NTP (trash → `size="icon"`), WG import submit, failover edit Save/Cancel, data-usage budget, LED schedule Save |
| **CardInset** | DNS tools, radio hardware rows, WG kill switch + peers, failover candidates, data-usage iface, LED schedule, repeater configure-AP panels (kept blue info banner) |
| **Badge variants** | VPN verify, USB tethering, Tailscale panel/peer, radio Recommended |
| **CardHeader icons** | Dashboard Quick status → trailing sibling |
| **ServiceCard** | `CardHeader` + `CardTitle`; keep `shadow-none` |
| **Docs** | `ui-theming.md` form density / Select h-10 / CardInset / Badge exceptions; plans README index |

## Verify

- `cd frontend && pnpm test` → **48 files, 262 tests passed**

## Leftovers (intentional / out of scope)

- Dense `size="sm"` toolbars (logs, header, service actions, failover Move up/down, USB WAN actions, AdGuard “Change”)
- `SourceCard` custom status pill + icon-in-title (documented exception)
- `LogsLevelBadge` parallel system (documented)
- System Header-less Cards (power / quick links)
- Optional: Switch label density; Label vs `Input label=` dual API; field-level `text-red-500` vs InlineError
- Other hand-rolled borders outside audit list
