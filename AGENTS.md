# Agent Instructions

## Project Plans

The plan directory is `plans/`. Do NOT modify plan files.

## Project Structure

This is a pnpm monorepo with:
- `frontend/` — React + TypeScript + Vite + TailwindCSS
- `backend/` — Go + Fiber
- `shared/` — Shared TypeScript types

## Development

- Run `pnpm install` for Node dependencies
- Run `cd backend && go mod tidy` for Go dependencies
- Run `make test` to run all tests
- Run `make dev` for development servers

## Testing

Follow TDD: write tests first, see them fail, write minimal code to pass.
- Go tests: `cd backend && go test ./...`
- Shared tests: `cd shared && pnpm test`
- Frontend tests: `cd frontend && pnpm test`
