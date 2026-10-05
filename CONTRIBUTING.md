# Contributing to Travo

Thank you for your interest in contributing! Here's how to get started.

Agent-facing instructions (workflow, guardrails, finish criteria) live in
[`AGENTS.md`](./AGENTS.md) — read that before you start.

## Getting Started

1. **Fork** the repository on GitHub
2. **Clone** your fork locally:
   ```bash
   git clone https://github.com/YOUR_USERNAME/travo.git
   cd travo
   ```
3. **Install dependencies** (through the pinned [mise](https://mise.jdx.dev/) toolchain —
   `pnpm install` plus `go mod download`):
   ```bash
   make install
   ```
4. **Create a branch** for your feature or fix:
   ```bash
   git checkout -b feat/my-feature
   ```

## Development Workflow

1. Make your changes
2. Write or update tests (TDD: see the failing test first where practical)
3. Run the test suite:
   ```bash
   make test
   ```
4. Ensure linting passes:
   ```bash
   make lint
   ```
5. Format your code:
   ```bash
   make format
   ```

## Code Style

- **TypeScript/React**: follow the oxlint (`.oxlintrc.json`) + Prettier configuration in the repo
- **Go**: follow standard `gofmt` formatting; `make lint` runs `golangci-lint` over the
  backend, which is stricter than `go vet`
- Use meaningful commit messages following [Conventional Commits](https://www.conventionalcommits.org/)

## Safety-Critical Changes

Anything that mutates live device state (UCI, firewall, routes, flash) must follow the
crash-guard and rollback rules in [`AGENTS.md`](./AGENTS.md) and
[`docs/architecture/overview.md`](docs/architecture/overview.md). In particular: never run
`wifi` / `wifi up` / `wifi reload` from a script or SSH flow, and never introduce a
background goroutine or scheduled task that changes live state without a crash guard.

## Pull Requests

1. Push your branch to your fork
2. Open a Pull Request against `main`
3. Describe what your PR does and link any relevant issues
4. Ensure CI checks pass
5. Request a review

## Testing

- **Backend (Go)**: write tests in `*_test.go` files alongside the code; `cd backend && go test ./...`
- **Frontend (TypeScript)**: write tests in `__tests__/` directories using Vitest; `cd frontend && pnpm test`
- **Shared**: write tests in `shared/src/__tests__/`; `cd shared && pnpm test`
- **On-device**: `make integration` runs the suites in `test/integration/` against a real
  router. They mutate the device and are never run by CI — ask before pointing them at
  anything you care about.

All PRs must have passing tests. Write tests first (TDD) when possible.

## Reporting Issues

- Use GitHub Issues to report bugs or request features
- Include device info (router model, firmware version) when reporting device-specific issues
- Include steps to reproduce for bug reports

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
