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

| Command           | Description                        |
| ----------------- | ---------------------------------- |
| `make dev`        | Run frontend + backend dev servers |
| `make build`      | Build frontend and backend         |
| `make test`       | Run all tests (Go + Vitest)        |
| `make lint`       | Run ESLint + go vet                |
| `make format`     | Run Prettier + gofmt               |
| `make clean`      | Remove all build artifacts         |
| `make docker-dev` | Start Docker dev environment       |

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
