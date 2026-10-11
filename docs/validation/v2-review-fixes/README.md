# Final review fixes

All three findings are addressed without changing production HTTP behavior, resource quotas, Redis settings, workload requests/scheduling or measured cutoffs. Historical raw experiment artifacts are unchanged. Only the verifier and shared Search preparation were changed; future sale/detail invocations record the helper SHA256. Existing measured-source hashes remain authoritative.

## Acceptance stage binding

The verifier binds sale IDs to configuration and round metadata, full initial230k/50k scale and actual project/database/start identity; a repeated measurement cannot occupy independent slots even if copied round metadata is relabelled. Details bind configuration/cold-hot-disabled state/pattern, full scale, canonical seed42 pool plus actual hash and complete archive cardinality; paired patterns must have identical pools. Every user's raw receipt sequence must be contiguous, unique and point to the deterministic pool position, with all50 represented. Cold/disabled preparation must be empty. Hot requires every involved product's successful real prewarm receipt and no subsequent FLUSHDB, without requiring ideal residence or zero natural Redis errors.

[Substitution red](stage-substitution-red.log) shows the old verifier accepted A1 for all nine slots; [green](stage-substitution-green.log) rejects it. Public CLI negative tests also corrupt scale, pool/order/hash, state/pattern, duplicate relabelled identity, duplicate request sequence, missing prewarm and post-prewarm reset. [Sequence red](detail-sequence-red.log) and [green](detail-sequence-green.log) preserve that additional counterexample.

## Competition authority

Each group is rederived from actual raw receipts and parsed `mongosh` stdout; seller-before comes from the preceding successful public `/users/me` receipt associated with publication. Verify100groups×50distinctkeys, one correct sold response and49product_sold conflicts, exact transaction association/ID/buyer/price/key, sold archive/removed listing/seller+1. Same-key must contain50concurrent+4sequential identical success responses, one database fact and4idempotency_conflict changed-content refusals. C requires both upstreams pergroup and for sequential retries/changed-content. Raw database outputs, fact entries and summaries must agree; overall and per-phase count/median/P95/P99 recompute from receipts.

[Raw seller-count red](competition-facts-red.log) shows unchanged summary hid a corrupted authoritative count; [green](competition-facts-green.log) rejects it. Tests also corrupt buyer/transactionID/businesscode/same-key receipt count/index/changed-content/Cupstreams/summary. [C recomputation](competition-C-recomputed.json) preserves an independently accepted original fixture.

## Shared readiness preparation

`scripts/search_readiness.py` serves both callers. It waits for exact full-cohort Search total16667 outside their measured windows, defaults to180seconds with1second retry and15second request bound capped by remaining preparation budget. It preserves sale UTC/expectedTotal/actualTotal and detail unixNs/expected/actual observation shapes, including every error/incomplete/success observation. This preparation deadline cap changes no sale/detail request timeout or production API. [Red](readiness-red.log), [green](readiness-green.log) and [both-format actual HTTP tests](readiness-both-formats.log) exercise HTTP500, incomplete cohort, eventual readiness and finite timeout against a local real HTTP server. No mock of internal collaborators, Docker or formal load was used.

## Final verification

[Final stricter matrix](verification-sequence-final/verification.json) independently passes all28original experiments/proofs, including nine distinct formal rounds and all eight matched detail groups. [Log](verification-sequence-final.log) records successful exit. Earlier fixed/final passes are preserved. [Focused tests](focused-tests-final.log) include all artifact suites; changed Python files compile and `git diff --check` passed. No owned external resources were created by these fixes, and no formal performance run was repeated for preparation/verifier changes. Report and source-reuse classification link this matrix and retain all prior failures and unavailable auxiliary stdout limitations.
