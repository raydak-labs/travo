---
title: Frontend UI and theming
description: ThemeProvider, dark class, Tailwind tokens, chart variables, contrast rules.
updated: 2026-10-05
tags: [docs, frontend, theming, tailwind]
---

# Frontend UI & theming

Short reference for Tailwind + dark mode in `frontend/`.

## Card / Label / feedback contracts

| Primitive | Default contract |
|-----------|------------------|
| **`CardTitle`** | `text-sm font-medium leading-none tracking-tight` + `text-gray-900 dark:text-white`. Prefer layout-only `className` overrides (`flex items-center gap-2`). Exceptions: Login hero (`text-2xl`); Setup step titles that are not `CardTitle`. |
| **`Label`** | `text-xs font-medium text-gray-500 dark:text-gray-400`. Dense forms use this; Login may override to `text-sm font-medium` for prominence. |
| **`SectionHeading`** | The only in-page group label (`mb-3 text-xs font-semibold uppercase tracking-wider` + gray tones). Six pages had hand-rolled it; a seventh copy is a defect. |
| **Loading** | `Skeleton` for card/page content. Spinners (`Loader2`) only for in-button/submit busy. |
| **Empty** | `EmptyState` from `@/components/ui/empty-state`. |
| **Load/display errors** | `InlineError` (`role="alert"`, `text-sm`, red border/bg light+dark pairs). Mutation success/failure stays on `sonner` toasts. |

## How theme works

- `ThemeProvider` (`frontend/src/components/layout/theme-provider.tsx`) toggles `dark` on `<html>` and persists `localStorage.theme`.
- A small inline script in `frontend/index.html` runs before the bundle so the first paint matches stored / system preference (reduces flash).
- Tailwind v4 uses `@custom-variant dark (&:is(.dark *));` in `frontend/src/index.css`.

## Global text and background

`index.css` applies `@layer base` rules on `body`:

- Light: `bg-gray-50 text-gray-900`
- Dark: `bg-gray-900 text-gray-100` (under `html.dark`)

That way elements that only set size/weight (e.g. `className="text-sm"`) inherit a readable color on dark panels such as `dark:bg-gray-900`.

## What to use in components

| Need                                | Pattern                                                                   |
| ----------------------------------- | ------------------------------------------------------------------------- |
| Default page/body copy              | Omit text color; inherit from `body`                                      |
| Card titles                         | Use `CardTitle` (compact `text-sm` default; see contracts above)          |
| Form field labels                   | Use `Label` (or identical classes)                                        |
| Inline load/display errors          | Use `InlineError`                                                         |
| Secondary / hint text               | `text-gray-500 dark:text-gray-400` or `text-gray-600 dark:text-gray-300`  |
| Primary emphasis on colored surface | Ensure both light and dark classes (e.g. `text-gray-900 dark:text-white`) |

## Motion

`index.css` imports `tw-animate-css`. Every `animate-in` / `fade-in-*` /
`zoom-in-*` / `slide-in-from-*` utility on a Dialog, Sheet or Select depends on
it — adding one without the import produces a class that silently does nothing
(no lint or type error). A `prefers-reduced-motion` block in `@layer`-adjacent
CSS neutralises all of it; keep it last in `index.css`.

## Native controls and `color-scheme`

`index.css` declares `color-scheme: light` on `:root` and `dark` on `.dark`.
Without it the UA keeps its own light rendering for `type="time"`,
`type="range"`, `type="file"` inputs and scrollbars on dark surfaces.

## Field chrome

`Input`, `Textarea` and `SelectTrigger` share one surface string
(`fieldClassName` in `components/ui/input.tsx`) and one invalid-state string
(`invalidFieldClassName`). Do not hand-roll a fourth copy, and do not restyle
`SelectTrigger` independently: a form mixing an `Input` and a Select must show
one surface and one focus-ring width.

Field validation errors use **`FieldError`** (`text-xs text-red-600
dark:text-red-400`, `role="alert"`). Do not write `text-red-500` directly:
that is 3.76:1 on white and fails AA at the sizes used here.

## Buttons

`buttonVariants` base owns `gap-2` for the icon-to-label distance. Do not add
`mr-*` between a button's icon and its label — 41 call sites had drifted across
`mr-2`, `mr-1.5` and `mr-1`.

## Card plane

Every panel uses `cardSurfaceVariants()` (`card-surface.tsx`). There is no exempt
screen: the dashboard source card and the setup wizard were normalized rather than
grandfathered, because "all but two pages differ" is what produced the drift
(ADR 0012). Use `CardInset` for nested regions rather than a second card.

## Status dots

Use `statusDotClass(up)` / `statusDotIdleClass` from `@/lib/status-dot` rather
than an inline `shadow-[0_0_6px_rgba(...)]`. State must never be colour-only:
pair the dot with a visually hidden text node and `role="status"`.

## Status colours

Normative rules are in [ADR 0012](../adr/0012-ui-consistency-and-status-language.md);
this is the token reference. Status colour comes from `--status-*` tokens in
`index.css` and reaches the screen through **`StatusPill`**
(`tone="ok|warn|danger|info|neutral|stale"`). Raw `bg-green-*` / `text-red-*`
status strings are a defect, not a style choice.

| Tone | Means |
| ---- | ----- |
| `ok` | working as configured |
| `warn` | degraded, user may want to act |
| `danger` | broken / not reachable |
| `info` | noteworthy but not a problem |
| `neutral` | inactive or not applicable |
| `stale` | last known value; weaker than `warn`, never blanks the value |

Use `StatValue stale` rather than recolouring a fact to grey.

## Summary band

A page with more than one status question starts with an always-visible
**`SummaryBand`** of flat `SummaryTile`s. Not collapsible; each tile carries its
own loading, error and stale state. `wifi-summary-band.tsx` is the reference
implementation.

## Nested regions

Inside a `Card`, use **`CardInset`** (`frontend/src/components/ui/card-inset.tsx`) for sub-groups — border only (`default`) or border + muted fill (`muted`). Do not nest a second shadowed `Card`. Prefer `CardInset` over hand-rolled `rounded-lg border p-3/4` nests on the card plane. Keep semantic alert/banner boxes (amber/red/blue health) as custom surfaces — not `CardInset`.

## Form density

| Tier | Height | When |
|------|--------|------|
| **Form rows** | Default control height (`Input` / `SelectTrigger` / submit `Button` = **`h-10`**) | Labeled fields, add/save rows next to `Input` |
| **Dense chrome** | `Button size="sm"` (and icon rows) | Logs filters, page header actions, service action icon rows, quick actions — **not** form rows with labeled Inputs |

`SelectTrigger` default is **`h-10`** with the same `fieldClassName` surface as
`Input` (card-plane consistency). Do not leave form submits at `size="sm"` beside
default Inputs.

## Badge exceptions

| Exception | Where | Why |
|-----------|--------|-----|
| **`LogsLevelBadge`** | `logs-level-badge.tsx` | Dense uppercase terminal chips; parallel to `Badge` by design |

Elsewhere prefer `Badge` variants (`success` / `destructive` / `default` / `secondary` / `outline` / `warning`) for neutral category labels, and `StatusPill` for anything that reports state. Hand-rolled `bg-green-*` / `bg-red-*` / `bg-blue-*` overrides are no longer accepted (ADR 0012).

## Borders in dark mode

Surfaces such as **`dark:bg-gray-950`** are darker than Tailwind **`gray-800`** (`#1f2937`). Using **`dark:border-gray-800`** on those panels makes the border **lighter than the fill**, which reads as a harsh, almost white outline.

Prefer:

- **Panel / card chrome** (same plane as `Card`, header, sidebar, dialogs): `dark:border-white/10`.
- **Hairline dividers** inside dark panels (table rows, list separators): `dark:border-white/[0.08]` or `/5`–`/10` depending on contrast.

Light mode keeps **`border-gray-200`** (and similar) on white/off-white surfaces.

## Navigation patterns (sidebar vs in-page tabs)

Normative IA: [`plans/2026-07-24-frontend-traveler-ia.md`](../plans/2026-07-24-frontend-traveler-ia.md). Summary:

| Pattern | Typical role |
| --------|--------------|
| **Sidebar** (group label → daily default; chevron toggles leaves) | *Where in the app am I?* Label click opens the group default (WiFi→Connect, Network→Status, …). Chevron alone expands Internet & LAN / Advanced leaves. Switching Connect/Advanced etc. is **sidebar-only** — do **not** mirror with an in-page tab bar. See [`plans/2026-07-25-sidebar-defaults-lighter-disclosure.md`](../plans/2026-07-25-sidebar-defaults-lighter-disclosure.md). |
| **In-page tabs** | Only when the axis is **not** a sidebar hierarchy (today: Logs **System Log / Kernel Log**). |
| **Page sections** (collapsed boxes) | Only for rare / power / geek cards. Everyday cards stay visible. |

Sidebar group open/closed is for nav density (defaults collapsed on first visit; auto-expand the active group on child routes). It does not replace page-section collapse for card content.

## Overview vs detail pages

- **Dashboard** (`/dashboard`): how you are connected right now, client counts, quick actions, and live throughput. Avoid duplicating long-form diagnostics (full interface tables, kernel strings, hour-long history charts) here; those belong on **Network**, **Clients**, **System**, or **Logs** as appropriate.
- Prefer **one obvious next step** (e.g. link to System for firmware and hardware) over packing every metric onto the first screen.

## Avoid

- Hard-coded hex or `rgb()` for **normal** UI text when Tailwind utilities or inheritance work.
- Light-only grays (`text-gray-900`, `text-gray-700`) on components that also use `dark:bg-*` without a matching `dark:text-*` or inheritance fix.
- **`dark:border-gray-800`** (or lighter grays) on **`dark:bg-gray-950`** panels — use **`dark:border-white/10`** (or similar opacity) instead.
