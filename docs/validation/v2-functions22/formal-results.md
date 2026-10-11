# Full-scale single-client feature measurements

Configuration A, one Gin with cache disabled. Exclusive machine slot: no other running containers before preparation or after stop. One excluded warmup per feature; each of eight features completed 100 measured HTTP requests with validated results and zero errors. These single-client function measurements are separate from the 50-user/600-second sale acceptance.

Source commit: `ebb4af166b4b7eee56fe60a2199b2099d68ec833`; runner SHA256: `df8c4e293d5ad9ccf0f4339cab7c33a51a4f060424b426e1fcac762f5484cc19`. Full environment and source/worktree state: [parameters](formal100/parameters.json), [CPU](formal100/cpu.txt), [memory](formal100/memory.txt), [machine](formal100/machine.txt), [Docker](formal100/docker.txt), [images](formal100/images.txt), [budgets](formal100/compose.yaml), [worktree](formal100/git-status.txt).

| Function | Samples | Median ms | P95 ms | P99 ms | Error rate |
|---|---:|---:|---:|---:|---:|
| 发布 | 100 | 5.591 | 6.655 | 7.123 | 0% |
| 分类浏览 | 100 | 3.276 | 4.083 | 4.539 | 0% |
| 搜索 | 100 | 2089.194 | 2304.843 | 2340.228 | 0% |
| 详情 | 100 | 2.055 | 2.660 | 3.079 | 0% |
| 卖家主页 | 100 | 17.593 | 21.907 | 22.951 | 0% |
| 成交 | 100 | 7.570 | 10.560 | 51.579 | 0% |
| 自动下架拒绝 | 100 | 4.810 | 6.090 | 6.329 | 0% |
| 分类统计 | 100 | 207.736 | 220.468 | 236.621 | 0% |

Nearest-rank P95/P99; all-attempt latency scope. Every measured response/input and timing is retained in [HTTP raw](formal100/http.jsonl); [summary](formal100/summary.json), [independent recalculation](formal100/independent-recalculation.json), and [result](formal100/result.json) show all 800 requests and eight excluded warmups. Login, fresh import, Search readiness, construction of independent sale products, expiry waits and TTL waits are excluded from sample latency.

Initial imported scale: 10,000 users, 230,000 products (50,000 active + 180,000 historical sold), 300,000 transactions and 575,000 price changes. Ten expiry fixture products/history were added during preparation, so measured publishing begins with 230,010 product archives. The publish phase adds 101 independent products including its warmup. Sale preparation adds another 101; each is sold once by its own request. Final actual scale is 230,212 products, 180,101 sold, 50,101 valid listings, 300,212 transactions and 575,010 price changes; these counts include excluded warmups and the recorded failed expiry attempts. [Initial](formal100/initial-scale.txt) and [final](formal100/final-scale.txt) external counts distinguish background/extension from measurement fixtures.

Ten products expired at the fixture timestamp and immediately exited detail eligibility, Search, browse, seller home and category statistics; ten sale attempts rejected. Actual TTL removal was observed for every sample in the 3.465–6.012-second interval after expiry, while ten archives and ten price histories remained and no successful sale existed. [Instant evidence](formal100/instant-expiry.json), [TTL polls](formal100/ttl-polls.jsonl), [per-sample intervals](formal100/ttl-observations.json), [retention](formal100/post-ttl-retention.txt). The observed interval is neither an exact deletion timestamp nor a guaranteed cleanup SLA.

The first invocation failed during initial Search readiness with 503, before any timing or writes. [Failed raw preparation](formal100-preparation-failed/http.jsonl) and [failure log](formal100.log) are retained. Readiness polling was corrected to accept temporary unavailability until a bounded exact-count readiness check succeeds; strict measured response/result checks and server timeout stayed unchanged. The retry used the untouched initial full dataset and completed successfully. Earlier two-sample smoke material is retained separately and is not substituted for formal samples.

Fresh preparation took 93 seconds ([preparation](formal100/preparation.json)). [All-profile stop](formal-stopped.log) and [empty final container inventory](exclusive-projects-after.txt) record release of the exclusive slot. Volumes are disposable experiment resources; this is a local-machine measurement, not cross-machine reproduction.
