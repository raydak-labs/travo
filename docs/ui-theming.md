---
title: Frontend UI and theming
description: ThemeProvider, dark class, Tailwind tokens, chart variables, contrast rules.
updated: 2026-04-13
tags: [docs, frontend, theming, tailwind]
---

# Frontend UI & theming

Short reference for Tailwind + dark mode in `frontend/`.

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
| Card titles                         | Prefer `CardTitle` / existing primitives (already include `dark:` pairs)  |
| Secondary / hint text               | `text-gray-500 dark:text-gray-400` or `text-gray-600 dark:text-gray-300`  |
| Primary emphasis on colored surface | Ensure both light and dark classes (e.g. `text-gray-900 dark:text-white`) |

## Charts and third-party SVG

Recharts ticks and tooltips cannot use Tailwind classes directly. Use CSS variables from `index.css` (`--chart-grid`, `--chart-axis`, `--chart-tooltip-*`) and pass them via `stroke` / `fill` / `contentStyle`.

## Borders in dark mode

Surfaces such as **`dark:bg-gray-950`** are darker than Tailwind **`gray-800`** (`#1f2937`). Using **`dark:border-gray-800`** on those panels makes the border **lighter than the fill**, which reads as a harsh, almost white outline.

Prefer:

- **Panel / card chrome** (same plane as `Card`, header, sidebar, dialogs): `dark:border-white/10`.
- **Hairline dividers** inside dark panels (table rows, list separators): `dark:border-white/[0.08]` or `/5`–`/10` depending on contrast.

Light mode keeps **`border-gray-200`** (and similar) on white/off-white surfaces.

## Navigation patterns (sidebar vs in-page tabs)

These solve different levels of hierarchy:

| Pattern | Typical role |
| --------|--------------|
| **Sidebar** (collapsible **WiFi** and **Network** groups + leaf **Clients**, etc.) | *Where in the app am I?* Sub-links map to real routes (e.g. `/wifi/advanced`, `/network/configuration`). |
| **In-page tabs** (WiFi Wireless / Advanced; Network Status / Configuration / Advanced) | Same page component; **tab changes call the router** so URL, sidebar highlight, and shareable links stay aligned. |

Collapsible **sidebar groups** are only for grouping nav links, not for hiding page content (page-level disclosure uses tabs + routes).

## Avoid

- Hard-coded hex or `rgb()` for **normal** UI text when Tailwind utilities or inheritance work.
- Light-only grays (`text-gray-900`, `text-gray-700`) on components that also use `dark:bg-*` without a matching `dark:text-*` or inheritance fix.
- **`dark:border-gray-800`** (or lighter grays) on **`dark:bg-gray-950`** panels — use **`dark:border-white/10`** (or similar opacity) instead.
