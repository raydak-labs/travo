.PHONY: dev build test lint format format-check shellcheck integration clean build-prod build-all package package-all deploy docker-dev install coverage

# Run a command through mise's resolved toolchain (go, node, pnpm, golangci-lint),
# so recipes are correct regardless of the caller shell's PATH/GOROOT/etc.
MISE := $(shell command -v mise 2>/dev/null)
ifneq (,$(MISE))
RUN := $(MISE) exec --
else
RUN :=
endif

# Install all dependencies (run once after cloning or after dep changes)
install:
	$(RUN) pnpm install
	cd backend && $(RUN) go mod download

# Run frontend and backend dev servers concurrently
dev:
	@bash scripts/dev.sh

# Emit the mise-pinned toolchain versions as KEY=VALUE lines so CI never keeps a
# second hardcoded copy (.mise.toml stays the single source of truth).
# Uses awk only, so it works in a bare CI runner with no mise/pnpm installed.
toolchain-versions:
	@awk '/^[[:space:]]*(node|pnpm|golangci-lint|shellcheck)[[:space:]]*=/ { \
		key = $$1; \
		val = $$3; gsub(/"/, "", val); \
		gsub(/-/, "_", key); \
		print key "=" val; \
	}' .mise.toml

# Build frontend and backend
build:
	cd frontend && $(RUN) pnpm build
	cd backend && CGO_ENABLED=0 $(RUN) go build -ldflags="-s -w" -o bin/server ./cmd/server

# Run all tests (Go + Vitest). -count=1 disables the test result cache so a
# local run never reports a stale green.
test:
	cd backend && $(RUN) go test -count=1 ./...
	cd shared && $(RUN) pnpm test
	cd frontend && $(RUN) pnpm test

# Run the Go test suite with the race detector (same as CI's race job)
test-race:
	cd backend && $(RUN) go test -count=1 -race ./...

# Go coverage summary per package (informational; no gate)
coverage:
	cd backend && $(RUN) go test -count=1 -coverprofile=coverage.out ./...
	cd backend && $(RUN) go tool cover -func=coverage.out | tail -20

# Lint all code
lint:
	$(RUN) pnpm lint
	cd backend && $(RUN) golangci-lint run ./...

# Format all code (repo-wide; prefer prettier --write on specific files)
format:
	$(RUN) pnpm format
	cd backend && $(RUN) goimports -w .

# Format check gate: fails (non-zero) when any file is unformatted.
format-check:
	@$(RUN) pnpm format:check
	@cd backend && out="$$($(RUN) gofmt -l . 2>/dev/null; $(RUN) goimports -l . 2>/dev/null)"; \
	  if [ -n "$$out" ]; then \
	    echo "Unformatted Go files:"; echo "$$out" | sort -u; \
	    echo "Run 'make format' to fix."; exit 1; \
	  fi
	@echo "format-check: OK"

# Shell lint gate for the device-mutating scripts
# Gate at warning severity. The remaining info-level findings in these scripts are
# intentional and must not be "fixed" blindly, because they are device-mutating
# scripts: SC2059 (styled printf banners in install.sh), SC2086 ($SSH_OPTS is a
# word-split option list on purpose) and SC2029 (remote shell expansion is the
# point of ssh_cmd). Raising severity means a real error or warning fails the build.
shellcheck:
	@$(RUN) shellcheck --severity=warning --external-sources scripts/*.sh test/integration/*.sh

# Cross-compile production binary for OpenWRT (aarch64)
build-prod:
	@bash scripts/build.sh

# Cross-compile for both aarch64 and x86_64
build-all:
	GOARCH=arm64 bash scripts/build.sh
	cp dist/travo dist/travo-aarch64
	GOARCH=amd64 bash scripts/build.sh
	cp dist/travo dist/travo-x86_64

# Create install tarball for OpenWRT (default: aarch64)
package:
	@bash scripts/package-tarball.sh

# Create install tarballs for both aarch64 and x86_64
package-all: build-all
	ARCH=aarch64_cortex-a53 bash -c 'cp dist/travo-aarch64 dist/travo && bash scripts/package-tarball.sh'
	ARCH=x86_64 bash -c 'cp dist/travo-x86_64 dist/travo && bash scripts/package-tarball.sh'

# Router IP for deploy targets (override: make deploy ROUTER_IP=10.0.0.1)
ROUTER_IP ?= 192.168.1.1
# direct = fast binary+UI; release = full tree like GitHub tarball
DEPLOY_METHOD ?= direct

# Deploy build to router over SSH (developer workflow; not install.sh)
deploy:
	@bash scripts/deploy-local.sh --method $(DEPLOY_METHOD) --ip $(ROUTER_IP)

# Same as deploy; pass extra args: make deploy-local DEPLOY_ARGS='--no-build'
deploy-local:
	@bash scripts/deploy-local.sh --method $(DEPLOY_METHOD) --ip $(ROUTER_IP) $(DEPLOY_ARGS)

# Start Docker dev environment
docker-dev:
	docker compose up

# Device integration suites. These mutate a REAL router, so they are never run
# by CI. Override the target and credentials as needed:
#
#   make integration ROUTER_IP=192.168.1.1
#   make integration SUITE=integration-device.sh
#   make integration LOGIN_PASSWORD=...
#
# Host-key verification is enforced by default; set TRAVO_INSECURE_SSH=1 only for
# a throwaway router whose key is not in your known_hosts.
ROUTER_IP ?= 192.168.1.1
SUITE ?= all
LOGIN_PASSWORD ?= admin
integration:
	@if [ "$$SUITE" = "all" ]; then \
	  set -e; for s in test/integration/*.sh; do \
	    echo "=== $$s ==="; bash "$$s" --ip $(ROUTER_IP) --login-password '$(LOGIN_PASSWORD)' || exit $$?; \
	  done; \
	else \
	  bash test/integration/$$SUITE --ip $(ROUTER_IP) --login-password '$(LOGIN_PASSWORD)'; \
	fi

# Report binary and bundle sizes
size-audit:
	@echo "=== Go binary ==="
	@ls -lh backend/bin/server 2>/dev/null || echo "(not built — run 'make build' first)"
	@echo "=== Frontend bundle (gzipped) ==="
	@find frontend/dist/assets -name "*.js" -exec gzip -c {} \; 2>/dev/null | wc -c | awk '{printf "%.1f KB\n", $$1/1024}' || echo "(not built)"
	@echo "=== Total dist size ==="
	@du -sh frontend/dist 2>/dev/null || echo "(not built)"

# Remove build artifacts
clean:
	rm -rf frontend/dist
	rm -rf backend/bin
	rm -rf backend/static
	rm -rf shared/dist
	rm -rf dist
	rm -rf node_modules frontend/node_modules shared/node_modules
