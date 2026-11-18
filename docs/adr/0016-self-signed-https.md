# 0016 Self-signed HTTPS beside plain HTTP

Status: accepted (#183; container-tested, browser check in the hardware playbook)

The daemon serves the same handler over TLS on `--listen-tls` (UCI `travo-greenfield.main.listen_tls`; empty or `off` = no TLS). Plain HTTP on `--listen` stays and does not redirect. TLS changes nothing in the trust boundary of [0008](0008-auth-and-trust-boundary.md): the source check (`RemoteAddr`), the `Host` check, default-deny, cookie session and CSRF header apply unchanged, because both listeners share one `http.Handler`.

## Certificate

- Self-signed ECDSA P-256 from the standard library (`internal/tlscert`); no new module dependency. The stripped arm64 binary grows by 589,824 bytes (9,633,952 → 10,223,776, budget 13,421,772): the server side of `crypto/tls` and certificate creation were not linked before (the update check only uses the client side).
- Generated on the router on the first TLS handshake, not at start: by then the trust provider knows the lan addresses for the SANs and the clock had time to sync. Stored in `/etc/travo-greenfield/tls/` (key 0600, atomic writes, key before certificate so a torn pair fails to parse and is regenerated). The state dir is in the sysupgrade keep-list, so the fingerprint survives upgrades.
- SANs: `localhost`, `127.0.0.1`, the lan IPv4 addresses at generation, the `--host-allow` names. A lan address change does not regenerate: the browser warns for a self-signed certificate anyway, and a new key would force a new exception.
- Validity 825 days (the maximum Apple platforms accept), starting one day before max(clock, build time): a router that boots at 1970 still gets a certificate valid today. Regenerated within 30 days of expiry or when unreadable. The SHA-256 fingerprint goes to the log so an operator can compare it with the browser's warning page.

## Why no redirect, no HSTS, no Let's Encrypt

- A redirect or HSTS would put a certificate warning in front of every first contact, including the first-run screen, and HSTS on an IP address would lock browsers out of HTTP after a certificate change. HTTPS is opt-in by URL.
- ACME needs a public name and either port 80 from the Internet or a DNS API; a travel router has neither. Out of scope.

## Cookies and the AdGuard proxy

The session cookie is `Secure` when issued over TLS (already the case since 0.1). Such a cookie never reaches the plain proxy on `:3001`, so the AdGuard proxy also listens with TLS on port 3443 (same certificate, same gate; the `Origin` check already follows the scheme). The UI links to the proxy on the page's own scheme. `--adguard-proxy-listen off` turns both off.

## Install

`install.sh` sets `listen_tls=:443` only when uhttpd no longer serves `:443` (after the LuCI move to 8080/8443) and nothing else listens there; an operator value, including `off`, is kept. Moving Travo off `:80` (the `--port N` mode, rollback, uninstall) removes a `:443` value again, since uhttpd keeps or gets back that port. A TLS listen failure is logged and never fatal: HTTP keeps working. Existing 0.1 installs that self-update get no TLS until the operator sets the option or re-runs `install.sh`.
