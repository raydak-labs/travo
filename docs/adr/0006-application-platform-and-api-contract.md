---
title: "ADR 0006: Application platform, repo shape, and API contract"
status: Accepted
date: 2026-05-14
tags: [adr, monorepo, openapi, fiber, luci, travo]
---

# ADR 0006: Application platform, repo shape, and API contract

## Status

Accepted.

## Context

Travo is a **pnpm monorepo**: React SPA, Go API, shared TypeScript contracts. It **coexists with LuCI** on the same OpenWrt image rather than replacing system configuration wholesale. Integrators and tests need a **stable machine-readable API** description.

## Decision

### 1. Repository layout

- **`frontend/`** — React + TypeScript + Vite + Tailwind SPA, served under `/www/travo` on device.
- **`backend/`** — Go + Fiber: device orchestration, UCI, ubus, filesystem access.
- **`shared/`** — Shared **types and API route constants** consumed by the frontend (and optionally tooling).
- **`docs/plans/`** — historical design notes; **not** runtime truth for agents (use `docs/architecture.md` + ADRs + backlog).

### 2. LuCI coexistence

- LuCI remains available; packaging moves **uhttpd** listen ports when Travo owns port 80 (`docs/deployment.md`).
- Travo mutations target the **same UCI and services** LuCI edits; operators can reconcile unexpected states via LuCI where appropriate.

### 3. Backend as mutator, frontend as driver

- The **Go backend** performs privileged **OpenWrt mutations** (UCI, service control, file writes under `/etc/travo/`).
- The **frontend** calls authenticated REST JSON endpoints and renders state; it does not embed alternate sources of truth for device config.

### 4. OpenAPI contract

- **`GET /api/openapi.json`** exposes **OpenAPI 3.0** generated from handler metadata (`openapi_handler.go` and related). It must stay available for automation, CI, and integrators (contract also stated in `AGENTS.md`).
- Protected routes use **`POST /api/v1/auth/login`** and **`Authorization: Bearer <token>`** (see ADR 0007).

### 5. Footprint constraints

- Backend ships as a **single static Go binary** (no CGO); frontend bundles are **tree-shaken** and route-code-split. Constraints in `docs/architecture.md` §8 remain in force.

## Consequences

- Breaking API or OpenAPI shape is a **contract change**: update shared types, frontend callers, and any external consumers in the same change train.
- New subsystems should add OpenAPI entries when endpoints are stabilized.

## References

- `docs/architecture.md` §1, §8
- `backend/internal/api/openapi_handler.go`
- `docs/deployment.md`
- `AGENTS.md` — API documentation endpoint section
