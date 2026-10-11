# Eight fixed functions and expiry

`scripts/measure-functions.py` uses one sequential client against existing HTTP APIs and a disposable full V2 seed (10,000 users, 230,000 products, 300,000 transaction records, 575,000 price changes). Initial independent MongoDB counts reject a partial/previously mutated dataset. `--samples 100` is formal; other counts are explicitly smoke runs. Every feature has one excluded HTTP warmup, followed by 100 measured attempts. End-to-end elapsed time spans request transmission through full response body receipt; validation and JSON decoding follow timing. Failed attempts remain in latency distribution and error rate. Median uses the conventional central value/average; P95/P99 use nearest rank `ceil(p*n)`.

Fixed cases: publish textbook with exact12345-cent price; bicycles browse/top20 cursor; synonym Search 单车 with independently verified current active bicycle count, twenty hits/highlight fields; initial seed product000000000000000000000001 detail/history; buyer seller home with independently verified eligible count; seller confirms independent new products for buyer at10001cents; automatic delist rejects already expired products with409; bicycle condition statistics exactly equal independently verified active count across four conditions. Publishing/sale warmups use separate new products and distinct keys. Sale fixture creation occurs outside its timed request.

Ten extra textbook products are published and each receives one price change. After actual Search ingestion, the agreed external fixture moves archive/listing expiration to a fixed near-future timestamp. At expiry, existing HTTP detail, browse, Search, seller home and category statistics exclude them; all ten sale attempts reject, archives and history remain. Separately, real TTL deletion is polled every2seconds for at most180seconds. Per-product disappearance is reported as an interval between last present and first absent observation, not an exact deletion timestamp or TTL SLA. No scanner or production test endpoint is added. Search ingestion, expiry wait, TTL wait, login and fixture construction occur outside measured latency samples.

Preparation:

```sh
COMPOSE_PROJECT_NAME=campus-v2-functions22 DB_NAME=campus_v2_functions22 \
 BASE_TIME=2026-10-11T00:00:00Z RESULT_DIR=/tmp/functions22-preparation API_PORT=8085 \
 scripts/restore-v2.sh A --reset
# Only after the machine is reserved for these measurements:
python3 scripts/measure-functions.py --project campus-v2-functions22 \
 --database campus_v2_functions22 --url http://127.0.0.1:8085 \
 --output /tmp/functions22-formal --samples 100
```

Unique output directories refuse accidental overwrite. Full artifacts include parameters and initial/final scales, every measured/warmup/preparation request, summaries, errors, fixed inputs, TTL polls and retention, Git/machine/Docker/images and resolved budgets. Tokens are excluded. Stop only this project's services/volumes with all profiles after evidence is copied.

Index/structure dependencies: publication writes product/listing in a MongoDB transaction; browse uses `category_browse` ordered category/sold/publication/id plus listing eligibility; Search uses `products_v1` smartcn fields and `synonyms_v1`, current MongoDB eligibility and selected-page highlights; detail uses product/user/listing primary keys and `priceChanges` product/time ordering; seller home uses `seller_browse` and facet for consistent list/count; sale uses seller-scoped idempotency transaction ID and conditional product/listing updates with user count in the same transaction; automatic delist relies on exact eligibility timestamp checks and listing `expiresAt` TTL index; statistics groups eligible category products by four condition values. Full model/index definitions remain in source and ADRs.
