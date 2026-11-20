# 0003 Generated API spec and client

Status: accepted; validation outside the generated client narrowed by [0031](0031-runtime-validation-only-where-needed.md)

Huma generates OpenAPI 3.1 from typed Go handlers; hey-api generates the TypeScript client (and optionally TanStack Query bindings) from that spec. Nothing is hand-maintained. CI regenerates both and fails on a diff. Spec served publicly at `/api/openapi.json`.

Zod Mini or Valibot validates anything not covered by the generated client, per the preset size rules.
