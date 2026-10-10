# #11 自动下架完整生命周期

2026-10-10，真实 Go/Gin HTTP API + MongoDB Atlas Local 8.0.4/Search；遵循 V1 spec 与 ADR 0002/0004/0005。主要边界为 HTTP，数据库观察仅核验资格清理和成交持久化不变量。

## 可执行入口

```sh
scripts/validate-expiry.sh
# 指定 Markdown 结果路径
scripts/validate-expiry.sh docs/validation/issue11-tests.md
```

入口等待 MongoDB 就绪并构建工具镜像，无需导入或重置已有实验数据。每个测试创建并清理独立数据库，Search 用有界轮询等待真实收录；核心生命周期测试仅在自己的数据库把 listings TTL 延迟设为 3600 秒，证明到期资格物理存在时的业务过滤。独立 TTL 测试从始至终使用真实零秒 TTL 索引，不通过应用扫描或手工删除模拟下架。HTTP server 在 Go 测试内启动，数据库/Search 不模拟。

## 边界与证据

- `TestExpiryLifecycleHTTP`：通过发布和改价 API 建立近期到期和过去到期商品；内部私有时钟冻结在期限前 1 ms、恰好到期和之后 1 ms。每个时刻验证分类浏览、中文同义词搜索结果及总数、改价、详情 available、卖家主页及在售数、分类统计。到期改价和内容更新返回 409，成交返回持久化业务失败。数据库确认两个到期在售条目仍物理存在、成功成交记录为零。
- `TestSaleExactExpirationHTTP`：有效期前可以成交，恰好到期及以后失败。`TestSaleConcurrentExactlyOnce` 以真实数据库确认并发同键及不同键只有一次成功、一次成功记录及一次卖家计数增加。
- `TestSaleTransactionRetryAcrossExpirationHTTP`：另一 MongoDB client 在事务快照读取后写入档案并推进时钟至期限，迫使成交事务重试；重试重新检查期限并拒绝成交。数据库证明无成交信息、无成功记录、无计数增加且原失败幂等重放。
- `TestExpiryTTLRetainsHistoryHTTP`：真实墙钟近期到期，零秒 TTL 最终仅删除在售条目；详情继续读取原商品、图片及最近改价，完整改价历史 API 保持原结果。到期前后改价和成交均不能恢复资格，不提供重新上架入口。

TTL 轮询每 500 ms 查询指定 `_id`，最长观察 3 分钟，记录库名、商品 ID、expiresAt、观察时间及实测延迟；超时报告索引配置和 MongoDB TTL 日志诊断建议。观察窗口不承诺固定业务清理时限，采样延迟也包含轮询误差。

实测入口 PASS，零秒 TTL 物理清理在到期后约 **14.587 秒**被观察到。完整原始 Markdown 输出见 [issue11-tests.md](issue11-tests.md)。初次红灯测试在恰好到期及之后发现私有成交时钟未覆盖其他入口；现已把同一个私有时钟传入所有商品在售操作，生产入口仍使用 `time.Now`，不暴露公开测试接口。Go vet（包含类型检查）通过。

这些是 V1 功能和存储层证据，不是 V2 性能验收或清理 SLA。

## Standards

code-review 技能以实施前提交 `d16e3e1a6caba3e3f7d08f4e768a76c3775c8634` 为固定基准，独立子代理审查文档规范与代码异味：0 项发现。领域术语、保留档案的 TTL 方案、真实 HTTP/数据库边界及私有时钟符合仓库规范；未增加公开测试接口。

## Spec

独立子代理审查 issue #11 与 V1 spec：0 项发现。边界矩阵、TTL 尚未删除时过滤、并发成交/跨到期重试持久化证据、保留图片与历史、有界诊断及可执行 Markdown 入口均满足要求，无范围扩张或 ADR 冲突。

审查统计：Standards 0 项；Spec 0 项，无遗留发现。

## 最终检查

在一次性 Compose 项目 `campus-issue11-check` 中启动独立 MongoDB/Search 和可行性 API，导入小规模夹具后执行 `go vet ./...` 与 `go test -v ./... -count=1 -timeout=12m`，均通过。`internal/market` 74.304 秒，`validation` 37.536 秒；完整运行中 V1 TTL 清理观察延迟约 33.150 秒，进一步说明实测延迟不是固定时限。`TestScaleSearch` 按既有配置跳过（未设置 `SCALE=1`，未进行完整规模或 V2 压测）。[完整测试日志](issue11-full-tests.log)。临时 Compose 项目与卷在验证后删除，现有项目数据保留。

`bash -n scripts/validate-expiry.sh` 与 diff 空白检查通过。
