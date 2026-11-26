# Triage Labels

The skills speak in terms of five canonical triage roles. This file maps those roles to the actual label strings used in this repo's issue tracker.

| Label in mattpocock/skills | Label in our tracker | Meaning                                  |
| -------------------------- | -------------------- | ---------------------------------------- |
| `needs-triage`             | `needs-triage`       | Maintainer needs to evaluate this issue  |
| `needs-info`               | `needs-info`         | Waiting on reporter for more information |
| `ready-for-agent`          | `ready-for-agent`    | Fully specified, ready for an AFK agent  |
| `ready-for-human`          | `ready-for-human`    | Needs the operator: hardware, secrets, or a decision |
| `wontfix`                  | `wontfix`            | Will not be actioned                     |

When a skill mentions a role (e.g. "apply the AFK-ready triage label"), use the corresponding label string from this table.

Edit the "Label in our tracker" column to match whatever vocabulary you actually use.

## Repo labels beside the roles

- Category: `bug`, `enhancement`. `epic` marks a placeholder that needs an agent brief (`/to-tickets` or a brief comment) before any code.
- `area/*`: the part of the system (daemon, web, packaging, wifi, dns, vpn, adguard, ci).
- `model:small` / `model:medium` / `model:large`: the smallest model the brief is written for; the coordinator dispatches by it.
- `needs-hardware`: the issue stays open after merge until its step in `docs/testing.md` "Hardware playbook" passes on the router.
- Blocking is GitHub's native "blocked by"; there is no `blocked` label.
