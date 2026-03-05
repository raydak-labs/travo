.PHONY: dev build test lint format clean build-prod package deploy docker-dev

# Run frontend and backend dev servers concurrently
dev:
	@bash scripts/dev.sh

# Build frontend and backend
build:
	cd frontend && pnpm build
	cd backend && CGO_ENABLED=0 go build -o bin/server ./cmd/server

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

# Create .ipk package for OpenWRT
package:
	@bash scripts/package-ipk.sh

# Deploy to OpenWRT device (requires ROUTER_IP)
deploy:
	@bash scripts/deploy.sh $(ROUTER_IP)

# Start Docker dev environment
docker-dev:
	docker compose up

# Remove build artifacts
clean:
	rm -rf frontend/dist
	rm -rf backend/bin
	rm -rf backend/static
	rm -rf shared/dist
	rm -rf dist
	rm -rf node_modules frontend/node_modules shared/node_modules
