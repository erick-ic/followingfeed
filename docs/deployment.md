# 部署指南

本文只描述 API 服务；前端需单独构建，并将 `NEXT_PUBLIC_API_BASE` 配置为公开 API 地址。

## 1. 准备依赖

生产环境使用独立的 MySQL 8 与 Redis 7 实例，并限制其网络访问范围。API 仅需能够访问这两个服务。

## 2. 配置运行时环境变量

不要将生产 `.env`、配置文件或密钥写入镜像、仓库或部署日志。通过服务器的环境变量或密钥管理机制注入：

```text
FOLLOWINGFEED_ENV=production
FOLLOWINGFEED_MYSQL_HOST=mysql.internal
FOLLOWINGFEED_MYSQL_PORT=3306
FOLLOWINGFEED_MYSQL_USER=followingfeed
FOLLOWINGFEED_MYSQL_PASSWORD=<secret>
FOLLOWINGFEED_MYSQL_DATABASE=followingfeed
FOLLOWINGFEED_REDIS_ADDR=redis.internal:6379
FOLLOWINGFEED_JWT_ACCESS_TOKEN_KEY=<random-32-plus-character-secret>
FOLLOWINGFEED_JWT_REFRESH_TOKEN_KEY=<different-random-32-plus-character-secret>
FOLLOWINGFEED_CORS_ALLOWED_ORIGINS=https://app.example.com
```

JWT 两个密钥必须独立、随机生成且至少 32 个字符。发生泄露时应立即轮换；轮换后旧登录会话会失效。

## 3. 构建并迁移

```bash
docker build -t followingfeed-api:<version> .
```

在启动新 API 副本前，以相同的环境变量运行一次迁移命令：

```bash
go run ./cmd/migrate
```

生产环境应将此步骤作为发布过程中的一次性操作执行，而不是每个 API 容器启动时执行。

## 4. 启动与探针

容器监听 8080 端口。负载均衡器使用：

- 存活探针：`GET /health/live`
- 就绪探针：`GET /health/ready`

`/health/ready` 会检查 MySQL 和 Redis；未就绪时返回 HTTP 503。服务收到 `SIGTERM` 或 `SIGINT` 会停止接收新请求并等待在途请求结束。

## 5. 上线检查

- 生产 CORS 仅列出实际前端域名。
- MySQL、Redis 不暴露到公网。
- HTTPS 由反向代理或云负载均衡器终止。
- 先执行迁移，再逐步发布 API；发布后检查 `/health/ready`、登录和发布文章流程。
