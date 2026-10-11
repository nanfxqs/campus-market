# C: real dual-instance validation

C uses Nginx 1.28.0-alpine (digest in `compose.v2.yaml`) and default round robin without sticky sessions. `X-Upstream-Addr` and Nginx access logs expose the selected instance on existing endpoints. Proxy retries are disabled so a proxy cannot silently replay writes. Shared MongoDB persists sessions and transaction idempotency; Redis caches only versioned product material.

Per Gin: 2 CPU / 512 MiB, identical to B; C totals 4 CPU / 1 GiB application allocation. Nginx separately receives 1 CPU / 128 MiB. MongoDB/Search (4 CPU / 8 GiB) and Redis (1 CPU / 256 MiB, 128 MiB maxmemory) remain unchanged. These are allocation limits, not utilization measurements or claims of speedup. Follow-up resource collection must include `api-c1`, `api-c2`, `nginx` alongside shared stores.

Reproduce with a fresh dedicated project (only the named experiment database is reset):

```sh
export API_PORT=8083 DB_NAME=campus_v2_experiment
DC='docker compose -p campus-v2-lb21 -f compose.v2.yaml'
$DC up -d --wait mongo redis
$DC build api-c1 api-c2 tools
$DC run --rm --no-deps api-c1 seed --mode full --random-seed 42 --base-time 2026-10-11T00:00:00Z
$DC --profile C up -d --wait
python3 scripts/validate-load-balancer.py --project campus-v2-lb21 --database "$DB_NAME" --url http://127.0.0.1:8083
$DC run --rm --no-deps -e V1_API_URL=http://nginx:8080 -e V1_DB_NAME="$DB_NAME" tools test -v ./internal/market -run '^TestV1FullSeedHTTP$' -count=1 -timeout=8m
$DC logs --no-color nginx
$DC down --volumes
```

`red-services.log` records C missing before implementation; `green-services.log` records the deployed service frontier. `compose-resolved.yaml`, `startup.log`, `seed.log`, `health.log`, `cross-instance.jsonl` and `nginx-distribution.log` preserve actual evidence, including failed starts. The initial unsynchronized application startup raced Search index creation; C now waits for api-c1 health before starting api-c2. A seed import requires an empty database; failed nonempty import is retained and the explicit dedicated database reset used in this run is recorded.

Cross-instance validation passed: token issued on one upstream accepted on the other; same-key same-content responses identical across upstreams; different content rejected; 20 simultaneous different-key attempts reached both instances with one success and 19 business conflicts. External read-only MongoDB evidence proves one successful transaction, sold product, removed eligibility and seller count increased by exactly two across the two test products. This is functionality evidence, not the separate 100×50 competition or formal 50-user performance acceptance.

The full-seed C regression initially failed twice at Search's existing 10-second request deadline; both outputs remain. Diagnosis/probe evidence is retained. Generating highlights for all 66,667 historical and active matches cost 15,942 ms; highlighted selected-page extraction cost 38 ms. Search now scans IDs without highlights, filters current MongoDB archive state, checks all remaining candidates against listings and current archives for exact count, then obtains Search highlight metadata only for the selected twenty while preserving their original rank. No timeout or expected count was weakened. Exact original HTTP query passed twice at 2.215/1.869 seconds. `http-regression-c-fixed.log` passes full-seed regression in 4.39 seconds, including total 6667/top20 highlighted matches and expired product removal from browse, Search and statistics, expired price/sale rejection and retained archive/history.

`proxy-core.jsonl` separately passes real C authentication, ownership, input validation, cached detail, edit/current price/profile/history, browse, seller home, statistics, actual Search ingestion and current eligibility, and expiry rejection. This run did not claim TTL physical cleanup, formal performance or cross-machine replication; those are separate evidence tasks.
