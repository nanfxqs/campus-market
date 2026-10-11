# Formal competition results

Each configuration used a fresh isolated demo-account project and 100 independent HTTP-published competition products plus one separate same-key product. No dedicated full-data sale-pool product was used. These are correctness experiments on independent fixtures, not full-data throughput or cache-benefit benchmarks.

| Configuration | Groups passed | Competition success/conflict/infra/other | Success median/P95/P99 ms | Conflict median/P95/P99 ms | Same-key success/changed-body conflict | Cleanup |
|---|---:|---|---|---|---|---|
| A | 100/100 | 100 / 4900 / 0 / 0 | 25.68/38.33/42.49 | 35.95/46.04/62.20 | 54 / 4 | exit 0 |
| B | 100/100 | 100 / 4900 / 0 / 0 | 25.04/39.49/42.23 | 35.72/46.08/53.00 | 54 / 4 | exit 0 |
| C | 100/100 | 100 / 4900 / 0 / 0 | 28.24/42.89/47.39 | 38.29/48.60/56.07 | 54 / 4 | exit 0 |

Every group had exactly one successful stored transaction with the expected buyer and 10,000-cent price, correct sold archive and sale values, no listing, and a seller counter increment of one. The 54 same-key responses per configuration retained one canonical transaction ID and one counter increment. C observed both upstream addresses in every group and in sequential same-key and changed-body retries. The original token obtained through Nginx remained valid on both instances.

Competition latency statistics use 100 successful and 4,900 conflicting responses per configuration. Same-key concurrency, sequential replay and changed-content rejection are reported separately in each `summary.json` under `statisticsByPhase`; their raw responses remain in `responses.jsonl`. Dispatch spread and all per-group facts remain in `database-facts.jsonl`. P95/P99 use nearest-rank samples; there is no additional latency pass threshold.

All formal attempts passed on their first run; there were no failed formal competition attempts to omit. The earlier two-group C functional run remains separately labeled. Orchestration preparation, readiness, seed, publishes, profile reads, database checks and cleanup are excluded from request latency statistics. Container/image/configuration identities, service distribution logs, source hashes and cleanup records are retained under each `formal-A`, `formal-B`, `formal-C` directory. All three source-hash records match the delivered scripts.

The exclusive experiment window finished after C cleanup at 2026-10-11T06:36:09Z. `docker ps` was empty immediately afterward, and the generated project guards verified no owned containers or volumes remained. The formal full-data sale window can proceed independently.
