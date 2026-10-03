---
title: "ADR 0011: Frontend lint toolchain (oxlint and TypeScript 7)"
status: Accepted
date: 2026-10-02
updated: 2026-10-02
tags: [adr, tooling, oxlint, eslint, typescript, frontend, ci]
---

# ADR 0011: Frontend lint toolchain (oxlint and TypeScript 7)

A developer-tooling decision. It changes no runtime behaviour and no safety
invariant; it constrains how contributors lint and how CI gates the frontend.

## Status

Accepted.

## Decision

The frontend is linted with **oxlint** and type-checked with **TypeScript 7**
(`7.0.2`).

Removed: ESLint, `@eslint/js`, `typescript-eslint`,
`eslint-plugin-react-hooks`, `eslint-plugin-react-refresh`, `globals`, and the
flat config `eslint.config.js`. Added: `oxlint`, and the root config
`.oxlintrc.json` in their place.

82 of the 85 previously-enabled ESLint rules are ported 1:1 at identical
severity; the three accepted losses are listed below. `eslint/no-undef` is
*added* on top — the typescript-eslint baseline had it off — giving 83 enabled
rules.

Prettier is untouched and remains the **only** formatter — oxlint does not
format, and `make format-check` is unchanged.

## Why

`typescript-eslint` declares a peer range of `>=4.8.4 <6.1.0` on the TypeScript
compiler. TypeScript 7 falls outside it, so the toolchain could not adopt TS 7
while ESLint was in place; the Renovate `allowedVersions` escape hatch for the
`TypeScript 7 — blocked by typescript-eslint` hold was only ever a pin, not a
solution.

oxlint has **no `typescript` peer at all**, and its type-aware backend is built
on **typescript-go**, so the constraint disappears rather than being negotiated
around. The lint baseline itself fits in a Rust binary that runs an order of magnitude
faster, which matters because lint is a per-commit gate on every package.

## Consequences and invariants

These are the parts a future contributor can accidentally undo.

### `.oxlintrc.json` is hand-enumerated, and that is deliberate

oxlint rule **categories** do not correspond to ESLint presets, so there is no
mechanical translation to preserve. The config lists rules one by one for that
reason.

Two things follow, and they are independent:

- `categories.correctness: "error"` on its own enables **every** correctness rule
  of the enabled plugins. The old ESLint baseline is a **subset** of that, not an
  equal of it.
- The explicit `rules` map therefore does not *create* that coverage. It **pins
  severities** (so `react/exhaustive-deps` stays `warn`) and **re-adds baseline
  rules that oxlint classifies outside `correctness`** — `react/rules-of-hooks`
  is `pedantic`, which is off. Removing the map would not weaken the config; it
  would let those severities and that one critical rule drift.

`plugins` is likewise explicit. `unicorn` and `oxc` are excluded because they
were never part of the ESLint baseline and would introduce hundreds of findings
the project has never seen. Do not "helpfully" add them without re-running a
parity check.

### A new oxlint minor can turn CI red on its own

oxlint may add a rule to the `correctness` category in a minor release, and this
config enables that category wholesale — so an unrelated dependency bump can
produce a red lint run (and a red Renovate PR). **This is intentional: the
config is self-gating.** Review the new finding and either fix it or pin the
severity; do not silence it by turning `correctness` off, which would drop the
entire baseline at once.

### Accepted parity losses

Three previously-enabled ESLint rules have no oxlint 1.86 equivalent:

| Old rule | oxlint namespace | Effect here |
| --- | --- | --- |
| `eslint/no-octal` | *(none)* | none — no legacy octal literals |
| `react-hooks/config` | `react/config` | none |
| `react-hooks/gating` | `react/gating` | none |

These are recorded, not worked around. If a future rule with one of these names
becomes needed, re-check the oxlint schema rather than assuming it is still
missing.

### Type-aware linting is off, and is not a CI gate

`oxlint-tsgolint` was evaluated and deliberately **not enabled**. It sits outside
oxlint's semver guarantee, and no enabled rule needs it.

It was run once against the whole tree to find out what it would cost: it surfaced
6 findings, all since fixed (2 genuine in app code, 4 in Vitest test files). The
tree is therefore clean under `oxlint --type-aware frontend/src shared/src` today,
so enabling it later is a config-only change — but it stays off until that
semver gap is worth closing deliberately.

It is nonetheless present in the dependency graph: oxlint declares it an
*optional* peer and pnpm auto-installs peers, so the lockfile resolves
`oxlint: 1.86.0(oxlint-tsgolint@7.0.2003)` and the platform binaries are fetched
on every install. That is harmless — behaviour is unaffected and CI does not
depend on it — but do not read the lockfile entry as type-aware linting being
active.

### One lint config, one command

`.oxlintrc.json` at the repo root is the single config. `pnpm lint` from the root
is the developer entry point; `pnpm lint:ci` (used by CI) adds
`--format=github` for deterministic GitHub annotations. The per-package
`frontend` and `shared` `lint` scripts rely on oxlint's upward config discovery
and pass no path flags.

Inline suppressions keep the `eslint-disable-next-line` prefix — oxlint honours
it via `respectEslintDisableDirectives: true` — but name rules in **oxlint's**
namespace: `react/only-export-components`, not the old
`react-refresh/only-export-components`.

**Related:** [ADR 0006](./0006-application-platform-and-api-contract.md) (repo
shape and tooling), `docs/development.md` (day-to-day commands).