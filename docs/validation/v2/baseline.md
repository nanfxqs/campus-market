# 配置 A 三轮正式成交基线入口

`scripts/run-formal-sales.py` 统一执行 A/B/C 任一配置的三轮独立恢复、负载与核验。#18 实际结果仅运行 A：单 Gin、关闭 Redis 资料缓存、HTTP 直连。B/C 复用入口，由相应任务另行保存结果。每轮必须独立满足 50 名实际就绪用户、配置截止 600 秒内至少 30,000 件专用商品新增按时成交、数据库一致性与完整资源采样；不使用平均或最佳轮代替失败。

```bash
scripts/run-formal-sales.py --configuration A \
 --project campus-v2-baseline18 --database campus_v2_baseline18 --port 8088 \
 --base-time 2026-10-11T00:00:00Z --random-seed 42 \
 --output docs/validation/v2/baseline/A-attempt-1
# 原始结果可以独立汇总，任何缺轮/失败轮使退出码为1。
scripts/run-formal-sales.py --summarize \
 --output docs/validation/v2/baseline/A-attempt-1
```

每次运行使用全新的结果目录；调优或复跑另建 attempt，全部失败输出保留。项目、数据库、固定随机种子及显式时间基准不随轮次改变。应用默认 2 CPU/512MiB、MongoDB/Search 共用容器默认 4 CPU/8GiB；Redis 虽停用业务缓存，仍是实验共用专用实例，默认 1 CPU/256MiB、128MiB/allkeys-lru。实际 Compose 配置、容器/image ID、机器/版本/代码状态及全部存储由同一采集器保存，不把共用容器内存重复写为 MongoDB 与 Search 两份。

每轮先停止业务、完整恢复 10,000 用户/20,000 基础在售/180,000 历史成交/300,000 交易记录与30,000 专用商品，清空专用缓存及 sessions，再只读核验后启动。规范化 SHA256 检查八个集合的初始逻辑文档跨轮完全一致；完整准备耗时单列。采集至少 10 秒空闲资源与压测前存储。负载程序完成健康/Search 收录与重新登录，真实 Locust PID 出现后挂接 1 Hz 采集器，采集器 ready 后释放挂接门，再由 50 位用户的实际就绪屏障开始成交。

600 秒是配置的硬收讫截止，库存耗尽可提前结束。每轮同时保留 `configuredCutoffSeconds`、`measuredWindowSeconds` 与 stopReason；吞吐量以实际测量长度为分母。采集持续至负载及有限提交观察/数据库核验结束，资源汇总仅裁剪实际 `startUnixNs..measurementEndedUnixNs`，登录、启动、准备及 settlement 不进入负载平均值。压测后数据库、Search 文件/卷及 Redis 实际内存/持久化状态继续以相同方法保存。

`round-N/result.json` 链接独立数量、错误分类、延迟、数据库一致性、逐卖家增量、资源覆盖与准备时间；`sale/` 保留 Locust 日志、逐请求 gzip、前后外部快照、窗口及完整独立核验；`idle/` 与 `resources/` 保留采集命令/原始输出/环境/每秒样本/存储和窗口汇总。每轮结束停全部项目服务但保留卷。恢复清单在负载结束后 gzip 原样保存，避免重复的大型明文 JSON。汇总器不改写旧轮的原始事实。

编排阶段完成 artifact 红灯/绿灯测试：某轮 29,999 即使其他两轮 30,000 与30,001，整组也必须失败。正式实验尚待独占运行授权；只有实际完成并核验后填入结果，不能把代码入口或短链路当作 #18 通过。
