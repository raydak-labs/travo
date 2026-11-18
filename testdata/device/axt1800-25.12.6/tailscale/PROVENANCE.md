# tailscale/ (2026-11-17, #179)

Captured on GL-AXT1800, OpenWrt 25.12.6, between 23:10 and 23:12 UTC (router clock) under the shared router lock. The package was installed for the capture and removed afterwards: `apk del tailscale kmod-tun`, then `rm -rf /etc/tailscale /var/lib/tailscale` (apk leaves the node state and SSH host keys behind), `nft delete table ip filter` and `ip nat` (empty tables tailscaled created), `rmmod tun`. `uci export` before and after compared byte-identical. No login happened: every file shows the logged-out node.

Commands, in capture order:
- apk-info.txt: `apk info -a tailscale` (before install)
- df-overlay-before.txt / df-overlay-after.txt: `df /overlay` around the install
- service-list-absent.json: `ubus call service list '{"name":"tailscale"}'` before install
- apk-add.txt: `apk update && apk add tailscale` (shell trace lines included); the package pulls `kmod-tun` and its post-install starts the service
- apk-contents.txt: `apk info -L tailscale` and `apk info -L kmod-tun`
- apk-version.txt: `apk version tailscale`
- uci-tailscale.txt: `cat /etc/config/tailscale`
- initd.txt: `cat /etc/init.d/tailscale`
- service-list-tailscale.json: `ubus call service list '{"name":"tailscale"}'` 3 s after install
- version.txt: `tailscale version`
- status-needslogin.json / .err: `tailscale status --json` (stdout / stderr plus exit code) right after install
- status-text-needslogin.txt: `tailscale status` (prints `Logged out.`, exit 1)
- prefs-default.json: `tailscale debug prefs`
- set-accept-dns.txt: `tailscale set --accept-dns=false` (no output, exit 0)
- prefs-accept-dns-false.json: `tailscale debug prefs` after that
- link-tailscale0.json: `ip -j link show tailscale0` (before `up`)
- ip-rule.txt, nft-tables.txt: `ip rule`, `nft list tables` before `up`
- up-timeout.out / .err: `tailscale up --accept-dns=false --timeout=8s` (prints the auth URL, times out after 8 s, exit 1)
- status-authurl.json: `tailscale status --json` after that: `NeedsLogin` with `AuthURL` set; the backend keeps the login URL after the CLI exits
- prefs-after-up.json: `tailscale debug prefs` after `up`
- up-revert.txt: `tailscale up --timeout=3s` without `--accept-dns=false` (refused: non-default flags must be repeated or `--reset` given)
- up-json.out / .err: `tailscale up --json --accept-dns=false --timeout=5s`
- set-exit-node-unknown.txt: `tailscale set --exit-node=100.64.0.1` while logged out (accepted, stored as `ExitNodeIP`)
- set-ssh-routes.txt: `tailscale set --ssh --advertise-routes=192.168.1.0/24 --exit-node-allow-lan-access=true`
- prefs-ssh-routes.json: `tailscale debug prefs` after that
- exit-node-list-needslogin.txt: `tailscale exit-node list` (exit 1)
- link-tailscale0-up.json, addr-tailscale0.json: `ip -j link show tailscale0`, `ip -j -4 addr show tailscale0` after `up`
- ip-rule-up.txt, route-table52.txt: `ip rule`, `ip route show table 52` after `up`
- nft-tables-up.txt, nft-ip-filter.txt, nft-ip-nat.txt: `nft list tables`, `nft list table ip filter`, `nft list table ip nat`
- set-exit-node-clear.txt: `tailscale set --exit-node=`
- logout.txt, status-after-logout.json, prefs-after-logout.json: `tailscale logout` while still in `NeedsLogin`, then status and prefs
- down.txt: `tailscale down`

Not captured yet: a `Running` status with peers, `CurrentTailnet` and an exit node. That needs the operator's real login (docs/testing.md, Hardware playbook, Tailscale). Unit tests for the running state reuse only field names present in these captures (`Self` has the same `PeerStatus` shape as `Peer` entries).

Redactions (values replaced with `REDACTED`): the login code in every `https://login.tailscale.com/a/<code>` URL, the base64 QR image in up-json.out (it encodes the same URL), and the `LogID` in apk-add.txt. Every other byte is verbatim.
