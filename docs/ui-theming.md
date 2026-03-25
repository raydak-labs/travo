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

## Avoid

- Hard-coded hex or `rgb()` for **normal** UI text when Tailwind utilities or inheritance work.
- Light-only grays (`text-gray-900`, `text-gray-700`) on components that also use `dark:bg-*` without a matching `dark:text-*` or inheritance fix.
