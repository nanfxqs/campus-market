# 自动下架验收实测

UTC: 2026-10-10T07:29:47Z

真实 HTTP + MongoDB/Search；覆盖所有入口到期边界、并发成交与跨到期重试、TTL 物理清理及档案/图片/历史保留。
TTL 采用 3 分钟有界观察；这不是业务清理时限。测试库自动清理。

```text
time="2026-10-10T15:29:48+08:00" level=warning msg="Found orphan containers (campus-issue12-harness, campus-seed-issue12-api) for this project. If you removed or renamed this service in your compose file, you can run this command with the --remove-orphans flag to clean it up."
 Container campus-market-validation-verify-run-b36363e6feb5 Creating
 Container campus-market-validation-verify-run-b36363e6feb5 Created
=== RUN   TestExpiryLifecycleHTTP
=== RUN   TestExpiryLifecycleHTTP/-1ms
    expiry_test.go:142: all entry points expiry=2026-10-10T07:30:50.971Z now=2026-10-10T07:30:50.97Z eligible=1
=== RUN   TestExpiryLifecycleHTTP/0s
2026/10/10 07:29:52 sale request rejected status=409 seller="seller" product="6ac9e96f7fb38c76b0689888"
    expiry_test.go:142: all entry points expiry=2026-10-10T07:30:50.971Z now=2026-10-10T07:30:50.971Z eligible=0
=== RUN   TestExpiryLifecycleHTTP/1ms
2026/10/10 07:29:52 sale request rejected status=409 seller="seller" product="6ac9e96f7fb38c76b0689888"
    expiry_test.go:142: all entry points expiry=2026-10-10T07:30:50.971Z now=2026-10-10T07:30:50.972Z eligible=0
--- PASS: TestExpiryLifecycleHTTP (1.64s)
    --- PASS: TestExpiryLifecycleHTTP/-1ms (0.02s)
    --- PASS: TestExpiryLifecycleHTTP/0s (0.03s)
    --- PASS: TestExpiryLifecycleHTTP/1ms (0.02s)
=== RUN   TestExpiryTTLRetainsHistoryHTTP
2026/10/10 07:29:54 sale request rejected status=409 seller="seller" product="6ac9e9707fb38c76b068988f"
2026/10/10 07:30:09 sale request rejected status=409 seller="seller" product="6ac9e9707fb38c76b068988f"
    expiry_test.go:201: TTL database evidence db=campus_expiry_6ac9e9707fb38c76b068988e listing=6ac9e9707fb38c76b068988f count=0 expiresAt=2026-10-10T07:29:54.762Z observed=2026-10-10T07:30:09.348787089Z cleanup_delay=14.586787663s; archive, images, price history readable; sale and price changes rejected
--- PASS: TestExpiryTTLRetainsHistoryHTTP (17.16s)
=== RUN   TestSaleExactExpirationHTTP
=== RUN   TestSaleExactExpirationHTTP/-1ms
    sales_expiry_test.go:71: expiry=2026-10-10T07:30:09.816Z confirmation=2026-10-10T07:30:09.815Z code=sold
=== RUN   TestSaleExactExpirationHTTP/0s
2026/10/10 07:30:10 sale request rejected status=409 seller="seller" product="6ac9e9827fb38c76b0689893"
    sales_expiry_test.go:71: expiry=2026-10-10T07:30:09.816Z confirmation=2026-10-10T07:30:09.816Z code=product_expired
=== RUN   TestSaleExactExpirationHTTP/1ms
2026/10/10 07:30:10 sale request rejected status=409 seller="seller" product="6ac9e9827fb38c76b0689894"
    sales_expiry_test.go:71: expiry=2026-10-10T07:30:09.816Z confirmation=2026-10-10T07:30:09.817Z code=product_expired
--- PASS: TestSaleExactExpirationHTTP (0.82s)
    --- PASS: TestSaleExactExpirationHTTP/-1ms (0.13s)
    --- PASS: TestSaleExactExpirationHTTP/0s (0.11s)
    --- PASS: TestSaleExactExpirationHTTP/1ms (0.11s)
=== RUN   TestSaleTransactionRetryAcrossExpirationHTTP
2026/10/10 07:30:10 sale request rejected status=409 seller="seller" product="6ac9e9827fb38c76b0689896"
2026/10/10 07:30:10 sale request rejected status=409 seller="seller" product="6ac9e9827fb38c76b0689896"
    sales_expiry_test.go:161: concurrent write forced transaction retry at expiry: no sale, eligibility retained, one replayable failure, no successful record or counter increment
--- PASS: TestSaleTransactionRetryAcrossExpirationHTTP (0.56s)
=== RUN   TestSaleConcurrentExactlyOnce
=== RUN   TestSaleConcurrentExactlyOnce/sameKey=false
2026/10/10 07:30:12 sale request rejected status=409 seller="seller" product="6ac9e9847fb38c76b068989a"
2026/10/10 07:30:12 sale request rejected status=409 seller="seller" product="6ac9e9847fb38c76b068989a"
2026/10/10 07:30:12 sale request rejected status=409 seller="seller" product="6ac9e9847fb38c76b068989a"
2026/10/10 07:30:12 sale request rejected status=409 seller="seller" product="6ac9e9847fb38c76b068989a"
2026/10/10 07:30:12 sale request rejected status=409 seller="seller" product="6ac9e9847fb38c76b068989a"
2026/10/10 07:30:12 sale request rejected status=409 seller="seller" product="6ac9e9847fb38c76b068989a"
2026/10/10 07:30:12 sale request rejected status=409 seller="seller" product="6ac9e9847fb38c76b068989a"
2026/10/10 07:30:12 sale request rejected status=409 seller="seller" product="6ac9e9847fb38c76b068989a"
2026/10/10 07:30:12 sale request rejected status=409 seller="seller" product="6ac9e9847fb38c76b068989a"
2026/10/10 07:30:12 sale request rejected status=409 seller="seller" product="6ac9e9847fb38c76b068989a"
2026/10/10 07:30:12 sale request rejected status=409 seller="seller" product="6ac9e9847fb38c76b068989a"
2026/10/10 07:30:12 sale request rejected status=409 seller="seller" product="6ac9e9847fb38c76b068989a"
2026/10/10 07:30:12 sale request rejected status=409 seller="seller" product="6ac9e9847fb38c76b068989a"
2026/10/10 07:30:12 sale request rejected status=409 seller="seller" product="6ac9e9847fb38c76b068989a"
2026/10/10 07:30:12 sale request rejected status=409 seller="seller" product="6ac9e9847fb38c76b068989a"
2026/10/10 07:30:12 sale request rejected status=409 seller="seller" product="6ac9e9847fb38c76b068989a"
2026/10/10 07:30:12 sale request rejected status=409 seller="seller" product="6ac9e9847fb38c76b068989a"
2026/10/10 07:30:12 sale request rejected status=409 seller="seller" product="6ac9e9847fb38c76b068989a"
2026/10/10 07:30:12 sale request rejected status=409 seller="seller" product="6ac9e9847fb38c76b068989a"
2026/10/10 07:30:12 sale request rejected status=409 seller="seller" product="6ac9e9847fb38c76b068989a"
2026/10/10 07:30:12 sale request rejected status=409 seller="seller" product="6ac9e9847fb38c76b068989a"
2026/10/10 07:30:12 sale request rejected status=409 seller="seller" product="6ac9e9847fb38c76b068989a"
2026/10/10 07:30:12 sale request rejected status=409 seller="seller" product="6ac9e9847fb38c76b068989a"
2026/10/10 07:30:12 sale request rejected status=409 seller="seller" product="6ac9e9847fb38c76b068989a"
2026/10/10 07:30:12 sale request rejected status=409 seller="seller" product="6ac9e9847fb38c76b068989a"
=== RUN   TestSaleConcurrentExactlyOnce/sameKey=true
2026/10/10 07:30:12 sale request rejected status=409 seller="seller" product="6ac9e9847fb38c76b068989d"
2026/10/10 07:30:12 sale request rejected status=409 seller="seller" product="6ac9e9847fb38c76b068989d"
--- PASS: TestSaleConcurrentExactlyOnce (2.22s)
    --- PASS: TestSaleConcurrentExactlyOnce/sameKey=false (1.38s)
    --- PASS: TestSaleConcurrentExactlyOnce/sameKey=true (0.83s)
PASS
ok  	github.com/nanfxqs/campus-market/internal/market	22.413s
```

结果：PASS
