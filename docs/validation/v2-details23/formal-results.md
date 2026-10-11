# Full paired detail measurements (#23)

All six measurements pass the protocol:50 ready users,60seconds,seed42,230,000 products including180,000 sold archives, unchanged full business counts, valid deterministic sequences and complete resource coverage. HTTP/business errors and telemetry omissions were zero. Default budgets,100ms Redis timeout and600-second TTL were unchanged. Fixed base time: `2026-10-11T00:00:00Z`.

| Run | Requests | RPS | Median/P95/P99 ms | Hit % | Cache errors | Queries/read | BSON bytes/read |
|---|---:|---:|---|---:|---:|---:|---:|
| [A-hotspot](formal/A-hotspot/summary.json) | 62383 | 1039.72 | 32.51/47.17/55.29 | disabled | 0 | 4.000 | 1504.8 |
| [A-dispersed](formal/A-dispersed/summary.json) | 62784 | 1046.40 | 32.10/47.66/58.89 | disabled | 0 | 4.000 | 1510.4 |
| [B-cold-hotspot](formal/B-cold-hotspot/summary.json) | 28517 | 475.28 | 53.35/199.93/227.11 | 94.43 | 1487 | 4.056 | 986.5 |
| [B-hot-hotspot](formal/B-hot-hotspot/summary.json) | 28512 | 475.20 | 54.78/199.25/220.53 | 94.52 | 1562 | 4.055 | 986.0 |
| [B-cold-dispersed](formal/B-cold-dispersed/summary.json) | 15781 | 263.02 | 190.19/400.48/696.79 | 0.00 | 3274 | 5.000 | 1805.0 |
| [B-hot-dispersed](formal/B-hot-dispersed/summary.json) | 17329 | 288.82 | 179.25/497.77/611.98 | 0.00 | 3532 | 5.000 | 1805.6 |

B was slower at these budgets despite hotspot returned BSON falling about34.5%. Query count did not fall: hits still validate the product version and read seller, eligibility and history. App CPU approached its two-core limit. Raw process/container/host/client/collector resources and volume sizes are retained; these observations do not prove a causal decomposition of latency.

Each pattern shares its exact seeded potential sequence and pool hash across configurations. Throughput changes actual consumed prefixes, preserved per user. Sequential run order is the table order, so backend warm state evolves. The source/invocation hashes and git state are recorded separately for every group.

## Full dispersed preparation

Real HTTP visited all230,000 IDs in900.503seconds, separately from measurement, with zero HTTP failures. At preparation end129,839 keys remained,44,485 expiries and zero evictions were recorded. The measured prefix had zero hits: hot denotes completed preparation, not guaranteed residence. TTL expiry and failed fills both matter.

[Prewarm audit](formal/B-hot-dispersed/prewarm-analysis.json) records197,549 misses and32,451 GET fallbacks. Retained keys plus expiries leave55,676 visits unretained. A server-counter interval covering229,936 visits observed58,017 cache errors and32,451 fallbacks, implying25,566 SET-error increments within that interval. Errors may occur after a write persisted, so error counts cannot be equated exactly to missing keys. Original warm receipts did not retain per-request error headers; whole-phase exact SET errors are unavailable, and first/last partial counter intervals are excluded. No capacity/TTL/timeout tuning or Redis preload shortcut was used.

## Evidence and validation

All six group directories linked above contain gzip raw HTTP receipts and pool, window/readiness, per-user prefixes, telemetry, preparation/cache state, unchanged-count checks, Locust CSV/history/HTML, and idle/load resource, storage and environment artifacts. [Restore counts](formal/preparation/check-seed.txt), [preparation timing](formal/preparation/preparation.json), and [restore log](formal/restore-A.log) establish the actual full fixture. API/Mongo/Redis logs are compressed at the formal root. [Cleanup](formal/cleanup.log) records removal of the dedicated project and volumes before exclusive machine release.

The B/cold hotspot [original rejected summary](formal/B-cold-hotspot/summary-original-rejected.json) is preserved. Commit9cea323 corrected an overstrict summary-only criterion: successful fallback remains a valid business measurement, while `cacheHealthy=false` reports errors. No request behavior or production configuration changed. Artifact red/green tests verify valid fallback versus HTTP failure or missing telemetry, literal percentiles and late-window cutoff. Real OOM/stopped-Redis/recovery header testing and three short50-user smoke runs are linked in the parent README.

The reusable C runner is available for #24; these results claim only A/B. Software/image versions, quotas, persistence configuration and dirty/source state are retained by the shared #17 recorder. Later integration documentation changes do not replace recorded measurement sources.
