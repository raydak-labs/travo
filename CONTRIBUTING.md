# Contributing to OpenWRT Travel GUI

Thank you for your interest in contributing! Here's how to get started.

## Getting Started

1. **Fork** the repository on GitHub
2. **Clone** your fork locally:
   ```bash
   git clone https://github.com/YOUR_USERNAME/openwrt-travel-gui.git
   cd travo
   ```
3. **Install dependencies:**
   ```bash
   pnpm install
   cd backend && go mod tidy && cd ..
   ```
4. **Create a branch** for your feature or fix:
   ```bash
   git checkout -b feat/my-feature
   ```

## Development Workflow

1. Make your changes
2. Write or update tests
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

- **TypeScript/React**: Follow the ESLint + Prettier configuration in the repo
- **Go**: Follow standard `gofmt` formatting and `go vet` checks
- Use meaningful commit messages following [Conventional Commits](https://www.conventionalcommits.org/)

## Pull Requests

1. Push your branch to your fork
2. Open a Pull Request against `main`
3. Describe what your PR does and link any relevant issues
4. Ensure CI checks pass
5. Request a review

## Testing

- **Backend (Go)**: Write tests in `*_test.go` files alongside the code
- **Frontend (TypeScript)**: Write tests in `__tests__/` directories using Vitest
- **Shared**: Write tests in `shared/src/__tests__/`

All PRs must have passing tests. Write tests first (TDD) when possible.

## Reporting Issues

- Use GitHub Issues to report bugs or request features
- Include device info (router model, firmware version) when reporting device-specific issues
- Include steps to reproduce for bug reports

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
