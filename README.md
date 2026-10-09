# campus-market

校园二手交易平台课程项目，已提供本地搜索、事务与 TTL 的 Go/Gin 可行性验证入口；完整 V1 功能仍在实施。

- [项目要求](docs/assignment.md)
- [版本路线图](ROADMAP.md)
- [V1 功能与测试 spec](https://github.com/nanfxqs/campus-market/issues/1)
- [开发与验收计划](docs/development-plan.md)
- [领域术语](GLOSSARY.md) 与 [架构决策](docs/adr/)

本地验证环境：`docker compose up -d --build --wait`。首次运行完整实验：`scripts/validate.sh`；重跑使用显式 `--reset`。见[复现步骤与实测记录](docs/validation/local-feasibility.md)。
