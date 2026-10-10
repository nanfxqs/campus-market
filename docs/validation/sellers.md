# Issue #9：卖家主页验证

## 接口与范围

公开 `GET /users/{id}/home` 返回 `profile`（复用公开资料序列化，不返回
`passwordHash`）、`creditScore`、`completedSales`、`currentOnSaleCount`、
`items`、`nextCursor`。资料每次读取当前用户记录，不使用商品发布时的资料副本；
信用分保持预设值，成交数复用成交事务维护的用户计数。

- 商品跨分类，但仅限路径指定卖家。
- 复用分类浏览的资格规则：商品未成交、商品档案期限严格大于请求时间，
  且同 ID 在售条目存在、其期限也严格大于该时间。TTL 尚未删除的到期条目不合格。
- 复用浏览分页解析：`publishedAt`、`id` 降序，默认 20，范围 1–100，
  严格 Base64URL/JSON 校验、毫秒时间精度、规范 ObjectID 字符串。
- 游标绑定卖家；不同卖家、分类浏览与主页之间不能混用。
  重复参数、空/非法/超长游标、非法 limit 返回 `400 invalid_input`；
  不存在用户返回 `404 user_not_found`，数据库错误返回 `503 unavailable`。
- `currentOnSaleCount` 是分页前完整合格集合的数量，不是当前页长度或剩余数量。
  空集合返回 `items: []`、`nextCursor: ""`。

资格流水线是分类浏览与主页的同一个函数；主页通过 `$facet` 从合格集合分出
count 和 page，游标与 limit 仅作用于 page。没有新增在售数量缓存或写入维护路径，
避免成交与 TTL 变化造成另一套计数不一致。

**一致性边界：** 用户资料/成交数和商品聚合是两次读取，不提供整个主页的事务快照；
并发成交发生在两次读取之间时可能短暂呈现不同读取时刻。翻页同样是实时查询，
不是跨请求快照。成交接口成功、幂等重放、竞争或回滚完成后再次读取的结果已验证。
本次不扩展 V2、重上架、信用评分计算或新的成交流程。

## 查询结构与索引理由

`Initialize` 添加幂等创建的 `products.seller_browse`：

```text
{sellerId: 1, sold: 1, publishedAt: -1, _id: -1}
```

卖家和未成交状态是等值前缀，后两个字段匹配确定的浏览顺序。
现有 `category_browse` 首列是 categoryId，不能代替跨分类的卖家查询。
到期条件是残余过滤而非排序前的索引范围字段，避免把 expiresAt 放在排序字段前
破坏此顺序。`listings` 通过既有 `_id` 索引关联；既有 expiresAt TTL 索引仅负责
物理清理，不作为出售资格的来源。

完整 count 必须遍历卖家的合格候选，不能只读 limit+1 行；facet 的游标过滤在
资格过滤之后，因此 count 不随翻页变化。该结构优先保证资格一致，未声称查询
只扫描一页或达到某个吞吐目标。此轮未运行大规模种子、explain 或性能基准。

## 已确认测试 seam 与覆盖

测试使用真实 HTTP（`httptest.NewServer` + HTTP client）和 Compose 中真实
MongoDB 事务；不 mock repository，不增加导出的测试 API。数据库仅用于设置
受控夹具/拒绝事务写入，主页结果均通过 HTTP 观察。

- `TestSellerHomeCurrentProfileAndIsolation`：本人资料 PATCH 后公开主页立即更新，
  不泄露密码散列；不同卖家商品隔离、信用分和初始成交数、未知用户。
- `TestSellerHomePaginationAndInvalidInput`：105 行跨分类夹具，发布时间优先于 ID、
  同时刻 ID 稳定排序；默认/最大页、全部分页无重复遗漏、每页完整 count、
  空卖家及非法输入、卖家/浏览游标作用域隔离。
- `TestSellerHomeExcludesUnavailableBeforeTTLCleanup`：在独立测试数据库停用 TTL
  清理，保留已到期条目；排除已成交、下架、档案到期和条目到期；
  剩余商品到期后 count/list 同时归零，不增加成交数、不改变信用分。
- `TestSellerHomeOpenAPI`：公开操作、路径及分页参数、响应状态。
- 既有成交 HTTP 测试增加主页观察：成功前后、成功幂等重放、同键/不同键
  24 请求竞争、失败重放、最后写入失败的完整事务回滚及恢复后成功、
  两件商品同键竞争（仅一件成交，另一件仍在售）。不另建一套竞争测试 harness。
- 分类浏览测试随定向测试重跑，验证共享规则提取不改变原浏览契约。

## Red → green 与执行记录

先运行首片主页 HTTP 测试，缺失路由导致失败；实现公开路由、读取和共享资格/
分页后通过。复用分页/资格的兼容性测试通过；OpenAPI 测试先因缺少操作失败，
补充契约后通过。之后在既有成交场景中增加主页断言并重跑。

宿主没有 Go，使用 tools 镜像、挂载当前工作树：

```sh
docker compose run --rm --no-deps -v "$PWD:/src" verify test ./internal/market \
  -run 'TestSellerHome|TestBrowse|TestSaleConfirmationAndReplay|TestSaleConcurrentExactlyOnce|TestSaleRollbackOnPersistenceFailure|TestSaleConcurrentKeyConflictRollsBackOtherProduct' \
  -count=1 -timeout=4m
docker compose run --rm --no-deps -v "$PWD:/src" verify vet ./...
git diff --check
```

最终定向测试通过（`ok .../internal/market 7.525s`），vet 通过，
diff 空白检查通过。Go 文件使用 tools 镜像中的 gofmt 格式化。

执行限制与异常：

- 默认 Compose verify 依赖的现有 api 容器退出，首次命令被依赖健康检查阻塞；
  改用 `--no-deps` 连接已健康的 MongoDB。测试仍在当前源码启动真实 HTTP server，
  并不借用旧 api 镜像的路由。
- 一次误并行启动两份同一定向测试，现有共享 setup 的秒粒度数据库名称碰撞，
  出现重复键/数据库正在删除错误；单份顺序重跑两次通过。本次没有扩大范围修改
  通用夹具，测试进程需串行使用此 MongoDB。独立测试进程数据库命名可另行改进。
- 本轮没有运行完整套件、code-review、大规模 Search/部署复现，也未人工打开浏览器
  操作 Swagger；这些不由定向 HTTP 测试或 vet 替代。完整套件和 review 由父代理执行。
  TTL 测试证明清理前资格正确，不声称本轮重新验证了 TTL 实际删除时序。
- 没有 commit。
