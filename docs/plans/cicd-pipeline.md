---
title: "Plan: CI/CD Pipeline"
description: "Planning / design notes: Plan: CI/CD Pipeline"
updated: 2026-04-13
tags: [cicd, plan, traceability]
---

# Plan: CI/CD Pipeline

**Status:** Not implemented
**Priority:** Medium
**Related requirements:** [13. Deployment & Packaging](../requirements/tasks_open.md#13-deployment-and-packaging)

---

## Goal

Automated build, test, and package pipeline that produces ready-to-deploy artifacts (IPK packages, firmware images) on every push/PR.

---

## Recommended Platform: GitHub Actions

---

## Phases

### Phase 1 — Basic CI (Test + Lint + Build)

**Trigger:** Every push and PR to `main`

```yaml
# .github/workflows/ci.yml
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: pnpm/action-setup@v4
      - uses: actions/setup-node@v4
        with: { node-version: '20' }
      - uses: actions/setup-go@v5
        with: { go-version: '1.23' }
      - run: pnpm install
      - run: make lint
      - run: make test
      - run: make build
```

**Expected time:** ~2-3 minutes

### Phase 2 — Cross-Compilation for ARM

**The backend must be compiled for the target architecture:**
- AXT1800: `GOOS=linux GOARCH=arm64` (aarch64)
- Other OpenWRT devices may need `GOARCH=arm` (armv7) or `GOARCH=mips`

```yaml
  build-arm:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        arch: [arm64, arm]
    steps:
      - run: |
          cd backend
          GOOS=linux GOARCH=${{ matrix.arch }} CGO_ENABLED=0 \
            go build -ldflags="-s -w" -o bin/server-${{ matrix.arch }} ./cmd/server
      - uses: actions/upload-artifact@v4
        with:
          name: server-${{ matrix.arch }}
          path: backend/bin/server-${{ matrix.arch }}
```

### Phase 3 — IPK Package Building

Use the existing `scripts/package-ipk.sh` to produce installable packages.

```yaml
  package:
    needs: [test, build-arm]
    runs-on: ubuntu-latest
    steps:
      - run: ./scripts/package-ipk.sh
      - uses: actions/upload-artifact@v4
        with:
          name: openwrt-travel-gui.ipk
          path: dist/*.ipk
```

### Phase 4 — Release Automation

**Trigger:** Git tag `v*`

- Build all architectures
- Create IPK packages
- Create GitHub Release with changelog
- Attach artifacts to release

### Phase 5 — Size Budget Check (Optional)

- Check frontend bundle size against budget (e.g., < 400KB gzipped)
- Check Go binary size (< 15MB stripped)
- Fail CI if budget exceeded

---

## Testing Strategy

- Run the full `make test` suite (Go + shared + frontend)
- `make lint` must pass with 0 errors
- `make build` must succeed for all target architectures

---

## Notes

- **Docker not needed** for basic CI — Go cross-compilation and Node.js run natively on GitHub Actions
- **Caching:** Cache `pnpm store`, Go module cache, and Go build cache for faster runs
- **OpenWRT SDK:** For proper IPK packaging with dependency resolution, may need the OpenWRT SDK Docker image in Phase 3
- **Self-hosted runner:** For actual device testing, could use a self-hosted runner with SSH access to the device
