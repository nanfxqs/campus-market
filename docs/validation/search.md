# Issue #6：中文同义词搜索

实现分支：`issue-6-chinese-search`。本次只交付 issue #6，合并由用户执行。

## HTTP 契约

`GET /search?q=单车` 无需认证，返回 `total` 与按 Search 相关性排序的前 20 条 `results`。每条包含 `id`、标题、描述、`matchedFields` 和 `highlights`；片段中的 `hit` 表示命中，`text` 表示上下文，均为纯文本。标题和描述可以同时命中，同义词命中也计入字段解释。

关键词必须为非空白、有效 UTF-8，最多 300 字节，不允许重复 q。非法输入返回 400 `invalid_input`；数据库、Search 或 10 秒查询超时返回 503 `unavailable`。Swagger 契约位于 `/openapi.json`，操作入口为 `/docs`。

## 索引与过滤取舍

遵循 ADR 0003，使用固定的本地 `mongodb/mongodb-atlas-local:8.0.4` 镜像及已验证的 digest。`Initialize` 安装 `products_v1` Search 索引：关闭动态映射，标题和描述均采用 `lucene.smartcn`。`synonyms_v1` 集合中的版本化等价组包含自行车、单车、脚踏车；索引引用 `bicycles_v1` 映射。初始化仅在缺少索引时创建，已有同义词记录不覆盖。未来改变分析器或词表应使用新版本并显式迁移。

查询使用 `matchCriteria:any`，任一分析后的词可命中。字段解释来自真实 `searchHighlights`；不手工替换关键词或用字符串包含判断模拟同义词。保留 Search 输出次序，不增加非相关性排序，也不承诺同分结果固定次序。

不在 Search 候选流上提前截取 20 条。应用按最多 500 个候选一批查询 MongoDB 当前在售条目及商品档案，要求条目存在、条目期限未到、档案期限未到且尚未成交。全部候选经过同一资格判断后计入总数，合格的前 20 条才保留为结果，因此失效的前排候选不会阻挡后续有效商品。

内存主要为一个候选批次和 20 条结果；查询代价随全部命中候选数增长，每批增加一次资格聚合和 `_id` 关联读取。精确当前资格总数不能直接使用 Search 的旧收录计数。这是 V1 的正确性取舍，尚非 V2 性能结论。

Search 收录异步；启动不等待每个商品收录，集成测试按预期 HTTP 总数有界轮询，最多 3 分钟，超时报告最后响应。资格校验每批获取当前时间，没有整个请求的事务快照，结果只承诺校验时有效在售。到期过滤不依赖 TTL 是否已经物理删除条目。

## 验证结果（2026-10-09）

沿用父 issue 已确认的真实 HTTP + MongoDB/Search 边界执行 TDD，先观察缺失搜索路由导致失败，然后实现接口。数据库写入仅用于构造尚无对应写入 API 的生命周期夹具，断言通过 HTTP 返回行为完成。

- 三组同义词分别检索标题、描述及双字段案例，检查命中字段与真实 hit 片段；空结果、空白、重复参数与超长输入通过。
- 先等待全部 70 个候选收录，再使原排名前 20 条和另外 25 条失效，覆盖成交、档案到期、在售条目移除、条目到期。停用隔离测试库的 TTL 索引以保留到期证据。随后的单次 HTTP 请求立即返回总数 25 和 20 个有效结果，无二次等待掩盖资格滞后；本次耗时 **24.140575 ms**。
- 搜索定向测试、完整 `go test -v ./... -count=1 -timeout=12m` 及 `go vet ./...` 通过。OpenAPI JSON 和全部内部引用校验通过。完整套件日志见 [issue6-tests.log](issue6-tests.log)。默认 `TestScaleSearch` 跳过，本次未进行大规模或 V2 性能验收。
- 首次全套运行使用共享的 20,000 条夹具，但旧可行性测试按默认 27 条断言，因此失败。保留 [原始日志](issue6-shared-fixture-tests.log)；随后使用独立 Compose 项目 `campus-issue6-check`、独立 MongoDB 和 API、小规模可行性夹具重跑，所有适用测试通过，没有重置共享数据库。
- Standards / Spec 双轴独立审查均为 0 项发现；加强后的立即资格断言经 Spec 复查无新增问题。

可在仓库所在分支运行搜索测试：

```sh
docker compose run --rm -v "$PWD:/src" verify test -v ./internal/market -run TestSearch -count=1
docker compose run --rm -v "$PWD:/src" verify vet ./...
```

完整可行性套件要求夹具规模与 `SCALE` 配置匹配。并行工作时应使用独立 Compose 项目，避免重置他人夹具。Swagger HTML 与契约经 HTTP 测试验证，本次没有进行浏览器交互验证；错别字纠正与逐词评分解释不在本 issue 范围内。
