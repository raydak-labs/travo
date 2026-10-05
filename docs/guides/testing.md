---
title: Testing
description: How to run the checks — local suites, device scripts, and the playbook for hardware.
updated: 2026-10-05
tags: [docs, testing, integration, openwrt]
---

# Testing

This page is the entry point: which check settles a question, and the command that runs it.
Procedures and hardware facts do not live here. They live in the playbooks under
[`docs/tests/`](../tests/), and a fact written in both places is one place wrong within a month.
When you find a procedure on this page, move it to the owning playbook rather than fixing it here.

## Local checks

Run from the repo root. `make` resolves the pinned toolchain through [mise](../../.mise.toml), so a
local run uses the same tool versions CI does.

| Command | What it runs |
| ------- | ------------ |
| `make test` | `go test -count=1 ./...`, then `pnpm test` in `shared/` and `frontend/` |
| `make test-race` | the backend suite with `-race`, same as the CI race job |
| `make lint` | oxlint over `frontend/src` and `shared/src`, `golangci-lint` in `backend/` |
| `make format-check` | prettier `--check`, `gofmt -l`, `goimports -l`; non-zero when unformatted |
| `make format` | the same tools in write mode |
| `make shellcheck` | `shellcheck --severity=warning` over `scripts/`, `test/integration/` |
| `make coverage` | per-package Go coverage summary; informational, not a gate |
| `make build` | frontend production build plus `backend/bin/server` |

Narrower loops, when one project is what you touched:

```sh
cd backend && go test ./...
cd shared && pnpm test
cd frontend && pnpm test
```

Docs have gates too. Run them in the same change as any edit under `docs/`:

```sh
cd backend && go test ./internal/services/
```

They fail on a crash-guard path outside `/etc/trafo/`, a persistent-store path outside
`/etc/travo/`, an ADR missing from the index or the Obsidian hub, a playbook in `docs/tests/`
that nothing links to, and a plans catalog that omits a plan.

A green local suite is not evidence about the router. The on-device playbook's §2 lists the
classes of defect it demonstrably cannot catch.

## On-device checks

Never run against a device you care about: these suites mutate live state.

### Deploy first

```sh
make deploy                              # frontend + backend, direct method
make deploy DEPLOY_METHOD=release        # full file tree, like the release tarball
./scripts/deploy-local.sh --binary-only  # backend only
```

`ROUTER_IP` overrides the target (default `192.168.1.1`). A deploy is also the sanctioned retry
path that clears crash-guard files left by an interrupted operation
([ADR 0003](../adr/0003-crash-guards-and-live-state.md) §2). Options: `deploy-local.sh --help`;
the install and packaging paths are in [`deployment.md`](./deployment.md).

### Scripted suites

`make integration` runs every script in `test/integration/`, or one with `SUITE=`. It defaults to
`ROUTER_IP=192.168.1.1` and `LOGIN_PASSWORD=admin`; override both for your lab. Set
`TRAVO_INSECURE_SSH=1` only for a throwaway router whose host key is not in your `known_hosts`.
Each script's header comment is its own reference for flags, artifacts and exit codes.

- `integration-api-smoke.sh` — API contract and state integrity: every documented GET
  answers, the served OpenAPI spec matches the registered routes both ways, writes round-trip
  and revert, invalid input is rejected without a change, and no request leaves a staged UCI
  delta or a crash guard behind.
- `integration-device.sh` — baseline capture, WiFi disconnect and reconnect through the API, and
  the online check; writes `tmp/integration-<timestamp>/result.json`. Needs the WPA secret in
  `test/integration/.wifi_pass` (`--wifi-ssid`, `--wifi-pass-file`).
- `integration-vpn-dns-killswitch.sh` — the DNS leak endpoint and kill switch state over the
  API, optionally with a read-only UCI snapshot (`--ssh-verify`) or a brief
  enable/verify/disable cycle (`--enable-killswitch`).
- `integration-vpn-wireguard-toggle.sh` — WireGuard enable then disable, asserting the router
  keeps its internet connection.

```sh
make integration
make integration SUITE=integration-device.sh LOGIN_PASSWORD='…'
```

### Playbooks

A script asserts what it can reach over the API. Anything that needs an operator, a second uplink
or a deliberate fault injection belongs to a playbook:

- [`on-device-verification.md`](../tests/on-device-verification.md) — the standing playbook. Read
  its §1 prerequisites before anything else; §4 settles whether the build on the device is the
  build under test, which every later suite depends on.
- [`failover-verification.md`](../tests/failover-verification.md) — multi-WAN failover with two
  real uplinks. `on-device-verification.md` §9.1 carries the mwan3 rpcd ACL finding that has to be
  resolved before it can run.

## How to read a playbook

- Sections are numbered and cited from ADRs and from code comments, so quote the section number
  when you report a result rather than paraphrasing it.
- Durable device facts change only when a measurement changes; §10 of the on-device playbook says
  where a run that found nothing belongs instead (the PR description or the task log).
- Read §2 before running anything. A suite run against a build with a known defect listed there
  tells you only that the build is broken.
- Commands are written for `ssh root@…` against `192.168.1.1` and use `grep`, not `rg`.

## When the device is unreachable

Some changes only settle on hardware. If `192.168.1.1` cannot be reached:

- **Do not report the change as verified.** A green local suite is not a substitute, and the
  playbook's defect table exists precisely because the two disagree.
- **Name the outstanding rows.** Cite the playbook section and row IDs the change has not been run
  against, in the PR description or the task log entry, so the next person runs them rather than
  assuming they were covered.
- **Expect a punch list, and expect it to be temporary.** The 2026-10-04 review could only do
  static analysis while the device was down
  ([report](../../2026-10-04-deep-code-review.md)); the checklist it produced was replaced by the
  standing playbook once the device came back. Add findings to the playbook, not to a new list.
