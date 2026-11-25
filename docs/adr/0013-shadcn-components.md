# 0013 Copied shadcn-svelte components

Status: accepted

Use the shadcn-svelte CLI to install the components the SPA renders. The pinned
devDependency is 1.8.1. Reapply with
`pnpm -C web exec shadcn-svelte add --overwrite --yes` and the component names.
That writes the registry source into `web/src/lib/components/ui`. The installed
set is button, input, label, textarea, card, badge, alert, skeleton, table,
collapsible, dialog, sheet, alert-dialog, and sonner (#170), dropdown-menu and select
(#330), switch, hover-card and checkbox (UI structure v2). The registry switch and checkbox style their states with
`data-checked:` / `data-unchecked:`, which bits-ui 2.21 does not set; the copies use
`data-[state=checked]:` / `data-[state=unchecked]:` instead, so reapply that after a reinstall. Adding a component
re-offers `button`; keep the app's button (disabled anchor rule below) instead of
overwriting it. The sonner wrapper takes the app's `data-theme` value as `theme`
instead of `mode-watcher`, which is not installed. Pages use these through the
confirm / toast pattern in [web-ui.md](../web-ui.md).

Those files use `tailwind-variants` and `cn`. `web/src/lib/utils.ts` re-exports
`cn`. An earlier probe at 95ecdf0 fit a 40 KB shell only by replacing that
default with a static class lookup (63,878 B shell and 19,152 B WiFi chunk for
stock Button and Dialog; 39,757 B shell after the fork). The fork is gone. Gzip
budgets are shell 128 KB, each other JS chunk 24 KB, and total 320 KB
([0004](0004-frontend-stack.md)). The CLI reinstall measures 69,108 B shell,
10,265 B largest lazy chunk, and 113,836 B total.

The product palette stays on `data-theme`. Default utilities read it through
the existing `--color-*` mappings, plus `--color-muted` and `--radius-md`.
App wrappers still own page contracts: Section keeps an h2, PageHeader owns the page's h1, status colors stay on
Badge, and the mobile sheet is constrained to the sidebar width. Close labels
use Paraglide. A disabled anchor Button does not run its click handler.
The component source is not an npm package, so `package.json` does not carry
its MIT notice. That notice ships once, as
`web/public/shadcn-svelte-LICENSE.txt`, which the build copies into `dist`.

Dialogs load on demand with their routes; the toaster is part of the shell
(79.7 KB shell gzip after #170). Accessibility remains implemented by
Bits UI. No router or device behavior changes are part of adoption. The
inventory lives in [the migration evidence](https://github.com/blackdark-org/travo-greenfield/blob/v0.2.0/docs/testing/shadcn-migration.md).
