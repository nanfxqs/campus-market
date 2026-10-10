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

`transactions._id_` 是唯一索引，ID 为 SHA-256(`卖家字节长度:卖家ID` + 原始幂等键)，长度前缀避免拼接歧义。每个卖家/键只允许一个结果，成功与业务失败永久保留、不设置 TTL。并发不同商品争用同键时，唯一键冲突会回滚整个输方事务，再读取胜方结果并检查内容。唯一键冲突后通过 majority 读确认胜方持久化结果。未来完整种子生成需要遵循此记录格式，不能将随机交易 ID 当作可重试幂等记录。商品和资格使用现有 `_id_` 等值索引，不增加不必要索引。

## 验证

- TDD 首个 HTTP 成交测试在路由缺失时失败，实现后通过；OpenAPI 测试先因成交操作缺失失败，补齐契约后通过。
- `TestSale*`：真实 HTTP/MongoDB 验证议价成交、服务器时间、原结果重试、自买自卖/非法买家/非所有者/非法金额拒绝且不产生记录；24 请求竞争恰好一次成交，24 同键请求返回一致原结果；不同商品并发同键冲突无部分更新；保留属性 `sale` 不允许伪造；不同卖家键隔离；成功及失败幂等持久化。
- 通过内部构造器注入时钟（没有新增导出接口或 HTTP 测试路由），仍从真实 HTTP 和 MongoDB 观察行为。`TestSaleExactExpirationHTTP` 精确验证 `expiresAt-1ms` 成功、`expiresAt` 和 `expiresAt+1ms` 失败。临时测试库暂停 TTL，过期资格仍在时必须拒绝。`TestSaleTransactionRetryAcrossExpirationHTTP` 通过独立 MongoDB 客户端并发修改档案，强制首次条件写发生事务重试，同时将时钟推进到到期时刻；重试重新检查期限，返回并持久化一条可重放业务失败，档案未成交、资格保留、成功记录及计数为 0。
- `TestSaleRollbackOnPersistenceFailure` 在独立库对 `transactions` 配置真实 MongoDB 校验器，在档案、资格、计数修改之后拒绝成功记录写入。HTTP 返回 503；持久化观察：档案 `sold=false`、`sale` 缺失、资格仍在、交易记录 0、卖家计数 0。撤除故障后用同键重试成功。
- 针对性测试、`go vet ./...` 和 OpenAPI 校验通过。全套测试与最终审查结果后附。

Swagger 操作、JSON 契约和 HTML 通过 HTTP 测试；此记录不宣称浏览器内点击或 V2 性能验收。#2 可行性夹具限制固定数据库名，因此全套验证使用独立 Compose 项目 `campus-issue8-check`，避免与其他 agent 的夹具互相修改。

## 最终验证与复现

完整套件在 `ed00eaf` 通过：`go test -v ./... -count=1 -timeout=12m`，包括 `internal/market`（15.800s）和 `validation`（24.386s）。使用 #2 的小规模夹具（2 用户、27 件有效在售），不宣称完整种子规模或 V2 性能验收。`go vet ./...`、OpenAPI validator、`git diff --check` 通过。最后提交 `0fce4fa` 仅提取测试 HTTP 辅助函数，两项到期 HTTP 测试复查通过（0.842s）。[原始日志](issue8-tests.log) 包含完整套件及最后辅助函数整理后的定向复查输出。

已有环境可运行本功能测试：

```sh
docker compose run --rm -e GIN_MODE=release -v "$PWD:/src" verify test -v ./internal/market -run TestSale -count=1 -timeout=3m
```

为隔离全套夹具，建立 `/tmp/issue8-compose.yaml`：

```yaml
services:
  api:
    ports: !reset []
  market:
    ports: !reset []
```

在本分支目录执行（该项目名和卷仅用于临时测试）：

```sh
export COMPOSE_PROJECT_NAME=campus-issue8-check
export COMPOSE_FILE=compose.yaml:/tmp/issue8-compose.yaml
docker compose up -d --build --wait api
docker compose build verify
docker compose run --rm api seed
docker compose run --rm -e GIN_MODE=release -v "$PWD:/src" verify test -v ./... -count=1 -timeout=12m
docker compose down -v
```

## Standards

固定基准 `1e0006629a941e4566657a171a351397932d0d6d`，独立子代理审查至 `0fce4fa`：0 项文档规范违反，0 项剩余代码异味。初审指出两处并发/边界测试重复 HTTP 辅助代码，分别通过 `sendSaleRequest` 和 `saleBoundaryRequest` 提取传输、状态及 JSON 处理；各场景独立保留夹具、并发控制和行为断言，复审确认问题已解决。领域术语、事务设计、HTTP/持久化观察边界及内部时钟注入符合仓库约定与 ADR。

## Spec

独立子代理复审至 `0fce4fa`：0 项剩余发现，无明确需求遗漏、错误行为或范围扩张。初审发现并发事务重试跨到期边界缺少验证，现已加入真实 HTTP/MongoDB 测试，确认过期后重试返回持久化业务失败、无部分成交信息、成功记录或计数；原失败可幂等重放。

审查统计：Standards 0 项；Spec 0 项，无遗留问题。分支保留供用户合并，未合并到 main。
