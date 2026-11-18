# 0008 Auth, sessions, trust boundary

Status: accepted

Login validates the root password through rpcd `session.login`. Cookie session (HttpOnly, SameSite=Strict) + CSRF header; SSE for live push on the same cookie (fixes the old token-in-query-string weakness). Memory-only sessions on the monotonic clock; a persisted epoch file kills all older sessions on password change. API tokens for automation (0.2, #172) sit alongside the cookie session; see below.

Admin and first-run are served on `br-lan` (wired + AP WiFi) only, never from uplink or guest zones, enforced by firewall rules AND a daemon source/bind check. First-run with no root password shows only "set password"; the installer also requires `--password` or leaves that path. No default password.

Implementation (`internal/auth`, `internal/trust`, `internal/api`):
- Password check: `ubus call session login` with `timeout:1` (rpcd denies `session destroy`); wrong password and unknown user are the same failure. Password only in the JSON argument (argv-safe) or on stdin for `passwd`, never in logs or responses. NUL in a password is rejected before rpcd (it would truncate).
- First-run: `/etc/shadow` root field empty = not set. Unreadable shadow fails closed (503, setup stays shut). Setup re-checks shadow under a lock; exactly one request wins.
- Sessions: 32-byte token, stored as sha256 only; monotonic expiry; epoch file persisted before the password-change reply; persist failure revokes all and returns 503.
- Trust: outer wrapper before routing, `RemoteAddr` only (no `X-Forwarded-For`), `lan` IPv4 subnets plus loopback; before the first interface read only loopback. **Loopback is trusted**, so a local reverse proxy in front of the daemon would make every client look trusted: packaging must not put a proxy in front, or must drop the loopback allowance.
- Default-deny: every route registers through one helper with an explicit `public` flag; a test walks the spec. Errors `{error, code}`; bad input is 400 `invalid_request` (unknown field, null, trailing data, case-variant keys).
- DNS-rebinding guard: after the source check, `Host` (port stripped, lowercased) must be loopback/`localhost`, an address of the router's `lan` interface, or a `--host-allow` name, else 421 `bad_host`; `/api/health` exempt. Before the first interface read only loopback/localhost pass.

API tokens (`internal/apitokens`, #172):
- `Authorization: Bearer tgt_<43 base64url chars>` on protected routes; 32 bytes from crypto/rand, no fallback. Only `sha256(token)` is stored in `api-tokens.json` (state dir, 0600, ADR 0009 rules: absent = empty, invalid fails closed and authenticates nothing). The raw token appears once, in the create reply. At most 16.
- No rate limit on bearer attempts: 256-bit secrets are not guessable, and the trust boundary already limits who can try.
- When the header is present it decides alone: a bad bearer is 401 even next to a valid cookie. Bearer requests skip the CSRF check (a browser never attaches the header by itself).
- Scopes: `read` = GET only, and not admin-only reads (`getSystemBackup`: it holds WiFi keys and the shadow file) → 403 `insufficient_scope`; `admin` = every protected route except session-only ones.
- Session-only routes (registered with `registerSessionOnly`): token management, password change, adding or removing a root SSH key (#191: a key is a root shell, like the password), logout, session → 403 `session_required` for any token. A token cannot mint, list or revoke tokens.
- Trust and Host checks apply before auth, unchanged: tokens work from the LAN only. The AdGuard proxy stays cookie-only.
- Password change does not revoke tokens (like root SSH keys); revocation is explicit `DELETE /api/v1/auth/tokens/{id}`, effective immediately and across restarts. An open SSE stream re-checks its token like a session.
- `created_at` is the router wall clock, for display only; tokens do not expire.
