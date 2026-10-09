# 使用本地 MongoDB Search 完成课程实验

一键部署与换机复现要求搜索能力在本地环境中可用，因此选择 Docker Compose 配合 `mongodb/mongodb-atlas-local`，固定经过种子规模验证的镜像版本，避免依赖外部 Atlas 服务。该镜像用于本地开发、测试和评估；本项目以课程实验为部署边界，版本及配置通过先行可行性验证确定。
