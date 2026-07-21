---
title: Primitive usage consistency audit
description: Independent audit of Card / Button / Badge (+ Label, Select, Switch) call-site consistency vs ui-theming + shadcn patterns.
updated: 2026-07-21
tags: [frontend, ux, audit, primitives]
worktree: /Users/marbaced/projects/raydak/travo/.worktrees/ui-consistency
branch: fix/ui-consistency
status: findings-only (no fixes)
---

# Primitive usage consistency audit

Audit of Travо frontend usage of shared UI primitives against:

- `docs/ui-theming.md`
- `docs/plans/_shadcn-ui-patterns.md`
- `docs/plans/2026-07-20-ui-consistency.md`
- `docs/plans/2026-07-20-nested-cards-approach-a.md`
- primitives under `frontend/src/components/ui/{card,button,badge,card-inset,input,label,empty-state,inline-error}.tsx`

Method: ripgrep counts + read of wifi / network / vpn / system / services / dashboard / clients / logs pages. Cross-checked prior nested-cards audit (`_ui-nested-cards-audit.md`) for delta after Approach A.

---

## 1. Verdict

**MOSTLY_CONSISTENT**

Primitives exist, theme contracts documented, and most pages compose `Card` → `CardHeader` → `CardTitle` → `CardContent`. Feedback (`EmptyState`, `InlineError`, `Skeleton`) and `Label` are adopted on many dense forms. Remaining pain is **control-height mismatch** (`Button size="sm"` / `SelectTrigger h-9` vs `Input h-10`), **incomplete `CardInset` migration**, and **Badge variant bypass** via hand-rolled color classes.

---

## 2. Scores (1–10)

| Area | Score | Notes |
|------|------:|-------|
| **Card** | **7** | Structure strong; WiFi core uses `CardInset`; VPN/network/wizard still raw borders; dual icon placement; few Header-less cards |
| **Button** | **6** | Destructive + outline mostly disciplined; ~134 `size="sm"`; many next to default `Input` |
| **Badge** | **7** | Variants used widely; several call sites re-paint success/destructive with `className`; one intentional parallel system (logs) |
| **Overall** | **6** | Design system present; call-site height + inset + badge color debt keeps score mid |

---

## 3. Issues table

| Sev | Primitive | Evidence | Fix suggestion |
|-----|-----------|----------|----------------|
| **HIGH** | Button | `wol-card.tsx:58` — `size="sm"` submit beside default `Input` (`h-10`) in labeled grid | Drop `size="sm"` (or set whole row to `sm` + shorter inputs); keep `items-end` |
| **HIGH** | Button | `mac-policy-add-form.tsx:64` — Add `size="sm"` next to `Input`s inside `CardInset` | Default button height |
| **HIGH** | Button | `firewall-port-forward-add-form-grid.tsx:104` — submit `size="sm"` + `self-end` with `Input`s | Default `h-10` |
| **HIGH** | Button | `tailscale-auth-section.tsx:33` — submit `sm` + `Input` | Default height |
| **HIGH** | Button | `reserve-ip-form.tsx:74` — submit `sm` + `Input`s | Default height |
| **HIGH** | Button | `adguard-password-card.tsx:94` — Save `sm` next to password `Input`s | Default height for form actions |
| **HIGH** | Button | `mac-address-clone-block.tsx:82–96` — Random / Apply / Cancel all `sm` beside `Input` | Default height (toolbar may stay `sm` if Input also compact) |
| **HIGH** | Button | `ntp-config-server-fields.tsx:41,77` — icon/`sm` actions next to server `Input`s | Match input height or use `size="icon"` only for trash |
| **HIGH** | Select | `select.tsx:20` — `SelectTrigger` default **`h-9`**; **0** call sites override `h-10` (14 triggers: SQM, DHCP, DDNS, guest, AP enc, timezone, hardware buttons, radio role, etc.) | Change primitive default to `h-10` (or add `h-10` at every form row) |
| **HIGH** | Card / CardInset | WiFi migrated (`ap-*`, guest, schedule, mac-policy-add, saved-networks) = **7** `<CardInset` sites; leftovers still raw: `dns-tools-card.tsx:73`, `wifi-radio-hardware-card.tsx:71`, `wireguard-profiles-kill-import.tsx:82`, `wireguard-config-peers-list.tsx:21`, `failover-card.tsx:189`, `data-usage-interface-card.tsx:18`, `led-schedule-form.tsx:33`, repeater wizard `configure-ap-step.tsx` (many `rounded-lg border p-3/4`) | Replace same-family insets with `CardInset` / `variant="muted"`; keep semantic alert boxes separate |
| **MEDIUM** | Badge | Custom colors duplicating variants: `vpn-verify-wireguard-card.tsx:74,78`, `usb-tethering-section.tsx:42,56`, `tailscale-logged-in-panel.tsx:46`, `tailscale-peer-row.tsx:25`, `wifi-radio-hardware-card.tsx:81` | Use `variant="success" \| "destructive" \| "default"`; drop color `className` |
| **MEDIUM** | Badge | Dashboard `SourceCard` status chip is custom pill, not `Badge` — `dashboard-page.tsx:249–258` | Keep intentional (plan Task 9) **or** theme-safe `Badge` on forced-dark card |
| **MEDIUM** | Card | `service-card.tsx:41–49` — nested flat `Card` without `CardHeader`/`CardTitle`; raw `<h3>` | Prefer `CardHeader`+`CardTitle` or non-Card list row if parent already cards |
| **MEDIUM** | Card | `system-power-section.tsx:21–22`, `system-quick-links-card.tsx:47–48` — `Card` + `CardContent` only; section title is external `<h2>` | OK pattern for sectioned system page; document as exception **or** add `CardHeader` |
| **MEDIUM** | Card | Header icon placement split: ~55 cards put Lucide **right** of title (`flex flex-row … justify-between`); dashboard Quick status puts icon **inside** `CardTitle` (`dashboard-page.tsx:382–384`); SourceCard wraps title+status in inner flex (`:236–246`) | Pick one: icon-in-title **or** trailing decorative icon; apply everywhere |
| **MEDIUM** | Button | `data-usage-budget-editor.tsx:78` — `size="sm" className="h-7"` beside inputs (extra short) | Align to shared dense row height or default |
| **MEDIUM** | Button | `wireguard-import-profile-form.tsx:52,88` — `sm` next to name `Input` / submit | Default submit height |
| **MEDIUM** | Button | `failover-card.tsx:327+` — edit-mode Save/Cancel `sm` with `Input`s in candidate blocks | Default height in edit forms |
| **MEDIUM** | CardInset border | VPN insets use `dark:border-gray-700` (`wireguard-profiles-kill-import.tsx:82`, `wireguard-config-peers-list.tsx:21`) vs theming rule `dark:border-white/10` on card-plane panels | Prefer `CardInset` (already `dark:border-white/10`) |
| **MEDIUM** | Label / Input | Dual label APIs: ~29 files import `Label`; many forms use `<Label>` + bare `<Input>`; others use `Input label=` (e.g. alert thresholds, login). Both OK via `Input`→`Label`, but visual rhythm differs when `label=` wraps vs external `space-y-1` | Prefer one form-row pattern per page family (`Label` + `Input` in `space-y-1`) |
| **MEDIUM** | Feedback | Field errors often bare `text-xs text-red-500` (`dns-entries-card.tsx:97`, `diagnostics-card.tsx:92`) while load errors use `InlineError` — contract says InlineError for load/display, not field validation | Keep field errors as-is; document exception |
| **LOW** | Button | Dense toolbars correctly use `sm` (logs filters, header, service action rows, quick actions) — not a bug | No change; treat as intentional density |
| **LOW** | Badge | `LogsLevelBadge` (`logs-level-badge.tsx:4–14`) parallel to `Badge` | Keep; comment already documents why |
| **LOW** | Switch | `Switch` built-in label is `text-sm text-gray-700 dark:text-gray-300` (`switch.tsx:16`) vs form `Label` `text-xs … gray-500` — denser forms look uneven next to switches | Optional: denser switch label variant |
| **LOW** | Switch / a11y | `split-tunnel-card.tsx:71,80` — bare `<label>` + checkbox instead of `Switch` | Migrate to `Switch` if product wants toggle chrome |
| **LOW** | Theming | `service-card.tsx:55,78` — `text-gray-400` without dark pair on version / hint | Add `dark:text-gray-500` (or inherit) |
| **LOW** | Card | Login `CardTitle` `text-2xl` (`login-page-card-header.tsx:11`) | Documented exception — keep |
| **LOW** | Empty / loading | EmptyState / Skeleton widely used; residual `"Loading…"` mainly in-button (e.g. quick links AdGuard Config) | OK per contract |

### Severity counts

| Severity | Count |
|----------|------:|
| HIGH | 10 |
| MEDIUM | 11 |
| LOW | 8 |

---

## 4. What's solid

- **Card chrome**: shared `Card` border/`dark:border-white/10` / shadow; ~69 import sites; almost all settings cards use Header + Title + Content.
- **CardTitle default**: `text-sm font-medium` matches contract; sizing overrides nearly gone (login `text-2xl` only intentional hero).
- **WiFi Approach A**: equal-height grid (`wifi-wireless-panel.tsx:28–34` + `h-full flex flex-col` on saved/current); `CardInset` on AP / guest / schedule / MAC add / auto-reconnect; scan row `flex-wrap` (`wifi-current-connection-card.tsx:51`).
- **DNS Add row**: `dns-entries-card.tsx:83–119` — `items-end` + **default** `Button` (fixed vs older `sm` audit).
- **Diagnostics Run**: default `Button` beside `Input` (`diagnostics-card.tsx:77–89`).
- **Badge variants**: success/secondary/outline/destructive used correctly on WAN, interfaces, firewall policy, services state map, VPN leak test.
- **Label**: dedicated primitive; Input wires `label=` through `Label`; dense network/wifi/vpn forms largely migrated.
- **EmptyState / InlineError / Skeleton**: feedback contract largely landed (EmptyState ~25 files; InlineError on diagnostics, speedtest paths, etc.).
- **Destructive buttons**: reboot/shutdown/factory, confirm dialogs, service remove — consistent `variant="destructive"`.
- **Service nested Card**: `shadow-none hover:shadow-none` (`service-card.tsx:41`) — Approach A Services flatten done.

---

## 5. Top 10 fix targets (ranked)

1. **Bump `SelectTrigger` default to `h-10`** (`select.tsx`) — one change fixes all 14 form selects.
2. **Form submit rows: remove `size="sm"` next to default `Input`** — WoL, MAC policy add, port-forward, Tailscale auth, reserve-IP, AdGuard password, MAC clone, NTP servers, WireGuard import, failover edit.
3. **Finish `CardInset` migration** for VPN peer/import panels, DNS tools inset, radio hardware rows, failover candidates, data-usage iface cards, LED schedule form.
4. **Badge: replace hand-rolled green/red/blue `className` with variants** (VPN verify, USB tethering, Tailscale panels, radio hardware).
5. **Normalize CardHeader decorative icons** (trailing sibling vs in-title) across ~55 flex-row headers + dashboard.
6. **VPN / wizard dark borders** → `dark:border-white/10` or `CardInset` (drop `dark:border-gray-700` on card-plane insets).
7. **ServiceCard title structure** — use `CardTitle` or drop Card wrapper in favor of list item chrome.
8. **Document form density tiers** — when `sm` is allowed (toolbars, icon rows) vs forbidden (labeled Input grids).
9. **Switch label density** optional align to `Label` on settings cards.
10. **SourceCard chip** — either keep + document, or map to outline/success Badge on slate surface.

---

## 6. Intentional exceptions

| Exception | Where | Why |
|-----------|--------|-----|
| Login hero `CardTitle text-2xl` | `login-page-card-header.tsx` | Plan + theming contract |
| Setup outside AppShell / step titles | `setup-page.tsx` | Plan |
| Dashboard `SourceCard` forced dark + custom status pill | `dashboard-page.tsx:234–258` | Plan Task 3 / Task 9; topology aesthetic |
| `LogsLevelBadge` ≠ `Badge` | `logs-level-badge.tsx` | Dense uppercase terminal chips; comment documents |
| `ServiceCard` flat nested Card | `service-card.tsx` | Parent Installed Services Card; `shadow-none` by design |
| System section `<h2>` + Header-less Card | power / quick links | Section rhythm, not card chrome |
| Header / logs / service action `size="sm"` | many | Dense chrome, not form-row with Input |
| Hostname inline `h-6` Input+Button | `hostname-inline-form.tsx:55–75` | Inline edit in glance row |
| Semantic alert boxes (amber/red/blue) | health banners, wizards, timezone alert | Not `CardInset`; status surfaces |
| Field-level `text-red-500` errors | forms | Not load/display `InlineError` |
| Tab bars (WiFi/Network/Diagnostics) | shared segmented control | Not Card nesting |

---

## Counts snapshot (worktree)

| Signal | Approx |
|--------|--------|
| Files importing `card` | 69 |
| Files importing `badge` | 34 |
| Files importing `label` | 29 |
| `<CardInset` call sites | 7 |
| `size="sm"` occurrences | 134 |
| `SelectTrigger` call sites | 14 (all default `h-9`) |
| Badge `variant="success"` | 7 |
| Badge custom `bg-green` overrides | 4+ files |
| EmptyState consumers | ~25 |
| InlineError consumers | ~9 |

---

## Delta vs `_ui-nested-cards-audit.md`

Already improved since that audit:

- Current/Saved equal-height + `CardInset` auto-reconnect
- AP / guest / schedule / MAC add → `CardInset`
- DNS Add button height fixed
- Diagnostics tab chrome + Run `h-10`
- Services nested shadow stripped

Still open from that audit + this pass: Select `h-9`, widespread form `sm` buttons, non-WiFi inset leftovers, Badge color bypasses, header icon placement.
