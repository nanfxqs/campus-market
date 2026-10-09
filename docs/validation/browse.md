# #5 按分类浏览有效在售商品

2026-10-09，真实 HTTP API + MongoDB Atlas Local 8.0.4，沿用 #1 已确认边界。

## 行为与索引

`GET /products?categoryId=textbooks` 公开浏览指定平面分类，按 `publishedAt`、字符串商品 `_id` 倒序；默认 20 条，最多 100 条。查询同时检查档案未成交、档案期限及独立在售资格期限严格大于查询时刻；资格移除的档案不可浏览。过滤资格后才应用 `limit+1`，避免无效条目造成短页或遗漏后续有效商品。返回完整商品档案数组及 `nextCursor`，末页游标为空，空结果数组为 `[]`。

游标使用无填充 base64url JSON，包含分类、毫秒精度发布时间和小写十六进制商品 ID。重复参数、空参数、未知字段、尾随 JSON、错分类和非法游标返回 400；未知分类返回 404。静态数据连续翻页无重复无遗漏；并发新增、修改、成交及到期不承诺分页快照。OpenAPI 记录格式、限制、排序、错误和响应，Swagger 直接使用该 GET 操作。

`products.category_browse`：`{categoryId:1,sold:1,publishedAt:-1,_id:-1}`。分类和成交状态等值前缀后保留游标排序，避免把 `expiresAt` 范围放在排序字段前导致额外排序；期限为残余过滤。查询经 `listings._id_` 等值关联检查资格；`listings.expiresAt_1` TTL 索引只负责物理清理，不承担业务有效性。大量过期档案可能增加扫描成本，此处未宣称 V2 性能验收。

## 验证

- TDD：过滤测试先失败（浏览路由尚不存在），实现后通过；分页测试先失败（默认页返回全部 105 条），加入游标与限制后通过。
- `docker compose run --rm -e GIN_MODE=release -v "$PWD:/src" verify test ./internal/market -run TestBrowse -count=1`：通过。覆盖分类隔离、空结果、成交和资格移除、档案和资格到期、默认/最大分页、相同时间及跨时间顺序、105 条连续分页、非法参数及 Swagger 契约。
- 到期夹具在独立临时测试库暂停 TTL 索引，确保已到期资格实际仍存在，验证业务查询不依赖物理删除；不改变应用或正常数据库 TTL 设置。
- `docker compose run --rm -v "$PWD:/src" verify vet ./...`：通过，针对性测试编译亦完成类型检查。
- 完整套件及复查结果见本文件后续记录和 [issue5-tests.log](issue5-tests.log)。

浏览器工具返回无可用浏览器，不能验证 Swagger 浏览器点击；HTTP 契约及接口由真实请求验证。完整 V1 与 V2 性能验收仍属于后续任务。

## 最终验证与审查

完整套件 `docker compose run --rm -e SCALE=1 -e GIN_MODE=release -v "$PWD:/src" verify test -v ./... -count=1 -timeout=12m` 通过，包含真实 Search、事务与 TTL 观察；原始日志见 [issue5-tests.log](issue5-tests.log)。OpenAPI 经 `uvx --from openapi-spec-validator openapi-spec-validator internal/market/openapi.json` 校验通过。`docker compose up -d --build --wait market` 成功，运行中的 8081 服务浏览接口和 `/docs` HTTP 请求均成功。

## Standards

固定基准 `553a3d6f9df5e54ae073c0bc71c86cea07d32308`，独立子代理审查：未发现规范违反或可操作代码异味。独立资格与显式到期检查符合档案保留和 TTL ADR；游标类型、查询、索引、契约及测试属于同一功能。0 项发现。

## Spec

独立子代理审查：分类隔离、成交/资格移除/到期过滤、默认及最大限制、确定性排序、105 条连续分页、非法输入与 OpenAPI 契约均满足 #5；索引依据已记录，无范围扩张或 ADR 冲突。0 项发现。Swagger 浏览器点击仍受工具无可用浏览器限制。

审查统计：Standards 0 项；Spec 0 项，无遗留代码问题。
