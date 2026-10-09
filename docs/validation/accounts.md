# #3 种子账号与本人资料验证

2026-10-09，Go 1.24.2 Docker 工具镜像，真实 MongoDB Atlas Local 8.0.4。沿用 #1 已确认的真实 HTTP API、种子 CLI 与必要持久化观察边界。

## 验证入口与结果

- `docker compose up -d --build --wait`：MongoDB、可行性 API 与账号 API 均健康。
- `docker compose run --rm market seed`：向独立 `campus_market` 导入 `seller`、`buyer` 两个账号。
- `docker compose run --rm -v "$PWD:/src" verify vet ./...`：通过；`go test` 同时完成 Go 类型检查。
- `docker compose run --rm -v "$PWD:/src" verify test -v ./internal/market -count=1`：登录成功/失败、缺失/伪造/过期令牌、本人资料更新、他人资料保护、信用分不可修改、输入校验、OpenAPI 和 Swagger HTML、CLI 非空与系统库拒绝及独立盐 bcrypt 哈希均通过。
- `docker compose run --rm -e SCALE=1 -v "$PWD:/src" verify test -v ./... -count=1 -timeout=12m`：完整套件通过，包含现有 10,000 用户规模的 Search、事务竞争与回滚、TTL 外部观察。原始输出见 [issue3-tests.log](issue3-tests.log)。该规模夹具是 #2 的验证数据，不是 #12 的完整种子交付。
- 已部署的 `http://localhost:8081`：通过真实 HTTP 登录、Bearer 授权 GET/PATCH 验证，令牌期限为 3600 秒，信用分仍为 100。复查持久化通过后续 GET 完成。

令牌到期测试使用同一个公开 HTTP 路由配置短生命周期，真实等待过期；认证请求每次查询会话到期时间，不依赖 TTL 及时删除。昵称 null 与有效头像同时提交的测试曾失败，修复后通过。CLI 测试仅观察种子账号哈希与数量，不依赖内部函数。

Swagger UI 提供登录操作和 Bearer Authorize，固定版本 CDN 脚本已返回 HTTP 200；机器可读契约与 HTML 路由均验证通过。当前工具环境没有可用浏览器，未执行浏览器内点击登录/授权操作。Swagger 静态资源依赖网络。

## 代码审查

以开始实现时的 `b99129359221066210f378e4febdcf8ec1970d1a` 为基准，分别执行 Standards 和 Spec 两个子代理审查。Standards 无硬性违反，指出测试请求助手重复，已合并；Spec 指出头像长度需明确 UTF-8 字节限制，已补充。未发现其他缺项或范围扩张。

#4 商品详情与 #9 卖家主页后续直接复用 `users` 当前资料；本次没有提前实现这些路由。账号 API 在 8081，可行性夹具 API 在 8080，各自使用独立数据库。
