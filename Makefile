.PHONY: dev build test lint format clean build-prod build-all package package-all deploy docker-dev install

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

# Build frontend and backend
build:
	cd frontend && $(RUN) pnpm build
	cd backend && CGO_ENABLED=0 $(RUN) go build -ldflags="-s -w" -o bin/server ./cmd/server

# Run all tests (Go + Vitest)
test:
	cd backend && $(RUN) go test ./...
	cd shared && $(RUN) pnpm test
	cd frontend && $(RUN) pnpm test

# Lint all code
lint:
	$(RUN) pnpm lint
	cd backend && $(RUN) golangci-lint run ./...

# Format all code
format:
	$(RUN) pnpm format
	cd backend && $(RUN) goimports -w .

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
