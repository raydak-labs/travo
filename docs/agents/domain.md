# Domain Docs

How the engineering skills should consume this repo's domain documentation when exploring the codebase.

## Before exploring, read these

- **`GLOSSARY.md`** at the repo root.
- **`docs/adr/`**: read ADRs that touch the area you're about to work in.
- **`docs/invariants.md`**: MUST rules for live state, auth, DNS, WiFi. Breaking one needs an ADR first.
- **`docs/device-facts.md`**: hardware lessons; read before touching anything that talks to the router.

The `/domain-modeling` skill (reached via `/grill-with-docs` and `/improve-codebase-architecture`) adds terms and ADRs when they get resolved.

## File structure

Single context:

```
/
├── GLOSSARY.md
├── docs/adr/        (0001-clean-room-greenfield.md, ...)
├── docs/invariants.md
├── docs/device-facts.md
└── internal/, cmd/, web/
```

## Use the glossary's vocabulary

When your output names a domain concept (in an issue title, a refactor proposal, a hypothesis, a test name), use the term as defined in `GLOSSARY.md`. Use the term itself, never a synonym.

If the concept you need isn't in the glossary yet, that's a signal: either you're inventing language the project doesn't use (reconsider) or there's a real gap (note it for `/domain-modeling`).

## Flag ADR conflicts

If your output contradicts an existing ADR, surface it explicitly rather than silently overriding:

> _Contradicts ADR-0007 (release tarball, not a package), but worth reopening because…_
