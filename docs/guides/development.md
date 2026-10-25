---
title: Development guide
description: Local prerequisites, install, dev servers, tests, lint, CI, MSW.
updated: 2026-10-25
tags: [docs, development, workflow]
---

# Development Guide

Tooling versions are also pinned in [`.mise.toml`](../../.mise.toml) (use [mise](https://mise.jdx.dev/) if you want a single installer).

## Prerequisites

- **Node.js** >= 22.18 (orval's minimum; mise and CI use the version in `.mise.toml`)
- **pnpm** 10.x (see `package.json` / lockfile)
- **Go** >= 1.23 (`backend/go.mod`)
- **Docker** + Docker Compose (optional)
- **golangci-lint** (for `make lint` backend pass)
- **air** (optional; enables `--mock` reload for backend — see below)

## Quick start

```bash
git clone https://github.com/raydak-labs/travo.git
cd travo
pnpm install
cd backend && go mod tidy && cd ..
make dev
```

- Frontend: http://localhost:5173  
- Backend API: http://localhost:3000  
- Local login password is whatever you configure for mock auth (often `admin` in examples).

`pnpm install` runs `postinstall` and generates `frontend/public/mockServiceWorker.js`. If the app is a blank page, run:

```bash
cd frontend && pnpm exec msw init public --save
```

## Docker

```bash
docker compose up
```

Same ports; Vite HMR + Air if configured in the image.

## Common `make` targets

| Command | Description |
| ------- | ----------- |
| `make dev` | Frontend (Vite) + backend (`scripts/dev.sh`) |
| `make build` | Production frontend build + `backend/bin/server` |
| `make test` | `go test` (backend) + `pnpm test` (shared + frontend) |
| `make lint` | `pnpm lint` (oxlint on frontend + shared) + `golangci-lint` (backend) |
| `make format` | Prettier + goimports |
| `make generate` | Regenerate `shared/openapi.json` and the TS API types from the backend handlers |
| `make build-prod` / `make build-all` | Cross-compile `dist/travo` via `scripts/build.sh` |
| `make package` / `make package-all` | Release tarballs `dist/travo_<version>_<arch>.tar.gz` |
| `make deploy` | `deploy-local.sh` over SSH (`DEPLOY_METHOD=direct` default, `ROUTER_IP`) |
| `make deploy-local` | Same as `deploy`; pass extra flags via `DEPLOY_ARGS` |
| `make docker-dev` | `docker compose up` |
| `make clean` | Remove build artifacts |

## Repo layout

```
travo/
├── frontend/     # React + Vite + Tailwind
├── backend/      # Go + Fiber, one package per domain
├── shared/       # TS API types (partly generated, see below)
├── tools/openapi-gen/  # orval: OpenAPI → TS types and request functions
├── scripts/      # dev, deploy, install helpers
├── test/integration/  # real-device integration runners (SSH + API)
├── docs/         # Obsidian vault; hub [`+ Start here.md`](../+ Start here.md); ADRs in docs/adr/
├── packaging/    # OpenWrt staging files
└── Makefile
```

## Changing the API

The OpenAPI document is generated from the handler types (ADR 0006 §4), and the frontend's API
types are generated from it:

1. Change or add the operation in `backend/internal/api/*_api.go` (`get`, `post`, `put`, `del`)
   and the Go types it uses. Narrow string fields with `enum:"a,b"` where the backend only ever
   sends those values.
2. Run `make generate`. It rewrites `shared/openapi.json`, the schema types in
   `shared/src/generated/model/`, the path union in `shared/src/generated/paths.ts` and the
   request functions in `frontend/src/api/endpoints.ts` (one per `operationId`).
3. Commit the generated files. CI's lint job fails when they are stale.

Hooks call the generated functions (`getWiFiConnection()`, `setAPConfig(section, body)`); they
go through `apiFetch` in `frontend/src/lib/api-client.ts`, which adds the token and turns errors
into `ApiError`. In tests, mock the function with `vi.mock('@/api/endpoints', …)`.

`API_ROUTES` (`shared/src/api/routes.ts`) is checked against the generated `ApiPath` union: a
route the backend does not serve fails to compile.

`shared/src/api/*.ts` re-exports the generated types under their existing names. Unions such as
`WifiMode` are derived from them; the few types still written by hand are UI-side (for example
`GroupedScanNetwork`) or request values the backend does not describe as a fixed set.

The generator is orval (`tools/openapi-gen/orval.config.mjs`), kept in its own workspace
package so its dependency tree stays out of the app. `postprocess.mjs` encodes path parameters
and writes `paths.ts`. Why orval and not openapi-typescript: ADR 0006 §Why.

## Tests

```bash
make test
cd backend && go test ./...
cd shared && pnpm test
cd frontend && pnpm test
```

## Linting

The repo root [`.oxlintrc.json`](../../.oxlintrc.json) is the single lint config.

```bash
mise exec -- pnpm lint          # frontend/src + shared/src
mise exec -- pnpm lint:ci       # same, but --format=github for CI annotations
```

`make lint` runs that plus `golangci-lint` on the backend. Per-package
`frontend` / `shared` scripts pick the root config up by upward discovery, so
they take no `-c` flag.

- **Prettier is still the formatter.** oxlint does not format; `make format-check`
  is unchanged.
- **Type-aware linting is deliberately off.** `oxlint-tsgolint` is present only
  as an auto-installed optional peer of oxlint; no type-aware rule is enabled
  and CI does not depend on it.

Why oxlint and not ESLint, what the rule map does and does not do, and the
accepted parity losses: [ADR 0011](../adr/0011-frontend-lint-toolchain-oxlint-and-typescript-7.md).

## Backend mock mode

Without OpenWrt, run with **in-memory** UCI/ubus:

- **Air** (`brew install air` / mise): `backend/.air.toml` runs `./tmp/server --mock`.
- **Once:** `cd backend && go run ./cmd/server --mock`
- **Env:** `MOCK_MODE=true go run ./cmd/server`

Plain `go run ./cmd/server` (no flag) expects real UCI/ubus and is mainly for on-device debugging.

## CI (GitHub Actions)

- [`.github/workflows/ci.yml`](../../.github/workflows/ci.yml): backend tests, shared + frontend tests, oxlint, golangci-lint, build check.
- [`.github/workflows/release.yml`](../../.github/workflows/release.yml): release artifacts on version tags.
- [`.github/workflows/race-detector.yml`](../../.github/workflows/race-detector.yml): optional Go race runs.

## Releases

Tag and push; the release workflow builds tarballs and publishes them (see `docs/guides/deployment.md`).

```bash
git tag v1.0.0
git push origin v1.0.0
```

## MSW

The dev frontend can use [MSW](https://mswjs.io/) 3 against the real backend or mocks.
Handlers import `http` and `HttpResponse` from `msw/http`. `setupServer` and
`setupWorker` stay on `msw/node` and `msw/browser`. Unhandled frames use
`onUnhandledFrame` (`bypass` in `main.tsx`, `error` in `src/test/setup.ts`).
Regenerate the worker if needed:

```bash
cd frontend && pnpm exec msw init public --save
```
