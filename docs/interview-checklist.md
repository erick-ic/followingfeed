# 面试交付检查清单

## 已完成

- 模块化单体 Go API
- Gin、JWT、bcrypt、MySQL、Redis、Wire
- Handler → Service → Repository → DAO
- 注册、登录、刷新、退出、资料和文章状态流转
- 关注关系、粉丝列表、关注 Feed、点赞和收藏
- DAO/Repository 并发一致性与 GORM 查询超时的真实 MySQL 集成测试
- 注册登录、文章发布、关注 Feed、点赞收藏 HTTP 端到端测试
- OpenAPI 3.0 接口契约与可配置 Swagger UI
- Next.js 演示前端，覆盖认证、文章、关注 Feed、互动、收藏和用户资料
- Docker Compose、健康检查、接口说明和架构文档
- Compose 全栈构建、迁移顺序及本地运行验证
- README 三分钟可复现演示步骤

## 后续规划

- Feed 查询的游标分页与 `EXPLAIN` 性能验证

## 面试提交前必须完成

- 执行 `make verify`，完成单元测试、迁移校验、静态检查、前端构建和 Compose 配置检查
- 执行 `make test-integration`，在一次性 MySQL 中验证迁移、并发事务和查询超时
- 陌生环境真实执行 `docker compose up --build`

在上述项目完成前，不应宣称已经实现完整高并发能力。
