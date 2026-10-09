# campus-market

校园二手交易平台课程项目，已提供本地搜索、事务与 TTL 的 Go/Gin 可行性验证入口；完整 V1 功能仍在实施。

- [项目要求](docs/assignment.md)
- [版本路线图](ROADMAP.md)
- [V1 功能与测试 spec](https://github.com/nanfxqs/campus-market/issues/1)
- [开发与验收计划](docs/development-plan.md)
- [领域术语](GLOSSARY.md) 与 [架构决策](docs/adr/)

本地验证环境：`docker compose up -d --build --wait`。首次运行完整实验：`scripts/validate.sh`；重跑使用显式 `--reset`。见[复现步骤与实测记录](docs/validation/local-feasibility.md)。

## 种子账号与资料 API（#3）

```sh
docker compose up -d --build --wait
# 仅向空的 campus_market 数据库导入；再次运行会拒绝，不覆盖已有资料。
docker compose run --rm market seed
```

打开 http://localhost:8081/docs，执行 `POST /auth/login`：演示用户名 `seller` 或 `buyer`，密码均为 `CampusDemo123!`。将响应中的 `accessToken` 粘贴到 **Authorize**，随后调用 `GET /users/me` 和 `PATCH /users/me`。令牌有效期为 3600 秒，过期后重新登录。Swagger UI 的固定版本资源从 unpkg 加载，需要网络；机器可读契约在 `/openapi.json`。

资料更新接受昵称（1–40 个 Unicode 字符，不能全为空白）和头像（最多 2048 字节的 HTTP/HTTPS 地址，不能含用户名密码）；至少提供一项，拒绝其他字段。身份只取自令牌，信用分保持种子值 100。用户当前资料保存在 `users`，供后续商品详情、卖家主页复用。密码使用独立盐 bcrypt 哈希，256 位随机令牌只保存 SHA-256 哈希；每次请求校验到期时间，TTL 仅清理已过期会话。账号数据库与 #2 的可行性夹具数据库隔离。

```sh
# 真实 MongoDB + HTTP 集成测试，自动创建并清理独立测试数据库
docker compose build verify
docker compose run --rm verify test -v ./internal/market -count=1
```

这两个账号是最小演示数据，不替代 #12 的完整种子交付。独立运行可用 `MONGO_URI=... DB_NAME=... /market serve|seed`；种子命令拒绝非空及系统数据库，无隐式重置。

验证结果与边界说明：[账号验证记录](docs/validation/accounts.md)。
