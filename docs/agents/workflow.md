# Workflow: from issue to merge

How work moves on this repo. The coordinator (a large model, or the operator) owns steps 1, 3 and 5; implementers own step 2; a fresh reviewer owns step 4.

## 1. Brief

An issue is **ready-for-agent** only when it carries an agent brief that a small model can execute without guessing. The brief follows the Task template (`.github/ISSUE_TEMPLATE/task.yml`) and Matt Pocock's agent-brief rules (`/triage`, AGENT-BRIEF.md): behavioural contract, key interfaces and signatures, explicit out-of-scope, checkable acceptance criteria.

- **Harvest** before writing tickets for an epic: a fresh subagent reads only the old Travo repo's paths for that area (the epic's "Old Travo" inventory comment lists them) and writes notes outside this repo: fix commits as regression scenarios, test-case names as scenarios, real payloads to re-capture, UI flows. Facts only, never code (ADR 0001). The coordinator turns the notes into acceptance criteria and test cases; payloads land in `testdata/` only when re-captured on our router with provenance.
- `epic` issues are placeholders. Run `/to-tickets` on the epic when its blockers are closed: it produces tracer-bullet tickets (one vertical slice each, one fresh context window each) as sub-issues with native blocked-by edges.
- Before labelling ready, run a **readiness probe**: a fresh small model reads the brief plus the repo and lists every sentence it would have to guess at, every referenced name that does not exist, and every acceptance item no command or test can check. Fix the brief and re-probe until it says "implementable: yes". Verify the facts a brief states (status codes, library behaviour) by running them, not from memory. This step is cheaper than any review loop.
- Shared tooling that several briefs need (a test helper, a dev task) is its own small ticket that blocks them, not a "reuse it if merged" clause.
- Rules live in `docs/` and the brief links them; the brief never copies a rule.
- Product intent lives in issues: a PRD issue, then `/to-tickets`. There is no spec file to update. A rejected feature request goes to `.out-of-scope/` (`/triage`).

## 2. Implement

- **Frontier**: open `ready-for-agent` issues with zero open blockers (`gh api repos/blackdark-org/travo-greenfield/issues/<n> --jq .issue_dependencies_summary.blocked_by` is `0`). Dispatch the whole frontier in parallel.
- One issue, one fresh implementer, one worktree (`../travo-greenfield-<slug>`, branch `<type>/<slug>` from `origin/main`). Model by the issue's `model:*` label. Move the card to `In progress`.
- The implementer runs `/implement` for the issue: `/tdd` red→green at the seams the brief names, `mise run check` green, then `/code-review` on its own diff. It commits (Conventional Commits, one logical unit each, every commit green), pushes, and opens a PR with `Closes #<n>` following the `/pr` shape (summary, evidence, merge danger). It stops and reports `blocked` when the brief contradicts the repo, instead of guessing.

## 3. Coordinate

- On a `blocked` report: fix the brief (step 1) and re-dispatch a fresh implementer.
- Cross-PR conflicts: rebase the later PR; a merge that compiles alone but not together is caught by `mise run check` on main, so re-run it after each merge.

## 4. Review

A fresh reviewer (large model, `/code-review`: Standards axis against `AGENTS.md` and `docs/`, Spec axis against the issue) with no implementer context. Findings go back to the implementer branch; loop until the reviewer says merge. Move the card to `In review` while this runs.

## 5. Merge and close

- Rebase-merge (squash only for single-topic fixups). The PR closes the issue unless it has `needs-hardware`: then move the card to `Needs hardware`, keep the issue open, and add the issue's step to `docs/testing.md` "Hardware playbook".
- Remove the worktree and branch. When the frontier changes, go back to step 2.
