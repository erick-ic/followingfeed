# FollowingFeed 架构说明

本文描述当前代码已经实现的架构、关键一致性边界和已知限制。
标记为“演进方向”的内容尚未实现，不应在面试中描述为现有能力。

## 1. 系统边界与分层

```text
Browser / Next.js Web
        │ HTTP /api/v1
        ▼
Gin Middleware
  请求体限制、CORS、JWT、Redis 限流
        ▼
Handler
  参数绑定、身份提取、响应 DTO
        ▼
Service
  用例编排、权限、状态流转、缓存失效
        ▼
Repository
  领域模型与持久化模型转换、缓存旁路
        ▼
DAO / Cache
  GORM + MySQL、go-redis + Redis
```

项目是模块化单体：用户、文章、关注和互动代码按业务模块拆分，但部署为一个 Go API。
Wire 在启动期完成依赖注入；MySQL 保存业务事实，Redis 保存会话状态、限流状态和
可重建缓存。数据库结构只通过 `migrations/` 中的版本化 SQL 迁移，不在应用启动时执行
`AutoMigrate`；DAO 中的 GORM 标签只负责运行时字段映射，不重复声明索引和建表约束。

### 1.1 数据完整性原则

本项目明确不使用数据库外键。最终架构会把用户、内容、关注和互动拆分为独立微服务，
跨服务数据可能位于不同数据库；当前即使共用 MySQL，也不通过外键形成未来必须解除的
存储层耦合。该选择还允许异步消息和批量消费按各自节奏落库，避免由跨领域写入顺序、
父记录锁或级联操作扩大服务之间的运行时依赖。

不使用外键不代表放弃数据约束。数据库继续负责主键、非空、唯一键、状态字段和查询索引；
应用层负责校验关联对象、业务权限和状态流转，同一服务内的多表修改使用事务，并通过唯一键
和条件更新处理并发。服务拆分并引入异步链路后，消费者必须提供幂等、重试和死信处理，
删除或失效状态通过领域事件传播，同时配套孤儿数据巡检、对账和可重复执行的修复工具。

因此，迁移文件不得为 `author_id`、`follower_id`、`following_id`、`uid`、`biz_id` 等
跨实体标识增加外键。若未来确需在单个服务私有数据库内部使用外键，应通过独立架构决策记录
明确边界、性能验证和迁移方案，而不是改变本原则的默认行为。

## 2. 登录、刷新与退出时序

### 2.1 登录

```text
Client -> UserHandler.Login: email + password
UserHandler -> UserService: 校验 bcrypt 密码
UserHandler -> JWTHandler: 创建随机 SSID
JWTHandler -> Client: X-JWT-Token（15 分钟，仅保存在前端内存）
JWTHandler -> Client: HttpOnly 刷新 Cookie（7 天）
```

访问令牌与刷新令牌均使用 HS256，但使用不同密钥和不同 audience。两类令牌都包含
`iss`、`sub`、`aud`、`exp`、`nbf`、`iat`、`jti`、`uid` 和 `ssid`。受保护请求经过
JWT 中间件后依次检查：

1. 签名算法与签名是否有效；
2. issuer、audience、有效期和用户身份声明是否有效；
3. Redis 中是否存在 `users:ssid:<ssid>` 注销标记。

### 2.2 刷新

```text
Browser -> POST /users/refreshToken: 自动携带 HttpOnly Cookie
Handler -> JWTHandler: 校验刷新令牌
JWTHandler -> Redis: 检查 SSID 是否已注销
JWTHandler -> Client: 签发新的访问令牌，沿用原 SSID
```

刷新 Cookie 使用 `HttpOnly` 和 `SameSite=Lax`；生产环境额外启用 `Secure`。
刷新不会延长刷新令牌有效期，也不会轮换 SSID。当前阶段保持实现简单，后续可增加
refresh-token rotation 和重放检测能力。

### 2.3 退出

退出时把 SSID 写入 Redis 黑名单，TTL 为 7 天，与刷新令牌期限一致。
之后使用同一 SSID 的访问令牌和刷新令牌都会被拒绝。该设计支持单会话注销；
Redis 不可用时认证检查采用失败关闭策略，受保护接口不会绕过会话校验。

## 3. 文章状态与事务边界

文章分为制作库 `articles` 和公开库 `publish_articles`：

- 保存草稿只写制作库；
- 发布在一个 MySQL 事务中新增或更新制作库，并 upsert 公开库；
- 撤回在一个事务中把制作库和公开库状态同时改为未发布；
- 删除在一个事务中同步软删除两份记录；
- 更新、撤回和删除条件都包含文章 ID、作者 ID 和未删除状态，
  防止越权修改。

```text
Service.Publish
    │ 设置 status=Published
    ▼
Repository.Sync
    ▼
MySQL Transaction
    ├─ INSERT/UPDATE articles
    └─ UPSERT publish_articles
       任一步失败 -> 整体回滚
```

事务只覆盖 MySQL 内部状态。事务提交后再删除 Redis 缓存，因此数据库是事实源：
缓存删除失败不会回滚文章写入，而是记录日志并依赖较短 TTL 最终恢复。
这是有意识的可用性与短暂陈旧之间的取舍。

## 4. Feed 查询与索引

当前 Feed 采用拉模式，不维护用户收件箱表：

```sql
SELECT publish_articles..., users.nickname
FROM publish_articles
JOIN follows
  ON follows.following_id = publish_articles.author_id
LEFT JOIN users
  ON users.id = publish_articles.author_id
WHERE follows.follower_id = ?
  AND publish_articles.status = 2
  AND publish_articles.deleted_at = 0
ORDER BY publish_articles.updated_at DESC, publish_articles.id DESC
LIMIT ? OFFSET ?;
```

这种方案写入简单、关注关系变更立即生效，适合当前项目规模；
代价是每次读取都需要关联关注关系和公开文章。当前分页使用 offset/limit，
深分页会扫描并跳过越来越多的行，并可能在连续翻页期间因新文章插入产生重复或遗漏。

主要列表索引由 `migrations/` 中的版本化 SQL 建立：

| 索引                                                              | 服务查询                       |
| ----------------------------------------------------------------- | ------------------------------ |
| `publish_articles(status, deleted_at, updated_at, id)`            | 公开文章列表                   |
| `publish_articles(author_id, status, deleted_at, updated_at, id)` | 作者文章及 Feed 关联           |
| `follows(follower_id, created_at, id)`                            | 关注列表、定位当前用户关注关系 |
| `follows(following_id, created_at, id)`                           | 粉丝列表                       |
| `user_collection_bizs(uid, biz, status, updated_at, id, biz_id)`  | 收藏列表                       |

索引顺序遵循“等值过滤列在前、排序列在后”。目前尚无正式的 `EXPLAIN ANALYZE`
和规模化压测报告，因此只能说明索引设计意图，不能宣称已经证明高并发性能。

## 5. 点赞、收藏和阅读计数

互动模块通过 `(biz, biz_id)` 支持不同业务对象，目前文章场景的 `biz`
固定由服务端注入为 `article`，客户端不能自行指定。

- `interactives` 保存阅读、点赞、收藏聚合值；
- `user_like_bizs` 保存用户点赞状态；
- `user_collection_bizs` 保存用户收藏状态；
- 用户明细表的 `(uid, biz_id, biz)` 唯一键防止重复关系；
- 聚合表的 `(biz_id, biz)` 唯一键保证每个业务对象只有一行计数。

点赞或收藏在一个 MySQL 事务内执行：

```text
首次操作
  INSERT 用户关系 status=1（唯一键兜底）
  -> 关系确实发生变化
  -> UPSERT interactives，聚合计数 +1

重复操作
  INSERT 冲突
  -> 仅尝试把 status=0 恢复为 1
  -> 若仍无行变化，按幂等成功返回，不增加计数

取消操作
  仅把 status=1 更新为 0
  -> 若关系确实变化，聚合计数 -1，并以 0 为下限
```

事务保证用户状态和聚合计数一起提交或一起回滚。阅读量没有用户明细，
直接通过 upsert 将 `read_cnt` 加一；公开文章详情即使记录阅读失败仍返回文章，
以阅读统计的完整性换取内容读取可用性。

当前热点文章的阅读、点赞和收藏都会竞争同一聚合行。
异步事件、Redis 累加和批量落库属于演进方向，当前没有 Kafka 消费链路。

## 6. 缓存策略与失效

系统采用 cache-aside，以 MySQL 为事实源。

### 6.1 用户与资料缓存

- 用户详情：缓存未命中后回源 MySQL；进程内 `singleflight` 合并相同 UID 的并发回源；
  不存在的用户写短 TTL 空值，降低缓存穿透。
- 公开资料：TTL 为 5～6 分钟随机抖动，不缓存邮箱和密码哈希；
  关注/取消关注后删除双方资料缓存。
- 当前用户文章互动汇总：TTL 为 30～60 秒随机抖动；
  文章发布、撤回、删除后主动删除。

随机 TTL 用于减少大量 key 同时过期。`singleflight` 只合并单实例内请求，
不提供跨实例协调。

### 6.2 文章列表缓存

只缓存高频首屏和公开文章总数：

- 作者制作库首屏：10 秒；
- 公开文章首屏：10 秒；
- 公开文章总数：10 秒。

缓存未命中时使用进程内锁二次检查，减少同一实例的缓存击穿。
文章写入后删除对应作者首屏；发布、撤回或删除还会删除公开首屏和公开总数。

### 6.3 一致性边界

缓存读取失败会回源数据库，缓存回写或删除失败通常只记录日志，不让主业务失败。
因此缓存与数据库之间是最终一致，而文章双表、点赞/收藏明细与聚合计数之间依赖
MySQL 事务实现强一致。

互动汇总缓存目前不会在每次点赞、收藏或阅读后主动失效，只依靠 30～60 秒 TTL 更新；
用户资料中的互动总数可能短暂陈旧。这是当前实现的已知限制。

## 7. 可用性与安全保护

全局中间件当前提供：

- 最大 1 MiB 请求体；
- 可配置 CORS 白名单；
- JWT 身份校验与 Redis 会话黑名单；
- Redis 分布式限流，当前配置为每秒 100 个请求；
- `/health/live` 进程存活检查；
- `/health/ready` MySQL、Redis 就绪检查。

服务使用统一的 Zap 结构化访问日志，每个请求携带或生成 `X-Request-ID`。HTTP 日志使用
路由模板、状态码和耗时等稳定字段；密码、JWT、SSID、请求体和响应体不进入日志。
缓存降级日志通过请求 Context 继承 `request_id` 与 `user_id`，便于定位同一次请求中的问题。
超过限流阈值时返回 HTTP 429；限流依赖 Redis 异常时当前返回 HTTP 500。已通过 JWT
验证但无法读取 Redis 会话黑名单时返回 HTTP 503，不会将依赖故障误报为凭证无效。

独立管理端口暴露 Prometheus 指标，不经过 JWT 和 Redis 限流。当前指标覆盖 HTTP RED、
认证、限流、Redis 命令、业务缓存 hit/miss/error、MySQL 操作耗时和连接池、依赖就绪状态、
Go runtime 和进程状态。标签只使用路由模板、表名、操作类型和有限枚举，不使用用户 ID、
文章 ID、请求 ID、Redis Key、SQL 参数或错误正文。

业务缓存通过 `cache` 标签区分 `article_author_first_page`、`public_profile` 等固定类型，
因此可以比较优化前后的命中率，而不是把 SET、DEL 和会话 Redis 操作混入缓存命中率。
数据库 Histogram 按固定的 operation、table、result 聚合，适合对比索引或查询改造前后的
P95/P99。Prometheus 基础规则关注实例不可用、5xx、HTTP P95、依赖故障、连接池饱和、
数据库慢操作、缓存错误和 Panic。Grafana 数据源与 Dashboard 使用文件自动配置并随代码
评审；Prometheus 负责采集和告警，Grafana 负责查询和展示。

## 8. 当前取舍与演进方向

| 当前实现              | 取舍                             | 演进触发条件与方向                     |
| --------------------- | -------------------------------- | -------------------------------------- |
| 模块化单体            | 部署简单，模块仍共享进程和数据库 | 团队或发布边界明确后再拆服务           |
| Feed 拉模式           | 写入便宜，读取需要 JOIN          | 关注规模和读 QPS 上升后评估推拉结合    |
| offset 分页           | API 简单，深分页退化             | 用 `(updated_at, id)` 游标分页         |
| MySQL 同步互动计数    | 一致性直接，热点行竞争           | 引入事件队列/Redis 聚合和批量落库      |
| cache-aside           | 降级简单，存在短暂陈旧           | 增加变更事件、重试或可靠失效机制       |
| JWT + SSID 黑名单     | 支持即时注销，依赖 Redis         | 增加刷新令牌轮换、设备会话管理         |
| 单实例锁/singleflight | 防止本进程击穿                   | 多实例热点场景采用分布式协调或逻辑过期 |

面试中可以说明已经实现事务幂等、缓存旁路、缓存击穿保护、限流、复合索引和
真实集成测试；在完成游标分页、`EXPLAIN ANALYZE` 与压测前，
不应宣称项目已经具备经过验证的完整高并发能力。
