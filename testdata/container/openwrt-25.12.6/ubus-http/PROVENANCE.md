Container capture (not hardware): `openwrt/rootfs:aarch64_generic-25.12.6` booted with `/sbin/init` (procd, rpcd, uhttpd with uhttpd_ubus), 2026-11-09.
Requests: JSON-RPC POST to `http://127.0.0.1/ubus` with the null session `00000000000000000000000000000000`.
- login-no-root-password.json: session login root/"x" while /etc/shadow root field was EMPTY: succeeds (any password).
- then `printf "s3cret-pass\ns3cret-pass\n" | passwd root` (shadow-after-passwd.txt: `$5$` sha256crypt, mode 0600)
- login-wrong-password.json, login-unknown-user.json: both `[6]` (UBUS_STATUS_PERMISSION_DENIED); identical.
- login-ok.json: root/"s3cret-pass", timeout 5.
- destroy-own-session.json, destroy-again.json: `session destroy` is denied by the default ACL; use a short login `timeout` instead.
`ubus_rpc_session` values are throwaway container sessions. ACL maps kept verbatim.
These captures are reference for the rpcd JSON-RPC shapes; the code uses the ubus CLI, covered by the booted-container integration test.
