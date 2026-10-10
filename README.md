# campus-market

校园二手交易平台课程项目，已提供本地搜索、事务与 TTL 的 Go/Gin 可行性验证入口；完整 V1 功能仍在实施。

- [项目要求](docs/assignment.md)
- [版本路线图](ROADMAP.md)
- [V1 功能与测试 spec](https://github.com/nanfxqs/campus-market/issues/1)
- [开发与验收计划](docs/development-plan.md)
- [领域术语](GLOSSARY.md) 与 [架构决策](docs/adr/)

本地验证环境：`docker compose up -d --build --wait`。首次运行完整实验：`scripts/validate.sh`；重跑使用显式 `--reset`。见[复现步骤与实测记录](docs/validation/local-feasibility.md)。

## 可复现种子数据（#12）

```sh
docker compose build market
docker compose up -d mongo --wait
# 默认完整规模；仅导入指定的空实验库
docker compose run --rm --no-deps -e DB_NAME=campus_market_experiment market seed \
  --random-seed 42 --base-time 2026-10-10T00:00:00Z
# 恢复相同初始状态：原命令加 --reset，仅删除 DB_NAME 指定的库
```

默认 `full` 为 10,000 用户、20,000 在售商品、180,000 已成交商品、300,000 交易记录；`--mode small` 为其 1/100 规模，适合功能验证。`--mode stress` 在独立数据库生成 30,000 件可成交商品，不执行 V2 压测。`--mode demo` 仅生成两个演示账号。

**时间基准不等于冻结时钟。** 默认日期固定，真实 API 与 MongoDB TTL 仍使用当前时间。换日期验收时，选定并记录接近当前 UTC 日期的 `--base-time`，之后复用该值；不要用未来时间延长出售资格。旧时间基准的数据会正常到期，不能声称仍有 20,000 件有效在售商品。种子命令成功前不要启动面向该库的 API；导入失败可能留下部分数据，须显式重置后重建。

完整命令、数据库只读核验入口、重置边界与实测结果见[种子验证记录](docs/validation/seeds.md)。

## 种子账号与资料 API（#3）

```sh
docker compose up -d --build --wait
# 仅向空的 campus_market 数据库导入两个演示账号；再次运行会拒绝。
docker compose run --rm market seed --mode demo
```

打开 http://localhost:8081/docs，执行 `POST /auth/login`：演示用户名 `seller` 或 `buyer`，密码均为 `CampusDemo123!`。将响应中的 `accessToken` 粘贴到 **Authorize**，随后调用 `GET /users/me` 和 `PATCH /users/me`。令牌有效期为 3600 秒，过期后重新登录。Swagger UI 的固定版本资源从 unpkg 加载，需要网络；机器可读契约在 `/openapi.json`。

资料更新接受昵称（1–40 个 Unicode 字符，不能全为空白）和头像（最多 2048 字节的 HTTP/HTTPS 地址，不能含用户名密码）；至少提供一项，拒绝其他字段。身份只取自令牌，信用分保持种子值 100。用户当前资料保存在 `users`，供后续商品详情、卖家主页复用。密码使用独立盐 bcrypt 哈希，256 位随机令牌只保存 SHA-256 哈希；每次请求校验到期时间，TTL 仅清理已过期会话。账号数据库与 #2 的可行性夹具数据库隔离。

```sh
# 真实 MongoDB + HTTP 集成测试，自动创建并清理独立测试数据库
docker compose build verify
docker compose run --rm verify test -v ./internal/market -count=1
```

`--mode demo` 是最小演示数据；默认 `seed` 生成完整数据。独立运行可用 `MONGO_URI=... DB_NAME=... /market serve|seed`；种子命令拒绝非空及系统数据库，无隐式重置。

验证结果与边界说明：[账号验证记录](docs/validation/accounts.md)。

## 分类、发布与详情 API（#4）

重建并启动：`docker compose up -d --build --wait`。已有演示账号库也可直接启动；服务只补齐缺失的默认分类和索引，不覆盖平台维护过的规则。

在 http://localhost:8081/docs 中登录并授权后：

1. `GET /categories` 读取平台维护的平面分类、属性类型、必填项和当前限制。默认分类为 `textbooks`（教材，必填作者）、`bicycles`（自行车，必填品牌和轮径）、`electronics`（数码，必填品牌和型号）。
2. `POST /products` 使用 Swagger 示例发布商品；身份由令牌确定，价格以人民币整数分表示，必须提供统一新旧程度和 1–9 个 HTTP/HTTPS 图片地址。`example.invalid` 图片也可使用；系统只保存地址，不访问图片。
3. `GET /products/{id}` 一次读取商品完整内容、图片、卖家当前昵称/头像/信用分、是否有效在售、最近 5 条改价及完整历史入口。首次挂牌价独立保存，新商品改价历史为 `[]`。
4. `PUT /products/{id}` 完整替换本人有效在售商品的分类、标题、描述、新旧程度、图片和属性；不改变所有权、首次挂牌价、当前价格和期限。当前分类规则同时适用于新发布与内容修改；规则变更不妨碍旧商品读取。
5. `GET /products/{id}/price-history` 读取完整历史，默认每页 20 条，最大 100，按时间及 ID 倒序；以响应 `nextCursor` 获取下一页。价格变更写入由 #7 实现。

默认属性限制：最多 32 项，其中额外属性最多 16 项；键为最多 40 个 ASCII 字符的 `[A-Za-z][A-Za-z0-9_]*`；字符串最多 256 个 Unicode 字符，数组最多 16 项。仅接受非 null 字符串、数值、布尔值及标量数组，不接受嵌套对象/数组或保留标准字段。额外数组可混合标量类型，分类规则指定的数组须同类型。标题 1–120 字符，描述最多 4000 字符；图片地址最多 2048 UTF-8 字节、不能含用户名密码。商品请求体最多 65536 字节，金额范围为 1–1,000,000,000,000 分。完整契约以 `/openapi.json` 和当前分类规则为准。

分类规则由平台维护 `categories` 集合，未开放学生修改规则的接口。`products` 保留档案；`listings` 只承载在售资格和到期时间，其 `expiresAt` 使用零秒 TTL 索引。发布通过事务共同写入两者，失败会全部回滚；期限为服务端发布时间加 1440 小时。到期、已成交或资格移除后不能修改内容，但详情和历史仍可读取。`priceChanges` 独立存放改价记录，并按商品、改价时间和 ID 建索引。

`GET /products?categoryId=textbooks` 按指定平面分类浏览有效在售商品，发布时间及 ID 倒序；默认 20 条，`limit` 为 1–100，响应 `nextCursor` 可用于同分类下一页的 `cursor`。缺失/非法参数返回 400，未知分类返回 404；到期条目即使 TTL 尚未删除也会排除。静态数据分页无重复无遗漏，并发变化不承诺快照。Swagger `/docs` 可直接执行。索引及验证见 [浏览验证记录](docs/validation/browse.md)。

[商品验证记录](docs/validation/products.md) 包含 HTTP、规则演化和事务回滚证据。

## 自动下架生命周期验收（#11）

运行 `scripts/validate-expiry.sh`，生成 Markdown 实测记录 `docs/validation/expiry-run.md`；可传入其他结果路径。入口启动 MongoDB/Search、构建测试工具并通过真实 HTTP 验证到期前、恰好到期及之后的浏览、搜索、改价、成交、主页和分类统计，复用并发成交与跨到期事务重试检查，再观察 MongoDB TTL 删除独立在售条目。测试使用独立数据库，不重置现有数据。详情、图片和改价历史继续可读，到期不能改价、修改内容或重新上架；TTL 延迟不延长出售资格。3 分钟观察超时仅为测试诊断边界。见[生命周期验证记录](docs/validation/expiry.md)。
