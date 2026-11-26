# 0031 Runtime validation only where a boundary needs it

Status: accepted (#281); narrows the validation sentence of [0003](0003-api-and-client-generation.md)

## Context

ADR 0003 said Zod Mini or Valibot validates anything not covered by the generated client. Through 0.2.0 no such boundary appeared: every request and response the UI handles goes through the hey-api client generated from the Huma spec, and neither library is in `web/package.json`.

## Decision

Add Zod Mini or Valibot only when a UI boundary outside the generated client needs runtime validation (for example, parsing a file the user uploads, or data from `localStorage` with a shape that can drift). The daemon path never uses Zod. Until such a boundary exists, neither library is a dependency.

## Consequences

The first boundary that needs it adds the library, within the preset size rules, in the same PR. `AGENTS.md` states the same rule.
