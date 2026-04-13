# Agent Instructions

## Read First

- `AGENTS.md` keeps repo workflow and guardrails.
- `docs/architecture.md` keeps stable architecture decisions, safety rules, and runtime invariants. Update it whenever new essential behavior, subsystem contract, or operational constraint is decided.
- `docs/requirements/tasks_open.md` is the working backlog.
- `docs/requirements/tasks_done.md` is the completed task log.
- `docs/_archive/requirements_done.md` is a legacy exhaustive archive; do not use it as the active backlog.
- `docs/README.md` is the broader documentation map.

## Tools & Setup

- everything should be installed via `mise`. See `.mise.toml`

## Project Plans

- All planning docs live under `docs/plans/` (see `docs/plans/README.md`).
- Do **not** modify plan file bodies except when executing an agreed change; prefer new dated plans for new work.

## Project Structure

This is a pnpm monorepo with:

- `frontend/` — React + TypeScript + Vite + TailwindCSS
- `backend/` — Go + Fiber
- `shared/` — Shared TypeScript types

## Documentation Workflow

- Keep `docs/architecture.md` current for stable design decisions, invariants, safety constraints, and important cross-component behavior.
- Keep `docs/requirements/tasks_open.md` and `docs/requirements/tasks_done.md` current when scope changes or work is completed.
- Keep `docs/README.md` as the documentation map (requirements pointers live there).
- Prefer short overview docs that link deeper instead of putting backlog, architecture, and historical notes into one file.

## Frontend UI & Theming

- Dark mode is the `dark` class on `document.documentElement`, driven by `ThemeProvider` and an inline boot script in `frontend/index.html`. See `docs/ui-theming.md` for tokens and patterns.
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

Follow full rationale and examples in `docs/architecture.md`. Core rules:

- Any automated action that modifies live system state must use a crash guard file in `/etc/travo/<feature>-in-progress`.
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
