# Locust 成交入口与独立结果核验

使用真实 Locust 2.46.6，Python 3.12（验证为 3.12.14）；完整传递依赖锁在 `experiments/uv.lock`。通过 `uv run --frozen --project experiments` 使用固定版本。Python 标准库负责外部脚本及统计，不把私有成交/缓存函数作为测试边界。

```bash
# 每轮先恢复；清空业务事实、缓存和登录状态，准备耗时另外保存。
COMPOSE_PROJECT_NAME=campus-v2-experiment DB_NAME=campus_v2_experiment \
 BASE_TIME=2026-10-11T00:00:00Z RESULT_DIR=/tmp/v2/A-1-restore \
 scripts/restore-v2.sh A --reset
# 正式入口：A/B 的直接地址、C 的 Nginx 地址由 --api 显式指定。
scripts/run-sale-load.py --formal --configuration A \
 --project campus-v2-experiment --database campus_v2_experiment \
 --api http://127.0.0.1:8082 --manifest /tmp/v2/A-1-restore/manifest.json \
 --output /tmp/v2/A-1-sale
# 短运行改变窗口和专用商品数量，不声明正式通过；仍为50个虚拟用户。
scripts/run-sale-load.py --configuration B \
 --project campus-v2-experiment --database campus_v2_experiment \
 --api http://127.0.0.1:8082 --manifest /tmp/v2/B-short-restore/manifest.json \
 --output /tmp/v2/B-short-sale --duration 10 --products 1000 --settle 25
# 可独立重算全部结果
python3 experiments/summarize_sales.py --run-dir /tmp/v2/A-1-sale \
 --manifest /tmp/v2/A-1-restore/manifest.json
python3 -m unittest discover -s experiments -p test_sales_artifacts.py
```

结果目录必须为空，旧失败、调优与成功运行均保留。正式参数固定 50 用户、600 秒、30,000 件专用商品，settlement 至少 20 秒；默认 25 秒。配置 A/B/C 只是对应环境标识，地址必须指向该环境。脚本不启动/重置环境，也不改变认证或成交 API。

## 就绪、分配与时钟

负载前只读核验实际完整背景、30,000 件专用未成交且有效商品、合法买卖双方、唯一商品/幂等键及全部卖家基线计数。等待 Search 对全部有效自行车商品的预期总数 16,667 收录；超时保留失败。之后每位虚拟用户分配一名已有卖家及专属串行队列。所有 50 位用户完成登录、验证令牌期限覆盖窗口、完成队列准备，才释放 gevent Event。

`spawning_complete` 不作为就绪依据。就绪记录保存卖家、准备时间、资格期限和分配数量，不保存令牌。最后一位就绪后重置 Locust HTTP 统计、记录统一 monotonic 起点并释放屏障；正式 600 秒不使用 Locust 的启动即计时 `--run-time`。时间截止由 monotonic 控制，UTC 纳秒起止用于关联外部采样。零思考时间，每位用户同时只有一个成交请求。所有 50 名卖家的队列耗尽且最后请求结束后可提前结束，或者到配置截止结束。`durationSeconds` 是配置的硬截止（正式 600），`measuredWindowSeconds` 是实际测量长度，`stopReason` 区分 `pool_exhausted/deadline`，`lastOnTimeSuccessSeconds` 单列最后按时成功时刻。不会强制库存耗尽后空闲至 600 秒。

基础设施错误/网络超时最多立即重试两次，保持同商品、买家、价格和幂等键。409 业务冲突及其他非重试错误独立报告。开始新请求前严格检查截止；已发出的请求允许有限超时完成，截止后响应继续留存，不能进入按时成功。

可以用 `--start-gate /tmp/collector-ready` 让真实 Locust 进程等待采集器挂接。`client-pid.json` 给出实际 Locust PID；采集器准备后创建 gate，再开始登录及 50 人就绪流程。采集器的准备和挂接均不进入成交窗口。

## 原始材料与统计口径

`requests.jsonl.gz` 每条包含商品、卖家、尝试号、monotonic 相对发出/收讫时间、UTC 纳秒、端到端毫秒、HTTP 状态、原始成交结果和传输错误。`window.json` 保存屏障、窗口及最终结束；`locust.log`、CSV、HTML 和退出码保留实际工具输出。`before.json.gz`/`after.json.gz` 是完整专用商品、资格、成功记录、逐卖家计数及全库数量的只读外部快照。准备、Search 收录和登录耗时另存。所有请求和快照 gzip 保存，不复制登录令牌。

统计从原始文件独立重算。仅 HTTP 200、`success=true/code=sold` 且商品/卖家/买家/成交价/幂等键与分配相符的响应算成功；monotonic 收讫时刻位于 `[0,duration]` 才进入按时唯一新增成交。重复响应及已存在于前快照的历史结果不增加成交数。截止后成功、超时但数据库提交、未提交/未知及未发起的商品分别列出。

端到端请求吞吐量按窗口内收到响应的请求数除以实际 measuredWindowSeconds；另外列出所有窗口内发出的请求及其截止后完成的延迟样本。唯一新增成交吞吐量按唯一按时成功商品数除以实际 measuredWindowSeconds。配置截止仍独立决定响应是否按时。P95/P99 使用逐请求真实毫秒的 nearest-rank；成功、业务冲突和其他失败分别提供样本数、延迟和错误率。Locust 自带直方图百分位为辅助近似，正文统一用原始统计。测得的请求错误保留，不给错误率另加通过阈值；`--exit-code-on-error 0` 仅避免已重试并完成的测量错误替代 spec 通过条件，准备/用户运行异常仍退出非零。是否通过由唯一按时新增数量和持久化一致性裁决。前后核验每个新增商品的一条成功记录、成交内容、资格移除、全部卖家计数增量和全库增量，重复、部分提交、错关联或未分配商品变化均使一致性失败。

## 已执行的短链路

2026-10-11：完整 V2 初始背景上，B 配置、50 位真实 Locust 用户、10 秒窗口、1,000 件专用商品：按时唯一新增 1,000；请求错误/冲突/重试为 0；P95 145.551ms、P99 169.746ms；窗口吞吐量 100 请求/s、100 新增成交/s。前后数据库核验成功，全部 50 名卖家各增加 20，新增成功记录/已成交商品各 1,000，资格减少 1,000；基础历史未计入。10 秒的完整采样由 #17 独立采集器核验。

这是可配置短链路验证，`formalPassed=false`，没有宣称 30,000/600 秒或九轮正式验收通过。原始材料在 `docs/validation/v2/load/short2/`。首次启动因 Locust 只接受整数时间跨度，`--stop-timeout 30.0` 被 argparse 拒绝，未开始成交；失败原始材料保存 `short1/`，改用 `30s` 后运行通过。六个独立字面量 artifact 测试覆盖截止等号、重复/晚响应、冲突与基础设施错误、超时最终提交、历史幂等结果排除、部分提交拒绝及50名不同用户就绪记录；红灯/绿灯记录也保留。

官方工具契约： [Locust 2.46.6 API](https://docs.locust.io/en/2.46.6/api.html)、[无 Web UI 运行](https://docs.locust.io/en/2.46.6/running-without-web-ui.html)。

随后在再次完整恢复的数据上实际验证提前完成：short3 配置截止 10 秒，50 名用户就绪，1,000 件全部完成后 `pool_exhausted`；实际测量 1.519498 秒、按时唯一新增 1,000、实际吞吐量 658.112 请求/s 与新增成交/s，P95 90.538ms、P99 104.305ms，错误为 0，全部数据库一致性通过，`formalPassed=false`。与保留的 short2 完整 10 秒统计分别报告，没有混用分母。短运行完成后已停止验证项目全部服务并保留卷和原始数据，给后续独占正式实验释放资源。
