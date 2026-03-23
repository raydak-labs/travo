# Agent Instructions

## Tools & setup

- everything should be installed via `mise`. See `.mise.toml`

## Project Plans

The plan directory is `plans/`. Do NOT modify plan files.

The important file for the requirements is in `docs/requirements/requirements.md`.
When we implement things ensure that you update the feature sets or add features to the document.

## Project Structure

This is a pnpm monorepo with:
- `frontend/` — React + TypeScript + Vite + TailwindCSS
- `backend/` — Go + Fiber
- `shared/` — Shared TypeScript types

## Development

- Run `pnpm install` for Node dependencies
- Run `cd backend && go mod tidy` for Go dependencies
- Run `make test` to run all tests
- Run `make dev` for development servers
- Run `make format` for formatting code
- Run `make lint` to run lint check
- Run `make build` to build the projects

## Testing

Follow TDD: write tests first, see them fail, write minimal code to pass.
- Go tests: `cd backend && go test ./...`
- Shared tests: `cd shared && pnpm test`
- Frontend tests: `cd frontend && pnpm test`

## Real tests

- if we are testing on the real system you have access to the OpenWRT test environemnt via ssh to IP 192.168.1.1
- You are allowed execute and copy everything to the system
- for scp you have to use the legacy option and commands like rg are not available on the target system, use the simpler alternatives like grep
- always test things directly on the target system either with a browser if you capable to or via curl
- if needed cross check with the real configurations on the system

## Automated System Changes — Mandatory Failsafe

Any automated action that modifies live system state (UCI commit + wifi reload,
firewall rules, network interface changes, etc.) **MUST** use a crash guard:

1. **Write a guard file** to persistent storage (`/etc/openwrt-travel-gui/`) in
   flash **before** executing the dangerous operation.
2. **On next startup**, if the guard file exists, skip the operation entirely and
   log a warning — a previous run crashed the system and must not be retried.
3. **Remove the guard file** only after the operation completes successfully.
4. A manual redeploy (`deploy-local.sh`) clears all guard files, giving explicit
   permission to retry on the next boot.

**Why:** OpenWRT runs on constrained hardware with sensitive kernel drivers
(ath11k/IPQ6018). Operations like `wifi up` can cause kernel panics that
reboot the device. Without a crash guard, the service restarts after reboot and
immediately retries the crashing operation — creating a soft-brick crash loop
where the router is permanently inaccessible.

**WiFi commands:** On ath11k/IPQ6018 we use `wifi up` (recommended by OpenWRT)
to apply wireless config and avoid `wifi reload`, which tears down all wireless
first and is known to trigger driver/firmware crashes on this hardware.

**LuCI vs script:** When you click "Save & Apply" in LuCI (Network → Wireless),
LuCI does **not** run `wifi` or `wifi up` directly. It calls rpcd's **uci apply**
RPC, which commits config and triggers procd/ubus to reload affected services,
then starts a **rollback timer** (default 30s). The browser polls and calls
**uci confirm**; if that succeeds, the rollback is cancelled. If the device
becomes unreachable (e.g. reboot or crash), the timer fires and rpcd
**reverts** the config to the pre-apply state, so after reboot the device has
the old config and stays reachable. Our setup script must **not** run `wifi` or
`wifi up` from SSH: there is no rollback, so a crash leaves the new config in
place and can create a soft-brick loop. The script only writes UCI; the user
applies via LuCI "Save & Apply" or by rebooting.

**Backend:** The backend uses the same apply/confirm flow for wireless: after any
UCI wireless change it calls rpcd **session login** → copy wireless/network/system
to session dir → **uci apply** (rollback timeout) → **uci confirm**. No `wifi up`
is run; see `services/uci_apply.go` and `WifiService.applyWireless()`.

**Rule:** If you implement any background goroutine or scheduled task that touches
system state, it must follow this pattern. No exceptions.

Guard file naming convention: `/etc/openwrt-travel-gui/<feature>-in-progress`

**Rule:** If implementing new zones or something ensure that all required firewall changes and required rules are implemented. Follow the existing default "WAN" things which we should use.

**Rule:** If things would make implementing easier ask user if you can access the test device to execute command verify things on it. For example try replicating the flow via ssh/cli commands before implementing them blindly.

## Commit / finish task

- before you finish the task you must ensure the lint, tests and build are working (check Makefile for general things)
- if you have failed tests focus only on running them and at the end again test with Makefile
