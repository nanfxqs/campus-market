# V1 clean-environment acceptance

UTC: 2026-10-10T08:03:58Z

Project: `campus-v1-20261010080358-1359053-27100`; base time: `2026-10-10T00:00:00Z`; random seed: 42; mode: full.

```text

$ git rev-parse HEAD
63abab0276b78ca1b075bf4ec6acc7bc3a2e47ec

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
/dev/mapper/omarchy_root  352G  131G  221G  38% /home

$ docker compose -p campus-v1-20261010080358-1359053-27100 -f compose.v1.yaml up -d --build --wait mongo
 Volume campus-v1-20261010080358-1359053-27100_mongo-data Creating
 Network campus-v1-20261010080358-1359053-27100_default Creating
 Volume campus-v1-20261010080358-1359053-27100_mongo-data Creating
 Volume campus-v1-20261010080358-1359053-27100_search-data Creating
 Volume campus-v1-20261010080358-1359053-27100_search-data Creating
 Volume campus-v1-20261010080358-1359053-27100_mongo-config Creating
 Network campus-v1-20261010080358-1359053-27100_default Creating
 Volume campus-v1-20261010080358-1359053-27100_mongo-config Creating
 Volume campus-v1-20261010080358-1359053-27100_mongo-data Created
 Volume campus-v1-20261010080358-1359053-27100_mongo-data Created
 Volume campus-v1-20261010080358-1359053-27100_mongo-config Created
 Volume campus-v1-20261010080358-1359053-27100_mongo-config Created
 Volume campus-v1-20261010080358-1359053-27100_search-data Created
 Volume campus-v1-20261010080358-1359053-27100_search-data Created
 Network campus-v1-20261010080358-1359053-27100_default Created
 Network campus-v1-20261010080358-1359053-27100_default Created
 Container campus-v1-20261010080358-1359053-27100-mongo-1 Creating
 Container campus-v1-20261010080358-1359053-27100-mongo-1 Created
 Container campus-v1-20261010080358-1359053-27100-mongo-1 Starting
 Container campus-v1-20261010080358-1359053-27100-mongo-1 Started
 Container campus-v1-20261010080358-1359053-27100-mongo-1 Waiting
 Container campus-v1-20261010080358-1359053-27100-mongo-1 Healthy

$ docker compose -p campus-v1-20261010080358-1359053-27100 -f compose.v1.yaml build market api verify
 Image campus-v1-20261010080358-1359053-27100-market Building
 Image campus-v1-20261010080358-1359053-27100-api Building
 Image campus-v1-20261010080358-1359053-27100-verify Building
#1 [internal] load local bake definitions
#1 reading from stdin 1.58kB done
#1 DONE 0.0s

#2 [api internal] load build definition from Dockerfile
#2 transferring dockerfile: 491B done
#2 DONE 0.0s

#3 [api internal] load metadata for docker.io/library/debian:bookworm-slim@sha256:7c7b2c966bc9ee8cedfeef67e0e279108992c77681fa595db4a9d65c06ccc587
#3 DONE 0.0s

#4 [verify internal] load metadata for docker.io/library/golang:1.24.2-bookworm@sha256:79390b5e5af9ee6e7b1173ee3eac7fadf6751a545297672916b59bfa0ecf6f71
#4 DONE 0.0s

#5 [market internal] load .dockerignore
#5 transferring context: 62B done
#5 DONE 0.0s

#6 [market stage-1 1/3] FROM docker.io/library/debian:bookworm-slim@sha256:7c7b2c966bc9ee8cedfeef67e0e279108992c77681fa595db4a9d65c06ccc587
#6 resolve docker.io/library/debian:bookworm-slim@sha256:7c7b2c966bc9ee8cedfeef67e0e279108992c77681fa595db4a9d65c06ccc587 0.0s done
#6 DONE 0.0s

#7 [api tools 1/7] FROM docker.io/library/golang:1.24.2-bookworm@sha256:79390b5e5af9ee6e7b1173ee3eac7fadf6751a545297672916b59bfa0ecf6f71
#7 resolve docker.io/library/golang:1.24.2-bookworm@sha256:79390b5e5af9ee6e7b1173ee3eac7fadf6751a545297672916b59bfa0ecf6f71 0.0s done
#7 DONE 0.0s

#8 [api internal] load build context
#8 transferring context: 586.74kB 0.1s done
#8 DONE 0.1s

#9 [api tools 3/7] COPY go.mod go.sum ./
#9 CACHED

#10 [api tools 2/7] WORKDIR /src
#10 CACHED

#11 [api tools 4/7] RUN go mod download
#11 CACHED

#12 [api tools 5/7] COPY . .
#12 DONE 0.3s

#13 [market tools 6/7] RUN go build -o /validation ./validation
#13 DONE 12.2s

#14 [api tools 7/7] RUN go build -o /market ./cmd/market
#14 DONE 1.4s

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

#18 [api] exporting to image
#18 exporting layers done
#18 exporting manifest sha256:fd179246ee5900ad2746eddcad4bea2d358b65cc7317be5c7b96320f7a4ecc99 0.0s done
#18 exporting config sha256:e3c018c0ff630965824038b48f535276f122241b03eff1e9d2da67dabbe874e8 0.0s done
#18 exporting attestation manifest sha256:ca9fea902347d83f211259d134f344b513826227522a0816f24c9fc07ac5cb8d 0.0s done
#18 exporting manifest list sha256:762f1b2a4a96cc35486213bdb1bc654c5dcdef1c0f1efa322e5cad572083a60f 0.0s done
#18 naming to docker.io/library/campus-v1-20261010080358-1359053-27100-api:latest done
#18 unpacking to docker.io/library/campus-v1-20261010080358-1359053-27100-api:latest done
#18 DONE 0.1s

#19 [market] exporting to image
#19 exporting layers done
#19 exporting manifest sha256:c37d47041d9debfba52b241fdc13359f85bca9945236c7bde33a15a6a734b92c 0.0s done
#19 exporting config sha256:427241083b3337fd8fe53320c7efdb9ffdcf4a38c989c3b884708e1b59c2faaf 0.0s done
#19 exporting attestation manifest sha256:dc54369bd36ccb73209458afcdfda4b6a5d42edccfcf254bca5a61eaf3c2cdde 0.0s done
#19 exporting manifest list sha256:0e9adfac809afd21e3e0939f7bf0c14036ca1769ee336a5af2b66f604127bc03 0.0s done
#19 naming to docker.io/library/campus-v1-20261010080358-1359053-27100-market:latest done
#19 unpacking to docker.io/library/campus-v1-20261010080358-1359053-27100-market:latest done
#19 DONE 0.1s

#20 [api] resolving provenance for metadata file
#20 DONE 0.0s

#21 [market] resolving provenance for metadata file
#21 DONE 0.0s

#15 [verify] exporting to image
#15 exporting layers 4.0s done
#15 exporting manifest sha256:f44e020276b2b1183625c1583a4ad1509039d19d89281c40611f2ba79bd3d546 0.0s done
#15 exporting config sha256:1674788f5abd98f6af01b5214ace68874e2a5765a07816bea59be76b011b73ef 0.0s done
#15 exporting attestation manifest sha256:a95dc4493c6ef449612eb4fe3fb285a8b835c796433b69b56de04b00bab46664 0.0s done
#15 exporting manifest list sha256:3a08599bc26ad8e2ac69270c1b16d40f70b2d6074ceb411230ac918f877c34b2 0.0s done
#15 naming to docker.io/library/campus-v1-20261010080358-1359053-27100-verify:latest done
#15 unpacking to docker.io/library/campus-v1-20261010080358-1359053-27100-verify:latest
#15 unpacking to docker.io/library/campus-v1-20261010080358-1359053-27100-verify:latest 1.8s done
#15 DONE 5.8s

#22 [verify] resolving provenance for metadata file
#22 DONE 0.0s
 Image campus-v1-20261010080358-1359053-27100-market Built
 Image campus-v1-20261010080358-1359053-27100-api Built
 Image campus-v1-20261010080358-1359053-27100-verify Built

$ docker compose -p campus-v1-20261010080358-1359053-27100 -f compose.v1.yaml run --rm --no-deps market seed --mode full --random-seed 42 --base-time 2026-10-10T00:00:00Z
 Container campus-v1-20261010080358-1359053-27100-market-run-09554282f266 Creating
 Container campus-v1-20261010080358-1359053-27100-market-run-09554282f266 Created
{"baseTime":"2026-10-10T00:00:00Z","counts":{"users":10000,"active":20000,"sold":180000,"transactions":300000,"priceChanges":500000},"database":"campus_v1","mode":"full","randomSeed":42}
2026/10/10 08:04:49 demo accounts: seller, buyer; password: CampusDemo123!

$ docker compose -p campus-v1-20261010080358-1359053-27100 -f compose.v1.yaml exec -T mongo mongosh --quiet campus_v1 --eval $'// Run with mongosh against an explicitly selected seed database; read-only.\nconst metadata = db.seed_metadata.findOne({_id: "dataset"});\nif (!metadata) throw new Error("No completed seed dataset metadata");\nconst expected = metadata.counts;\nfunction check(name, actual, wanted) {\n  print(`${name}: ${actual} (expected ${wanted})`);\n  if (actual !== wanted) throw new Error(`${name} mismatch`);\n}\nfunction count(collection, pipeline) {\n  const result = db[collection].aggregate([...pipeline, {$count: "n"}],\n    {allowDiskUse: true}).toArray();\n  return result.length ? result[0].n : 0;\n}\ncheck("users", db.users.countDocuments({}), expected.users);\ncheck("unsold", db.products.countDocuments({sold: false}), expected.active);\ncheck("sold", db.products.countDocuments({sold: true}), expected.sold);\ncheck("listings", db.listings.countDocuments({}), expected.active);\ncheck("transactions", db.transactions.countDocuments({}), expected.transactions);\ncheck("priceChanges", db.priceChanges.countDocuments({}), expected.priceChanges);\ncheck("successes", db.transactions.countDocuments({success: true}), expected.sold);\ncheck("image violations", db.products.countDocuments({\n  $expr: {$or: [{$lt: [{$size: "$images"}, 1]}, {$gt: [{$size: "$images"}, 9]}]}\n}), 0);\ncheck("price chain violations", count("priceChanges", [\n  {$sort: {productId: 1, changedAt: 1, _id: 1}},\n  {$group: {_id: "$productId", history: {$push: {old: "$oldPriceCents", next: "$newPriceCents"}}}},\n  {$lookup: {from: "products", localField: "_id", foreignField: "_id", as: "product"}},\n  {$unwind: "$product"},\n  {$match: {$expr: {$or: [\n    {$ne: [{$arrayElemAt: ["$history.old", 0]}, "$product.initialPriceCents"]},\n    {$ne: [{$arrayElemAt: ["$history.next", -1]}, "$product.priceCents"]},\n    {$anyElementTrue: {$map: {\n      input: {$range: [1, {$size: "$history"}]}, as: "i",\n      in: {$ne: [\n        {$arrayElemAt: ["$history.old", "$$i"]},\n        {$arrayElemAt: ["$history.next", {$subtract: ["$$i", 1]}]}\n      ]}\n    }}}\n  ]}}}\n]), 0);\ncheck("sale record violations", count("transactions", [\n  {$match: {success: true}},\n  {$lookup: {from: "products", localField: "productId", foreignField: "_id", as: "product"}},\n  {$unwind: {path: "$product", preserveNullAndEmptyArrays: true}},\n  {$match: {$expr: {$or: [\n    {$ne: ["$product.sold", true]},\n    {$ne: ["$sellerId", "$product.sellerId"]},\n    {$ne: ["$buyerId", "$product.sale.buyerId"]},\n    {$ne: ["$priceCents", "$product.sale.priceCents"]},\n    {$ne: ["$confirmedAt", "$product.sale.confirmedAt"]}\n  ]}}}\n]), 0);\ncheck("seller count violations", count("products", [\n  {$match: {sold: true}},\n  {$group: {_id: "$sellerId", n: {$sum: 1}}},\n  {$lookup: {from: "users", localField: "_id", foreignField: "_id", as: "seller"}},\n  {$unwind: {path: "$seller", preserveNullAndEmptyArrays: true}},\n  {$match: {$expr: {$ne: ["$n", "$seller.completedSales"]}}}\n]), 0);\nprintjson(db.priceChanges.aggregate([\n  {$group: {_id: "$productId", n: {$sum: 1}}},\n  {$group: {_id: null, min: {$min: "$n"}, max: {$max: "$n"}, mean: {$avg: "$n"}}}\n]).toArray());\nprint("Seed observations passed. Listing counts are wall-clock sensitive; see seed documentation.");'
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

$ docker compose -p campus-v1-20261010080358-1359053-27100 -f compose.v1.yaml up -d --wait market api
 Container campus-v1-20261010080358-1359053-27100-mongo-1 Running
 Container campus-v1-20261010080358-1359053-27100-api-1 Creating
 Container campus-v1-20261010080358-1359053-27100-market-1 Creating
 Container campus-v1-20261010080358-1359053-27100-api-1 Created
 Container campus-v1-20261010080358-1359053-27100-market-1 Created
 Container campus-v1-20261010080358-1359053-27100-mongo-1 Waiting
 Container campus-v1-20261010080358-1359053-27100-mongo-1 Waiting
 Container campus-v1-20261010080358-1359053-27100-mongo-1 Healthy
 Container campus-v1-20261010080358-1359053-27100-mongo-1 Healthy
 Container campus-v1-20261010080358-1359053-27100-api-1 Starting
 Container campus-v1-20261010080358-1359053-27100-market-1 Starting
 Container campus-v1-20261010080358-1359053-27100-api-1 Started
 Container campus-v1-20261010080358-1359053-27100-market-1 Started
 Container campus-v1-20261010080358-1359053-27100-market-1 Waiting
 Container campus-v1-20261010080358-1359053-27100-api-1 Waiting
 Container campus-v1-20261010080358-1359053-27100-mongo-1 Waiting
 Container campus-v1-20261010080358-1359053-27100-mongo-1 Healthy
 Container campus-v1-20261010080358-1359053-27100-api-1 Healthy
 Container campus-v1-20261010080358-1359053-27100-market-1 Healthy

$ docker compose -p campus-v1-20261010080358-1359053-27100 -f compose.v1.yaml run --rm --no-deps verify vet ./...
 Container campus-v1-20261010080358-1359053-27100-verify-run-cfcc314a76ee Creating
 Container campus-v1-20261010080358-1359053-27100-verify-run-cfcc314a76ee Created

$ docker compose -p campus-v1-20261010080358-1359053-27100 -f compose.v1.yaml run --rm --no-deps -e V1_API_URL=http://market:8080 -e V1_DB_NAME=campus_v1 verify test -v ./internal/market -run \^TestV1FullSeedHTTP\$ -count=1 -timeout=8m
 Container campus-v1-20261010080358-1359053-27100-verify-run-810ccec8a00a Creating
 Container campus-v1-20261010080358-1359053-27100-verify-run-810ccec8a00a Created
=== RUN   TestV1FullSeedHTTP
    v1_test.go:29: full seed: synonym search total=6667, top 20 with matched fields
    v1_test.go:73: full seed: publish, browse, detail/current profile, price history, seller home, sale/replay and ownership PASS
=== RUN   TestV1FullSeedHTTP/full-seed_expiry
    v1_test.go:146: full seed: expired bicycle excluded from all browse pages, Search total and statistics; sale/price rejected; archive/images/history retained
--- PASS: TestV1FullSeedHTTP (52.20s)
    --- PASS: TestV1FullSeedHTTP/full-seed_expiry (21.23s)
PASS
ok  	github.com/nanfxqs/campus-market/internal/market	52.211s

$ docker compose -p campus-v1-20261010080358-1359053-27100 -f compose.v1.yaml run --rm --no-deps api seed --large
 Container campus-v1-20261010080358-1359053-27100-api-run-c63ed5e4a220 Creating
 Container campus-v1-20261010080358-1359053-27100-api-run-c63ed5e4a220 Created
2026/10/10 08:06:22 fixture ready users=10000 active=20000 history=180000 transactions=300000 candidates=20040 search_ms=1641 base=2026-10-10T08:06:11.00109122Z

$ docker compose -p campus-v1-20261010080358-1359053-27100 -f compose.v1.yaml run --rm --no-deps -e GIN_MODE=release -e SCALE=1 verify test -v ./... -count=1 -timeout=15m
 Container campus-v1-20261010080358-1359053-27100-verify-run-e9cd8bd2d721 Creating
 Container campus-v1-20261010080358-1359053-27100-verify-run-e9cd8bd2d721 Created
?   	github.com/nanfxqs/campus-market/cmd/market	[no test files]
=== RUN   TestExpiryLifecycleHTTP
=== RUN   TestExpiryLifecycleHTTP/-1ms
    expiry_test.go:142: all entry points expiry=2026-10-10T08:07:25.93Z now=2026-10-10T08:07:25.929Z eligible=1
=== RUN   TestExpiryLifecycleHTTP/0s
2026/10/10 08:06:27 sale request rejected status=409 seller="seller" product="6ac9f202ea8a0a9447d88946"
    expiry_test.go:142: all entry points expiry=2026-10-10T08:07:25.93Z now=2026-10-10T08:07:25.93Z eligible=0
=== RUN   TestExpiryLifecycleHTTP/1ms
2026/10/10 08:06:27 sale request rejected status=409 seller="seller" product="6ac9f202ea8a0a9447d88946"
    expiry_test.go:142: all entry points expiry=2026-10-10T08:07:25.93Z now=2026-10-10T08:07:25.931Z eligible=0
--- PASS: TestExpiryLifecycleHTTP (1.72s)
    --- PASS: TestExpiryLifecycleHTTP/-1ms (0.03s)
    --- PASS: TestExpiryLifecycleHTTP/0s (0.03s)
    --- PASS: TestExpiryLifecycleHTTP/1ms (0.02s)
=== RUN   TestExpiryTTLRetainsHistoryHTTP
2026/10/10 08:06:29 sale request rejected status=409 seller="seller" product="6ac9f203ea8a0a9447d8894d"
2026/10/10 08:07:06 sale request rejected status=409 seller="seller" product="6ac9f203ea8a0a9447d8894d"
    expiry_test.go:201: TTL database evidence db=campus_expiry_6ac9f203ea8a0a9447d8894c listing=6ac9f203ea8a0a9447d8894d count=0 expiresAt=2026-10-10T08:06:29.807Z observed=2026-10-10T08:07:06.983442889Z cleanup_delay=37.17644343s; archive, images, price history readable; sale and price changes rejected
--- PASS: TestExpiryTTLRetainsHistoryHTTP (39.73s)
=== RUN   TestSaleExactExpirationHTTP
=== RUN   TestSaleExactExpirationHTTP/-1ms
    sales_expiry_test.go:71: expiry=2026-10-10T08:07:07.35Z confirmation=2026-10-10T08:07:07.349Z code=sold
=== RUN   TestSaleExactExpirationHTTP/0s
2026/10/10 08:07:07 sale request rejected status=409 seller="seller" product="6ac9f22bea8a0a9447d88951"
    sales_expiry_test.go:71: expiry=2026-10-10T08:07:07.35Z confirmation=2026-10-10T08:07:07.35Z code=product_expired
=== RUN   TestSaleExactExpirationHTTP/1ms
2026/10/10 08:07:07 sale request rejected status=409 seller="seller" product="6ac9f22bea8a0a9447d88952"
    sales_expiry_test.go:71: expiry=2026-10-10T08:07:07.35Z confirmation=2026-10-10T08:07:07.351Z code=product_expired
--- PASS: TestSaleExactExpirationHTTP (0.76s)
    --- PASS: TestSaleExactExpirationHTTP/-1ms (0.12s)
    --- PASS: TestSaleExactExpirationHTTP/0s (0.10s)
    --- PASS: TestSaleExactExpirationHTTP/1ms (0.10s)
=== RUN   TestSaleTransactionRetryAcrossExpirationHTTP
2026/10/10 08:07:08 sale request rejected status=409 seller="seller" product="6ac9f22cea8a0a9447d88954"
2026/10/10 08:07:08 sale request rejected status=409 seller="seller" product="6ac9f22cea8a0a9447d88954"
    sales_expiry_test.go:161: concurrent write forced transaction retry at expiry: no sale, eligibility retained, one replayable failure, no successful record or counter increment
--- PASS: TestSaleTransactionRetryAcrossExpirationHTTP (0.58s)
=== RUN   TestBrowseFiltersUnavailableAndOtherCategories
--- PASS: TestBrowseFiltersUnavailableAndOtherCategories (1.14s)
=== RUN   TestBrowseStablePaginationAndLimits
--- PASS: TestBrowseStablePaginationAndLimits (0.58s)
=== RUN   TestBrowseSwaggerContract
--- PASS: TestBrowseSwaggerContract (0.53s)
=== RUN   TestSeedLogin
--- PASS: TestSeedLogin (0.83s)
=== RUN   TestOwnProfileAndIdentityProtection
--- PASS: TestOwnProfileAndIdentityProtection (0.77s)
=== RUN   TestExpiredAccessToken
--- PASS: TestExpiredAccessToken (0.73s)
=== RUN   TestOpenAPIAndSwagger
--- PASS: TestOpenAPIAndSwagger (0.53s)
=== RUN   TestSeedCLIRefusesNonemptyDatabaseAndHashesCredentials
    http_test.go:164: seed output: {"baseTime":"2026-10-10T00:00:00Z","counts":{"users":2,"active":0,"sold":0,"transactions":0,"priceChanges":0},"database":"campus_seed_test_6ac9f231ea8a0a9447d88965","mode":"demo","randomSeed":42}
        2026/10/10 08:07:13 demo accounts: seller, buyer; password: CampusDemo123!
    http_test.go:164: seed output: 2026/10/10 08:07:14 seed requires an empty database; use --reset to rebuild only the specified DB_NAME
        exit status 1
    http_test.go:164: seed output: 2026/10/10 08:07:14 MONGO_URI and a non-system DB_NAME are required
        exit status 1
--- PASS: TestSeedCLIRefusesNonemptyDatabaseAndHashesCredentials (0.90s)
=== RUN   TestSellerChangesPriceWithoutExtendingEligibility
--- PASS: TestSellerChangesPriceWithoutExtendingEligibility (0.77s)
=== RUN   TestPriceHistoryThroughChangesAndUnavailableArchives
=== RUN   TestPriceHistoryThroughChangesAndUnavailableArchives/expired
=== RUN   TestPriceHistoryThroughChangesAndUnavailableArchives/delisted
=== RUN   TestPriceHistoryThroughChangesAndUnavailableArchives/sold
--- PASS: TestPriceHistoryThroughChangesAndUnavailableArchives (0.75s)
    --- PASS: TestPriceHistoryThroughChangesAndUnavailableArchives/expired (0.02s)
    --- PASS: TestPriceHistoryThroughChangesAndUnavailableArchives/delisted (0.02s)
    --- PASS: TestPriceHistoryThroughChangesAndUnavailableArchives/sold (0.02s)
=== RUN   TestPriceHistoryFailureRollsBackCurrentPrice
--- PASS: TestPriceHistoryFailureRollsBackCurrentPrice (0.68s)
=== RUN   TestPriceSwaggerContract
--- PASS: TestPriceSwaggerContract (0.50s)
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
    --- PASS: TestUnavailableProductsRemainReadableButCannotBeEdited/sold (0.01s)
=== RUN   TestPublishingRollsBackArchiveWhenEligibilityCannotBeCreated
--- PASS: TestPublishingRollsBackArchiveWhenEligibilityCannotBeCreated (0.69s)
=== RUN   TestDetailShowsRecentPriceChangesAndLinksToCompleteHistory
--- PASS: TestDetailShowsRecentPriceChangesAndLinksToCompleteHistory (0.65s)
=== RUN   TestProductSwaggerContract
--- PASS: TestProductSwaggerContract (0.53s)
=== RUN   TestExpandedCategoryLimitsAndHistoricalReadsMatchContract
--- PASS: TestExpandedCategoryLimitsAndHistoricalReadsMatchContract (0.66s)
=== RUN   TestSaleConfirmationAndReplay
2026/10/10 08:07:23 sale request rejected status=409 seller="seller" product="6ac9f23bea8a0a9447d889a2"
--- PASS: TestSaleConfirmationAndReplay (0.67s)
=== RUN   TestSaleInvalidAttemptsAreNotRecorded
2026/10/10 08:07:23 sale request rejected status=401 seller="" product="6ac9f23bea8a0a9447d889a5"
2026/10/10 08:07:23 sale request rejected status=403 seller="buyer" product="6ac9f23bea8a0a9447d889a5"
2026/10/10 08:07:23 sale request rejected status=400 seller="seller" product="6ac9f23bea8a0a9447d889a5"
2026/10/10 08:07:23 sale request rejected status=400 seller="seller" product="6ac9f23bea8a0a9447d889a5"
2026/10/10 08:07:23 sale request rejected status=400 seller="seller" product="6ac9f23bea8a0a9447d889a5"
2026/10/10 08:07:23 sale request rejected status=400 seller="seller" product="6ac9f23bea8a0a9447d889a5"
2026/10/10 08:07:23 sale request rejected status=400 seller="seller" product="6ac9f23bea8a0a9447d889a5"
2026/10/10 08:07:23 sale request rejected status=400 seller="seller" product="6ac9f23bea8a0a9447d889a5"
2026/10/10 08:07:23 sale request rejected status=400 seller="seller" product="6ac9f23bea8a0a9447d889a5"
2026/10/10 08:07:23 sale request rejected status=400 seller="seller" product="6ac9f23bea8a0a9447d889a5"
2026/10/10 08:07:23 sale request rejected status=400 seller="seller" product="6ac9f23bea8a0a9447d889a5"
2026/10/10 08:07:23 sale request rejected status=400 seller="seller" product="6ac9f23bea8a0a9447d889a5"
--- PASS: TestSaleInvalidAttemptsAreNotRecorded (0.74s)
=== RUN   TestSaleConcurrentExactlyOnce
=== RUN   TestSaleConcurrentExactlyOnce/sameKey=false
2026/10/10 08:07:24 sale request rejected status=409 seller="seller" product="6ac9f23cea8a0a9447d889a8"
2026/10/10 08:07:24 sale request rejected status=409 seller="seller" product="6ac9f23cea8a0a9447d889a8"
2026/10/10 08:07:24 sale request rejected status=409 seller="seller" product="6ac9f23cea8a0a9447d889a8"
2026/10/10 08:07:24 sale request rejected status=409 seller="seller" product="6ac9f23cea8a0a9447d889a8"
2026/10/10 08:07:24 sale request rejected status=409 seller="seller" product="6ac9f23cea8a0a9447d889a8"
2026/10/10 08:07:24 sale request rejected status=409 seller="seller" product="6ac9f23cea8a0a9447d889a8"
2026/10/10 08:07:24 sale request rejected status=409 seller="seller" product="6ac9f23cea8a0a9447d889a8"
2026/10/10 08:07:24 sale request rejected status=409 seller="seller" product="6ac9f23cea8a0a9447d889a8"
2026/10/10 08:07:24 sale request rejected status=409 seller="seller" product="6ac9f23cea8a0a9447d889a8"
2026/10/10 08:07:24 sale request rejected status=409 seller="seller" product="6ac9f23cea8a0a9447d889a8"
2026/10/10 08:07:24 sale request rejected status=409 seller="seller" product="6ac9f23cea8a0a9447d889a8"
2026/10/10 08:07:24 sale request rejected status=409 seller="seller" product="6ac9f23cea8a0a9447d889a8"
2026/10/10 08:07:24 sale request rejected status=409 seller="seller" product="6ac9f23cea8a0a9447d889a8"
2026/10/10 08:07:24 sale request rejected status=409 seller="seller" product="6ac9f23cea8a0a9447d889a8"
2026/10/10 08:07:24 sale request rejected status=409 seller="seller" product="6ac9f23cea8a0a9447d889a8"
2026/10/10 08:07:24 sale request rejected status=409 seller="seller" product="6ac9f23cea8a0a9447d889a8"
2026/10/10 08:07:24 sale request rejected status=409 seller="seller" product="6ac9f23cea8a0a9447d889a8"
2026/10/10 08:07:24 sale request rejected status=409 seller="seller" product="6ac9f23cea8a0a9447d889a8"
2026/10/10 08:07:24 sale request rejected status=409 seller="seller" product="6ac9f23cea8a0a9447d889a8"
2026/10/10 08:07:24 sale request rejected status=409 seller="seller" product="6ac9f23cea8a0a9447d889a8"
2026/10/10 08:07:24 sale request rejected status=409 seller="seller" product="6ac9f23cea8a0a9447d889a8"
2026/10/10 08:07:24 sale request rejected status=409 seller="seller" product="6ac9f23cea8a0a9447d889a8"
2026/10/10 08:07:24 sale request rejected status=409 seller="seller" product="6ac9f23cea8a0a9447d889a8"
2026/10/10 08:07:24 sale request rejected status=409 seller="seller" product="6ac9f23cea8a0a9447d889a8"
2026/10/10 08:07:24 sale request rejected status=409 seller="seller" product="6ac9f23cea8a0a9447d889a8"
=== RUN   TestSaleConcurrentExactlyOnce/sameKey=true
2026/10/10 08:07:25 sale request rejected status=409 seller="seller" product="6ac9f23dea8a0a9447d889ab"
2026/10/10 08:07:25 sale request rejected status=409 seller="seller" product="6ac9f23dea8a0a9447d889ab"
--- PASS: TestSaleConcurrentExactlyOnce (1.48s)
    --- PASS: TestSaleConcurrentExactlyOnce/sameKey=false (0.76s)
    --- PASS: TestSaleConcurrentExactlyOnce/sameKey=true (0.72s)
=== RUN   TestSaleSellerKeyScopeAndProductConflict
2026/10/10 08:07:26 sale request rejected status=409 seller="seller" product="6ac9f23eea8a0a9447d889af"
--- PASS: TestSaleSellerKeyScopeAndProductConflict (0.78s)
=== RUN   TestSaleUnavailableRecordsAndTTLDelay
=== RUN   TestSaleUnavailableRecordsAndTTLDelay/before
=== RUN   TestSaleUnavailableRecordsAndTTLDelay/at
2026/10/10 08:07:26 sale request rejected status=409 seller="seller" product="6ac9f23eea8a0a9447d889b4"
2026/10/10 08:07:26 sale request rejected status=409 seller="seller" product="6ac9f23eea8a0a9447d889b4"
=== RUN   TestSaleUnavailableRecordsAndTTLDelay/after
2026/10/10 08:07:26 sale request rejected status=409 seller="seller" product="6ac9f23eea8a0a9447d889b5"
2026/10/10 08:07:26 sale request rejected status=409 seller="seller" product="6ac9f23eea8a0a9447d889b5"
=== RUN   TestSaleUnavailableRecordsAndTTLDelay/listing_expired
2026/10/10 08:07:26 sale request rejected status=409 seller="seller" product="6ac9f23eea8a0a9447d889b6"
2026/10/10 08:07:26 sale request rejected status=409 seller="seller" product="6ac9f23eea8a0a9447d889b6"
=== RUN   TestSaleUnavailableRecordsAndTTLDelay/delisted
2026/10/10 08:07:26 sale request rejected status=409 seller="seller" product="6ac9f23eea8a0a9447d889b7"
2026/10/10 08:07:26 sale request rejected status=409 seller="seller" product="6ac9f23eea8a0a9447d889b7"
--- PASS: TestSaleUnavailableRecordsAndTTLDelay (0.75s)
    --- PASS: TestSaleUnavailableRecordsAndTTLDelay/before (0.04s)
    --- PASS: TestSaleUnavailableRecordsAndTTLDelay/at (0.02s)
    --- PASS: TestSaleUnavailableRecordsAndTTLDelay/after (0.01s)
    --- PASS: TestSaleUnavailableRecordsAndTTLDelay/listing_expired (0.01s)
    --- PASS: TestSaleUnavailableRecordsAndTTLDelay/delisted (0.01s)
=== RUN   TestSaleRollbackOnPersistenceFailure
2026/10/10 08:07:27 sale transaction failed product="6ac9f23fea8a0a9447d889ba": write exception: write errors: [Document failed validation: {"failingDocumentId": "db3360f0f64abfb24e7450c04054fc4fc0f1bd2c5d4b53a2c7070bd7cff2056b","details": {"operatorName": "$eq","specifiedAs": {"success": false},"reason": "comparison failed","consideredValue": true}}]
2026/10/10 08:07:27 sale request rejected status=503 seller="seller" product="6ac9f23fea8a0a9447d889ba"
    sales_test.go:248: persisted rollback evidence: archive unsold, sale absent, eligibility retained, transactions=0, completedSales=0
--- PASS: TestSaleRollbackOnPersistenceFailure (0.69s)
=== RUN   TestSaleOpenAPIContract
--- PASS: TestSaleOpenAPIContract (0.53s)
=== RUN   TestSaleConcurrentKeyConflictRollsBackOtherProduct
2026/10/10 08:07:28 sale request rejected status=409 seller="seller" product="6ac9f240ea8a0a9447d889c0"
--- PASS: TestSaleConcurrentKeyConflictRollsBackOtherProduct (0.68s)
=== RUN   TestSearchChineseSynonymsAndExplanations
--- PASS: TestSearchChineseSynonymsAndExplanations (1.63s)
=== RUN   TestSearchRefillsAfterCurrentEligibilityFiltering
    search_test.go:141: 70 indexed candidates; 45 currently invalid; search response elapsed=15.488606ms
--- PASS: TestSearchRefillsAfterCurrentEligibilityFiltering (1.99s)
=== RUN   TestSearchSwaggerContract
--- PASS: TestSearchSwaggerContract (0.52s)
=== RUN   TestSeedCLIResetAndReproduction
--- PASS: TestSeedCLIResetAndReproduction (7.30s)
=== RUN   TestSellerHomeCurrentProfileAndIsolation
--- PASS: TestSellerHomeCurrentProfileAndIsolation (0.78s)
=== RUN   TestSellerHomePaginationAndInvalidInput
--- PASS: TestSellerHomePaginationAndInvalidInput (0.56s)
=== RUN   TestSellerHomeExcludesUnavailableBeforeTTLCleanup
--- PASS: TestSellerHomeExcludesUnavailableBeforeTTLCleanup (0.59s)
=== RUN   TestSellerHomeOpenAPI
--- PASS: TestSellerHomeOpenAPI (0.52s)
=== RUN   TestConditionCounts
--- PASS: TestConditionCounts (0.59s)
=== RUN   TestConditionCountsOpenAPI
--- PASS: TestConditionCountsOpenAPI (0.51s)
=== RUN   TestV1FullSeedHTTP
    v1_test.go:17: run scripts/validate-v1.sh for full-seed deployment acceptance
--- SKIP: TestV1FullSeedHTTP (0.00s)
PASS
ok  	github.com/nanfxqs/campus-market/internal/market	78.247s
=== RUN   TestChineseSynonyms
--- PASS: TestChineseSynonyms (4.45s)
=== RUN   TestTransactionCommitAndRollback
--- PASS: TestTransactionCommitAndRollback (0.48s)
=== RUN   TestTTLRetainsArchiveAndForbidsSale
    persistence_test.go:159: expired listing remains physically present; HTTP rejects sale without writes before TTL cleanup
    persistence_test.go:173: TTL physical deletion observed 31.389574717s after expiry; archive and history retained
--- PASS: TestTTLRetainsArchiveAndForbidsSale (36.52s)
=== RUN   TestScaleSearch
    persistence_test.go:208: scale search candidates=20040 valid=20000 top=20 server_ms=1460 http=1.462489323s
    persistence_test.go:208: scale search candidates=20040 valid=20000 top=20 server_ms=1438 http=1.439425437s
    persistence_test.go:208: scale search candidates=20040 valid=20000 top=20 server_ms=1404 http=1.404577539s
--- PASS: TestScaleSearch (4.42s)
=== RUN   TestSearchChecksCurrentEligibility
    persistence_test.go:258: current eligibility: 20040 candidates, 19997 eligible, 20 returned
--- PASS: TestSearchChecksCurrentEligibility (1.55s)
PASS
ok  	github.com/nanfxqs/campus-market/validation	47.431s

$ docker compose -p campus-v1-20261010080358-1359053-27100 -f compose.v1.yaml ps
NAME                                              IMAGE                                                                                                       COMMAND                  SERVICE   CREATED         STATUS                   PORTS
campus-v1-20261010080358-1359053-27100-api-1      campus-v1-20261010080358-1359053-27100-api                                                                  "/validation serve"      api       2 minutes ago   Up 2 minutes (healthy)
campus-v1-20261010080358-1359053-27100-market-1   campus-v1-20261010080358-1359053-27100-market                                                               "/market serve"          market    2 minutes ago   Up 2 minutes (healthy)
campus-v1-20261010080358-1359053-27100-mongo-1    mongodb/mongodb-atlas-local:8.0.4@sha256:1ce32a37610486d753a81cc7ca9c719964679e37eb0aa8b4b521578b883d0147   "/usr/local/bin/runn…"   mongo     3 minutes ago   Up 3 minutes (healthy)   27017/tcp
 Container campus-v1-20261010080358-1359053-27100-market-1 Stopping
 Container campus-v1-20261010080358-1359053-27100-api-1 Stopping
 Container campus-v1-20261010080358-1359053-27100-api-1 Stopped
 Container campus-v1-20261010080358-1359053-27100-api-1 Removing
 Container campus-v1-20261010080358-1359053-27100-market-1 Stopped
 Container campus-v1-20261010080358-1359053-27100-market-1 Removing
 Container campus-v1-20261010080358-1359053-27100-api-1 Removed
 Container campus-v1-20261010080358-1359053-27100-market-1 Removed
 Container campus-v1-20261010080358-1359053-27100-mongo-1 Stopping
 Container campus-v1-20261010080358-1359053-27100-mongo-1 Stopped
 Container campus-v1-20261010080358-1359053-27100-mongo-1 Removing
 Container campus-v1-20261010080358-1359053-27100-mongo-1 Removed
 Volume campus-v1-20261010080358-1359053-27100_mongo-data Removing
 Volume campus-v1-20261010080358-1359053-27100_mongo-config Removing
 Volume campus-v1-20261010080358-1359053-27100_search-data Removing
 Network campus-v1-20261010080358-1359053-27100_default Removing
 Volume campus-v1-20261010080358-1359053-27100_mongo-config Removed
 Volume campus-v1-20261010080358-1359053-27100_mongo-data Removed
 Volume campus-v1-20261010080358-1359053-27100_search-data Removed
 Network campus-v1-20261010080358-1359053-27100_default Removed

```

Result: PASS (exit 0)
