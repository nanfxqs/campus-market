# Issue #7：卖家改价与历史查询

分支 `issue-7-price-history`，基于 `e39b2bd`。实现 `PATCH /products/{id}/price`，请求示例 `{"priceCents":12000}`，金额为人民币整数分，范围 1–1,000,000,000,000。

只有本人有效在售商品可改价。商品当前价与旧价／新价／服务端时间记录在同一 MongoDB 事务中提交；同价成功但不增加历史，首次挂牌价与原在售期限保持不变。详情返回最近 5 条，全部历史使用现有 `price-history` 游标分页，按时间和记录 ID 倒序，历史不随在售条目 TTL 删除。

Swagger 演示：访问 `/docs`，使用种子账号登录并 Authorize，发布商品，复制商品 ID 调用改价接口，再读取详情和历史分页；重复同价请求可观察历史数量保持不变。

## 验证

测试边界沿用 V1 spec 已确认的真实 HTTP + MongoDB；数据库写入只用于构造不可售状态与历史写入故障。测试断言通过 HTTP 获取。

- 首个 HTTP 测试先失败：改价路由缺失；实现后通过。
- `pricing_test.go` 四个测试通过：所有权与认证、同价、服务端时间与价格、原期限、7 次改价及完整分页、非法金额、过期／下架／已成交拒绝及历史保留、历史写入失败时事务回滚，以及 OpenAPI 契约。
- `go vet ./...` 通过。
- 初次 `go test ./... -count=1 -timeout=12m`：应用测试通过；旧 validation 测试因共享库已含 20,000 件规模 fixture 而与默认 27 件期望冲突，并出现重复 fixture。未重置共享库。
- 全量重跑 `go test ./... -count=1 -timeout=12m` 全部通过：`internal/market` 9.239s，`validation` 33.278s。使用独立 Compose 项目 `campus-market-issue7-check`，独立 MongoDB 卷，空库导入小规模 validation fixture；不修改其他 agent 的环境。

## 双轴审查

范围 `git diff e39b2bd...422da0d`，两个并行审查 agent 分别检查规范与规格。

Standards：0 项规范违规；1 项非阻塞 possible Duplicated Code：改价与既有商品编辑均检查相同在售资格谓词。该重复很小，当前保留以避免扩大跨模块修改范围。

Spec：0 项发现；未发现缺失、行为错误或范围扩张。审查自身不代替执行测试。
