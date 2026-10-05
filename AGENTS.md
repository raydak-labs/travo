# Agent Instructions

## Read First

Pick by question, not by habit:

- `AGENTS.md` keeps repo workflow and guardrails, including the documentation rules below.
- `docs/README.md` is the **documentation map**: one table telling you which file answers which question.
- `docs/architecture/overview.md` keeps cross-cutting invariants, safety rules, and runtime invariants that predate or span ADRs. Update it whenever new essential behavior, subsystem contract, or operational constraint is decided.
- `docs/adr/README.md` indexes **architecture decision records** (ADRs). Before changing a subsystem, read the ADR for that area from that index (e.g. wireless → 0002, crash guards → 0003, firewall → 0004, mwan3 failover → 0005, platform/OpenAPI → 0006, auth → 0007, DNS/VPN/captive → 0001, UI language → 0012, cross-cutting invariants and device findings → 0013).
- `docs/guides/` holds task-shaped how-tos: `development.md`, `deployment.md`, `testing.md`.
- `docs/reference/` holds what things are: `ui-theming.md` (tokens, component contracts).
- `docs/tests/` holds on-device verification playbooks.
- `docs/plans/README.md` indexes live and normative plans. A shipped plan is deleted, not archived.
- `docs/requirements/tasks_open.md` is the working backlog; `tasks_done.md` is the completed log.
- `GLOSSARY.md` at the repo root fixes the words: summary band, status tone, stale, traffic history, configured vs live radio.
- `docs/_archive/requirements_done.md` is a legacy exhaustive archive; never a source of truth, never edit it.
- `docs/+ Start here.md` is the **Obsidian hub** (wikilinks) for the vault rooted at `docs/`.

## Documentation

Structure, enforced in parts by tests in `backend/internal/services/*_docs_test.go` and
`*_index_test.go`.

### Where a fact goes

One canonical home per fact. When you learn something, put it where a reader will
look for it, and link rather than duplicate:

| Fact | Home | Not |
| ---- | ---- | --- |
| A durable decision (hard to reverse, surprising without context, real trade-off) | a numbered ADR in `docs/adr/` | a plan, a code comment, a task |
| An invariant or safety rule that spans subsystems | `docs/architecture/overview.md` | an ADR, if one already owns it |
| How to do something | `docs/guides/<task>.md` | `docs/reference/` |
| What something is (tokens, tables, contracts) | `docs/reference/` | a guide |
| A check that only settles on hardware | `docs/tests/<area>-verification.md` | a guide, if the guide only runs commands |
| Work not yet shipped | `docs/plans/` (listed in `docs/plans/README.md`) | anywhere else |
| A term with a specific meaning in this project | `GLOSSARY.md` | a paragraph in a plan |
| Backlog / shipped log | `docs/requirements/tasks_open.md` / `tasks_done.md` | a plan's checklist |

Prefer a link over a second copy. A fact maintained in two places is one place
wrong within a month.

### Writing rules

- **Frontmatter on every doc**: `title`, `description`, `updated` (ISO date), `tags`.
  Plans also carry `status` (`active`, `normative`, `superseded`).
- **Section order inside an ADR** is fixed: Context → Decision → Why →
  Consequences and invariants → Deliberate ceilings. An ADR without a "Why" or
  without stated ceilings is a note, not a decision record.
- **Keep lines under 100 characters** and prose over bullet walls. Documents are read
  in a terminal and by agents with limited context.
- **Do not restate code in prose.** Point at the symbol (`VpnService.restoreDefaultRouteAfterWireGuardDisable`)
  instead of paraphrasing its body, or the doc rots on the next refactor.
- **Cite device findings with their evidence** (what was observed, on which firmware).
  A finding without evidence is an opinion.
- **Update the index you touched.** `docs/README.md`, `docs/adr/README.md` and
  `docs/plans/README.md` are checked mechanically; `docs/+ Start here.md` must link
  every ADR.
- **Link with paths that survive a move**: relative markdown links or Obsidian
  wikilinks, never a bare filename that resolves only by luck.

### Plans

- A plan is a unit of **unfinished** work. When it ships: fold its normative content
  into the ADR that owns the subsystem, add the outcome to `tasks_done.md`, then
  **delete the plan**. Shipped plans are not history worth keeping — git is the history.
- A plan that becomes a standing specification (a navigation rule, a disclosure table)
  keeps `status: normative` and stays listed; link to it from `docs/reference/`.
- Device findings and cross-cutting invariants go to
  [ADR 0013](docs/adr/0013-operational-invariants-and-device-findings.md), not into
  the plan that happened to discover them.
- Do not edit a plan's body to match current behaviour. Its value is that it records
  what was believed at the time.

### When you change behaviour

In the same change: update the owning ADR or guide, update
`docs/requirements/tasks_open.md` / `tasks_done.md`, and run
`cd backend && go test ./internal/services/` — the docs gates fail on unlinked
playbooks, ADR index drift, crash-guard paths outside `/etc/trafo/`, persistent-store
paths outside `/etc/travo/`, and a plans catalog that omits a plan.


## Tools & Setup

- everything should be installed via `mise`. See `.mise.toml`
- Run `make install` once after cloning or whenever dependencies change (installs pnpm packages and downloads Go modules)

## Project Plans

- All planning docs live under `docs/plans/` (see [`docs/plans/README.md`](docs/plans/README.md)).
- The directory holds **live and normative plans only**; a shipped plan is deleted once
  its decisions are folded into an ADR ([ADR 0013](docs/adr/0013-operational-invariants-and-device-findings.md)).
- Do **not** modify plan bodies except when executing an agreed change; prefer new dated plans for new work.

## Project Structure

This is a pnpm monorepo with:

- `frontend/` — React + TypeScript + Vite + TailwindCSS
- `backend/` — Go + Fiber
- `shared/` — Shared TypeScript types

## Documentation Workflow

The rules above are the source of truth. Operationally:

- `docs/README.md` is the documentation map; `docs/+ Start here.md` is its Obsidian twin. Update both when the layout changes.
- `docs/architecture/overview.md` stays a short overview that links deeper — never a second copy of an ADR.
- New normative cross-cutting decision → new numbered ADR + index entry + hub wikilink, in the same commit.
- Shipped plan → fold into an ADR, move to `tasks_done.md`, delete the plan.
- Before finishing: `cd backend && go test ./internal/services/` and a link check for anything you moved.

## Documentation retrieval (Obsidian CLI)

**Default for agents:** When working with project documentation, prefer the **Obsidian CLI** if it is installed and registered, and this checkout’s vault is visible as `docs` pointing at `<repo>/docs`.

1. **One-time setup (humans):** Obsidian **1.12+** → **Settings → General → Command line interface** → enable and install shell shim. After PATH changes: `hash -r` (bash) or `rehash` (zsh).

2. **Verify:**
   - `command -v obsidian`
   - `obsidian vault list` — expect `name` `docs` with `path` matching this repository’s `docs/` directory.

3. **Typical commands** (pass `vault=docs` explicitly; cwd can be anywhere once the vault is registered):
   - Search: `obsidian search query="mwan3 failover" vault=docs`
   - Read note body to stdout: `obsidian read path=architecture/overview.md vault=docs` — paths are **vault-relative** (`adr/0002-wireless-model-and-luci-apply.md`, `requirements/tasks_open.md`, …). Quote paths that start with `+` or contain spaces, e.g. `obsidian read path='+ Start here.md' vault=docs`. Quote paths that start with `+` or contain spaces, e.g. `obsidian read path='+ Start here.md' vault=docs`.
   - Backlinks: `obsidian backlinks path=adr/0001-dns-vpn-captive-portal-architecture.md vault=docs`
   - More: `obsidian help`

4. **Graph / wikilinks in the app:** Open the vault folder `docs/` in Obsidian; start at **`+ Start here.md`**.

**Fallback:** No CLI, wrong vault path, headless CI, or command errors → use `Read` / `Grep` / `SemanticSearch` on `docs/` (same Markdown files). Do not block work on the Obsidian app being open.

## Frontend UI & Theming

- Dark mode is the `dark` class on `document.documentElement`, driven by `ThemeProvider` and an inline boot script in `frontend/index.html`. See `docs/reference/ui-theming.md` for tokens and patterns.
- **Do not** set explicit text colors (`text-gray-900`, hex, inline `color`, etc.) for ordinary copy unless there is no reasonable alternative. Prefer inheriting from global `body` styles in `frontend/src/index.css` or using shared primitives (`CardTitle`, `CardDescription`, buttons) that already pair light/dark.
- When you must set color, always provide **both** light and dark variants (for example `text-gray-500 dark:text-gray-400`). Charts and embedded SVGs may use the `--chart-*` CSS variables in `index.css` instead of hard-coded text fills.
- **Exceptions**: status or semantic hues, charts, third-party widgets, badges, and contrast inside intentionally colored surfaces. Still keep dark-mode variants where users can enable dark theme.

## Development

- Run `pnpm install` for Node dependencies
- Run `cd backend && go mod tidy` for Go dependencies
- Run `make test` to run all tests
- Run `make dev` for development servers
- Run `make format` for formatting code
- Run `make lint` to run lint checks
- Run `make build` to build the projects

## Testing

Follow TDD: write tests first, see them fail, write minimal code to pass.

- Go tests: `cd backend && go test ./...`
- Shared tests: `cd shared && pnpm test`
- Frontend tests: `cd frontend && pnpm test`

## Real Device Validation

- You have access to the OpenWRT test environment via SSH at `192.168.1.1`.
- You may execute commands on the device and copy files to it.
- For `scp`, use the legacy option.
- Commands like `rg` are not available on the target system; use simpler tools like `grep`.
- Always test important device behavior directly on the target system, either with a browser, `curl`, or SSH-level validation.
- If useful, cross-check with real configs on the device.
- If direct device access would reduce guesswork, ask the user so you can validate behavior before implementing blindly.

## Safety-Critical System Changes

Follow full rationale and examples in `docs/architecture/overview.md`. Core rules:

- Any automated action that modifies live system state must use a crash guard file in `/etc/trafo/<feature>-in-progress` (ADR 0003 §2 lists every path; non-guard state stays in `/etc/travo/`).
- Remove guard files only after the dangerous operation completes successfully.
- A manual redeploy via `deploy-local.sh` clears guard files and is the explicit retry path.
- Wireless changes must preserve LuCI-style rollback semantics: backend uses rpcd `uci apply` plus rollback plus `uci confirm`.
- Scripts and SSH flows must not run `wifi` or `wifi up` as part of applying user wireless changes. They write UCI only; the user applies via LuCI "Save & Apply" or reboot.
- Do not use `wifi reload` on ath11k/IPQ6018. Where `wifi up` already exists for bounded recovery logic, keep that exception explicit and documented.
- If you add new zones, interfaces, or routing paths, include all required firewall changes and follow existing default `wan` patterns.
- Any new background goroutine or scheduled task that changes live state must follow the same guard and rollback rules. No exceptions.

## API Documentation Endpoint

Contract:

```text
GET /api/openapi.json
```

- This endpoint is meant to be machine-readable OpenAPI 3.0 for automation and tests.
- It should remain available on the test device at `http://192.168.1.1/api/openapi.json`.
- Protected endpoints use `POST /api/v1/auth/login` and `Authorization: Bearer <token>`.

## Finish Task

- Before finishing, ensure lint, tests, and build pass. Check `Makefile` for the canonical commands.
- If tests fail, focus on fixing and rerunning them, then run the top-level `make` checks again before finishing.
