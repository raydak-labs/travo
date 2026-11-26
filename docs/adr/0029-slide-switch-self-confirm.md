# 0029 Slide switch actions confirm themselves

Status: accepted

Amends the confirm rule of [invariants](../invariants.md) §1 (second automated self-confirm
exception) and the "only self-confirming path" sentence of
[0027](0027-wifi-schedule.md). Builds on [0024](0024-daemon-hooks-for-cron-and-buttons.md).

## Context

#195 lets the operator bind a hardware slide switch (EV_SW, `BTN_0`..`BTN_9`) to the
WireGuard tunnel or the guest AP (`travo-greenfield.button_<node>.action`). Nobody sits at a
browser when the switch moves, so a transaction left for a user confirm would always roll
back after 60 s. The implementation (`internal/buttons/fire.go`) therefore confirms the
transaction itself once its probe passed. Invariants §1 forbade that outside a named list
that held only the WiFi schedule, so the code broke the invariant until this record.

## Decision

The slide switch is the second automated self-confirm exception.

- Trigger: procd runs `/etc/hotplug.d/button/50-travo-greenfield` for a `BTN_<n>`
  `pressed`/`released` event; it calls the `button-btn-<n>-pressed|released` hook action
  (ADR 0024). `pressed` turns the configured action on, `released` off. Push buttons and
  the reset button never reach it. The operator gave the confirmation in advance by binding
  an action to the switch.
- Preconditions: a configured action other than `none`; no held guard (`skipped_guard`); no
  pending transaction (`skipped_busy`). The guest action needs a configured guest AP and
  skips when it is already in the wanted state.
- Probe before the confirm, each the feature's own:
  - VPN on: `wg0` up and a configured peer shows a handshake newer than the transaction's
    start (`vpntunnel.proveEnabled`). VPN off: the `wg0` link is gone and a default route
    via an uplink exists (`vpntunnel.proveDisabled`).
  - Guest WiFi: the AP probe sees the guest AP gone or up and every other enabled AP up
    (the guest transaction's expected AP layout).
  A failed probe rolls back exactly as in a user transaction (`rolled_back`); the confirm
  runs only after the probe passed, never on a timer.
- Lockout: the action runs with an unknown caller (`netip.Addr{}`) and never sets
  `acknowledge_lockout`, so the fail-closed rule of §2 applies: an unknown caller needs a
  non-guest AP that stays up, otherwise the change is refused (`skipped_lockout`). There is
  no stored acknowledgement for the switch.
- Outcomes stay in memory per switch (`last` in `GET /api/v1/system/buttons`) and in the
  hook's log line; nothing is written to flash per slide.

## Consequences

- The exception list in invariants §1 has two entries; a third needs its own ADR naming
  trigger, probe and lockout handling.
- A switch slid while a user transaction is pending does nothing; the user slides again.
- A slide whose change would leave no non-guest AP up is always refused; the operator
  makes that change from the UI with the acknowledgement instead.
