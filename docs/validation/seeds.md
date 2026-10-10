# #12：可复现种子数据与安全重置

## 使用方式

```sh
docker compose build market verify
docker compose up -d mongo --wait
# 完整规模；DB_NAME 必须显式指定非系统实验数据库
docker compose run --rm --no-deps -e DB_NAME=campus_market_experiment market seed \
  --mode full --random-seed 42 --base-time 2026-10-10T00:00:00Z
# 小规模功能数据（另库）
docker compose run --rm --no-deps -e DB_NAME=campus_market_functional market seed \
  --mode small --random-seed 42 --base-time 2026-10-10T00:00:00Z
# 独立压测初始数据（不执行压测）
docker compose run --rm --no-deps -e DB_NAME=campus_market_stress market seed \
  --mode stress --random-seed 42 --base-time 2026-10-10T00:00:00Z
# 显式恢复：仅清理该 DB_NAME，其他数据库/Compose 卷不变
docker compose run --rm --no-deps -e DB_NAME=campus_market_stress market seed \
  --mode stress --random-seed 42 --base-time 2026-10-10T00:00:00Z --reset
# 只读外部观察；替换成要验证的 DB_NAME
docker compose exec -T mongo mongosh --quiet campus_market_experiment \
  --eval "$(cat scripts/check-seed.js)"
```

运行接口时，使用与导入相同的 `DB_NAME` 启动 `market serve`。默认导入超时 15 分钟，可通过 `--timeout 30m` 调整。默认模式 `full`、随机种子 42、基准 `2026-10-10T00:00:00Z`；小规模和完整规模均无隐式日期更新。CLI 成功返回 JSON 数量，不代表 V2 性能结果。

| 模式 | 用户 | 未成交/在售条目 | 已成交商品 | 交易记录 | 改价记录 |
|---|---:|---:|---:|---:|---:|
| full | 10,000 | 20,000 | 180,000 | 300,000 | 500,000 |
| small | 100 | 200 | 1,800 | 3,000 | 5,000 |
| stress | 10,000 | 30,000 | 0 | 0 | 75,000 |
| demo | 2 | 0 | 0 | 0 | 0 |

## 可复现性、时钟与安全边界

- 同模式、随机种子、时间基准恢复相同实验文档。完整/功能/压测数据使用公开演示密码的固定 bcrypt 哈希，目的为可复现，不能用于真实用户账户；`seller`、`buyer` 及其余账号的演示密码均为 `CampusDemo123!`。`demo` 账号夹具仍使用独立随机盐，不承诺字节级复现。
- **真实业务时钟没有冻结**：发布日期为基准前 1–25 天，期限仍是发布后 60 天；随真实时间推进，API 会排除到期商品，TTL 最终删除在售条目。相同基准重建的是相同初始文档，不是永远不变的有效在售集合。超过有效窗口的历史基准无法通过在售数量核验。
- 换机功能验收时选择接近当前 UTC 日期的基准，明确记录该值，后续恢复使用相同值；不要使用远未来基准维持有效性，那不是合法的真实发布时间实验。日期语义不会被 CLI 隐式改写。
- 重置只调用指定数据库的删除，不删除其他数据库或 Docker 卷。无 `--reset` 时任何集合存在数据都会拒绝；参数校验先于删除。不要把生产数据库名传给种子 CLI。
- 导入按批写入，**不是整个百万文档规模的原子事务**。仅在 CLI 成功且 `seed_metadata` 完成标记存在后，才把实验初始状态视为可用；导入期间保持对应 API 停止，失败时显式 `--reset` 重建，不尝试追加或静默续传。
- 这不是 ADR 0005 的成交路径：运行时确认成交仍使用既有事务共同提交商品、资格、成功记录与卖家计数；离线实验导入不对外提供部分成功状态，也不声称具备该运行时原子性。
- 商品含 1–9 张图片、连续的 2–3 次实际改价、不同分类的标量/数组属性、中文错别字与 `bicycles_v1` 同义词数据。`profileChanges`/`categoryChanges` 是种子实验的变化证据，不是新增用户 API；旧教材档案保留旧字符串版次，当前教材规则和在售数据使用数值版次。

## 真实验证记录（2026-10-10）

在 Compose 固定 MongoDB Atlas Local 镜像、Go 1.24.2 工具镜像中，导入独立数据库 `campus_seed_issue12`，使用 `full / 42 / 2026-10-10T00:00:00Z`，CLI 成功。

外部数据库观察：

- users 10,000、products 200,000（未成交 20,000 / 已成交 180,000）、listings 20,000、transactions 300,000、priceChanges 500,000；
- 成功记录 180,000、业务失败 120,000，卖家成交计数合计 180,000；
- 商品图片最小 1、最大 9；每商品改价最小 2、最大 3、平均 2.5；
- `scripts/check-seed.js` 全规模核验的图片数量违规、价格链断裂、成交记录与商品不一致、卖家计数不一致均为 **0**；
- 再次执行同一 CLI 无 `--reset` 返回非零状态并明确拒绝非空库。

真实 HTTP 抽样：完整数据的 `GET /products/000000000000000000000001` 返回有效在售、连续价格历史、错别字描述及卖家当前资料；自行车分类浏览返回合法条目和下一页游标。首次搜索请求返回 503（Search 尚未就绪）；后续请求 `GET /search?q=单车` 返回 200、总数 6,667、前 20 条及标题/描述命中片段。不能把刚导入后的瞬间当作搜索同步保证。

自动化 CLI/数据库与 HTTP 测试还核验小规模全部记录的一致性、同参数重置的完整文档快照相等、改随机种子产生不同数据、旁库哨兵保留、无效重置不修改数据、压测规模，以及种子成功交易的幂等重试。

验证过程中发现旧测试数据库名的时间格式把“纳秒”写成字面量零，导致并发测试进程相互删除同秒数据库；已改为独立 ObjectID 后缀。首次全套运行还因可行性夹具的实际规模与 `SCALE` 标志不一致失败，重新生成大规模夹具并使用 `SCALE=1` 运行。

最终检查通过：

```sh
docker compose build market verify
docker compose run --rm --no-deps -v "$PWD:/src" verify vet ./...
# 8080 已被其他容器占用，验证服务使用同一 Compose 网络内的临时容器，无宿主端口
docker compose run -d --no-deps --name campus-issue12-harness api serve
docker compose run --rm --no-deps api seed --large --reset
docker compose run --rm --no-deps -v "$PWD:/src" -e GIN_MODE=release \
  -e API_URL=http://campus-issue12-harness:8080 -e SCALE=1 verify \
  test -v ./... -count=1 -timeout=12m
git diff --check
```

应用包通过（33.816s），可行性包通过（73.039s），包括真实 Search、事务和 TTL；这不是 V2 正式压测。最终镜像还以相同基准对完整验证库执行 `--reset` 并再次通过只读全规模核验。临时验证 API 已停止，完整验证库保留供观察。

最终双轴审查：Standards 0 项，Spec 0 项。复查另行通过应用包（34.221s）和 `go vet ./...`；此前的明文种子元数据与未来测试日期问题已修复，时间语义和离线批量导入边界已明确记录。

未执行 V2 正式压测，未进行浏览器 Swagger 点击验收。
