# 0032 Self-update accepts signed additions; CI stages with the previous release

Status: accepted (#287); extends the release staging rules of [0007](0007-distribution-and-self-update.md)

## Context

Self-update runs the INSTALLED release's stager on the NEW tarball. Up to 0.2.0 the stager accepted only a fixed member list, so 0.2.0 adding `root/etc/hotplug.d/button/50-travo-greenfield` (#195) made 0.1.0 refuse the whole archive ("Could not verify release package"). CI only staged builds of the same commit, so it never saw the break.

Shipping a 0.2.x without the new file cannot help 0.1.0: the 0.2.0 stager lists that file as required, so the same tarball would then break 0.2.0 → 0.2.x. Release asset names are fixed per version, so one release cannot serve both.

## Decision

- From 0.2.1 the stager accepts any regular file under `root/etc/`, `root/lib/` or `root/usr/` that the signed `checksums.txt` lists, besides the required members and the flat UI directory. The signature already proves integrity; path cleaning, member type, size and count limits stay. Staging only extracts; the update worker still installs only the binary and UI.
- `mise run test:package` (run on every release-please PR) stages the candidate tarball with the stager built from the latest release tag. A release that the installed version cannot stage fails CI.
- 0.1.0 installs update once with `install.sh` (verified live in #287: config byte-identical, services kept). No bridge release.

## Consequences

A release may add installer-owned files under those prefixes without breaking self-update from 0.2.1 on; until the update worker learns to install them, they reach the router only through `install.sh`. Releases still cannot drop a member the previous stager requires, and the CI step catches that too.
