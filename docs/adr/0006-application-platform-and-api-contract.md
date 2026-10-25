---
title: "ADR 0006: Application platform, repo shape, and API contract"
description: Platform contract: one static Go binary, tree-shaken frontend bundles, the API surface, and the OpenAPI document generated from the handler types.
status: Accepted
date: 2026-05-14
updated: 2026-10-25
tags: [adr, monorepo, openapi, fiber, luci, travo]
---

# ADR 0006: Application platform, repo shape, and API contract

## Status

Accepted.

## Context

Travo is a **pnpm monorepo**: React SPA, Go API, shared TypeScript contracts. It **coexists with
LuCI** on the same OpenWrt image rather than replacing system configuration wholesale. Integrators,
tests and the frontend need a **stable machine-readable API** description that cannot drift from
what the server does.

## Decision

### 1. Repository layout

- **`frontend/`** — React + TypeScript + Vite + Tailwind SPA, served under `/www/travo`.
- **`backend/`** — Go + Fiber: device orchestration, UCI, ubus, filesystem access, one package per
  domain ([backend packages](../reference/backend-packages.md)).
- **`shared/`** — TypeScript API types for the frontend, generated from the OpenAPI document.
- **`docs/plans/`** — live and normative plans only; a shipped plan is deleted once folded into an
  ADR (ADR 0013).

### 2. LuCI coexistence

- LuCI remains available; packaging moves **uhttpd** listen ports when Travo owns port 80
  ([deployment](../guides/deployment.md)).
- Travo mutates the **same UCI and services** LuCI edits; operators can reconcile unexpected states
  via LuCI.

### 3. Backend as mutator, frontend as driver

- The **Go backend** performs privileged OpenWrt mutations (UCI, service control, file writes under
  `/etc/travo/`).
- The **frontend** calls authenticated REST JSON endpoints and renders state; it does not embed
  alternate sources of truth for device config.

### 4. OpenAPI contract, generated from the handlers

- **`GET /api/openapi.json`** serves an **OpenAPI 3.1** document. It is public, for automation, CI
  and integrators. Protected routes use `POST /api/v1/auth/login` and `Authorization: Bearer <token>`
  (ADR 0007).
- **There is no hand-written spec.** Every REST operation is registered through the helpers in
  `backend/internal/api/engine.go` (`get`, `post`, `put`, `del`) with a typed handler
  `func(context.Context, *Input) (*Output, error)`. The operation's parameters, request body and
  response come from those Go types through Huma's JSON Schema registry
  (`github.com/danielgtaylor/huma/v2`); `json`, `doc`, `example` and `enum` tags shape the schema.
  The same registration mounts the Fiber route, so a route cannot exist without its spec entry.
- **Request binding** is the engine's own: `path`/`query` string fields, a JSON `Body` (required
  unless it is a pointer) and `RawBody`. Handlers keep their own validation; the schema is not
  enforced on input.
- **Whole-configuration PUTs decode strictly** (`strict` / `strictBody` → `decodeStrictJSON`):
  unknown fields, `null` and trailing content are a 400, so a misspelled body cannot overwrite the
  user's settings with zero values. Such operations carry `x-strict-body: true` in the spec.
- **Errors** are `{"error": "...", "code"?: "..."}` (`APIError`) on every status; `code` is stable
  for clients that branch on the cause (`wifi_lockout_risk`).
- **Gates**: `TestEveryRouteIsInTheSpec` (no raw Fiber route under `/api/v1`),
  `TestOperationsAreIdentifiedAndDescribed` (unique `operationId`, a summary) and
  `TestWholeConfigPutsDecodeStrictly` (every multi-field PUT is strict, with a short, reasoned
  exception list). `go run ./cmd/openapi` writes the document without a device.

### 5. HTTP server limits (part of the platform contract)

These are set once in `fiber.New` (`backend/internal/app/app.go`) and are contract, not tuning
knobs:

| Setting | Value | Why |
| ------- | ----- | --- |
| `BodyLimit` | **64 MB** | Fiber's 4 MB default rejected firmware upgrades and restores; sysupgrade images are routinely larger |
| `ReadTimeout` | **5 min** | bounds one request (headers + body); above a slow firmware upload, far below "forever" |
| `WriteTimeout` | **45 min** | the package-install streams run up to 3 packages × `execx.Package` (10 min each) |
| `IdleTimeout` | **60 s** | closes silent keep-alives so graceful shutdown does not wait on them (Slowloris) |
| graceful shutdown | `ShutdownWithTimeout` **20 s** | in-flight handlers finish **before** services and the store are torn down |

### 6. Footprint constraints

- The backend ships as a **single static Go binary** (no CGO); frontend bundles are tree-shaken and
  route-code-split. Constraints in `docs/architecture/overview.md` §8 remain in force.

## Why

- A spec written by hand next to the handlers drifted in both directions and needed a 1,000-line
  test to catch it. Deriving it from the handler types removes the drift instead of detecting it,
  and gives the frontend types it can generate.
- Huma is used for its OpenAPI model and schema registry only. `huma.Register` is generic over each
  input/output pair; measured on this API it added **3.5 MB** to the stripped binary
  (15.35 → 18.87 MB) for 166 operations. The non-generic engine keeps the same handler style at
  **15.12 MB**, below the hand-written baseline (ADR 0013 records the same flash budget that ruled
  out SQLite).
- Validation stays in the handlers because the existing request types were never written as
  schemas (required fields, partial updates); enforcing the schema would have changed behaviour on
  every endpoint at once.

## Consequences and invariants

- Changing an API shape is changing a Go type; the spec and the generated frontend types follow.
  Breaking a shape is still a contract change: update the frontend in the same change.
- A new endpoint is added with `get`/`post`/`put`/`del`, never with `app.Get` on the Fiber app.
- Type names must be unique across packages: the schema namer drops the package name.
- Raising or lowering a value in §5 needs a reason tied to a device constraint.

## Deliberate ceilings

- Path and query parameters are strings; handlers parse numbers themselves.
- Uploads (`/system/restore`, `/system/firmware/upgrade`) and streams (backup download, NDJSON
  package installs) read and write through the Fiber context; their bodies are documented by hand
  (`upload`, `ndjson`).
- The schema documents responses precisely but does not validate requests.
- The WebSocket at `/api/v1/ws` is outside the REST contract.
