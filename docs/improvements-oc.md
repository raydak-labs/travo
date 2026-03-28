# Codebase improvement findings (OpenCode analysis)

This document records a **read-only** review of the **openwrt-travel-gui** monorepo (`frontend/`, `shared/`, `backend/`, tooling, CI). It is a prioritized backlog across correctness, security, architecture, testing, UX/a11y, build/deploy, and operations—not an implementation plan.

**Sources:** direct reads/greps of auth, routing, WebSocket, and API client code; an internal **explore** agent architecture pass; attempted parallel **security**/**a11y**/**librarian** background tasks (some outputs timed out or were partial). **No repository code was modified** to produce this document.

---

## 1. Correctness bugs and high-risk inconsistencies

### 1.1 Wrong storage key for backup/restore auth (fixed)

`frontend/src/lib/api-client.ts` stores the JWT under **`openwrt-auth-token`** (`TOKEN_KEY`).

Previously, **`useBackup`** and **`useRestore`** read **`'token'`** from storage while **`useFirmwareUpgrade`** inlined **`openwrt-auth-token`**. **This was corrected:** backup, restore, and firmware upgrade now use **`getToken()`** from `api-client` so all three paths share the same key and remember-me behavior.

**Remaining recommendation:** Prefer an `authorizedFetch()` or `apiClient` helpers for blob/FormData so 401 handling matches JSON requests.

### 1.2 Raw `fetch` vs `apiClient`

Most data hooks use `apiClient`, but raw `fetch` appears in:

- `frontend/src/router.tsx` — `requireSetupComplete` calls `API_ROUTES.system.setupComplete`.
- `frontend/src/hooks/use-system.ts` — backup, restore, firmware (binary/multipart flows).
- Additional modules called out in a broader scan (re-verify after refactors): `frontend/src/pages/system/system-page.tsx`, `frontend/src/pages/logs/logs-page.tsx`, `frontend/src/pages/network/data-usage-section.tsx`, `frontend/src/pages/wifi/wifi-scan-dialog.tsx`, `frontend/src/components/wifi/repeater-wizard.tsx`.

**Risks:** Duplicated Bearer header logic; inconsistent 401 handling (`handleUnauthorized` is implemented in `api-client.ts`’s JSON client, not necessarily in raw `fetch` paths); inconsistent parsing of `{ error: string }` bodies.

**Edge case:** `requireSetupComplete` sets `Authorization: Bearer ${getToken()}`. If `getToken()` is null, the header can become the literal `Bearer null`—prefer treating “no token” as unauthenticated consistently with `requireAuth()`.

**Recommendation:** Add `apiClient`-style helpers for `blob`, `FormData`, and streaming responses, or a single internal `authorizedFetch()` wrapper.

### 1.3 WebSocket path duplication

`frontend/src/hooks/use-websocket.ts` and `frontend/src/hooks/use-alerts.ts` both build `…/api/v1/ws?token=…` manually. That path is **not** part of `shared/src/api/routes.ts`, unlike REST `API_ROUTES`.

**Recommendation:** Export a single constant from `shared` (for example under `API_ROUTES` or a small `realtime` object) and reuse it in both hooks.

### 1.4 WebSocket reconnect clears chart buffers

`useWebSocket` clears `dataPoints` and `interfaceDataPoints` on **every** `onclose`, then reconnects. Brief network blips erase history even though the hook’s purpose is buffering recent samples.

**Recommendation:** Consider retaining the last N points across reconnects, or only reset on auth loss.

---

## 2. Security

### 2.1 JWT in WebSocket query string

The upgrade flow validates `token` query param server-side (`backend/internal/ws/handler.go`), and the global auth middleware exempts `/api/v1/ws` because browsers cannot attach arbitrary headers during the WebSocket handshake the same way as `fetch`.

**Risk:** JWTs in URLs are more likely to appear in logs (proxies, diagnostic dumps) than `Authorization` headers.

**Recommendation:** Document the LAN-focused threat model; longer term, consider short-lived WS tickets, cookie sessions, or `Sec-WebSocket-Protocol`—each needs coordinated backend changes.

### 2.2 OpenAPI route vs middleware comments

`backend/internal/api/openapi_handler.go` states the spec is served at `GET /api/openapi.json` for automation. `backend/internal/api/router.go` repeats that intent.

`backend/internal/auth/auth.go` middleware skips JWT for `/api/health`, `/api/v1/auth/login`, `/api/v1/ws`, and `/api/v1/system/time-sync` only. **`/api/openapi.json` is not exempt**, so it likely **requires** a Bearer token in practice.

**Recommendation:** Decide whether OpenAPI should be public on the LAN; then either add an explicit exemption or update comments/tooling to always pass a JWT.

### 2.3 JWT in web storage

Tokens live in `localStorage` / `sessionStorage` (`api-client.ts`, theme and dismiss flags elsewhere). This is standard for SPAs but XSS-prone.

**Recommendation:** If moving to HTTPS on the router, evaluate HttpOnly cookies and CSRF protections; add CSP when you have a concrete policy.

### 2.4 `streamRequest` error surface

`streamRequest` in `frontend/src/lib/api-client.ts` does not extract `{ error }` from JSON bodies the way `request()` does, and error messages are more generic.

### 2.5 Backend: ignored bcrypt error on startup

`auth.NewAuthService` uses `hash, _ := bcrypt.GenerateFromPassword(...)`, ignoring errors. Startup should fail loudly if hashing cannot be computed.

### 2.6 Password policy

`ChangePassword` enforces **minimum length 6** (`auth.go`). For an admin router UI, consider stronger policy (length, complexity, breach checks are usually overkill on-device—at least document the choice).

---

## 3. Architecture and maintainability

### 3.1 Monorepo graph

`pnpm-workspace.yaml` includes `frontend` and `shared`. The frontend resolves `@shared` through Vite and TS path aliases to `../shared/src`; **`frontend/package.json` does not list `shared` as a workspace dependency**.

**Recommendation:** Add `"@openwrt-travel-gui/shared": "workspace:*"` (or equivalent) so package managers and future publishing see the edge explicitly.

### 3.2 `shared` package entrypoints

`shared/package.json` points `main`/`types` at `./src/index.ts` while `build` emits `dist/`. Consumers import source via aliases—clarify whether `dist` is vestigial or should become the official artifact.

### 3.3 Large MSW layer

`frontend/src/mocks/handlers.ts` is a single large file of handlers. It will drift relative to `API_ROUTES` and backend behavior.

**Recommendation:** Split handlers by domain (`system`, `network`, `wifi`, …) or generate a minimal mock subset for dev.

### 3.4 `requireSetupComplete` resilience vs strictness

On non-redirect failures, `router.tsx` intentionally allows access so a flaky network does not brick the UI.

**Recommendation:** Surface a non-blocking banner (“Could not verify setup status”) and retry in the background if you need stronger guarantees.

### 3.5 TanStack Query defaults

`frontend/src/App.tsx` sets global `staleTime: 30_000` and `retry: 1`. Mutations often should default to **`retry: false`** to avoid duplicate side effects.

**Recommendation:** Use `QueryErrorResetBoundary` (and route-level error UI) for query failures outside the lazy `Suspense` shell.

### 3.6 NDJSON / install streams

`streamRequest` supports NDJSON POST streams (e.g. service install logs). Keep parity between streaming and REST for 401 handling and user-visible error text.

---

## 4. Backend (Go) and operations

- **Wide privileged API** (firmware, factory reset, SSH keys, firewall, WoL). Global JWT middleware in `cmd/server/main.go` helps; consider additional rate limits on destructive routes.
- **CORS** configured via `cfg.CorsOrigins`—validate defaults for shipped images.
- **Deferred WiFi maintenance goroutines** in `main.go` (sleep then UCI repair / script rewrite) can surprise operators—ensure logging is sufficient and failures are visible.
- **Tests:** Good spread of `_test.go` files across services and handlers—continue property/table tests for validation.

---

## 5. Shared TypeScript contract

- **`API_ROUTES` comment** documents sync with `backend/internal/api/router.go`—optional CI grep/check could catch drift early.
- **Types** colocated with routes are a strength for the React app and MSW.

---

## 6. Frontend UX, a11y, i18n

- **Loading states:** Lazy routes use a minimal `Suspense` fallback (`router.tsx`). Prefer skeleton components aligned with `components/ui/skeleton.tsx`.
- **a11y:** Some forms use `label`/`htmlFor` (login, parts of WiFi/system). Audit **icon-only buttons**, **dialog focus**, and **live regions** for dynamic alerts/toasts.
- **i18n:** No framework detected—fine for a single-locale hackathon/device UI; revisit for broader locales.

---

## 7. Build, Vite, TypeScript, and production hardening

- **Vite proxy:** `frontend/vite.config.ts` proxies `/api` and `/ws` during dev—ensure production relies on same-origin `/api` (as README implies) or document reverse proxy rules.
- **Source maps:** Decide explicitly for production builds on embedded devices (privacy vs debuggability).
- **Environment variables:** `import.meta.env` is primarily used for `DEV` (MSW). If introducing `VITE_*`, validate at build time.
- **MSW:** `main.tsx` only enables mocks in dev—keep it that way; README already documents regenerating the worker.
- **CSP:** Plan before enforcing strict policies on HTTPS—Vite/React may need nonces/hashes.
- **Tooling noise:** `frontend/vite.config.ts` uses Node builtins (`path`, `__dirname`). If the IDE reports missing Node types, align `tsconfig` `types`/`moduleResolution` for config files or split config to `.mts` with `@types/node`.

**References:** [Vite config](https://vitejs.dev/config/), [TanStack Query important defaults](https://tanstack.com/query/latest/docs/framework/react/guides/important-defaults), [MSW docs](https://mswjs.io/docs/), [React security](https://react.dev/learn/security).

---

## 8. Testing and CI

- **Hook coverage:** Many hooks under `frontend/src/hooks/` lack colocated tests (exceptions include `use-session-timeout`).
- **`vitest run --passWithNoTests`:** Allows silent empty suites in `frontend/package.json`—tighten when the suite stabilizes.
- **Coverage gates:** No enforced coverage thresholds were observed in Vitest configs/CI—consider adding incremental targets.
- **Shared tests:** `shared/src/__tests__` focuses on routes/types—complements but does not replace integration tests against the Go API.

---

## 9. Developer experience

- **ESLint scope:** Root `eslint.config.js` ignores `backend/`; `make lint` pairs ESLint with `go vet`—document for new contributors (`docs/development.md`).
- **Console usage:** A grep shows `console.warn`/`console.error` in MSW bootstrap and `error-boundary`—acceptable; avoid adding noisy logs in production paths.
- **Repository naming:** README branding (`openwrt-travel-gui`) vs checkout folder names—cosmetic but worth aligning in docs for forks.

---

## 10. Prioritization (opinionated)

| Priority | Item                                                        | Rationale                                      |
| -------- | ----------------------------------------------------------- | ---------------------------------------------- |
| ~~P0~~   | ~~Fix backup/restore token key mismatch~~ (done)            | Was breaking JWT on backup/restore             |
| P1       | Unify authenticated HTTP + 401 handling                     | Prevents recurrence; reduces security footguns |
| P1       | Reconcile OpenAPI auth behavior with docs                   | Automation and security expectations           |
| P2       | Shared WebSocket URL constant; JWT-in-query hardening plan  | Contract drift + logging risk                  |
| P2       | Handle bcrypt errors at auth service construction           | Fail-safe startup                              |
| P3       | MSW modularization, Query/mutation defaults, coverage in CI | Velocity and regression safety                 |

---

## 11. Methodology

Manual verification focused on: `frontend/src/lib/api-client.ts`, `frontend/src/hooks/use-system.ts`, `frontend/src/hooks/use-websocket.ts`, `frontend/src/hooks/use-alerts.ts`, `frontend/src/router.tsx`, `frontend/src/App.tsx`, `frontend/src/main.tsx`, `frontend/vite.config.ts`, `shared/src/api/routes.ts`, `pnpm-workspace.yaml`, `backend/internal/auth/auth.go`, `backend/internal/api/router.go`, `backend/internal/ws/handler.go`, `backend/cmd/server/main.go`, and `backend/internal/api/openapi_handler.go`.

Parallel **explore** task output contributed monorepo structure notes (aliases, test layout, duplication patterns). **Security/a11y/librarian** tasks were launched; not all completed transcripts were merged before this file was finalized—spot-check with ripgrep when implementing.
