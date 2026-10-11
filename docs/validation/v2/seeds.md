# V2 完整背景与专用商品恢复

`market seed --mode v2` 在 V1 完整背景后追加 30,000 件专用商品；不用无历史记录的 stress 模式替代。背景 ID、生成顺序及随机序列不变，50 名已有卖家各有 600 件专用商品，合法买家是另一已注册用户。所有账号仍复用 10,000 名背景用户。`seedCohort=background|dedicated` 是离线实验标记，不增加公开 API 字段。

| 数据 | 背景 | 专用扩展 | 初始总量 |
|---|---:|---:|---:|
| 用户 | 10,000 | 0 | 10,000 |
| 有效在售商品 | 20,000 | 30,000 | 50,000 |
| 历史成交商品 | 180,000 | 0 | 180,000 |
| 交易记录 | 300,000 | 0 | 300,000 |
| 改价记录 | 500,000 | 75,000 | 575,000 |

沿用 V1 的 1–9 张图片、2–3 次连续改价、异构属性、卖家资料变化、中文错别字与自行车同义词、旧版教材属性证据。专用商品也具有这些资料。所有产品资料与日期通过固定随机种子、显式 UTC 基准重建；不是冻结业务时钟。V2 拒绝未来基准及剩余资格少于 24 小时的基准，参数核验先于删除。

## 恢复入口

```bash
COMPOSE_PROJECT_NAME=campus-v2-experiment DB_NAME=campus_v2_experiment \
  BASE_TIME=2026-10-11T00:00:00Z RESULT_DIR=/tmp/v2/A-1 \
  scripts/restore-v2.sh A --reset
# B 开缓存；C 是 Nginx 与两实例，其他参数完全相同。
```

数据库与项目名必须显式传入并使用实验前缀。无 `--reset` 时非空库拒绝导入。脚本停止同项目全部业务实例，初始化 MongoDB，导入，仅清空该项目专用 Redis 的当前数据库，执行只读规模/关联/状态/卖家计数及清单对应核验，最后启动选定服务。任何失败保留停止状态和原始输出。脚本不删除 Docker 卷、不操作其他项目、不尝试恢复部分导入。

准备阶段输出 `import.txt`、`cache-reset.txt`、`check-seed.txt`、`check-manifest.txt`、`check-manifest-db.txt`、`start.txt` 与 `preparation.json`；准备耗时不进入正式成交窗口。恢复清空 sessions，因此负载必须重新登录；Search 收录等待、登录与预热由负载准备流程另记。

`manifest.json` 为 v1 格式：`database/mode/randomSeed/baseTime/counts` 标识数据；`accounts` 是已注册用户的公开演示凭据及历史 `completedSales`；`products` 只含 30,000 件专用商品，记录 `productId/sellerId/buyerId/priceCents/expiresAt/idempotencyKey`。负载只需登录清单实际引用的 50 名卖家；这些凭据仅供合成实验数据使用。每商品唯一分配，重试保持同键同内容。manifest 不包含登录令牌。

## 真实独立验证

```bash
BASE_TIME=2026-10-11T00:00:00Z scripts/validate-v2-seed.sh
```

使用独立 `campus-v2-seed15` 项目和数据库，完整导入后执行数据库外部只读核验，拒绝非空追加和无效重置，保留旁库哨兵，再完整恢复，比较清单一致。初次红灯 CLI 与绿灯/完整核验原始材料位于 `docs/validation/v2/seed/`。完整清单按同参数生成，避免在仓库复制重复大型合成凭据；原始运行路径及 SHA256 在记录中保留。此验证不替代 #14 九轮正式性能实验。

最终合并真实 Redis 实现后的 B 恢复耗时 71 秒；恢复前详情响应实际为 miss/hit，恢复后首次读取 miss、第二次 hit，原令牌返回 401，重新登录及认证资料读取返回 200。完整初始数量及全部只读一致性检查通过。`seed/green-cli.txt` 的两轮完整恢复还核验八个集合规范化 SHA256 一致、清单一致及旁库保留；重复导入和核验准备耗时 110 秒。独立 V1 种子 CLI/重置/随机种子/完整小规模与 stress 回归通过（10.400 秒）。这些是准备与正确性证据，不是正式负载成绩。
