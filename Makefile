.PHONY: dev build test lint format clean build-prod build-all package package-all deploy docker-dev

# Run frontend and backend dev servers concurrently
dev:
	@bash scripts/dev.sh

# Build frontend and backend
build:
	cd frontend && pnpm build
	cd backend && CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/server ./cmd/server

# Run all tests (Go + Vitest)
test:
	cd backend && go test ./...
	cd shared && pnpm test
	cd frontend && pnpm test

# Lint all code
lint:
	pnpm lint
	cd backend && go vet ./...

# Format all code
format:
	pnpm format
	cd backend && gofmt -w .

# Cross-compile production binary for OpenWRT (aarch64)
build-prod:
	@bash scripts/build.sh

# Cross-compile for both aarch64 and x86_64
build-all:
	GOARCH=arm64 bash scripts/build.sh
	cp dist/openwrt-travel-gui dist/openwrt-travel-gui-aarch64
	GOARCH=amd64 bash scripts/build.sh
	cp dist/openwrt-travel-gui dist/openwrt-travel-gui-x86_64

# Create .ipk package for OpenWRT (default: aarch64)
package:
	@bash scripts/package-ipk.sh

# Create .ipk packages for both aarch64 and x86_64
package-all: build-all
	ARCH=aarch64_cortex-a53 bash -c 'cp dist/openwrt-travel-gui-aarch64 dist/openwrt-travel-gui && bash scripts/package-ipk.sh'
	ARCH=x86_64 bash -c 'cp dist/openwrt-travel-gui-x86_64 dist/openwrt-travel-gui && bash scripts/package-ipk.sh'

# Deploy to OpenWRT device (requires ROUTER_IP)
deploy:
	@bash scripts/deploy.sh $(ROUTER_IP)

# Deploy locally via direct copy (fast iteration)
deploy-local:
	@bash scripts/deploy-local.sh $(DEPLOY_ARGS)

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
