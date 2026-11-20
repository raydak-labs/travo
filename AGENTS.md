# travo-greenfield

Travel-router web UI and API (Go daemon + Svelte SPA) that runs on OpenWrt next to LuCI. Clean-room rewrite of Travo.

## Verify

`mise run check` green before every commit (Go, web, generated client, size). After API changes run `mise run gen` and commit the result. Hardware-only behaviour is gated by the playbook in `docs/testing.md`.

## Stack

Follows https://github.com/BlackDark/agent-presets/blob/main/stack/core.md and `stack/embedded-ui.md`.
Deviations:
- Backend Go with Huma-generated OpenAPI + hey-api client: no hand-written spec (docs/adr/0003).
- Paraglide with `en` only (docs/adr/0004).
- No Zod in the daemon path; Zod Mini/Valibot only if a UI boundary outside the generated client needs runtime validation (none so far, docs/adr/0031).
- Distribution is a release tarball + install script, not a package (docs/adr/0007).

## Map

- `docs/invariants.md` MUST rules; read before touching live state, auth, DNS, WiFi. Breaking one needs an ADR.
- `docs/device-facts.md` hardware lessons; `docs/testing.md` test layers and fixture rules.
- `docs/web-ui.md` page rules, and how pages confirm risky changes (dialog) and report results (toasts).
- `docs/adr/README.md` decision index; `GLOSSARY.md` terms (use them verbatim in code, tests, issues).
- `testdata/device/<device>-<firmware>/` verbatim device captures with `PROVENANCE.md` and redaction policy; `SPIKES.md` there has hardware results.
- `.out-of-scope/` rejected feature requests, one file per concept; `/triage` checks it.

There is no spec file. Product intent lives in issues (PRD, then tickets); shipped behaviour lives in tests and the generated OpenAPI.

## Work

Work is GitHub issues, rules are `docs/`. Before any task, read `docs/agents/workflow.md`: how an issue becomes a brief, who implements, reviews and merges, and when a `needs-hardware` issue may close.
- Implement only `ready-for-agent` issues with no open blocker; an `epic` gets `/to-tickets` first.
- The brief is the contract. When it contradicts the repo, stop and report `blocked`: the coordinator fixes the brief.
- A decision made while implementing lands in an ADR or `docs/invariants.md` in the same PR; the issue links it.

## Agent skills

### Issue tracker

GitHub issues on `blackdark-org/travo-greenfield`, milestone of the next release (`gh api repos/blackdark-org/travo-greenfield/milestones --jq ".[].title"`), project board 1, native sub-issues and blocked-by. See `docs/agents/issue-tracker.md`.

### Triage labels

Canonical role names, plus `epic`, `area/*`, `model:*`, `needs-hardware`. See `docs/agents/triage-labels.md`.

### Domain docs

Single context: `GLOSSARY.md`, `docs/adr/`, `docs/invariants.md`, `docs/device-facts.md`. See `docs/agents/domain.md`.

## Every change

Same PR, only what the change touches:
- tests for the changed behaviour; fixtures are verbatim device payloads with provenance, never invented fields
- docs that describe it (invariants, ADR when a decision changes)
- README when a user-visible feature changes
- screenshots after visual changes

## Code style

- Data in, data out: pure functions, side effects (I/O, time, randomness) at the edges and passed in.
- Expected failures are values (`error` in Go, `Result`/unions in TS); panic/throw for bugs only.
- Composition over inheritance.
- Small units: split a file past ~400 lines, a function past ~60.
- Names explain what, comments explain why.
- Refactors keep behaviour and land in their own commit.
- All OpenWrt access goes through the `Device` / `PackageManager` ports; all live-state changes through the transaction module; all DNS changes through the layer stack.

## Stewardship

Leave every touched area cleaner than found.
- Small fixes beside the change (dead code, stale doc, missing test, unclear name): own commit, same PR.
- Whatever the change makes obsolete goes in the same PR.
- Larger problems: report with `file:line` and a proposal; the PR stays on topic.

## Git

- Conventional Commits. One topic per PR.
- Merge: rebase when commits are logically separate, squash otherwise.
- PR already merged → branch anew from `origin/main`, open a new PR.
- `--force-with-lease` on own branches only.

## Boundaries

- `sudo`, system packages, global installs: the user runs them; ask with what and why.
- No `.github/FUNDING.yml` yet: ask, then copy https://github.com/BlackDark/agent-presets/blob/main/templates/FUNDING.yml
- Running user processes stay up; test on other ports.
- Never run destructive probes against a real device; GET-only first.
- Language: code, comments, docs, commits in English; UI English via Paraglide.

## Gotchas

- Never call `wifi`, `wifi up/down/reload` in an apply path (ath11k crash).
- Netifd route state lies; read the kernel table.
- Fixtures that invent fields caused the worst shipped bugs.
