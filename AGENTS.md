# Agent Instructions

## Project Plans

The plan directory is `plans/`. Do NOT modify plan files.

The important file for the requirements is in `docs/requirements/requirements.md`.
When we implement things ensure that you update the feature sets or add features to the document.

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
- Run `make format` for formatting code
- Run `make lint` to run lint check
- Run `make build` to build the projects

## Testing

Follow TDD: write tests first, see them fail, write minimal code to pass.
- Go tests: `cd backend && go test ./...`
- Shared tests: `cd shared && pnpm test`
- Frontend tests: `cd frontend && pnpm test`

## Real tests

- if we are testing on the real system you have access to the OpenWRT test environemnt via ssh to IP 192.168.1.1
- You are allowed execute and copy everything to the system
- for scp you have to use the legacy option and commands like rg are not available on the target system, use the simpler alternatives like grep
- always test things directly on the target system either with a browser if you capable to or via curl
- if needed cross check with the real configurations on the system

## Commit / finish task

- before you finish the task you must ensure the lint, tests and build are working (check Makefile for general things)
- if you have failed tests focus only on running them and at the end again test with Makefile
