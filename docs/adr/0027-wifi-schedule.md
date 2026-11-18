# 0027 Scheduled access point on/off

Status: accepted

Amends the "backend NEVER self-confirms" rule of [invariants](../invariants.md) §1 and
[0005](0005-transactions-and-watchdog.md) for one case (#39, #241). Builds on the hook
socket and the shared crontab writer of [0024](0024-daemon-hooks-for-cron-and-buttons.md).

## Context

Operators want an AP off at night and back on in the morning. The router has no zoneinfo:
the time zone is a POSIX string in `/etc/TZ` that Go's `time` cannot load (device-facts
§8), while BusyBox crond applies it. A scheduled toggle has nobody to press Confirm.

## Decision

- Clock: BusyBox crond, through `internal/crontab` (owner `wifi-schedule`). Each enabled
  rule renders two lines, `M H * * <days>` for its off and on times, each running
  `travo-greenfield --hook wifi-schedule-<id>-<off|on>` (ADR 0024). Days apply to both times.
- Rules live in `/etc/travo-greenfield/wifi-schedule.json` (ADR 0009: atomic, 0600,
  unreadable fails closed and is never overwritten). This is a deliberate exception to
  "the block is the source of truth" in 0024: a cron line cannot carry the AP section (UCI
  names are not valid hook action names) or the lockout acknowledgement. The block is
  rendered from the file: on save (crontab first, then the file; a failed file write
  restores the previous block) and at daemon start.
- Hook actions are registered per rule id at start and on save; actions of a removed or
  paused rule stay registered and answer `unchanged` (hook actions cannot be removed).
- Apply: the existing AP transaction (`UpdateAP`), SSID and key unchanged, caller Unknown,
  so the lockout rules of §2 apply as written: without another enabled non-guest AP the
  toggle is refused unless the rule carries `acknowledge_lockout`. Saving such a rule
  without it is a 409 `wifi_lockout_risk`; the UI warns before the operator acknowledges.
  Any held guard or a pending transaction skips the toggle.
- Confirm: the scheduler confirms the transaction itself once the AP probe proved every
  expected AP (the edited one gone or up, every other enabled AP up). The operator gave the
  confirmation in advance by saving the rule. A failed probe rolls back exactly as in a
  user transaction. This is the only self-confirming path; it is limited to an AP's
  `disabled` flag on a saved rule.
- Outcomes (`applied`, `unchanged`, `rolled_back`, `skipped_guard`, `skipped_busy`,
  `skipped_lockout`, `failed`) stay in memory and show in
  `GET /api/v1/wifi/schedule/status` with whether crond runs. Anything but `applied` and
  `unchanged` is also the hook's error code, so the cron job's `logger` line shows it.

## Consequences

- A toggle fired while the daemon is down fails in `logread` and is not applied later.
- If crond is stopped or the crontab replaced, nothing fires; the status shows
  `cron_running:false`, and a daemon restart re-renders the block.
- An overnight window has its "on" line on every listed day's morning, which is an
  `unchanged` no-op when the AP is already on.
