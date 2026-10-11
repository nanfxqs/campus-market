# B/C formal measurements (#24)

All six independent sale rounds passed: 50 ready users, a configured 600-second cutoff, and 30,000 unique new dedicated-product sales per round. The pool exhausted early, so throughput uses each actual measured window. Cold and hot C hotspot reads also passed their exact 60-second protocol. No failed or tuning formal run was discarded.

## Sale rounds

| Configuration / round | Sales | Window s | Sales/s | Success P95 / P99 ms | Errors / retries | DB / resource coverage |
|---|---:|---:|---:|---:|---:|---|
| B1 | 30,000 | 27.028 | 1109.95 | 67.50 / 76.77 | 0 / 0 | pass / pass |
| B2 | 30,000 | 26.998 | 1111.18 | 67.39 / 77.14 | 0 / 0 | pass / pass |
| B3 | 30,000 | 26.985 | 1111.74 | 67.37 / 78.42 | 0 / 0 | pass / pass |
| C1 | 30,000 | 23.703 | 1265.69 | 47.81 / 72.62 | 0 / 0 | pass / pass |
| C2 | 30,000 | 23.666 | 1267.64 | 49.08 / 68.48 | 0 / 0 | pass / pass |
| C3 | 30,000 | 23.704 | 1265.59 | 47.24 / 67.79 | 0 / 0 | pass / pass |

Every round has exactly 30,000 newly successful transaction records, 30,000 newly sold archives, and 30,000 removed listings. Every successful product has one matching transaction, sale buyer/price/archive association, and no listing. All 50 involved sellers gained exactly 600 sales; other seller counters did not change. Conflicts, infrastructure/transport/other errors, retries, historical/duplicate success responses, late/timeout commits, unknown and unresolved products were zero. Empty conflict/failure latency classes are null, not zero-duration successes. Complete ID-level facts remain in each sale summary and before/after snapshots.

The full fixture was restored before all six rounds, cache cleared, and login/Search preparation excluded from timing. Seed42 and fixed base `2026-10-11T00:00:00Z` match A. All eight persisted collection fingerprints match every other round and A reference SHA256 `8b08050cc049ae34e7f6144831ebbe8f81cace322659e445e138707eaa4088af`. The untouched restore before C reads also matches. Full counts are 10,000 users, 230,000 products (180,000 sold), 50,000 listings, 300,000 transactions and 575,000 price changes.

[A baseline](../v2/baseline.md) measured 44.283/44.437/44.430-second windows (675–677 sales/s). These later B/C differences describe measured configurations and execution order. The sale path does not use material-detail cache, so these timings establish no cache speedup causality.

## Budgets and measured resources

B is one direct Gin instance; C uses Nginx and two Gin instances. Each app retains exactly 2 CPU / 512MiB: B total 2 CPU / 512MiB, C total 4 CPU / 1GiB. C additionally reserves Nginx 1 CPU / 128MiB. Both configurations retain shared MongoDB/Search 4 CPU / 8GiB and Redis 1 CPU / 256MiB, with maxmemory128MiB/allkeys-lru. Redis TTL600 seconds and timeout100ms were unchanged. Resolved Compose plus runtime container/cgroup limits, versions, mounts, machine, client and source provenance are authoritative.

| Round | Samples | App CPU mean % | App RSS peak MiB | Mongo CPU mean % | Search RSS peak MiB | Nginx CPU mean % | Client CPU mean % |
|---|---:|---:|---:|---:|---:|---:|---:|
| B1 | 27 | 197.6 | 33.7 | 257.1 | 1699.4 | absent | 71.1 |
| B2 | 27 | 197.7 | 34.1 | 259.7 | 1704.0 | absent | 70.5 |
| B3 | 27 | 197.1 | 49.3 | 259.7 | 1811.4 | absent | 71.3 |
| C1 | 24 | 288.2 | 55.9 | 306.4 | 1782.0 | 15.2 | 90.4 |
| C2 | 24 | 289.7 | 54.2 | 309.2 | 1863.5 | 15.3 | 90.6 |
| C3 | 24 | 284.6 | 50.3 | 302.8 | 2710.1 | 14.9 | 90.7 |

CPU100% means one host logical core. C application rows sum both Gin processes; per-process and per-container means/peaks remain separate in raw/window summaries. MongoDB and Search share one container budget and their RSS is not a double allocation. All rounds retain 1Hz raw process/container/host/client/collector samples, cgroup counters, collector cost and coverage, idle baselines, and full before/after storage including Search paths/indexes, Mongo collections/indexes and volumes, Redis runtime/persistence state. JVM/index/host warm state varies across ordered rounds despite identical logical data.

## Comparable C hotspot reads

All five matched cases below use the same100-ID pool hash `4180065e92b77e9da98e8f88e287f6927a2b754943676b234bcc31967932f384`, seed42, deterministic per-VU sequence,50users,60seconds and30seconds idle. Cold starts empty; hot separately flushes then HTTP-prewarms100 IDs outside measurement. C prewarm completed100/100 without HTTP errors. Full product/sold/transaction counts remain unchanged. Actual consumed prefixes and late receipts are retained; throughput differences mean prefixes have different lengths.

| Case | Responses | RPS | Median / P95 / P99 ms | Hit % | Cache errors | Queries/read | BSON bytes/read |
|---|---:|---:|---:|---:|---:|---:|---:|
| A-hotspot | 62383 | 1039.72 | 32.51 / 47.17 / 55.29 | disabled | 0 | 4.000 | 1504.8 |
| B-cold-hotspot | 28517 | 475.28 | 53.35 / 199.93 / 227.11 | 94.43 | 1487 | 4.056 | 986.5 |
| B-hot-hotspot | 28512 | 475.20 | 54.78 / 199.25 / 220.53 | 94.52 | 1562 | 4.055 | 986.0 |
| C-cold | 57640 | 960.67 | 22.74 / 112.96 / 298.63 | 97.58 | 1297 | 4.024 | 959.6 |
| C-hot | 57721 | 962.02 | 24.06 / 91.61 / 292.68 | 97.81 | 1266 | 4.022 | 957.5 |

[A/B paired evidence](../v2-details23/formal-results.md) supplies the original comparison cases. C recorded natural cache fallbacks/errors1297 cold and1266 hot; HTTP errors and telemetry omissions were zero. CacheHealthy=false honestly records this while valid business fallback passes. Mongo query/BSON counters cover all logical detail lookups and BSON returned to the app; they do not measure disk I/O, Search or server scan bytes. C throughput approaches A hotspot here and exceeds the earlier single-app B cases, while C P95/P99 exceed A. Neither cache health nor latency is an extra pass gate.

These are sequential experiments on one recorded physical host. C doubles total application quota and adds a proxy; results do not prove equal-total-resource efficiency, a Nginx-only effect, or cross-machine scale. Shared Mongo/Search and changing backend/OS state preclude a controlled decomposition of the difference.

## Evidence, provenance and limitations

[B complete summary](formal-B/summary.json), [C complete summary](formal-C/summary.json), [round comparison](round-comparison.json), [resource comparison](resource-comparison.json), and [hotspot comparison](hotspot-comparison.json) link every independent result. Round directories retain restore/import/manifest checks, original gzip manifest, all collection fingerprints, full client receipts/windows/stats/HTML, external before/after DB facts, per-seller deltas, readiness/login preparation, and idle/load resources/environment/storage. C [cold](hotspot/C-cold/summary.json) and [hot](hotspot/C-hot/summary.json) preserve identical detail artifacts and pool/sequence/cache counters. All measured code used commit1ed9b63 atop f17c1e8; source hashes and worktree states remain in invocation/environment artifacts. Later auxiliary-capture code did not alter these measurements.

Application/Mongo/Redis/Nginx service stdout was not exported before project cleanup in these runs or the reused A sale runner. It is unavailable, and is not claimed as captured. Required client, DB reconciliation, server resource/storage and HTTP telemetry evidence is retained. A follow-up shared runner captures owned service stdout/stderr gzip before future cleanup with explicit status/failures; the [post-cleanup unavailable capture](post-cleanup-capture-unavailable/status.json) correctly fails rather than inventing successful capture. No passed run was repeated solely for auxiliary stdout.

[Cleanup](cleanup.json) verifies zero owned containers, networks and volumes for both projects before exclusive release. Formal output is unchanged by reporting; no failed experiment was omitted.

## Artifact validation

Public CLI red/green: C summary fixture initially failed with KeyError(resourceBudgets), then passed after explicit per-instance/total app and separate proxy budgets were reported. The existing failed-round test also passed, preserving independent thresholds. `python3 -m unittest experiments/test_formal_matrix.py`:2passed. Bytecode compilation checked the three changed orchestration scripts. No production API, timeout, TTL, resource or routing behavior changed.
