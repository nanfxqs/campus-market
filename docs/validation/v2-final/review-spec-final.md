## Spec

Both prior P2 findings are resolved at `3d4be5b98d266536b335758d3b4885acf89595ad`.

- **Stage binding:** sale evidence is checked against configuration, round, full initial scale, project/database and distinct actual start identity. Detail evidence is checked against configuration/state/pattern, full230k background, deterministic seed42 pool, paired hashes, all50 users and contiguous per-user receipts. Cold preparation must be empty; hot preparation requires every involved product's successful HTTP receipt and rejects subsequent FLUSHDB. This satisfies “A/B/C 各做三轮正式验收” and “同组不同配置的商品序列相同…热缓存指事先访问该组涉及商品后不清空”. It correctly avoids inventing a zero-cache-error or all-keys-resident gate.

- **Competition authority:** outcomes are rederived from raw HTTP receipts and parsed read-only mongosh stdout. The verifier checks100independent groups, distinct keys, exact success/product_sold counts, linked transaction ID/buyer/price/key, sold/listing facts, and seller+1 using the preceding public baseline. Same-key concurrent/sequential responses, changed-content idempotency_conflict and C's two-upstream evidence are now checked. Copied summaries must agree with raw facts and recomputed statistics. This satisfies “逐组核验成功记录、档案状态、资格移除与卖家计数” and “另验证同键重试多响应单事实”.

The retained stricter verification matrix reports all28original stages/proofs passing with9distinct sale identities. The focused log records25passing tests, including stage substitution, relabelled duplicate measurement, wrong scale/pool/state, missing prewarm, duplicate detail sequence, corrupted database facts, missing same-key receipts and incorrect conflict codes. Red counterexamples and original performance artifacts remain available.

No remaining or new concrete Spec-axis finding found. The readiness helper changes only bounded preparation outside measured windows; the reviewed fixes do not alter production behavior or formal workload parameters. No additional formal-performance rerun is required for these verifier/preparation changes.
