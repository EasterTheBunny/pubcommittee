# M2-04 — API Contract for Review and Writes

**Phase:** local.

## Goal

`api/openapi.yaml` covers M2's read routes on `read-api` (threads, suggestions, published counts, moderation log entries for review items, brief page data) and its `write-api` routes, with generated Go and TypeScript types and contract tests.

## Read first

- [architecture § API contract](../../../architecture/README.md#api-contract) (resources, conventions, contract workflow)
- [10 § Provenance in every response](../../10-machine-access.md#provenance-in-every-response), [§ Instruction injection](../../10-machine-access.md#instruction-injection), [§ Write barriers](../../10-machine-access.md#write-barriers)
- [03](../../03-review-features-and-support.md), [06](../../06-sharing.md), [07 § Brief structure](../../07-legislator-brief-and-responses.md#brief-structure)
- `api/MACHINE-API.md` (versioning; `/v1` is additive from here on)

## Scope

**In**
- `read-api`: line comments and suggested edits by bill version and anchor; published counts with batch id and "fewer than N"; review-item moderation state; brief page data; `GET /v1/me/notifications` lives on `write-api` (credentialed).
- `write-api`: sign-in start and callback, sign-out, session, anti-forgery token; create and withdraw comments, suggestions, signals, markup requests; flag; steward actions ([M2-10](10-moderation-and-steward-actions.md)); push subscription; inbox.
- `content_class` on every text field, with `user` for every person-written field.
- Error problems for rate limits, conflicts (second signal), barrier refusals.

**Out**
- Office responses, delivery, two-party decisions (M3).

## Requirements

- Additive changes to `/v1` only; no breaking change (MACHINE-API).
- No route lists or searches review items by author (MILESTONES § M2; [10](../../10-machine-access.md)).
- Public timestamps are coarse (day) on review items and signals.
- Summary and preview fields never carry `user` content ([10 § Instruction injection](../../10-machine-access.md#instruction-injection)); a contract test enforces it.
- `write-api` routes are not on the API edge's cached paths and never cacheable.

## Acceptance criteria

- Spec validates as OpenAPI 3.1; generated code current; contract tests pass for every new route on `memory`.
- A spec test fails if any text field lacks `content_class`, or if a summary or preview field allows `user`.
- No path or query parameter filters by author or account.

## Open questions

- Display names or per-bill pseudonyms on pages ([M2-00](00-decisions-and-prerequisites.md)).
