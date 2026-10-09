# #8 原子成交、幂等与并发正确性

2026-10-09，真实 HTTP API + MongoDB Atlas Local 8.0.4。分支 `issue-8-atomic-sales`；工作区 `/tmp/campus-market-issue8`。沿用父 issue #1 已确认的 HTTP 和必要持久化观察边界。

## 使用与契约

登录 seller 并在 Swagger Authorize 粘贴令牌；发布商品后调用 `POST /products/{id}/sale`：

```json
{"buyerId":"buyer","priceCents":10001,"idempotencyKey":"offline-sale-001"}
```

仅所有者确认，买家必须是另一已注册用户。成交价为 1–1,000,000,000,000 人民币整数分，可以不同于挂牌价。请求最多 4096 字节，禁止未知字段和补录时间。成交时间由服务端确定，UTC 毫秒精度，不支持撤销。

成功返回 200 `SaleAttempt`；已成交、到期、资格移除返回 409 的持久化失败结果（`product_sold`、`product_expired`、`product_delisted`）。同卖家/键及相同商品、买家、金额重试返回相同状态和完整原结果；不同内容返回 409 `Error`（`idempotency_conflict`）。不同卖家可使用同键。缺失身份 401、非本人 403、缺失商品 404、非法参数或买家 400、基础设施故障 503 均不生成交易记录，只写日志；幂等冲突也不生成新记录。全部响应结构、限制、失败及重试语义在 OpenAPI 中说明。

详情 `product.sale` 保存买家、成交价和确认时间，原挂牌价及档案保留；卖家当前 `completedSales` 增加一次。有效资格必须存在，档案及资格的期限均严格大于确认时间，TTL 延迟不能允许过期成交。

## 原子性与索引

事务采用 snapshot read concern 和 majority write concern。数据库条件更新 `products` 仲裁竞争，与 `listings` 删除、`users.completedSales` 增量及 `transactions` 插入共同提交。失败全部回滚；MongoDB 驱动重试写冲突后重新检查当前资格、期限及记录。

`transactions._id_` 是唯一索引，ID 为 SHA-256(`卖家字节长度:卖家ID` + 原始幂等键)，长度前缀避免拼接歧义。每个卖家/键只允许一个结果，成功与业务失败永久保留、不设置 TTL。并发不同商品争用同键时，唯一键冲突会回滚整个输方事务，再读取胜方结果并检查内容。未来完整种子生成需要遵循此记录格式，不能将随机交易 ID 当作可重试幂等记录。商品和资格使用现有 `_id_` 等值索引，不增加不必要索引。

## 验证

- TDD 首个 HTTP 成交测试在路由缺失时失败，实现后通过；OpenAPI 测试先因成交操作缺失失败，补齐契约后通过。
- `TestSale*`：真实 HTTP/MongoDB 验证议价成交、服务器时间、原结果重试、自买自卖/非法买家/非所有者/非法金额拒绝且不产生记录；24 请求竞争恰好一次成交，24 同键请求返回一致原结果；不同商品并发同键冲突无部分更新；不同卖家键隔离；成功及失败幂等持久化。
- 通过内部构造器注入时钟（没有新增导出接口或 HTTP 测试路由），仍从真实 HTTP 和 MongoDB 观察行为。`TestSaleExactExpirationHTTP` 精确验证 `expiresAt-1ms` 成功、`expiresAt` 和 `expiresAt+1ms` 失败。临时测试库暂停 TTL，过期资格仍在时必须拒绝。
- `TestSaleRollbackOnPersistenceFailure` 在独立库对 `transactions` 配置真实 MongoDB 校验器，在档案、资格、计数修改之后拒绝成功记录写入。HTTP 返回 503；持久化观察：档案 `sold=false`、`sale` 缺失、资格仍在、交易记录 0、卖家计数 0。撤除故障后用同键重试成功。
- 针对性测试、`go vet ./...` 和 OpenAPI 校验通过。全套测试与最终审查结果后附。

Swagger 操作、JSON 契约和 HTML 通过 HTTP 测试；此记录不宣称浏览器内点击或 V2 性能验收。#2 可行性夹具限制固定数据库名，因此全套验证使用独立 Compose 项目 `campus-issue8-check`，避免与其他 agent 的夹具互相修改。
