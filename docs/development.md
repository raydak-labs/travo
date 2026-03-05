# Development Guide

## Prerequisites

- **Node.js** >= 20 (22 recommended)
- **pnpm** >= 9
- **Go** >= 1.23
- **Docker** + Docker Compose (optional, for containerized dev)

## Quick Start

```bash
git clone https://github.com/openwrt-travel-gui/openwrt-travel-gui.git
cd openwrt-travel-gui
pnpm install
cd backend && go mod tidy && cd ..
make dev
```

> **Note:** `pnpm install` automatically generates the MSW (Mock Service Worker) file
> needed for frontend development via the `postinstall` script. If you see a white page
> after starting the frontend, run `cd frontend && pnpm exec msw init public --save`
> to regenerate it.

- Frontend: http://localhost:5173
- Backend API: http://localhost:3000
- Default password: `admin`

## Docker Development

```bash
docker compose up
```

Same ports. Hot reload for both frontend (Vite HMR) and backend (Air).

## Commands

| Command            | Description                            |
| ------------------ | -------------------------------------- |
| `make dev`         | Run frontend + backend dev servers     |
| `make build`       | Build frontend and backend             |
| `make test`        | Run all tests (Go + shared + frontend) |
| `make lint`        | Run ESLint + go vet                    |
| `make format`      | Run Prettier + gofmt                   |
| `make clean`       | Remove all build artifacts             |
| `make build-prod`  | Cross-compile for OpenWRT (aarch64)    |
| `make build-all`   | Cross-compile for aarch64 and x86_64   |
| `make package`     | Create .ipk package (default arch)     |
| `make package-all` | Create .ipk for aarch64 and x86_64     |
| `make deploy`      | Deploy to router (needs `ROUTER_IP`)   |
| `make docker-dev`  | Start Docker dev environment           |

## Project Structure

```
openwrt-travel-gui/
├── frontend/          # React + TypeScript + Vite + TailwindCSS
├── backend/           # Go + Fiber REST API
├── shared/            # Shared TypeScript types
├── scripts/           # Dev, build, and deploy scripts
├── packaging/         # OpenWRT package files
├── docs/              # Documentation
└── Makefile           # Task runner
```

## Testing

`make test` runs all tests across the monorepo: Go backend, shared TypeScript types,
and frontend React components.

```bash
# All tests
make test

# Backend only
cd backend && go test ./...

# Frontend only
cd frontend && pnpm test

# Shared only
cd shared && pnpm test
```

## Linting

```bash
make lint     # ESLint + go vet
make format   # Prettier + gofmt
```

## Backend Mock Mode

The backend runs with `MOCK_MODE=true` during development, providing in-memory fakes for UCI/ubus so no actual OpenWRT device is needed.

## CI/CD

### Continuous Integration

The CI workflow (`.github/workflows/ci.yml`) runs on every push to `main` and on pull requests:

1. **Test Backend** — `go vet`, `go test ./... -count=1 -race`
2. **Test Frontend** — shared and frontend Vitest suites
3. **Lint** — ESLint + Go vet
4. **Build Check** — verifies both frontend and backend build successfully

### Release Workflow

The release workflow (`.github/workflows/release.yml`) triggers on version tags:

1. Builds the binary and `.ipk` package for both **aarch64** and **x86_64**
2. Generates SHA256 checksums
3. Creates a GitHub Release with all artifacts and install instructions

### Creating a Release

Tag the commit and push:

```bash
git tag v1.0.0
git push origin v1.0.0
```

The release workflow will automatically build binaries and packages for all
supported architectures and create a GitHub Release with download links.

## MSW (Mock Service Worker)

The frontend uses [MSW](https://mswjs.io/) to mock API responses during development
and testing. The service worker file (`frontend/public/mockServiceWorker.js`) is
generated automatically by the `postinstall` script when running `pnpm install`.

If the frontend shows a blank page, the MSW worker file may be missing. Regenerate it:

```bash
cd frontend && pnpm exec msw init public --save
```
