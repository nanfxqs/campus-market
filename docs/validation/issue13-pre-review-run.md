# V1 clean-environment acceptance

UTC: 2026-10-10T07:55:35Z

Project: `campus-v1-20261010075535-1289890-13302`; base time: `2026-10-10T00:00:00Z`; random seed: 42; mode: full.

```text

$ git rev-parse HEAD
5773665c9f567c36b8a55b0e6de0d4d0a840d1fc

$ uname -a
Linux thinkbook-omarchy 7.2.8-5-omarchy-bore #1 SMP PREEMPT_DYNAMIC Wed, 07 Oct 2026 02:17:41 +0000 x86_64 GNU/Linux

$ docker version
Client:
 Version:           29.9.0
 API version:       1.56
 Go version:        go1.27.1-X:nodwarf5
 Git commit:        f415da838b
 Built:             Fri Oct  9 00:19:20 2026
 OS/Arch:           linux/amd64
 Context:           default

Server:
 Engine:
  Version:          29.9.0
  API version:      1.56 (minimum version 1.40)
  Go version:       go1.27.1-X:nodwarf5
  Git commit:       a5b58c94d3
  Built:            Fri Oct  9 00:19:20 2026
  OS/Arch:          linux/amd64
  Experimental:     false
 containerd:
  Version:          v2.4.1
  GitCommit:        f2551031d7276a770f65f98c9b52e57e7dad07e8.m
 runc:
  Version:          1.5.2
  GitCommit:
 docker-init:
  Version:          0.19.0
  GitCommit:        de40ad0

$ docker compose version
Docker Compose version 5.6.0

$ docker info --format CPUs=\{\{.NCPU\}\}\ Memory=\{\{.MemTotal\}\}\ OS=\{\{.OperatingSystem\}\}\ Architecture=\{\{.Architecture\}\}
CPUs=18 Memory=33225965568 OS=Omarchy Architecture=x86_64

$ df -h .
Filesystem                Size  Used Avail Use% Mounted on
/dev/mapper/omarchy_root  352G  130G  222G  37% /home

$ docker compose -p campus-v1-20261010075535-1289890-13302 -f compose.v1.yaml up -d --build --wait mongo
 Volume campus-v1-20261010075535-1289890-13302_search-data Creating
 Network campus-v1-20261010075535-1289890-13302_default Creating
 Volume campus-v1-20261010075535-1289890-13302_search-data Creating
 Volume campus-v1-20261010075535-1289890-13302_mongo-data Creating
 Network campus-v1-20261010075535-1289890-13302_default Creating
 Volume campus-v1-20261010075535-1289890-13302_mongo-data Creating
 Volume campus-v1-20261010075535-1289890-13302_mongo-config Creating
 Volume campus-v1-20261010075535-1289890-13302_mongo-config Creating
 Volume campus-v1-20261010075535-1289890-13302_search-data Created
 Volume campus-v1-20261010075535-1289890-13302_search-data Created
 Volume campus-v1-20261010075535-1289890-13302_mongo-data Created
 Volume campus-v1-20261010075535-1289890-13302_mongo-data Created
 Volume campus-v1-20261010075535-1289890-13302_mongo-config Created
 Volume campus-v1-20261010075535-1289890-13302_mongo-config Created
 Network campus-v1-20261010075535-1289890-13302_default Created
 Network campus-v1-20261010075535-1289890-13302_default Created
 Container campus-v1-20261010075535-1289890-13302-mongo-1 Creating
 Container campus-v1-20261010075535-1289890-13302-mongo-1 Created
 Container campus-v1-20261010075535-1289890-13302-mongo-1 Starting
 Container campus-v1-20261010075535-1289890-13302-mongo-1 Started
 Container campus-v1-20261010075535-1289890-13302-mongo-1 Waiting
 Container campus-v1-20261010075535-1289890-13302-mongo-1 Healthy

$ docker compose -p campus-v1-20261010075535-1289890-13302 -f compose.v1.yaml build market api verify
 Image campus-v1-20261010075535-1289890-13302-market Building
 Image campus-v1-20261010075535-1289890-13302-api Building
 Image campus-v1-20261010075535-1289890-13302-verify Building
#1 [internal] load local bake definitions
#1 reading from stdin 1.58kB done
#1 DONE 0.0s

#2 [verify internal] load build definition from Dockerfile
#2 transferring dockerfile: 491B done
#2 DONE 0.0s

#3 [market internal] load metadata for docker.io/library/debian:bookworm-slim@sha256:7c7b2c966bc9ee8cedfeef67e0e279108992c77681fa595db4a9d65c06ccc587
#3 DONE 0.0s

#4 [verify internal] load metadata for docker.io/library/golang:1.24.2-bookworm@sha256:79390b5e5af9ee6e7b1173ee3eac7fadf6751a545297672916b59bfa0ecf6f71
#4 DONE 0.0s

#5 [api internal] load .dockerignore
#5 transferring context: 62B done
#5 DONE 0.0s

#6 [api tools 1/7] FROM docker.io/library/golang:1.24.2-bookworm@sha256:79390b5e5af9ee6e7b1173ee3eac7fadf6751a545297672916b59bfa0ecf6f71
#6 resolve docker.io/library/golang:1.24.2-bookworm@sha256:79390b5e5af9ee6e7b1173ee3eac7fadf6751a545297672916b59bfa0ecf6f71 0.0s done
#6 DONE 0.0s

#7 [api stage-1 1/3] FROM docker.io/library/debian:bookworm-slim@sha256:7c7b2c966bc9ee8cedfeef67e0e279108992c77681fa595db4a9d65c06ccc587
#7 resolve docker.io/library/debian:bookworm-slim@sha256:7c7b2c966bc9ee8cedfeef67e0e279108992c77681fa595db4a9d65c06ccc587 0.0s done
#7 DONE 0.0s

#8 [verify internal] load build context
#8 transferring context: 905.84kB 0.1s done
#8 DONE 0.1s

#9 [verify tools 2/7] WORKDIR /src
#9 CACHED

#10 [verify tools 3/7] COPY go.mod go.sum ./
#10 CACHED

#11 [verify tools 4/7] RUN go mod download
#11 CACHED

#12 [verify tools 5/7] COPY . .
#12 DONE 0.4s

#13 [api tools 6/7] RUN go build -o /validation ./validation
#13 DONE 12.9s

#14 [verify tools 7/7] RUN go build -o /market ./cmd/market
#14 DONE 1.3s

#15 [verify] exporting to image
#15 exporting layers
#15 ...

#16 [api stage-1 2/3] COPY --from=tools /validation /validation
#16 CACHED

#17 [api stage-1 3/3] COPY --from=tools /market /market
#17 CACHED

#16 [market stage-1 2/3] COPY --from=tools /validation /validation
#16 CACHED

#17 [market stage-1 3/3] COPY --from=tools /market /market
#17 CACHED

#18 [market] exporting to image
#18 exporting layers done
#18 exporting manifest sha256:01dc4ebb334d18c642ff947b52ec8cd4df2dec30394998c5d950a9dcd96018c4 0.0s done
#18 exporting config sha256:94504c9e6bdd54a4716b5cca0161f71effb31e9af4221bf2aff96caa965feb20 0.0s done
#18 exporting attestation manifest sha256:85a198d23b15984f1aebe676c5824596682011fb603eaa8dff842c458465d78b 0.0s done
#18 exporting manifest list sha256:6eb5bc30813db7a0f955c03212dc91d12a77a69b3f3e758d2d6c74726542e813 0.0s done
#18 naming to docker.io/library/campus-v1-20261010075535-1289890-13302-market:latest done
#18 unpacking to docker.io/library/campus-v1-20261010075535-1289890-13302-market:latest done
#18 DONE 0.1s

#19 [api] exporting to image
#19 exporting layers done
#19 exporting manifest sha256:735610b7cc67256db132c3f23da522a646bd29fcb460828f474ac730900fd91f 0.0s done
#19 exporting config sha256:2f6378f4702936c495bba96fa5036dd3c121987ca336f14238ff785118c404bc 0.0s done
#19 exporting attestation manifest sha256:08ad8661d2eb9ce894e49c3381c09e95c3084e7b74b17ed7b863dd3c4e9d0ca8 0.0s done
#19 exporting manifest list sha256:c72100e390f374b322fcfbd3ca0aa7dbd082316e781d00fdb67bd6c4ce2f6094 0.0s done
#19 naming to docker.io/library/campus-v1-20261010075535-1289890-13302-api:latest done
#19 unpacking to docker.io/library/campus-v1-20261010075535-1289890-13302-api:latest done
#19 DONE 0.1s

#20 [market] resolving provenance for metadata file
#20 DONE 0.0s

#21 [api] resolving provenance for metadata file
#21 DONE 0.0s

#15 [verify] exporting to image
#15 exporting layers 4.0s done
#15 exporting manifest sha256:e906bac2414fec72df3bcd8b884fe174eae136e4b1af675268ce10e9494c7ae0 done
#15 exporting config sha256:59a0793eeefc290e7c8e0122b6a0a38b45bd6ce6ac7fa5fe792aba5ba878e195 0.0s done
#15 exporting attestation manifest sha256:8726d17b63ce76c54fe2305e0ec2099fec15d0a744cec38e5180d5358a592311 0.0s done
#15 exporting manifest list sha256:4d327b7938025de634e983614184672c47b3b5f073691c5f11102e2c3ff1ccdc done
#15 naming to docker.io/library/campus-v1-20261010075535-1289890-13302-verify:latest done
#15 unpacking to docker.io/library/campus-v1-20261010075535-1289890-13302-verify:latest
#15 unpacking to docker.io/library/campus-v1-20261010075535-1289890-13302-verify:latest 1.8s done
#15 DONE 5.9s

#22 [verify] resolving provenance for metadata file
#22 DONE 0.0s
 Image campus-v1-20261010075535-1289890-13302-market Built
 Image campus-v1-20261010075535-1289890-13302-api Built
 Image campus-v1-20261010075535-1289890-13302-verify Built

$ docker compose -p campus-v1-20261010075535-1289890-13302 -f compose.v1.yaml run --rm --no-deps market seed --mode full --random-seed 42 --base-time 2026-10-10T00:00:00Z
 Container campus-v1-20261010075535-1289890-13302-market-run-e22cfd3f5822 Creating
 Container campus-v1-20261010075535-1289890-13302-market-run-e22cfd3f5822 Created
{"baseTime":"2026-10-10T00:00:00Z","counts":{"users":10000,"active":20000,"sold":180000,"transactions":300000,"priceChanges":500000},"database":"campus_v1","mode":"full","randomSeed":42}
2026/10/10 07:56:27 demo accounts: seller, buyer; password: CampusDemo123!

$ docker compose -p campus-v1-20261010075535-1289890-13302 -f compose.v1.yaml exec -T mongo mongosh --quiet campus_v1 --eval $'// Run with mongosh against an explicitly selected seed database; read-only.\nconst metadata = db.seed_metadata.findOne({_id: "dataset"});\nif (!metadata) throw new Error("No completed seed dataset metadata");\nconst expected = metadata.counts;\nfunction check(name, actual, wanted) {\n  print(`${name}: ${actual} (expected ${wanted})`);\n  if (actual !== wanted) throw new Error(`${name} mismatch`);\n}\nfunction count(collection, pipeline) {\n  const result = db[collection].aggregate([...pipeline, {$count: "n"}],\n    {allowDiskUse: true}).toArray();\n  return result.length ? result[0].n : 0;\n}\ncheck("users", db.users.countDocuments({}), expected.users);\ncheck("unsold", db.products.countDocuments({sold: false}), expected.active);\ncheck("sold", db.products.countDocuments({sold: true}), expected.sold);\ncheck("listings", db.listings.countDocuments({}), expected.active);\ncheck("transactions", db.transactions.countDocuments({}), expected.transactions);\ncheck("priceChanges", db.priceChanges.countDocuments({}), expected.priceChanges);\ncheck("successes", db.transactions.countDocuments({success: true}), expected.sold);\ncheck("image violations", db.products.countDocuments({\n  $expr: {$or: [{$lt: [{$size: "$images"}, 1]}, {$gt: [{$size: "$images"}, 9]}]}\n}), 0);\ncheck("price chain violations", count("priceChanges", [\n  {$sort: {productId: 1, changedAt: 1, _id: 1}},\n  {$group: {_id: "$productId", history: {$push: {old: "$oldPriceCents", next: "$newPriceCents"}}}},\n  {$lookup: {from: "products", localField: "_id", foreignField: "_id", as: "product"}},\n  {$unwind: "$product"},\n  {$match: {$expr: {$or: [\n    {$ne: [{$arrayElemAt: ["$history.old", 0]}, "$product.initialPriceCents"]},\n    {$ne: [{$arrayElemAt: ["$history.next", -1]}, "$product.priceCents"]},\n    {$anyElementTrue: {$map: {\n      input: {$range: [1, {$size: "$history"}]}, as: "i",\n      in: {$ne: [\n        {$arrayElemAt: ["$history.old", "$$i"]},\n        {$arrayElemAt: ["$history.next", {$subtract: ["$$i", 1]}]}\n      ]}\n    }}}\n  ]}}}\n]), 0);\ncheck("sale record violations", count("transactions", [\n  {$match: {success: true}},\n  {$lookup: {from: "products", localField: "productId", foreignField: "_id", as: "product"}},\n  {$unwind: {path: "$product", preserveNullAndEmptyArrays: true}},\n  {$match: {$expr: {$or: [\n    {$ne: ["$product.sold", true]},\n    {$ne: ["$sellerId", "$product.sellerId"]},\n    {$ne: ["$buyerId", "$product.sale.buyerId"]},\n    {$ne: ["$priceCents", "$product.sale.priceCents"]},\n    {$ne: ["$confirmedAt", "$product.sale.confirmedAt"]}\n  ]}}}\n]), 0);\ncheck("seller count violations", count("products", [\n  {$match: {sold: true}},\n  {$group: {_id: "$sellerId", n: {$sum: 1}}},\n  {$lookup: {from: "users", localField: "_id", foreignField: "_id", as: "seller"}},\n  {$unwind: {path: "$seller", preserveNullAndEmptyArrays: true}},\n  {$match: {$expr: {$ne: ["$n", "$seller.completedSales"]}}}\n]), 0);\nprintjson(db.priceChanges.aggregate([\n  {$group: {_id: "$productId", n: {$sum: 1}}},\n  {$group: {_id: null, min: {$min: "$n"}, max: {$max: "$n"}, mean: {$avg: "$n"}}}\n]).toArray());\nprint("Seed observations passed. Listing counts are wall-clock sensitive; see seed documentation.");'
users: 10000 (expected 10000)
unsold: 20000 (expected 20000)
sold: 180000 (expected 180000)
listings: 20000 (expected 20000)
transactions: 300000 (expected 300000)
priceChanges: 500000 (expected 500000)
successes: 180000 (expected 180000)
image violations: 0 (expected 0)
price chain violations: 0 (expected 0)
sale record violations: 0 (expected 0)
seller count violations: 0 (expected 0)
[
  {
    _id: null,
    min: 2,
    max: 3,
    mean: 2.5
  }
]
Seed observations passed. Listing counts are wall-clock sensitive; see seed documentation.

$ docker compose -p campus-v1-20261010075535-1289890-13302 -f compose.v1.yaml up -d --wait market api
 Container campus-v1-20261010075535-1289890-13302-mongo-1 Running
 Container campus-v1-20261010075535-1289890-13302-market-1 Creating
 Container campus-v1-20261010075535-1289890-13302-api-1 Creating
 Container campus-v1-20261010075535-1289890-13302-api-1 Created
 Container campus-v1-20261010075535-1289890-13302-market-1 Created
 Container campus-v1-20261010075535-1289890-13302-mongo-1 Waiting
 Container campus-v1-20261010075535-1289890-13302-mongo-1 Waiting
 Container campus-v1-20261010075535-1289890-13302-mongo-1 Healthy
 Container campus-v1-20261010075535-1289890-13302-api-1 Starting
 Container campus-v1-20261010075535-1289890-13302-mongo-1 Healthy
 Container campus-v1-20261010075535-1289890-13302-market-1 Starting
 Container campus-v1-20261010075535-1289890-13302-api-1 Started
 Container campus-v1-20261010075535-1289890-13302-market-1 Started
 Container campus-v1-20261010075535-1289890-13302-api-1 Waiting
 Container campus-v1-20261010075535-1289890-13302-mongo-1 Waiting
 Container campus-v1-20261010075535-1289890-13302-market-1 Waiting
 Container campus-v1-20261010075535-1289890-13302-mongo-1 Healthy
 Container campus-v1-20261010075535-1289890-13302-api-1 Healthy
 Container campus-v1-20261010075535-1289890-13302-market-1 Healthy

$ docker compose -p campus-v1-20261010075535-1289890-13302 -f compose.v1.yaml run --rm --no-deps verify vet ./...
 Container campus-v1-20261010075535-1289890-13302-verify-run-7eacb57cda19 Creating
 Container campus-v1-20261010075535-1289890-13302-verify-run-7eacb57cda19 Created

$ docker compose -p campus-v1-20261010075535-1289890-13302 -f compose.v1.yaml run --rm --no-deps -e V1_API_URL=http://market:8080 verify test -v ./internal/market -run \^TestV1FullSeedHTTP\$ -count=1 -timeout=8m
 Container campus-v1-20261010075535-1289890-13302-verify-run-b2dffec0c3dd Creating
 Container campus-v1-20261010075535-1289890-13302-verify-run-b2dffec0c3dd Created
=== RUN   TestV1FullSeedHTTP
    v1_test.go:24: full seed: synonym search total=6667, top 20 with matched fields
    v1_test.go:68: full seed: publish, browse, detail/current profile, price history, seller home, sale/replay and ownership PASS
--- PASS: TestV1FullSeedHTTP (52.22s)
PASS
ok  	github.com/nanfxqs/campus-market/internal/market	52.235s

$ docker compose -p campus-v1-20261010075535-1289890-13302 -f compose.v1.yaml run --rm --no-deps api seed --large
 Container campus-v1-20261010075535-1289890-13302-api-run-3706a0d9db30 Creating
 Container campus-v1-20261010075535-1289890-13302-api-run-3706a0d9db30 Created
2026/10/10 07:57:59 fixture ready users=10000 active=20000 history=180000 transactions=300000 candidates=20040 search_ms=1816 base=2026-10-10T07:57:47.81698501Z

$ docker compose -p campus-v1-20261010075535-1289890-13302 -f compose.v1.yaml run --rm --no-deps -e GIN_MODE=release -e SCALE=1 verify test -v ./... -count=1 -timeout=15m
 Container campus-v1-20261010075535-1289890-13302-verify-run-b54e2f249ac0 Creating
 Container campus-v1-20261010075535-1289890-13302-verify-run-b54e2f249ac0 Created
?   	github.com/nanfxqs/campus-market/cmd/market	[no test files]
=== RUN   TestExpiryLifecycleHTTP
=== RUN   TestExpiryLifecycleHTTP/-1ms
    expiry_test.go:142: all entry points expiry=2026-10-10T07:59:02.731Z now=2026-10-10T07:59:02.73Z eligible=1
=== RUN   TestExpiryLifecycleHTTP/0s
2026/10/10 07:58:03 sale request rejected status=409 seller="seller" product="6ac9f00a1e93c39c573b4db3"
    expiry_test.go:142: all entry points expiry=2026-10-10T07:59:02.731Z now=2026-10-10T07:59:02.731Z eligible=0
=== RUN   TestExpiryLifecycleHTTP/1ms
2026/10/10 07:58:04 sale request rejected status=409 seller="seller" product="6ac9f00a1e93c39c573b4db3"
    expiry_test.go:142: all entry points expiry=2026-10-10T07:59:02.731Z now=2026-10-10T07:59:02.732Z eligible=0
--- PASS: TestExpiryLifecycleHTTP (1.73s)
    --- PASS: TestExpiryLifecycleHTTP/-1ms (0.03s)
    --- PASS: TestExpiryLifecycleHTTP/0s (0.04s)
    --- PASS: TestExpiryLifecycleHTTP/1ms (0.03s)
=== RUN   TestExpiryTTLRetainsHistoryHTTP
2026/10/10 07:58:06 sale request rejected status=409 seller="seller" product="6ac9f00c1e93c39c573b4dba"
2026/10/10 07:58:44 sale request rejected status=409 seller="seller" product="6ac9f00c1e93c39c573b4dba"
    expiry_test.go:201: TTL database evidence db=campus_expiry_6ac9f00c1e93c39c573b4db9 listing=6ac9f00c1e93c39c573b4dba count=0 expiresAt=2026-10-10T07:58:06.61Z observed=2026-10-10T07:58:44.772448531Z cleanup_delay=38.162449058s; archive, images, price history readable; sale and price changes rejected
--- PASS: TestExpiryTTLRetainsHistoryHTTP (40.76s)
=== RUN   TestSaleExactExpirationHTTP
=== RUN   TestSaleExactExpirationHTTP/-1ms
    sales_expiry_test.go:71: expiry=2026-10-10T07:58:45.192Z confirmation=2026-10-10T07:58:45.191Z code=sold
=== RUN   TestSaleExactExpirationHTTP/0s
2026/10/10 07:58:45 sale request rejected status=409 seller="seller" product="6ac9f0351e93c39c573b4dbe"
    sales_expiry_test.go:71: expiry=2026-10-10T07:58:45.192Z confirmation=2026-10-10T07:58:45.192Z code=product_expired
=== RUN   TestSaleExactExpirationHTTP/1ms
2026/10/10 07:58:45 sale request rejected status=409 seller="seller" product="6ac9f0351e93c39c573b4dbf"
    sales_expiry_test.go:71: expiry=2026-10-10T07:58:45.192Z confirmation=2026-10-10T07:58:45.193Z code=product_expired
--- PASS: TestSaleExactExpirationHTTP (0.75s)
    --- PASS: TestSaleExactExpirationHTTP/-1ms (0.12s)
    --- PASS: TestSaleExactExpirationHTTP/0s (0.10s)
    --- PASS: TestSaleExactExpirationHTTP/1ms (0.11s)
=== RUN   TestSaleTransactionRetryAcrossExpirationHTTP
2026/10/10 07:58:46 sale request rejected status=409 seller="seller" product="6ac9f0351e93c39c573b4dc1"
2026/10/10 07:58:46 sale request rejected status=409 seller="seller" product="6ac9f0351e93c39c573b4dc1"
    sales_expiry_test.go:161: concurrent write forced transaction retry at expiry: no sale, eligibility retained, one replayable failure, no successful record or counter increment
--- PASS: TestSaleTransactionRetryAcrossExpirationHTTP (0.60s)
=== RUN   TestBrowseFiltersUnavailableAndOtherCategories
--- PASS: TestBrowseFiltersUnavailableAndOtherCategories (1.10s)
=== RUN   TestBrowseStablePaginationAndLimits
--- PASS: TestBrowseStablePaginationAndLimits (0.56s)
=== RUN   TestBrowseSwaggerContract
--- PASS: TestBrowseSwaggerContract (0.54s)
=== RUN   TestSeedLogin
--- PASS: TestSeedLogin (0.84s)
=== RUN   TestOwnProfileAndIdentityProtection
--- PASS: TestOwnProfileAndIdentityProtection (0.78s)
=== RUN   TestExpiredAccessToken
--- PASS: TestExpiredAccessToken (0.76s)
=== RUN   TestOpenAPIAndSwagger
--- PASS: TestOpenAPIAndSwagger (0.55s)
=== RUN   TestSeedCLIRefusesNonemptyDatabaseAndHashesCredentials
    http_test.go:164: seed output: {"baseTime":"2026-10-10T00:00:00Z","counts":{"users":2,"active":0,"sold":0,"transactions":0,"priceChanges":0},"database":"campus_seed_test_6ac9f03b1e93c39c573b4dd2","mode":"demo","randomSeed":42}
        2026/10/10 07:58:51 demo accounts: seller, buyer; password: CampusDemo123!
    http_test.go:164: seed output: 2026/10/10 07:58:51 seed requires an empty database; use --reset to rebuild only the specified DB_NAME
        exit status 1
    http_test.go:164: seed output: 2026/10/10 07:58:52 MONGO_URI and a non-system DB_NAME are required
        exit status 1
--- PASS: TestSeedCLIRefusesNonemptyDatabaseAndHashesCredentials (0.92s)
=== RUN   TestSellerChangesPriceWithoutExtendingEligibility
--- PASS: TestSellerChangesPriceWithoutExtendingEligibility (0.75s)
=== RUN   TestPriceHistoryThroughChangesAndUnavailableArchives
=== RUN   TestPriceHistoryThroughChangesAndUnavailableArchives/expired
=== RUN   TestPriceHistoryThroughChangesAndUnavailableArchives/delisted
=== RUN   TestPriceHistoryThroughChangesAndUnavailableArchives/sold
--- PASS: TestPriceHistoryThroughChangesAndUnavailableArchives (0.73s)
    --- PASS: TestPriceHistoryThroughChangesAndUnavailableArchives/expired (0.02s)
    --- PASS: TestPriceHistoryThroughChangesAndUnavailableArchives/delisted (0.02s)
    --- PASS: TestPriceHistoryThroughChangesAndUnavailableArchives/sold (0.02s)
=== RUN   TestPriceHistoryFailureRollsBackCurrentPrice
--- PASS: TestPriceHistoryFailureRollsBackCurrentPrice (0.65s)
=== RUN   TestPriceSwaggerContract
--- PASS: TestPriceSwaggerContract (0.56s)
=== RUN   TestPublishHeterogeneousProductsAndReadCurrentSeller
=== RUN   TestPublishHeterogeneousProductsAndReadCurrentSeller/textbooks
=== RUN   TestPublishHeterogeneousProductsAndReadCurrentSeller/bicycles
=== RUN   TestPublishHeterogeneousProductsAndReadCurrentSeller/electronics
--- PASS: TestPublishHeterogeneousProductsAndReadCurrentSeller (0.70s)
    --- PASS: TestPublishHeterogeneousProductsAndReadCurrentSeller/textbooks (0.02s)
    --- PASS: TestPublishHeterogeneousProductsAndReadCurrentSeller/bicycles (0.01s)
    --- PASS: TestPublishHeterogeneousProductsAndReadCurrentSeller/electronics (0.01s)
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/unknown_category
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/required_author
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/author_type
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/number_type
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/boolean_type
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/array_type
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/nested_object
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/nested_array
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/null_scalar
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/null_array_element
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/null_attributes
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/too_many_extras
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/long_attribute_key
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/unsafe_key
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/long_attribute_string
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/long_array
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/long_string_in_array
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/zero_images
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/null_images
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/ten_images
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/file_URL
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/FTP_URL
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/relative_URL
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/missing_hostname
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/URL_credentials
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/long_URL
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/unknown_condition
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/missing_condition
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/empty_title
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/long_title
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/long_description
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/zero_price
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/negative_price
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/fractional_cents
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/price_limit
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/forged_seller
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/client_expiry
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/reserved_priceCents
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/reserved_images
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/reserved_condition
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/reserved_title
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/reserved_description
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/reserved_categoryId
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/reserved_sellerId
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/reserved_id
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/reserved_initialPriceCents
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/reserved_publishedAt
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/reserved_expiresAt
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/reserved_sold
=== RUN   TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/reserved_priceHistory
--- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries (0.72s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/unknown_category (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/required_author (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/author_type (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/number_type (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/boolean_type (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/array_type (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/nested_object (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/nested_array (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/null_scalar (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/null_array_element (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/null_attributes (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/too_many_extras (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/long_attribute_key (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/unsafe_key (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/long_attribute_string (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/long_array (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/long_string_in_array (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/zero_images (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/null_images (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/ten_images (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/file_URL (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/FTP_URL (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/relative_URL (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/missing_hostname (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/URL_credentials (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/long_URL (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/unknown_condition (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/missing_condition (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/empty_title (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/long_title (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/long_description (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/zero_price (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/negative_price (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/fractional_cents (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/price_limit (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/forged_seller (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/client_expiry (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/reserved_priceCents (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/reserved_images (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/reserved_condition (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/reserved_title (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/reserved_description (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/reserved_categoryId (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/reserved_sellerId (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/reserved_id (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/reserved_initialPriceCents (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/reserved_publishedAt (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/reserved_expiresAt (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/reserved_sold (0.00s)
    --- PASS: TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries/reserved_priceHistory (0.00s)
=== RUN   TestRuleChangesPreserveReadsAndControlNewWritesAndOwnEdits
--- PASS: TestRuleChangesPreserveReadsAndControlNewWritesAndOwnEdits (0.78s)
=== RUN   TestUnavailableProductsRemainReadableButCannotBeEdited
=== RUN   TestUnavailableProductsRemainReadableButCannotBeEdited/expired
=== RUN   TestUnavailableProductsRemainReadableButCannotBeEdited/delisted
=== RUN   TestUnavailableProductsRemainReadableButCannotBeEdited/sold
--- PASS: TestUnavailableProductsRemainReadableButCannotBeEdited (0.68s)
    --- PASS: TestUnavailableProductsRemainReadableButCannotBeEdited/expired (0.01s)
    --- PASS: TestUnavailableProductsRemainReadableButCannotBeEdited/delisted (0.01s)
    --- PASS: TestUnavailableProductsRemainReadableButCannotBeEdited/sold (0.02s)
=== RUN   TestPublishingRollsBackArchiveWhenEligibilityCannotBeCreated
--- PASS: TestPublishingRollsBackArchiveWhenEligibilityCannotBeCreated (0.65s)
=== RUN   TestDetailShowsRecentPriceChangesAndLinksToCompleteHistory
--- PASS: TestDetailShowsRecentPriceChangesAndLinksToCompleteHistory (0.63s)
=== RUN   TestProductSwaggerContract
--- PASS: TestProductSwaggerContract (0.51s)
=== RUN   TestExpandedCategoryLimitsAndHistoricalReadsMatchContract
--- PASS: TestExpandedCategoryLimitsAndHistoricalReadsMatchContract (0.66s)
=== RUN   TestSaleConfirmationAndReplay
2026/10/10 07:59:00 sale request rejected status=409 seller="seller" product="6ac9f0441e93c39c573b4e0f"
--- PASS: TestSaleConfirmationAndReplay (0.67s)
=== RUN   TestSaleInvalidAttemptsAreNotRecorded
2026/10/10 07:59:01 sale request rejected status=401 seller="" product="6ac9f0451e93c39c573b4e12"
2026/10/10 07:59:01 sale request rejected status=403 seller="buyer" product="6ac9f0451e93c39c573b4e12"
2026/10/10 07:59:01 sale request rejected status=400 seller="seller" product="6ac9f0451e93c39c573b4e12"
2026/10/10 07:59:01 sale request rejected status=400 seller="seller" product="6ac9f0451e93c39c573b4e12"
2026/10/10 07:59:01 sale request rejected status=400 seller="seller" product="6ac9f0451e93c39c573b4e12"
2026/10/10 07:59:01 sale request rejected status=400 seller="seller" product="6ac9f0451e93c39c573b4e12"
2026/10/10 07:59:01 sale request rejected status=400 seller="seller" product="6ac9f0451e93c39c573b4e12"
2026/10/10 07:59:01 sale request rejected status=400 seller="seller" product="6ac9f0451e93c39c573b4e12"
2026/10/10 07:59:01 sale request rejected status=400 seller="seller" product="6ac9f0451e93c39c573b4e12"
2026/10/10 07:59:01 sale request rejected status=400 seller="seller" product="6ac9f0451e93c39c573b4e12"
2026/10/10 07:59:01 sale request rejected status=400 seller="seller" product="6ac9f0451e93c39c573b4e12"
2026/10/10 07:59:01 sale request rejected status=400 seller="seller" product="6ac9f0451e93c39c573b4e12"
--- PASS: TestSaleInvalidAttemptsAreNotRecorded (0.76s)
=== RUN   TestSaleConcurrentExactlyOnce
=== RUN   TestSaleConcurrentExactlyOnce/sameKey=false
2026/10/10 07:59:02 sale request rejected status=409 seller="seller" product="6ac9f0461e93c39c573b4e15"
2026/10/10 07:59:02 sale request rejected status=409 seller="seller" product="6ac9f0461e93c39c573b4e15"
2026/10/10 07:59:02 sale request rejected status=409 seller="seller" product="6ac9f0461e93c39c573b4e15"
2026/10/10 07:59:02 sale request rejected status=409 seller="seller" product="6ac9f0461e93c39c573b4e15"
2026/10/10 07:59:02 sale request rejected status=409 seller="seller" product="6ac9f0461e93c39c573b4e15"
2026/10/10 07:59:02 sale request rejected status=409 seller="seller" product="6ac9f0461e93c39c573b4e15"
2026/10/10 07:59:02 sale request rejected status=409 seller="seller" product="6ac9f0461e93c39c573b4e15"
2026/10/10 07:59:02 sale request rejected status=409 seller="seller" product="6ac9f0461e93c39c573b4e15"
2026/10/10 07:59:02 sale request rejected status=409 seller="seller" product="6ac9f0461e93c39c573b4e15"
2026/10/10 07:59:02 sale request rejected status=409 seller="seller" product="6ac9f0461e93c39c573b4e15"
2026/10/10 07:59:02 sale request rejected status=409 seller="seller" product="6ac9f0461e93c39c573b4e15"
2026/10/10 07:59:02 sale request rejected status=409 seller="seller" product="6ac9f0461e93c39c573b4e15"
2026/10/10 07:59:02 sale request rejected status=409 seller="seller" product="6ac9f0461e93c39c573b4e15"
2026/10/10 07:59:02 sale request rejected status=409 seller="seller" product="6ac9f0461e93c39c573b4e15"
2026/10/10 07:59:02 sale request rejected status=409 seller="seller" product="6ac9f0461e93c39c573b4e15"
2026/10/10 07:59:02 sale request rejected status=409 seller="seller" product="6ac9f0461e93c39c573b4e15"
2026/10/10 07:59:02 sale request rejected status=409 seller="seller" product="6ac9f0461e93c39c573b4e15"
2026/10/10 07:59:02 sale request rejected status=409 seller="seller" product="6ac9f0461e93c39c573b4e15"
2026/10/10 07:59:02 sale request rejected status=409 seller="seller" product="6ac9f0461e93c39c573b4e15"
2026/10/10 07:59:02 sale request rejected status=409 seller="seller" product="6ac9f0461e93c39c573b4e15"
2026/10/10 07:59:02 sale request rejected status=409 seller="seller" product="6ac9f0461e93c39c573b4e15"
2026/10/10 07:59:02 sale request rejected status=409 seller="seller" product="6ac9f0461e93c39c573b4e15"
2026/10/10 07:59:02 sale request rejected status=409 seller="seller" product="6ac9f0461e93c39c573b4e15"
2026/10/10 07:59:02 sale request rejected status=409 seller="seller" product="6ac9f0461e93c39c573b4e15"
2026/10/10 07:59:02 sale request rejected status=409 seller="seller" product="6ac9f0461e93c39c573b4e15"
=== RUN   TestSaleConcurrentExactlyOnce/sameKey=true
2026/10/10 07:59:03 sale request rejected status=409 seller="seller" product="6ac9f0471e93c39c573b4e18"
2026/10/10 07:59:03 sale request rejected status=409 seller="seller" product="6ac9f0471e93c39c573b4e18"
--- PASS: TestSaleConcurrentExactlyOnce (1.46s)
    --- PASS: TestSaleConcurrentExactlyOnce/sameKey=false (0.75s)
    --- PASS: TestSaleConcurrentExactlyOnce/sameKey=true (0.71s)
=== RUN   TestSaleSellerKeyScopeAndProductConflict
2026/10/10 07:59:03 sale request rejected status=409 seller="seller" product="6ac9f0471e93c39c573b4e1c"
--- PASS: TestSaleSellerKeyScopeAndProductConflict (0.79s)
=== RUN   TestSaleUnavailableRecordsAndTTLDelay
=== RUN   TestSaleUnavailableRecordsAndTTLDelay/before
=== RUN   TestSaleUnavailableRecordsAndTTLDelay/at
2026/10/10 07:59:04 sale request rejected status=409 seller="seller" product="6ac9f0481e93c39c573b4e21"
2026/10/10 07:59:04 sale request rejected status=409 seller="seller" product="6ac9f0481e93c39c573b4e21"
=== RUN   TestSaleUnavailableRecordsAndTTLDelay/after
2026/10/10 07:59:04 sale request rejected status=409 seller="seller" product="6ac9f0481e93c39c573b4e22"
2026/10/10 07:59:04 sale request rejected status=409 seller="seller" product="6ac9f0481e93c39c573b4e22"
=== RUN   TestSaleUnavailableRecordsAndTTLDelay/listing_expired
2026/10/10 07:59:04 sale request rejected status=409 seller="seller" product="6ac9f0481e93c39c573b4e23"
2026/10/10 07:59:04 sale request rejected status=409 seller="seller" product="6ac9f0481e93c39c573b4e23"
=== RUN   TestSaleUnavailableRecordsAndTTLDelay/delisted
2026/10/10 07:59:04 sale request rejected status=409 seller="seller" product="6ac9f0481e93c39c573b4e24"
2026/10/10 07:59:04 sale request rejected status=409 seller="seller" product="6ac9f0481e93c39c573b4e24"
--- PASS: TestSaleUnavailableRecordsAndTTLDelay (0.74s)
    --- PASS: TestSaleUnavailableRecordsAndTTLDelay/before (0.04s)
    --- PASS: TestSaleUnavailableRecordsAndTTLDelay/at (0.02s)
    --- PASS: TestSaleUnavailableRecordsAndTTLDelay/after (0.02s)
    --- PASS: TestSaleUnavailableRecordsAndTTLDelay/listing_expired (0.01s)
    --- PASS: TestSaleUnavailableRecordsAndTTLDelay/delisted (0.02s)
=== RUN   TestSaleRollbackOnPersistenceFailure
2026/10/10 07:59:05 sale transaction failed product="6ac9f0491e93c39c573b4e27": write exception: write errors: [Document failed validation: {"failingDocumentId": "db3360f0f64abfb24e7450c04054fc4fc0f1bd2c5d4b53a2c7070bd7cff2056b","details": {"operatorName": "$eq","specifiedAs": {"success": false},"reason": "comparison failed","consideredValue": true}}]
2026/10/10 07:59:05 sale request rejected status=503 seller="seller" product="6ac9f0491e93c39c573b4e27"
    sales_test.go:248: persisted rollback evidence: archive unsold, sale absent, eligibility retained, transactions=0, completedSales=0
--- PASS: TestSaleRollbackOnPersistenceFailure (0.68s)
=== RUN   TestSaleOpenAPIContract
--- PASS: TestSaleOpenAPIContract (0.53s)
=== RUN   TestSaleConcurrentKeyConflictRollsBackOtherProduct
2026/10/10 07:59:06 sale request rejected status=409 seller="seller" product="6ac9f04a1e93c39c573b4e2d"
--- PASS: TestSaleConcurrentKeyConflictRollsBackOtherProduct (0.69s)
=== RUN   TestSearchChineseSynonymsAndExplanations
--- PASS: TestSearchChineseSynonymsAndExplanations (1.65s)
=== RUN   TestSearchRefillsAfterCurrentEligibilityFiltering
    search_test.go:141: 70 indexed candidates; 45 currently invalid; search response elapsed=17.152309ms
--- PASS: TestSearchRefillsAfterCurrentEligibilityFiltering (1.96s)
=== RUN   TestSearchSwaggerContract
--- PASS: TestSearchSwaggerContract (0.52s)
=== RUN   TestSeedCLIResetAndReproduction
--- PASS: TestSeedCLIResetAndReproduction (7.09s)
=== RUN   TestSellerHomeCurrentProfileAndIsolation
--- PASS: TestSellerHomeCurrentProfileAndIsolation (0.77s)
=== RUN   TestSellerHomePaginationAndInvalidInput
--- PASS: TestSellerHomePaginationAndInvalidInput (0.55s)
=== RUN   TestSellerHomeExcludesUnavailableBeforeTTLCleanup
--- PASS: TestSellerHomeExcludesUnavailableBeforeTTLCleanup (0.59s)
=== RUN   TestSellerHomeOpenAPI
--- PASS: TestSellerHomeOpenAPI (0.51s)
=== RUN   TestConditionCounts
--- PASS: TestConditionCounts (0.59s)
=== RUN   TestConditionCountsOpenAPI
--- PASS: TestConditionCountsOpenAPI (0.53s)
=== RUN   TestV1FullSeedHTTP
    v1_test.go:12: run scripts/validate-v1.sh for full-seed deployment acceptance
--- SKIP: TestV1FullSeedHTTP (0.00s)
PASS
ok  	github.com/nanfxqs/campus-market/internal/market	79.020s
=== RUN   TestChineseSynonyms
--- PASS: TestChineseSynonyms (4.79s)
=== RUN   TestTransactionCommitAndRollback
--- PASS: TestTransactionCommitAndRollback (0.47s)
=== RUN   TestTTLRetainsArchiveAndForbidsSale
    persistence_test.go:159: expired listing remains physically present; HTTP rejects sale without writes before TTL cleanup
    persistence_test.go:173: TTL physical deletion observed 32.391848721s after expiry; archive and history retained
--- PASS: TestTTLRetainsArchiveAndForbidsSale (37.52s)
=== RUN   TestScaleSearch
    persistence_test.go:208: scale search candidates=20040 valid=20000 top=20 server_ms=1693 http=1.695334354s
    persistence_test.go:208: scale search candidates=20040 valid=20000 top=20 server_ms=1647 http=1.647931486s
    persistence_test.go:208: scale search candidates=20040 valid=20000 top=20 server_ms=1611 http=1.611883305s
--- PASS: TestScaleSearch (5.07s)
=== RUN   TestSearchChecksCurrentEligibility
    persistence_test.go:258: current eligibility: 20040 candidates, 19997 eligible, 20 returned
--- PASS: TestSearchChecksCurrentEligibility (1.65s)
PASS
ok  	github.com/nanfxqs/campus-market/validation	49.502s

$ docker compose -p campus-v1-20261010075535-1289890-13302 -f compose.v1.yaml ps
NAME                                              IMAGE                                                                                                       COMMAND                  SERVICE   CREATED         STATUS                   PORTS
campus-v1-20261010075535-1289890-13302-api-1      campus-v1-20261010075535-1289890-13302-api                                                                  "/validation serve"      api       2 minutes ago   Up 2 minutes (healthy)
campus-v1-20261010075535-1289890-13302-market-1   campus-v1-20261010075535-1289890-13302-market                                                               "/market serve"          market    2 minutes ago   Up 2 minutes (healthy)
campus-v1-20261010075535-1289890-13302-mongo-1    mongodb/mongodb-atlas-local:8.0.4@sha256:1ce32a37610486d753a81cc7ca9c719964679e37eb0aa8b4b521578b883d0147   "/usr/local/bin/runn…"   mongo     3 minutes ago   Up 3 minutes (healthy)   27017/tcp
 Container campus-v1-20261010075535-1289890-13302-api-1 Stopping
 Container campus-v1-20261010075535-1289890-13302-market-1 Stopping
 Container campus-v1-20261010075535-1289890-13302-market-1 Stopped
 Container campus-v1-20261010075535-1289890-13302-market-1 Removing
 Container campus-v1-20261010075535-1289890-13302-api-1 Stopped
 Container campus-v1-20261010075535-1289890-13302-api-1 Removing
 Container campus-v1-20261010075535-1289890-13302-market-1 Removed
 Container campus-v1-20261010075535-1289890-13302-api-1 Removed
 Container campus-v1-20261010075535-1289890-13302-mongo-1 Stopping
 Container campus-v1-20261010075535-1289890-13302-mongo-1 Stopped
 Container campus-v1-20261010075535-1289890-13302-mongo-1 Removing
 Container campus-v1-20261010075535-1289890-13302-mongo-1 Removed
 Volume campus-v1-20261010075535-1289890-13302_mongo-data Removing
 Volume campus-v1-20261010075535-1289890-13302_mongo-config Removing
 Volume campus-v1-20261010075535-1289890-13302_search-data Removing
 Network campus-v1-20261010075535-1289890-13302_default Removing
 Volume campus-v1-20261010075535-1289890-13302_mongo-config Removed
 Volume campus-v1-20261010075535-1289890-13302_mongo-data Removed
 Volume campus-v1-20261010075535-1289890-13302_search-data Removed
 Network campus-v1-20261010075535-1289890-13302_default Removed

```

Result: PASS (exit 0)
