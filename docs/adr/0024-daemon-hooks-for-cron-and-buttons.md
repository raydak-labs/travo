# 0024 Cron jobs and button scripts call the daemon over a hook socket

Status: accepted

## Context

Schedules (LED night mode, WiFi schedule) and the hardware switch run outside the daemon: BusyBox crond reads `/etc/crontabs/root` (`/etc/cron.d` does not exist, device-facts §8) and procd runs `/etc/hotplug.d/button/*` as root. Old Travo's helpers ran `uci set/commit/apply` in shell with a stored rpcd login: a missing login file made every toggle a silent no-op, and the shell writes bypassed the in-process UCI locks and destroyed the API's staged delta (invariants §6).

## Decision

- Helpers never touch UCI. They run `/usr/bin/travo-greenfield --hook <action>`, which connects to `/var/run/travo-greenfield/hook.sock` and asks the running daemon to do the action with its own locks, guards and lockout rules (`internal/hook`).
- Authentication is the file system: the socket is mode 0600 in a 0700 directory, owned by the daemon's user (root). No token, password or login file is stored. The socket is in RAM and recreated on every start.
- Protocol: one action name and a newline in, `ok` or `error <code>` and a newline out. Actions are registered by name in the daemon; unknown names answer `error unknown-action`. One action runs at a time, bounded by a 90 s timeout. It is IPC, not an HTTP route: it never appears in the OpenAPI spec and is not reachable from any network.
- Cron lines pipe the client's output to `logger -t travo-greenfield-hook`, so a failed job (daemon down, action refused) is visible in `logread` instead of vanishing.
- `internal/crontab` is the one writer of `/etc/crontabs/root` in the daemon. Each feature owns a block between `# travo-greenfield:<owner> begin` and `... end`; lines outside Travo's blocks are kept byte for byte. The block is the source of truth for that feature's schedule (no shadow copy). A change sets `system.@system[0].cronloglevel=9` when unset (keeps the operator's value, invariants §15), writes the file atomically and restarts `cron` (its init script does not start crond while `/etc/crontabs` is empty).

## Consequences

- A helper fired while the daemon is down fails loudly in the log and does nothing; it never falls back to shell writes.
- Removing Travo leaves its crontab blocks behind until the uninstaller removes them; each line only calls the absent binary and logs the failure.
- The "external writers outside the lock model" gap (invariants §6) no longer applies to Travo's own helpers.
