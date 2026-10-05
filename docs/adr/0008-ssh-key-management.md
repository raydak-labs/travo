---
title: "ADR 0008: SSH key management (root authorized_keys)"
description: Root authorized_keys management: adding and deleting keys, and what the API refuses to do.
status: Accepted
date: 2026-09-28
updated: 2026-09-28
tags: [adr, auth, ssh, dropbear, security, system]
---

# ADR 0008: SSH key management (root authorized_keys)

## Status

Accepted.

## Context

Travo manages a **travel router** whose only real administrative credential is the
**root** password (shared with LuCI, validated via rpcd — see ADR 0007). Operators who
prefer key-based SSH — the common case for anyone automating a router they carry
between networks — need a way to install a public key **without** hand-editing files
over a serial console.

That convenience is the whole feature, and it is also the sharpest edge in the API:
the endpoint below writes a file that grants **unauthenticated root shell access**,
entirely bypassing the password, the JWT session, the IP allowlist and every other
control Travo enforces. A bug in it is a remote-root vulnerability, not a UI defect.

## Decision

### 1. What the endpoint may write

- The only file this feature touches is **`/etc/dropbear/authorized_keys`**
  (`authorizedKeysFile`, `backend/internal/services/system_service.go`), i.e. dropbear's
  root key file. It writes **exactly one line per accepted call** and never rewrites,
  reorders, normalizes or de-duplicates the rest of the file.
- The API surface is three operations (`backend/internal/api/router.go`):

  | Method | Path | Service method |
  | ------ | ---- | -------------- |
  | `GET` | `/api/v1/system/ssh-keys` | `GetSSHKeys` |
  | `POST` | `/api/v1/system/ssh-keys` | `AddSSHKey` |
  | `DELETE` | `/api/v1/system/ssh-keys/:index` | `DeleteSSHKey` |

- `AddSSHKey` appends with `O_APPEND|O_CREATE` at mode `0600` after `MkdirAll` at
  `0700`. Append-only is deliberate: a read-modify-write would let a concurrent
  request, or a truncated file, destroy keys the UI never showed.
- `DeleteSSHKey` removes by **line index into the same split `GetSSHKeys` reports**, so
  the number a client saw always maps to the same line. Negative and out-of-range
  indexes are **rejected**, not clamped. The index space is positional, not
  content-derived: re-adding a key after a delete shifts the indexes. This is a known
  wart of the existing contract, kept because the alternative (key fingerprints) is a
  breaking API change.
- The three operations are documented in the OpenAPI spec
  (`backend/internal/api/openapi_handler.go`, paths `/system/ssh-keys` and
  `/system/ssh-keys/{index}`), which the route-drift test enforces
  (see [ADR 0006](./0006-application-platform-and-api-contract.md)).

### 2. The validation boundary

`AddSSHKey` is the only write, so it is where validation lives. Two checks, in order
(`AddSSHKey` plus `sshKeyRe` in the same file):

1. **Single line** — `strings.ContainsAny(key, "\n\r")` is rejected. This is the
   security-critical one: the value is written verbatim with `Fprintln`, so an
   embedded newline would silently append *additional* `authorized_keys` lines, i.e.
   grant root access to a key the caller never submitted.
2. **Key format** — a regexp anchored at both ends requiring
   `<type> <base64>` with an **optional** trailing comment, where `<type>` is limited to
   the algorithms dropbear accepts (`ssh-rsa`, `ssh-ed25519`,
   `ecdsa-sha2-nistp256/384/521`, the two `sk-` FIDO types) and the body is
   `[A-Za-z0-9+/]+={0,3}`. The comment, when present, is any non-newline text
   (`GetSSHKeys` only surfaces a comment when the line has three or more fields).

Because the write is `fmt.Fprintln` of an already-validated single line, the file can
never receive a line the validator would have rejected: **validation is the
authorization for the write, not a best-effort filter**. The comment field is not
escaped or interpreted by anything — dropbear parses only the type and the base64 body
— so a comment containing shell metacharacters is inert.

These are the same class of guard as `services.ValidateHHMM` and
`services.ValidateButtonName` (`backend/internal/services/validate.go`): values that
get **formatted into a root-owned file or a cron line** are validated at the point
where they are about to be written, not only at the HTTP boundary.

### 3. Authentication is mandatory, and the default is deny

- These routes are registered on the `v1` group (`/api/v1`) with **no exemption**.
  `AuthService.Middleware` (`backend/internal/auth/auth.go`) is default-deny: it
  short-circuits only for an explicit allowlist — `/api/health`,
  `/api/openapi.json`, `/api/v1/auth/login`, `/api/v1/ws` and
  `/api/v1/system/time-sync`. Everything else under `/api/` requires
  `Authorization: Bearer …` **and** passes `ValidateToken` **and** is not on the
  blocklist. So the SSH-key routes are covered by ADR 0007 end to end: session TTL,
  revocation on logout, and revocation of every session on password change.
- The feature is therefore never reachable pre-login. It is **not** added to the
  IP-allowlist bypass list (ADR 0007 §3), and it is **not** exempted from the
  time-sync endpoint's bootstrap exemption, which is a different endpoint.
- If SSH key management is ever exposed **before** login, it becomes equivalent to
  handing out the device. That requires a new ADR; it is not a config flag.

### 4. Blast radius if the validator is wrong

A key this API accepts is a **root** key for the device's SSH daemon, and it keeps
working after Travo is uninstalled. Consequences accepted here:

- Adding a key is intentionally equivalent to handing out the device, so it is a
  **high-signal, low-frequency** operation. There is no separate "trust this key"
  flow, no quarantine, and no expiry — dropbear's `authorized_keys` model has none.
- The UI surface must state plainly what the operation does. A key added here is
  **not** revoked by a Travo redeploy, by a password change, or by a session
  revocation; only `DELETE /api/v1/system/ssh-keys/:index` or an out-of-band edit of
  the file removes it.
- `GetSSHKeys` returns the **public** key bodies and comments, never private
  material; there is no private-key path in Travo at all.

## Consequences

- Any new field written into `authorized_keys` must pass the same single-line plus
  format check. Do not add an "options" or "restriction" parameter (e.g. `command=`,
  `from=`) without a new ADR: those are honoured by other SSH implementations and
  silently ignored by dropbear, which is a security-relevant inconsistency.
- Adding a route that writes a root-owned file means adding the matching OpenAPI
  entry in the same change, or the route-drift test fails
  (see [ADR 0006](./0006-application-platform-and-api-contract.md)).
- `GetHardwareButtons` has a sibling pattern worth copying: a config-parse failure is
  **surfaced to the operator** rather than reported as "nothing configured", because
  the on-disk artifact keeps the previous behavior.

## Open questions

- The positional index delete is re-order-fragile. A fingerprint-addressed delete
  would be strictly better and is a breaking API change.
- Whether `AddSSHKey` should log (an audit line for "root SSH key added") is not
  decided here; see `docs/requirements/tasks_open.md`.

## References

- `backend/internal/services/system_service.go` — `AddSSHKey`, `DeleteSSHKey`,
  `GetSSHKeys`, `authorizedKeysFile`
- `backend/internal/api/router.go` — route registration
- `backend/internal/api/system_handlers.go` — handlers
- `backend/internal/api/openapi_handler.go` — spec entries
- `backend/internal/auth/auth.go` — default-deny `Middleware`
- [ADR 0006](./0006-application-platform-and-api-contract.md) — OpenAPI drift gate
- [ADR 0007](./0007-authentication-and-access-control.md) — auth and session rules
- [ADR 0003](./0003-crash-guards-and-live-state.md) — the rule this write does **not**
  need (it does not change connectivity or routing), and why that is a considered
  omission rather than an oversight
