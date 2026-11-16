# 0004 Frontend stack

Status: accepted

Svelte 5 + plain Vite static SPA, shadcn-svelte (only used components), hash routing, pre-gzipped assets, split gzip size budgets, system font stack, own SVG charts. Follows agent-presets `stack/embedded-ui.md`.

Deviation from the preset: Paraglide is used with `en` as the only locale for now (preset default kept; English-only is a locale list, not a rewrite). Record in `AGENTS.md`.

Implementation (`web/`):
- Pinned exactly in `web/package.json`; TypeScript 6.0.3. TS 7.0 (native port) has no classic JS compiler API: `@hey-api/openapi-ts` 0.99 crashes (`ts.SyntaxKind` undefined; fixed only in `@next` prereleases, hey-api#4235) and `svelte-check` 4.7 needs TS 6 installed next to TS 7 via an npm alias (sveltejs/language-tools#3063). Move to TS 7 when hey-api ships the TypeScript-free generator as `latest` and svelte-check drops the TS 6 requirement (planned with TS 7.1 APIs). Old Travo runs TS 7.0.2 (its ADR 0011) because it is React with oxlint: neither blocker applies there.
- pnpm's default `minimumReleaseAge` (24 h) is kept with no exclusions: pin versions older than a day.
- Generated: `web/src/lib/api/gen/**` (hey-api) is committed so the UI builds and reviews show API changes; `mise run check:gen` fails on drift, including new untracked files. `web/openapi.json` is only an intermediate written by `go run ./cmd/openapi` and git-ignored: the Go handlers are the single source, and the daemon serves the spec at runtime. Paraglide output is git-ignored (its default) and compiled in `postinstall`; the inlang message-format plugin is an npm devDependency, never fetched from a CDN.
- Dev: `mise run dev` / `mise run dev:firstrun` (mock daemon on `:18081`, Vite on `:18092`); `DAEMON_URL` overrides the proxy target.
- Below `md` the sidebar is an on-demand shadcn Sheet (ADR 0013): absent when closed, focus trapped while open, Escape closes, focus moves to Close and back to Menu.
- Biome overrides for `.svelte` files switch off `noUnusedImports`, `noUnusedVariables`, `useConst` and `useImportType`: Biome only parses the `<script>` block, so it cannot see template usage or runes (`$state`, `$props`) and would flag or rewrite code that is in fact used.
- Nav groups with collapse state and `GroupHeading` usage arrive with the first real grouped pages; the sidebar is flat until then.
- Pages are code-split per route (`web/src/lib/pages/lazy.ts`): each chunk loads on first visit behind the `LoadState` skeleton, a failed import shows the error with retry. The size budget counts `index.html`, its direct references and their transitive static JS imports as the shell; every other JS chunk is checked separately.
- `mise run check` enforces three budgets on the gzip level 9 `.gz` files written by `web/scripts/gzip-dist.mjs`: shell ≤ 128 KB, each other `.js` chunk ≤ 16 KB, and total `.html/.js/.css/.svg` ≤ 256 KB. The shell limit protects first load over slow hotel WiFi. It was raised from 40 KB so the UI can keep the default shadcn-svelte components (ADR 0013). The separate route-chunk limit and larger total leave room for page growth. Extra flash for those pages is negligible next to the ~12.8 MB Go binary.
