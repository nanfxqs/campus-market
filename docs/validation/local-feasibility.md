# V1 本地搜索、事务与 TTL 可行性验证（Issue #2）

本目录记录先行可行性实验。`validation/` 是独立验证入口，不是完整 V1 应用；遵循 ADR 0003–0005，不使用外部 Atlas、替代搜索或应用定时扫表。认证、分类、完整种子交付和 Swagger 属于后续任务。

## 复现

需要 Linux amd64 Docker Engine 和 Compose、可拉取镜像与 Go 模块的网络，以及空闲本地端口 8080。Go 编译与 Go testing 均在容器内执行，不要求宿主安装 Go。首次镜像拉取、模块下载和 Search 建索引可能较慢。

```bash
# 一条命令启动 Go/Gin + MongoDB + Search，并等待健康检查。
docker compose up -d --build --wait

# 首次仅允许空实验数据库；执行小案例后重建规模夹具并运行全套。
scripts/validate.sh

# 再次执行时显式授权重置专用实验数据库。
scripts/validate.sh --reset
```

脚本会先执行小规模搜索、事务及 TTL 案例，随后导入规模夹具并运行全套 Go testing；完成后环境保留，可继续请求 API。所有数据库连接仅允许 `DB_NAME=campus_validation`，`--reset` 只删除该数据库，不能作用于其他数据库。默认再次导入非空库报错。测试必须独占这个实验环境，不要并行运行多套脚本。

单独运行：

```bash
docker compose build verify
docker compose run --rm api seed --reset
docker compose run --rm verify vet ./...
docker compose run --rm verify test -v ./validation -run TestChineseSynonyms -count=1
docker compose run --rm verify test -v ./validation -run TestSearchChecksCurrentEligibility -count=1
docker compose run --rm verify test -v ./validation -run TestTransactionCommitAndRollback -count=1
docker compose run --rm verify test -v ./validation -run TestTTLRetainsArchiveAndForbidsSale -count=1
docker compose run --rm api seed --large --reset
docker compose run --rm -e SCALE=1 verify test -v ./... -count=1 -timeout=12m
curl 'http://localhost:8080/search?q=单车'
```

`FIXTURE_TIME` 可通过 `docker compose run --rm -e FIXTURE_TIME=2026-10-09T06:00:00Z api seed --large --reset` 指定 RFC3339 时间基准。确定性的序号生成规则没有随机来源，相同基准与模式生成相同业务文档（Search 内部元数据除外）。在售期限由基准加 60×24 小时生成；使用超过 60 天的旧基准会使在售规模就绪检查失败。规模夹具用于验证，不代替完整 V1 种子账号、图片、异构分类等交付。

停止使用 `docker compose down`，保留卷。实验卷不包含其他项目数据。API 只绑定 127.0.0.1:8080，MongoDB 不映射宿主端口。验证接口使用固定卖家 `seller` 与买家 `buyer`，无用户登录，不用于实际业务服务。

## 方案与可观察证据

- MongoDB Atlas Local 是包含 MongoDB 和 Search 的单节点副本集；固定 `8.0.4` 及实际拉取的 SHA256，Go `1.24.2-bookworm` 和运行时 Debian 也固定摘要。
- `products_v1` 对标题和描述使用 `lucene.smartcn` 中文分析器；`synonyms_v1` 中的 equivalent 映射为自行车／单车／脚踏车，映射名 `bicycles_v1`。响应直接返回 Search 原生 highlight 文本片段（`hit`/`text`），`matchedFields` 从命中片段的路径得到。依据 [MongoDB 同义词配置文档](https://www.mongodb.com/docs/search/indexes/synonyms/) 和 [高亮文档](https://www.mongodb.com/docs/search/query/highlighting/)。
- 索引就绪由有界轮询实际搜索验证，等待命中总数达到预期；没有用固定 sleep 假定完成同步。超时应查看 `products_v1`、同义词集合以及 `docker compose logs mongo`。
- Search 返回相关性顺序的全部候选，每 500 条检查当前独立在售条目（到期时间严格大于检查时间）及当前商品未成交状态。全部合格候选累加总数，按原顺序收集前 20 条；不先截断候选，不采用过滤前 Search 总数。内存仅保留一批候选和前 20 条，代价随匹配候选数线性增加；每批观察时间不同，不承诺请求级事务快照。响应附 `candidates`、`elapsedMs` 作为实验诊断字段。
- 小夹具有 27 个有效匹配和 40 个故意无效的 Search 候选，含已成交条目、已过期条目及 TTL 删除后缺少条目的商品。索引就绪后额外改变在售资格的测试验证当前数据库事实优先于 Search 收录状态，仍返回满 20 条。
- 成交请求 `POST /products/:id/confirm` 使用 `{"buyer":"buyer","priceCents":8500,"key":"example"}`。在同一真实数据库事务内删除有效在售条目、条件更新商品、插入成功记录及增加卖家计数。通过事务重试处理写冲突。固定卖家内同键同内容可复用成功结果，同键不同内容冲突；完整业务失败记录与身份验证不属于该最小实验。
- 事务失败注入使用真实 `transactions` 集合 validator，拒绝 `rollback-case` 的写入；HTTP 返回失败后检查商品未成交、在售资格保留、无成功记录、计数未变。validator 在测试清理中恢复。八个请求竞争一个商品只允许一个成功，并观察四项持久化事实。
- `listings.expiresAt` TTL 索引 `expireAfterSeconds=0` 只删除在售条目。TTL 测试临时将索引清理延迟设为 3,600 秒，预置近期到期商品；到期后先观察在售条目仍存在，验证 HTTP 成交被拒绝且所有持久化事实未变，再恢复 `expireAfterSeconds=0`，有界轮询观察物理删除以及商品和改价历史保留。清理中再次恢复索引，以免失败影响后续实验。3 分钟是实验诊断超时，不是业务清理 SLA，没有应用清理线程。

## 实测记录

环境：2026-10-09，Linux amd64，Docker Engine 29.8.2、Compose 5.6.0，宿主内存约 30 GiB。镜像和模块版本见 Compose、Dockerfile 与 go.mod/go.sum；镜像摘要固定，版本兼容性仅限本次实测环境。没有进行 V2 正式压测或资源验收。

小规模首次实测：索引就绪后 `单车` 查询扫描 67 个候选，总数 27、返回 20；Go 测试三个同义词均通过（0.14 秒）。Search 原生高亮覆盖标题、描述及同时命中。事务失败案例初次发现小夹具未创建空交易集合，已修正并通过重新验证；修正后的事务竞争与回滚测试约 0.06 秒。首次 TTL 观察到到期后约 39.10 秒物理删除，档案及两项历史仍保留（整个测试约 44.11 秒）。

完整 `scripts/validate.sh --reset` 于同日通过：

| 案例 | 实测结果 |
| --- | --- |
| 小规模 Search 就绪 | 67 候选，27 有效，254 ms |
| 小规模三同义词／当前资格变化 | 均通过；资格改变后总数 24，返回满 20 条 |
| 小规模事务竞争与回滚 | 通过，约 0.06 s |
| 小规模 TTL | 到期后约 26.07 s 删除 |
| 规模导入 | 10,000 用户、20,000 有效在售、180,000 历史成交、300,000 交易记录；额外 40 个失效候选 |
| 规模 Search 就绪 | 20,040 候选，20,000 有效，2,226 ms |
| 规模 Search 三次服务端查询 | 1,365／1,447／1,402 ms；每次总数 20,000、前 20 条 |
| 规模当前资格变化 | 20,040 候选，19,997 有效，前 20 条 |
| 规模 TTL | 到期后约 32.17 s 删除，档案与历史保留 |
| 完整 Go 测试套件 | 五个测试均通过，47.786 s；Go vet 通过 |
| 非空库导入保护 | 再次 `api seed` 返回非零，提示必须显式 `--reset` |

[原始测试与导入输出](run-2026-10-09.log) 保留命令实际输出中的测试及就绪行。通过数据库外部观察确认 MongoDB 版本 `8.0.4`、副本集名 `mongo`、`expiresAt_1` 索引 `expireAfterSeconds: 0`。

结论：本次环境中的中文分析、指定同义词原生片段、全候选当前资格过滤、真实多文档事务和独立在售条目 TTL 可行，不需要重开 ADR。匹配范围较大的搜索需要全候选扫描，实测约 1.4 秒，后续 V1 应保留此成本约束，V2 再进行正式负载与优化评估；本实验不承诺吞吐量、P95/P99 或固定 TTL 清理时限。

代码评审指出原 TTL 测试没有明确证明成交拒绝时条目尚未删除，已按上述受控清理流程加强；下方补充修正后实测证据。

修正后再次执行 `docker compose run --rm verify vet ./...` 与 `docker compose run --rm -e SCALE=1 verify test -v ./... -count=1 -timeout=12m`，均通过。[原始回归输出](run-2026-10-09-review.log) 明确记录：已过期条目仍物理存在，HTTP 拒绝成交且没有写入；恢复正常 TTL 后约 53.40 秒观察到物理删除，档案与历史仍保留。五个测试全通过，套件用时 68.640 秒；三次规模搜索服务端耗时 1,374／1,252／1,288 ms。测试结束后 TTL 索引仍为 `expireAfterSeconds: 0`。

## Standards

没有违反已记录标准的情况。一个非阻塞启发式建议：`seed` 的用户、商品、条目与交易循环重复执行批次阈值和错误处理，未来夹具扩大时可考虑提取批写入辅助结构；当前 `flush` 已集中数据库插入，保留显式循环更便于阅读。0 个标准违规，1 个可选重复代码建议。

## Spec

首次评审发现 TTL 测试未证明拒绝发生在物理删除之前，已通过受控延迟、前后数据库观察和实际清理回归修正。复审无剩余发现、无范围扩张；其余搜索、事务回滚、规模夹具与测量符合 Issue #2。0 个未解决发现。

## 批量写入优化回归

后续已落实 Standards 评审的可选重复代码建议：`insertFixture` 统一每 1,000 条写入、末批写入及错误传播，各集合仅定义文档生成规则。空夹具不执行写入；成功写入后清除缓冲区引用。四类夹具内容与规模保持一致。

重新执行 `scripts/validate.sh --reset` 成功：Go vet、小规模搜索／事务／TTL 案例均通过；规模夹具重建后五个测试全部通过，完整套件耗时 50.133 秒。规模断言涵盖 10,000 用户、200,040 商品（包含 40 个无效候选）与 300,000 交易记录，验证整批与末批导入。见[原始优化回归输出](run-2026-10-09-batching.log)。
