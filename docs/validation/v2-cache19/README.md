# Redis detail version cache (#19)

Run a dedicated project (never share its Redis with other databases):

```sh
docker compose -p campus-v2-cache19 -f compose.v2.yaml up -d --wait mongo redis
docker compose -p campus-v2-cache19 -f compose.v2.yaml --profile tools run --rm -e GIN_MODE=release -e CACHE_TEST_ENABLED=true tools test -v ./internal/market -count=1 -timeout=12m
```

Configuration A uses `--profile A up -d api-a`; B uses `--profile B up -d api-b`. Both direct HTTP port 8082, same 2 CPU / 512 MiB per-app budget; Mongo/Search shares 4 CPU / 8 GiB container budget, Redis 1 CPU / 256 MiB container budget. Environment variables in Compose override all limits. Redis has no persistence volume, defaults to 128 MiB maximum memory and allkeys-lru. Cache TTL defaults to 10m and each Redis operation is bounded by 100ms. The image is fixed by version and digest. Container limits are budgets, not measured use.

Only categoryId/title/description/images/attributes are cached. Condition, prices, sold/sale facts, timestamps, seller, eligibility and latest five price changes remain MongoDB reads. `contentVersion` is internal (not JSON): new HTTP products and V2 seed products initialize at 1; missing legacy versions mean 0; atomic `$inc` with each content update promotes legacy documents to 1. Old versions expire independently. A missing matching version retries the current dynamic read twice then falls back to a full archive read; an old read can only populate its old version key. No cross-collection snapshot is promised.

Successful detail responses include `X-Detail-Cache` (`disabled`, `miss`, `hit`, `fallback`), `X-Mongo-Queries`, `X-Mongo-BSON-Bytes`. Queries count logical product, current-seller, eligibility and recent-history reads, including retries/fallback, excluding authentication (details are public), connection/health operations and server-side internal work. Bytes sum actual BSON document lengths returned by those reads, including projected products and raw seller/history documents; they exclude wire framing, request bytes, indexes, server disk IO and empty-result overhead. These are transferred logical result bytes, not a claim about physical storage IO. Warm hits still use four queries; cold misses normally use five. Cumulative per-instance `detail_metrics` log snapshots are emitted at most once per second on requests; sum response headers over experiment requests for exact sample totals. Redis errors and fallback counters are in snapshots; failed SET increments errors without changing HTTP success. Process restarts reset counters.

The HTTP tests verify current content after edits, current seller, concurrent readers overlapping committed edits, TTL and FLUSHDB rebuild, immediate expired eligibility using the agreed external fixture, unreachable Redis bounded fallback, and cold/hot query telemetry. Existing HTTP regression tests can enable B via `CACHE_TEST_ENABLED=true`. Dedicated restore scripts stop applications and flush the experiment Redis before importing and validating data; actual full restore is recorded jointly with #15.

Recorded 2026-10-11 on the implementation worktree:

- `http-cache.log`: dedicated real-service HTTP tests pass; cold 5 queries / 860 returned BSON bytes, hot 4 / 486 for the fixed sample (not a benchmark).
- `http-regression-b.log`: entire existing internal/market suite with cache enabled passes in 50.851s; full-seed deployment test explicitly delegates to the existing deployment script and is skipped here.
- `faults.jsonl` and `api-faults.log`: actual eviction confirmed by Redis `evicted_keys`, real OOM SET errors, paused old cache writes, stopped/restarted Redis and edits/price/seller/history/sale while stopped pass. Repeat using a freshly seeded isolated B project and `python3 scripts/validate-cache-faults.py --project campus-v2-cache19 --url http://127.0.0.1:8093`. The script mutates only its published sample and named Redis experiment service, restores normal Redis limits in `finally`, and leaves the sample sold. Do not run during performance sampling.

No performance improvement is claimed. The minimal RESP adapter opens one bounded TCP connection per operation; its connection overhead is included in HTTP measurements. Snapshot logs are cumulative across request concurrency, so the fields can describe slightly different instants; response-header totals are the exact completed-request measurement seam. Old-reader timing is exercised with Redis WRITE pause and concurrent HTTP requests; no production hook is added to control internal scheduling.
