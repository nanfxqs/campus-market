# V2 paired detail reads (#23)

The reusable runner, real fault/smoke validation and six full A/B measurements are delivered. See [formal results and evidence index](formal-results.md). All six meet the measurement protocol; Redis failures and regressions are measured outcomes.

`run-detail-load.py` invokes pinned Locust 2.46.6 using the shared frozen `experiments` project supplied by #16. `--locust` can select an explicitly installed executable; its version is checked. Runtime is 50 serial users with zero think time and an actual all-ready health barrier. Formal mode requires exactly 60 seconds and the untouched full V2 dataset.

```sh
python3 scripts/run-detail-load.py --project campus-v2-detail-example --database campus_v2_experiment --api http://127.0.0.1:8082 --configuration B --state cold --pattern dispersed --formal --confirm-isolated-cache --output artifacts/details/B-cold-dispersed
```

Run A/disabled, B/cold and B/hot for each of hotspot and dispersed, using the same restored dataset and seed (default42). A hotspot100 reads the first100 entries of a deterministic shuffled sorted complete product ID pool. Dispersed uses all230,000 archives, including180,000 historical sold products. Every VU `i` reads `pool[(i + 50 * sequenceIndex) % poolSize]` serially. Configurations share exactly this potential sequence; elapsed-time experiments consume different prefix lengths when throughput differs. Every raw request records VU, sequence index, pool position and product ID; summaries preserve each consumed prefix. The pool and its hash are saved, so paired runs can be checked without assuming equal completion counts.

The runner requires an explicit isolated-cache confirmation and exact Docker project ownership, and verifies running/healthy application instances' actual DB/cache configuration. It resets that dedicated Redis DB for each run and confirms `DBSIZE=0`; readiness probes only `/health`, so cold preparation does not read products. Hot preparation reads every involved ID once with50 workers, saves raw HTTP outcomes and elapsed preparation time, and retains actual Redis memory/key count/evictions/configuration. This can exceed128MiB and does not imply all items remain cached. Full-pool warming time is separate from the60-second window.

After preparation, the same #17 recorder captures30seconds of idle by default and the real detail interval, using client wrapper and descendant PIDs. Window coverage includes start/deadline boundaries; two seconds after client exit are retained to cover the last boundary. Exact query count and returned BSON bytes come from successful detail response headers, including product projection, seller, eligibility and recent history queries. These bytes are returned BSON sizes, not disk reads or network traffic. Cache hits/misses/fallbacks and per-request errors, transport/business errors, median/P95/P99 (nearest rank), throughput and late receipts are computed from raw gzip JSONL. Enabled B/C per-request Redis errors come from the tested `X-Detail-Cache-Errors` header; disabled A records zero because its cache is disabled.

Outputs include initial/final business counts, gzip command evidence and fixed pool, prewarm receipts, Locust CSV/history/HTML/log, request receipts, preparation timing, window readiness, per-request telemetry summaries, Redis state, idle resources and load resources/storage/environment. `experiments/summarize_details.py` and #17 `--summarize --window` recompute aggregates. Detail reads are independently checked not to change product/sold/transaction counts. No cache speedup or zero Redis errors is a passing condition. Successful business responses with fallback remain valid measurements; HTTP/business failures or missing telemetry invalidate the run. `cacheHealthy` reports Redis errors separately.

The C configuration is accepted by the same runner for #24's supplementary hotspot comparison. Use a new output directory for every attempt; retain failed runs. Global machine resource comparisons require the coordinator's exclusive window, including full warming. Do not run this command alongside another formal experiment.

Preparation checks: CLI rejects invalid A/cold without invoking Docker; the missing-output artifact tracer bullet failed before implementation. `tools/details/verify_summary.py` checks a clearly synthetic mathematical fixture at the summary CLI boundary: three on-time latencies10/20/30 and one late receipt produce median20/P95/P9930,13 queries,260 BSON bytes and2/3 hit rate. This is calculation verification, not actual HTTP evidence. `tools/details/verify.py` will check the actual 50-user HTTP/resource artifacts after actual runs.

## Actual engineering smoke

Fresh isolated demo project `campus-v2-detail-smoke23`, DB `campus_v2_detail_smoke`, port8087. The actual OOM/stopped-Redis metric red/green validation is recorded in [cache-error-metrics.md](cache-error-metrics.md). Then A/disabled hotspot, B/cold hotspot and B/hot dispersed each used50 actual ready Locust users for3seconds, plus3seconds idle recording and the shared resource/storage method. The small fixture has two independently HTTP-published archives; these runs validate protocol, sequence cycling, response correctness, live per-request telemetry and interval coverage, and do not substitute for100-hotspot/230,000-dispersed formal runs. A used the same compiled binary/image retagged to its Compose image name, with actual cache configuration disabled.

| Run | HTTP receipts within window | Errors | Missing telemetry | Cache error count | Formal passed |
|---|---:|---:|---:|---:|---|
| [smoke-A-disabled](smoke-A-disabled/summary.json) | 3439 | 0 | 0 | 0 | false |
| [smoke-B-cold](smoke-B-cold/summary.json) | 3429 | 0 | 0 | 0 | false |
| [smoke-B-hot](smoke-B-hot/summary.json) | 3160 | 0 | 0 | 0 | false |

All three artifact seam checks passed:50 user prefixes, valid actual HTTP details, no telemetry omissions, deterministic pool positions, unchanged business counts, and resource samples covering both window boundaries. B/hot prewarmed every involved archive and retained actual Redis capacity/configuration. The complete dedicated smoke project and volumes were cleaned after the final run; raw setup/build/runtime/failed-red/green/cleanup evidence is retained. The six full-scale60-second results are recorded separately in the formal results report.
