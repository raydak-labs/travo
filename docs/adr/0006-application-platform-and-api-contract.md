---
title: "ADR 0006: Application platform, repo shape, and API contract"
status: Accepted
date: 2026-05-14
updated: 2026-09-28
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
- **`docs/plans/`** — live and normative plans; **not** runtime truth for agents (use `docs/architecture/overview.md` + ADRs + backlog). A shipped plan is deleted once folded into an ADR (ADR 0013).

### 2. LuCI coexistence

- LuCI remains available; packaging moves **uhttpd** listen ports when Travo owns port 80 (`docs/guides/deployment.md`).
- Travo mutations target the **same UCI and services** LuCI edits; operators can reconcile unexpected states via LuCI where appropriate.

### 3. Backend as mutator, frontend as driver

- The **Go backend** performs privileged **OpenWrt mutations** (UCI, service control, file writes under `/etc/travo/`).
- The **frontend** calls authenticated REST JSON endpoints and renders state; it does not embed alternate sources of truth for device config.

### 4. OpenAPI contract

- **`GET /api/openapi.json`** exposes **OpenAPI 3.0** generated from handler metadata (`openapi_handler.go` and related). It must stay available for automation, CI, and integrators (contract also stated in `AGENTS.md`).
- Protected routes use **`POST /api/v1/auth/login`** and **`Authorization: Bearer <token>`** (see ADR 0007).
- **The spec is a tested contract, not documentation.** `backend/internal/api/openapi_drift_test.go` introspects every route registered by `SetupRoutes` on the same app object and compares the set against the operations in the **served** `/api/openapi.json`, failing on **both** directions:
  - a registered operation with no spec entry (undocumented endpoint), and
  - a spec entry with no registered route (a promise the server does not keep).
  Fiber's `:param` is normalised to the OpenAPI `{param}` template, and Fiber's auto-registered `HEAD` companions plus the `/api/v1/ws` upgrade route are excluded. A second test requires every spec operation to carry a `summary` and a `200` response, so an operation cannot be "present but undescribed".
  **Adding or removing a route without the matching spec change fails CI.** That is the point: drift can no longer be silent.

### 5. HTTP server limits (part of the platform contract)

These are set once in `fiber.New` (`backend/cmd/server/main.go`) and are contract, not tuning knobs:

| Setting | Value | Why |
| ------- | ----- | --- |
| `BodyLimit` | **64 MB** | Fiber's 4 MB default rejected `POST /api/v1/system/firmware/upgrade` and `POST /api/v1/system/restore`; OpenWrt sysupgrade images are routinely larger |
| `ReadTimeout` | **5 min** | bounds a single request (headers + body); must stay well above a slow firmware/backup upload, and far below "forever" |
| `WriteTimeout` | **45 min** | the package-install SSE endpoints legitimately stream for 3 packages × `execx.Package` (10 min each), so this sits above that ceiling |
| `IdleTimeout` | **60 s** | closes silent keep-alives, so graceful shutdown does not wait on connections that never send another byte (Slowloris) |
| graceful shutdown | `ShutdownWithTimeout` **20 s** | in-flight handlers must finish **before** services and the store are torn down |

Together with the deny-all CORS default, these mean a hung client cannot pin a
connection slot and a large legitimate upload is not rejected.

### 6. Footprint constraints

- Backend ships as a **single static Go binary** (no CGO); frontend bundles are **tree-shaken** and route-code-split. Constraints in `docs/architecture/overview.md` §8 remain in force.

## Consequences

- Breaking API or OpenAPI shape is a **contract change**: update shared types, frontend callers, and any external consumers in the same change train.
- New subsystems should add OpenAPI entries when endpoints are stabilized — and the drift test means they must do so in the same change, whether stabilized or not.
- Raising or lowering a value in §5 needs a reason: each is tied to a concrete device constraint (sysupgrade image size, install-stream duration, keep-alive drain).

## References

- `docs/architecture/overview.md` §1, §8
- `backend/internal/api/openapi_handler.go`
- `backend/internal/api/openapi_drift_test.go` — the route↔spec drift gate
- `backend/cmd/server/main.go` — Fiber config, timeouts, body limit, shutdown order
- `docs/guides/deployment.md`
- [ADR 0007](./0007-authentication-and-access-control.md) — auth contract
- `AGENTS.md` — API documentation endpoint section
