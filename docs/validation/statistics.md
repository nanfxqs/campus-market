# #10 分类新旧程度统计

`GET /categories/{id}/condition-counts` 是公开接口，Swagger `/docs` 中的 `categoryConditionCounts` 操作可直接调用。返回 `categoryId` 和固定顺序的 `items`（全新、几乎全新、轻度使用、明显使用），每项包含 `condition` 与 `count`。空分类四档均为零，未知分类返回 404，超过 64 字节的分类 ID 返回 400，数据库失败返回 503。

统计从 `products` 筛选指定平面分类、`sold:false`、`expiresAt > 查询时刻`，通过商品 ID 关联 `listings`，要求资格存在且资格期限也严格大于同一查询时刻，再按新旧程度分组。独立资格消失表示下架；TTL 只删除资格，不删除商品档案。查询不承诺并发写入的请求级事务快照。

复用 `products.category_browse` 的 `{categoryId:1,sold:1}` 等值前缀筛选，档案期限为残余过滤；通过 `listings._id_` 唯一索引等值关联资格。`listings.expiresAt_1` TTL 索引只负责物理清理，业务正确性不依赖其调度。无需 Search 索引，不新增统计索引或缓存；大量同分类档案可能增加扫描成本，V2 是否优化应基于实测瓶颈。

## 验证

沿用 V1 spec 已确认的真实 HTTP + MongoDB 测试边界。TestConditionCounts 首先因统计路由不存在失败，实现后通过。测试覆盖四档 1/2/3/4、分类隔离、零档位、空分类、未知分类、已成交、资格移除、档案及资格到期。临时测试库移除 TTL 索引，确保过期资格未被清理时仍被业务查询排除。TestConditionCountsOpenAPI 检查操作入口、四档数组约束、状态码及超长分类输入。

针对性命令：`docker compose run --rm -e GIN_MODE=release -v "$PWD:/src" verify test ./internal/market -run TestConditionCounts -count=1` 通过。
`docker compose run --rm -v "$PWD:/src" verify vet ./...` 通过；测试编译完成 Go 类型检查。
分支 `issue-10-condition-counts`，基准 `e39b2bdda1ded25627d86397b30c3695d3a230d2`，独立 worktree `/tmp/campus-market-issue10`。完整验证使用 Compose 项目 `campus-issue10-check` 与独立数据库卷，临时端口覆盖文件 `/tmp/issue10-compose.yaml` 将 API 发布到 18010。

完整套件原始输出见 [issue10-tests.log](issue10-tests.log)。本次未执行 Swagger 浏览器交互，入口由现有 Swagger 页面及 OpenAPI HTTP 契约覆盖。

## Standards

独立子代理审查：0 项规范违规。原有 1 项可选维护建议已修复：`conditions.go` 集中定义四档有序列表，发布校验和统计共同使用，测试保留独立的预期值。

## Spec

独立子代理审查：0 项遗留问题；最初指出的统计索引与结构说明已补齐并复查确认。无范围扩张或 ADR 冲突。

独立环境首次完整运行因未导入 validation 规模夹具失败，应用包全部通过。按 `scripts/validate.sh` 导入 `api seed --large --reset`：10,000 用户、20,000 在售、180,000 历史、300,000 交易记录；随后重跑完整套件。OpenAPI 官方结构校验工具 `openapi-spec-validator` 校验通过。

最终完整套件通过：`docker compose -p campus-issue10-check -f compose.yaml -f /tmp/issue10-compose.yaml run --rm -e SCALE=1 -e GIN_MODE=release -v /tmp/campus-market-issue10:/src verify test -v ./... -count=1 -timeout=12m`。应用包 8.606s，validation 包 57.001s；真实 TTL 在到期后约 45.246s 观察到资格物理删除，档案与历史保留。独立环境最终 `go vet ./...` 亦通过。

2026-10-10 枚举去重验证：完整应用测试包 `go test ./internal/market -count=1` 通过（12.155s），包含品相非法值拒绝、四档统计及契约测试；`go vet ./...` 通过。此处为应用包验证，未重跑独立 validation 环境套件。
