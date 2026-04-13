---
title: Development guide
description: Local prerequisites, install, dev servers, tests, lint, CI, MSW.
updated: 2026-04-13
tags: [docs, development, workflow]
---

# Development Guide

Tooling versions are also pinned in [`.mise.toml`](../.mise.toml) (use [mise](https://mise.jdx.dev/) if you want a single installer).

## Prerequisites

- **Node.js** >= 20 (CI uses 24)
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
| `make lint` | `pnpm lint` (ESLint on frontend + shared) + `golangci-lint` (backend) |
| `make format` | Prettier + goimports |
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
├── backend/      # Go + Fiber
├── shared/       # Shared TS types
├── scripts/      # dev, deploy, install helpers
├── test/integration/  # real-device integration runners (SSH + API)
├── docs/plans/   # Design history (read [`docs/plans/README.md`](./plans/README.md))
├── packaging/    # OpenWrt staging files
├── docs/         # This documentation
└── Makefile
```

## Tests

```bash
make test
cd backend && go test ./...
cd shared && pnpm test
cd frontend && pnpm test
```

## Backend mock mode

Without OpenWrt, run with **in-memory** UCI/ubus:

- **Air** (`brew install air` / mise): `backend/.air.toml` runs `./tmp/server --mock`.
- **Once:** `cd backend && go run ./cmd/server --mock`
- **Env:** `MOCK_MODE=true go run ./cmd/server`

Plain `go run ./cmd/server` (no flag) expects real UCI/ubus and is mainly for on-device debugging.

## CI (GitHub Actions)

- [`.github/workflows/ci.yml`](../.github/workflows/ci.yml): backend tests, shared + frontend tests, ESLint, golangci-lint, build check.
- [`.github/workflows/release.yml`](../.github/workflows/release.yml): release artifacts on version tags.
- [`.github/workflows/race-detector.yml`](../.github/workflows/race-detector.yml): optional Go race runs.

## Releases

Tag and push; the release workflow builds tarballs and publishes them (see `docs/deployment.md`).

```bash
git tag v1.0.0
git push origin v1.0.0
```

## MSW

The dev frontend can use [MSW](https://mswjs.io/) against the real backend or mocks. Regenerate the worker if needed:

```bash
cd frontend && pnpm exec msw init public --save
```
