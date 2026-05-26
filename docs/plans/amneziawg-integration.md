---
title: "Plan: AmneziaWG Integration"
description: "Architecture decision and phased implementation plan for optional AmneziaWG support alongside WireGuard."
updated: 2026-05-26
tags: [plan, vpn, amneziawg, wireguard, openwrt]
---

# Plan: AmneziaWG Integration

**Status:** Approved for implementation  
**Priority:** High  
**Estimated effort:** 9-14 focused days  
**Target device:** OpenWrt 25.12.3 / kernel 6.12.85 / aarch64 qualcommax ipq60xx  
**Related ADR:** [`../adr/0008-wireguard-family-protocol-coexistence.md`](../adr/0008-wireguard-family-protocol-coexistence.md)

## 1. Overview and motivation

Travo already has a working WireGuard client stack: profile import, activation, firewall plumbing, kill switch, split tunnel, DNS leak checks, and speed test. AmneziaWG is the highest-value next VPN addition because it keeps the same operational model as WireGuard while adding DPI-evasion parameters that matter on travel-router deployments.

Why this is worth doing now:

- **Travel-router fit:** bypassing hostile DPI is a core travel-router use case.
- **Low conceptual overhead:** AWG configs are WireGuard-compatible plus a small set of extra interface keys.
- **Low runtime overhead:** still kernel-space tunnel encryption; obfuscation is additive, not a proxy chain.
- **Good coexistence story:** users can keep standard WireGuard profiles and add AWG profiles without changing the rest of the product mental model.
- **Hardware precedent:** GL.iNet ships AmneziaWG on similar OpenWrt + Qualcomm hardware.

### Scope

This plan covers **client-side AmneziaWG support** in the existing VPN stack:

- importing AWG 1.x / 2.0 `.conf` files
- persisting AWG-aware profiles beside standard WireGuard profiles
- applying `proto amneziawg` UCI configuration on device
- runtime status / verification / DNS / kill-switch behavior for AWG tunnels
- UI affordances for install prompts, badges, advanced AWG parameters, and profile editing
- deployment/documentation work needed because AWG packages are not in official OpenWrt feeds

### Explicit non-goals for v1

- simultaneous active WireGuard + AmneziaWG tunnels
- server-side AWG provisioning or parameter tuning wizard
- generic proxy-chain support (Shadowsocks/V2Ray/Xray/etc.)
- replacing the existing WireGuard UX with a completely separate AmneziaWG page

## 2. Final architecture decisions

### 2.1 Product model

- **Standard WireGuard remains the default VPN path.**
- **AmneziaWG is additive, not a replacement.**
- Travo keeps the existing **one active VPN tunnel at a time** constraint across WireGuard, AmneziaWG, and Tailscale exit-node style routing.

### 2.2 API surface

- The existing **`/api/v1/vpn/wireguard*`** endpoints remain the primary API for the WireGuard-family tunnel slot.
- Those payloads will be **extended** with protocol metadata (`is_amnezia` and AWG params) instead of creating a second full CRUD surface.
- Add **`GET /api/v1/vpn/amneziawg/available`** for capability probing.
- **Installer logic should stay centralized in `ServiceManager`.** The frontend can call the existing services install route for `amneziawg`; if product wants a VPN-local CTA, add only a thin alias that delegates to the same service install path.

### 2.3 Runtime ownership

- Travo continues to own a single app-managed VPN interface slot and shared firewall objects (`wg0`, `wg0_zone`, `wg0_fwd`) **if device validation confirms `proto amneziawg` can back the same logical interface name**.
- Do **not** scatter hard-coded `wg` assumptions further. Add protocol helpers so proto name, userspace tool, dump parser, and UCI option mapping are chosen from one place.
- If the OpenWrt helper forces a different runtime ifname on real hardware, contain that behind a helper such as `vpnTunnelInterfaceName()` instead of branching the UI/API contract.

### 2.4 Detection and persistence

- A profile is treated as **AmneziaWG** when the imported config contains any AWG-only interface keys: `Jc`, `Jmin`, `Jmax`, `S1`, `S2`, `H1`, `H2`, `H3`, `H4`.
- Raw `.conf` content remains stored for round-tripping and future export.
- Saved profile metadata should also persist **derived protocol information** so the UI can badge profiles without reparsing every render.

### 2.5 Shared semantics

The following behaviors remain **shared** across WireGuard and AmneziaWG and should not fork unless device testing proves a hard requirement:

- kill switch semantics
- split tunnel semantics
- firewall zone / forwarding layout
- DNS forwarding / DNS leak checks
- profile activation workflow
- single-active-tunnel interaction with Tailscale exit-node routing

### 2.6 Packaging / release rule

- AWG support ships only when the release process can provide a **kernel-matched package set** for the target firmware.
- If packages are missing or kernel-incompatible, the feature must degrade gracefully: **WireGuard continues to work, AWG profiles remain visible, and the UI shows install / unavailable messaging instead of failing late during enable.**

## 3. Phase breakdown

| Phase | Focus | Estimate | Depends on |
| --- | --- | --- | --- |
| 1 | Parser + models | 2-3 days | none |
| 2 | Service + UCI + runtime | 3-4 days | Phase 1 |
| 3 | API + shared contract | 1-2 days | Phase 2 helpers defined |
| 4 | Frontend UX | 2-3 days | Phases 1-3 |
| 5 | Docs + device validation | 1-2 days | all earlier phases |

### Phase 1 — Backend parser and models (2-3 days)

**Goal:** teach the existing WireGuard-family parser and data models to understand AWG configs without breaking standard WireGuard imports.

#### Files to modify

- `backend/internal/services/wireguard_parser.go`
  - Extend `WireguardParsedConfig` / `WireguardInterface` with AWG fields.
  - Parse `Jc`, `Jmin`, `Jmax`, `S1`, `S2`, `H1`, `H2`, `H3`, `H4` from `[Interface]`.
  - Add a helper such as `IsAmnezia()` / `HasAmneziaParams()` so later phases do not re-implement detection.
  - Keep ordinary WireGuard configs valid and unchanged.
- `backend/internal/services/wireguard_parser_test.go`
  - Add AWG 1.x fixture coverage (`Jc/Jmin/Jmax/S1/S2`).
  - Add AWG 2.0 fixture coverage (`H1-H4`).
  - Add mixed WireGuard/AWG whitespace and case-insensitive parsing checks.
  - Add invalid-integer tests for AWG-only keys.
- `backend/internal/models/vpn.go`
  - Extend `WireguardConfig` with `IsAmnezia bool` and AWG parameters.
  - Extend `WireGuardProfile` with protocol metadata used by the UI (`is_amnezia`, optional AWG summary fields).
  - Keep JSON tags backward-compatible so old profile files still decode.
- `backend/internal/models/models_test.go`
  - Add JSON round-trip assertions for new VPN fields.
- `shared/src/api/vpn.ts`
  - Mirror new shared contract fields for frontend use.
- `shared/src/__tests__/vpn.test.ts`
  - Cover new `WireguardConfig` / profile metadata shapes.

#### Implementation notes

- Do **not** rename the existing WireGuard parser files yet; broaden them to “WireGuard-family” behavior while keeping file churn low.
- Treat absent AWG keys as the normal WireGuard path.
- Store AWG values as typed numeric fields, not a loose string map, so validation and UI editing remain explicit.

#### Exit criteria

- Standard WireGuard imports still pass existing parser tests.
- AWG 1.x and 2.0 sample configs parse cleanly.
- Shared TypeScript types expose AWG metadata without breaking existing callers.

### Phase 2 — Backend UCI, runtime, and package service layer (3-4 days)

**Goal:** make the backend capable of configuring and running either WireGuard or AmneziaWG using the existing tunnel lifecycle.

#### Files to modify

- `backend/internal/services/vpn_service.go`
  - Add a protocol selection layer so enable/import/status paths know whether the active profile is standard WG or AWG.
  - Add AWG availability probes (`/usr/bin/awg`, netifd proto helper, kernel module presence / loadability).
  - When AWG is active, write `network.wg0.proto=amneziawg` and map interface params to `awg_jc`, `awg_jmin`, `awg_jmax`, `awg_s1`, `awg_s2`, `awg_h1`, `awg_h2`, `awg_h3`, `awg_h4`.
  - Keep shared DNS / kill-switch / split-tunnel / firewall logic common.
  - Switch runtime status commands from `wg show` to `awg show` when needed.
  - Keep the existing Tailscale interaction rule: enabling a full-tunnel WireGuard-family path clears Tailscale exit-node routing.
  - Extend profile activation/import to carry protocol metadata end-to-end.
- `backend/internal/services/vpn_service_test.go`
  - Add proto-switching tests for WireGuard vs AWG.
  - Assert AWG UCI options are written correctly.
  - Assert AWG availability failures produce a clean user-facing error before enable.
  - Cover WG↔AWG profile activation transitions and no-regression behavior for DNS/kill switch helpers.
- `backend/internal/services/service_manager.go`
  - Add a new service definition for `amneziawg`.
  - Pin package list / detection package names once packaging is finalized.
  - Ensure “installed but no init script” semantics match WireGuard.
- `backend/internal/services/service_manager_test.go`
  - Add install/detect coverage for the new service ID.
- `backend/internal/api/validation.go`
  - Add validation helpers for AWG integer fields and any new payload constraints.

#### Optional extraction if `vpn_service.go` becomes too branchy

Create:

- `backend/internal/services/vpn_protocol.go`
  - Centralize protocol metadata (`proto name`, `show command`, `UCI key mapping`, `availability check`).
- `backend/internal/services/vpn_protocol_test.go`
  - Cover helper selection logic separately from the larger service tests.

#### Implementation notes

- Prefer keeping the logical Travo tunnel slot name at `wg0` so existing firewall, verify, speed-test, and split-tunnel logic can stay mostly intact.
- Verify that the AWG dump format is close enough to parse with existing `ParseWgDump`; if not, add a small protocol-aware adapter rather than duplicating the entire status pipeline.
- Do not fork kill-switch implementation by protocol; the firewall decision is about the app-owned VPN interface, not the userspace tool.

#### Exit criteria

- Importing an AWG config writes correct UCI proto and AWG options.
- Enabling AWG uses the correct runtime command path.
- Standard WireGuard enable/disable tests still pass.
- Service list can report AWG install state independently of WireGuard.

### Phase 3 — API layer and shared contract updates (1-2 days)

**Goal:** surface AWG-aware capabilities through stable backend and shared API contracts.

#### Files to modify

- `backend/internal/api/vpn_handlers.go`
  - Extend config/profile handlers to accept and return AWG metadata.
  - Add `GetAmneziaWGAvailabilityHandler` for runtime capability checks.
  - If product wants a VPN-local install CTA, add a thin `POST /api/v1/vpn/amneziawg/install` alias that delegates to `ServiceManager`; otherwise keep install on `/api/v1/services/:id/install` and document that as canonical.
- `backend/internal/api/router.go`
  - Register the new availability route (and install alias, if implemented).
- `backend/internal/api/openapi_handler.go`
  - Add AWG fields to the VPN schemas / examples.
  - Document the availability endpoint and any install alias.
- `backend/internal/api/vpn_handlers_test.go`
  - Add request/response tests for AWG-aware payloads.
  - Add availability endpoint tests for installed / missing package scenarios.
- `shared/src/api/routes.ts`
  - Add route constants for `vpn.amneziawg.available` and any install alias.
- `shared/src/__tests__/routes.test.ts`
  - Assert new route constants.
- `shared/src/api/vpn.ts`
  - Add an availability response type (for example: `installed`, `binary_present`, `helper_present`, `kernel_module_present`, `ready`, `reason`).

#### API shape recommendation

Use one shared WireGuard-family config shape, for example:

```json
{
  "private_key": "...",
  "address": "10.8.0.2/32",
  "dns": ["1.1.1.1"],
  "is_amnezia": true,
  "amnezia": {
    "jc": 5,
    "jmin": 40,
    "jmax": 70,
    "s1": 15,
    "s2": 25,
    "h1": 11111111,
    "h2": 22222222,
    "h3": 33333333,
    "h4": 44444444
  },
  "peers": [ ... ]
}
```

#### Exit criteria

- Backend route table exposes AWG availability.
- OpenAPI stays accurate.
- Shared route/type tests pass with the new fields.

### Phase 4 — Frontend UI and UX (2-3 days)

**Goal:** make AWG support feel like a natural extension of the current WireGuard page rather than a second product.

#### Files to modify

- `frontend/src/hooks/use-vpn.ts`
  - Add AWG availability query and optional install mutation/alias.
  - Extend existing WireGuard-family hooks to consume new fields.
- `frontend/src/lib/schemas/vpn-forms.ts`
  - Add AWG advanced-settings schema validation.
- `frontend/src/lib/schemas/__tests__/vpn-forms.test.ts`
  - Cover AWG numeric validation and import form behavior.
- `frontend/src/pages/vpn/wireguard-section.tsx`
  - Query AWG availability alongside WireGuard config/status.
  - Decide when to show “Install AmneziaWG” vs normal WireGuard UI.
- `frontend/src/pages/vpn/wireguard-card-body-types.ts`
  - Thread AWG availability and metadata through the existing component tree.
- `frontend/src/pages/vpn/wireguard-card-body.tsx`
  - Pass AWG-aware props into detail/import/profile components.
- `frontend/src/pages/vpn/wireguard-status-and-config-peers.tsx`
  - Show protocol badge / obfuscation indicator near runtime status.
- `frontend/src/pages/vpn/wireguard-profiles-kill-import.tsx`
  - Show AWG badge on imported profiles.
  - Keep one profile list for both WG and AWG entries.
- `frontend/src/pages/vpn/wireguard-import-profile-file.ts`
  - Detect AWG configs client-side for immediate form hints after file upload.
- `frontend/src/pages/vpn/wireguard-install-prompt.tsx`
  - Generalize or replace with copy that distinguishes base WireGuard from AWG package readiness.
- `frontend/src/pages/vpn/vpn-page.tsx`
  - Keep one WireGuard-family section; do not create a separate AmneziaWG page in v1.
- `frontend/src/pages/services/services-page.tsx`
  - Ensure `amneziawg` is installable from Services and can launch a useful post-install flow.
- `frontend/src/pages/services/service-card.constants.ts`
  - Add an icon / label entry for `amneziawg`.
- `frontend/src/mocks/data.ts`
  - Add AWG mock service + profile data.
- `frontend/src/mocks/handlers.ts`
  - Add handlers for AWG availability (and install alias if used).
- `frontend/src/pages/vpn/__tests__/vpn-page.test.tsx`
  - Cover AWG badge, install prompt, and mixed profile rendering.
- `frontend/src/pages/services/__tests__/services-page.test.tsx`
  - Cover AWG install card / CTA behavior.

#### New frontend components worth adding

Create when it improves clarity instead of bloating existing files:

- `frontend/src/pages/vpn/amneziawg-advanced-settings.tsx`
  - Dedicated editor/read-only display for `Jc/Jmin/Jmax/S1/S2/H1-H4`.
- `frontend/src/pages/vpn/amneziawg-availability-banner.tsx`
  - Reusable install / unavailable / kernel-mismatch messaging.

#### UX rules

- AWG should be **auto-detected**, not forced through a manual protocol dropdown during import.
- Keep ordinary WireGuard imports as the shortest path.
- AWG-specific fields belong in **advanced settings / protocol details**, not the basic happy path.
- Copy should explain the benefit in plain language: **better DPI resistance**, not “more secure than WireGuard”.

#### Exit criteria

- Users can tell whether a profile is WireGuard or AWG at a glance.
- Missing AWG packages produce a clear install prompt, not a failed toggle.
- Existing WireGuard UI flows remain usable without AWG installed.

### Phase 5 — Documentation and device validation (1-2 days)

**Goal:** lock in the decision, document operational constraints, and prove the feature on the target device.

#### Files to modify

- `docs/adr/0008-wireguard-family-protocol-coexistence.md`
  - Keep the final coexistence / packaging decision synchronized with implementation.
- `docs/adr/README.md`
  - Index the ADR.
- `docs/architecture.md`
  - Keep the stable WireGuard-family decision visible from the main architecture overview.
- `docs/deployment.md`
  - Document AWG package source, install assumptions, and release caveats.
- `docs/testing.md`
  - Add an AmneziaWG verification checklist.
- `docs/requirements/tasks_done.md`
  - Mark the work done once shipped.

#### Real-device validation checklist

On `192.168.1.1` validate all of the following:

1. **Availability detection**
   - AWG packages absent → API reports unavailable with a useful reason.
   - AWG packages installed → API reports ready.
2. **UCI application**
   - `uci show network.wg0` reflects `proto='amneziawg'` for AWG profiles.
   - `uci show network | grep awg_` shows the expected AWG parameters.
3. **Runtime status**
   - `ip link show wg0` (or protocol-resolved helper interface) is up.
   - `awg show wg0` returns peer / handshake data.
4. **Behavioral checks**
   - import AWG 1.x config
   - import AWG 2.0 config
   - switch between plain WG and AWG profiles
   - enable/disable kill switch
   - run DNS leak test
   - run VPN speed test
   - verify Tailscale exit-node path is cleared before enabling the WireGuard-family tunnel
5. **Regression checks**
   - plain WireGuard profile still imports and connects
   - Services page still installs ordinary WireGuard packages cleanly
   - OpenAPI endpoint reflects the new fields

#### Exit criteria

- At least one real AWG profile works on target hardware.
- Documentation explains package sourcing and user-visible limitations.
- WireGuard regressions are explicitly checked before sign-off.

## 4. Testing strategy

### 4.1 Automated tests

Run the project’s existing test stack during implementation:

- `cd backend && go test ./...`
- `cd shared && pnpm test`
- `cd frontend && pnpm test`
- `make test`
- `make lint`
- `make build`

### 4.2 Test matrix

#### Parser / model tests

- plain WireGuard config
- AWG 1.x config with only `J*` / `S*`
- AWG 2.0 config with `H1-H4`
- malformed AWG integers
- profile JSON backward compatibility with old saved profiles

#### Backend service tests

- import WG profile → `proto=wireguard`
- import AWG profile → `proto=amneziawg`
- runtime status command selection (`wg` vs `awg`)
- enable failure when AWG packages are missing
- WG↔AWG profile switch reuses shared DNS/firewall paths
- Tailscale exit-node interaction still obeys single-active-tunnel rule

#### API / shared contract tests

- AWG fields appear in JSON responses
- availability endpoint returns deterministic reasons
- OpenAPI contains the new fields and endpoint paths

#### Frontend tests

- AWG badge on imported profile cards
- install prompt when profile is AWG but service is unavailable
- advanced settings render only when relevant
- no regression for plain WireGuard import flow

### 4.3 Device validation gate

Do not close the work on unit tests alone. This feature depends on:

- kernel module compatibility
- userspace `awg` behavior
- netifd proto helper semantics
- exact OpenWrt packaging on target hardware

Real-device validation is a release gate, not a nice-to-have.

## 5. Deployment considerations

### 5.1 Package source strategy

AmneziaWG is **not in official OpenWrt feeds**, so shipping the UI alone is not enough. Travo needs a repeatable package story for:

- kernel module (`kmod-amneziawg` or equivalent)
- userspace tool (`awg`)
- netifd / LuCI proto integration (`amneziawg.sh`, LuCI proto package if used)

### 5.2 Recommended release model

- Build and publish AWG packages **per firmware / kernel build** alongside Travo releases.
- Treat the kernel module as **ABI-pinned**: a firmware kernel bump requires a matching AWG rebuild.
- Keep package names and feed URL in one release-time manifest so device install behavior is deterministic.

### 5.3 UI / API fallback behavior

When the package set is missing or incompatible:

- plain WireGuard stays fully functional
- AWG availability endpoint returns `ready=false` with a reason
- AWG profiles remain visible but cannot be enabled
- UI shows install / unavailable messaging instead of silently downgrading to WireGuard

### 5.4 Suggested future packaging touchpoints

If Travo later owns AWG package publishing inside this repository, likely update:

- `.github/workflows/release.yml`
- `docs/deployment.md`
- any release artifact manifest consumed by the device installer

That work is adjacent to this feature but may be staged separately if package publishing remains external.

## 6. Risk mitigation

| Risk | Impact | Mitigation |
| --- | --- | --- |
| Kernel module / firmware mismatch | AWG cannot start after firmware update | Gate with availability endpoint, pin package builds per kernel, test on target image before release |
| Hard-coded `wg0` assumptions fail for AWG helper | Firewall/status code breaks | Validate early on device; centralize ifname/proto/tool selection in helpers |
| Code duplication between WG and AWG paths | Regressions and drift | Share one WireGuard-family pipeline; isolate only proto/tool/UCI differences |
| Existing profiles JSON lacks new metadata | Profile load failures | Keep new JSON fields optional and derive metadata when absent |
| UI becomes confusing with too many protocol details | Support burden | Keep AWG as an advanced extension of the current WireGuard UX, not a separate product surface |
| Late installation failure due to missing packages | Bad UX | Surface readiness before toggle/import activation and provide install CTA |
| DNS / kill-switch regressions | Connectivity leaks | Reuse existing shared DNS/firewall logic and add protocol-switch regression tests |

## 7. Recommended execution order

1. Lock the package names / source of truth for AWG artifacts.
2. Finish parser + model work first so all later layers can share one contract.
3. Implement backend protocol switching before changing frontend behavior.
4. Extend API/shared types only after backend helpers stabilize.
5. Build frontend install / badge UX on top of the finished API.
6. End with device validation and documentation updates.

This keeps the highest-risk unknowns (package/runtime behavior) front-loaded while preserving the existing WireGuard path throughout development.
