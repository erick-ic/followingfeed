# FollowingFeed 架构说明

```text
Next.js Web
    ↓ HTTP /api/v1
Gin Handler / Middleware
    ↓
Service（用例、权限、状态转换）
    ↓
Repository（领域与持久化隔离）
    ↓
DAO（SQL、索引和约束）
    ↓
MySQL
```

Redis 当前用于登录会话相关状态与文章列表缓存；文章详情缓存属于后续优化项，不阻塞核心业务。

关注关系与 Feed 尚未实现。后续计划采用拉模式查询当前用户关注作者的已发布文章，不预生成 `feed_items` 表。
