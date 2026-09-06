# 部署指南

本文以 API 的生产部署为主；前端需单独构建。`docker-compose.yaml` 用于本地演示；
小服务器独立部署使用 `compose.production.yaml`，步骤见[小服务器部署说明](production-small-server.md)。

仓库根目录的 `docker-compose.yaml` 默认只启动 MySQL、迁移任务、Redis、API 和前端。
Prometheus 和 Grafana 位于可选的 `observability`
Profile，通过 `make observability-up` 按需启动。生产环境应使用独立依赖和发布流程，
不直接照搬本地 Compose 配置。

## 1. 准备依赖

生产环境使用独立的 MySQL 8 与 Redis 7 实例，并限制其网络访问范围。API 仅需能够访问
这两个服务。

## 2. 配置运行时环境变量

不要将生产 `.env`、配置文件或密钥写入镜像、仓库或部署日志。通过服务器的环境变量
或密钥管理机制注入：

```text
FOLLOWINGFEED_ENV=production
# 仅配置真实反向代理的 IP/CIDR；API 直连公网时留空。
FOLLOWINGFEED_SERVER_TRUSTED_PROXIES=10.0.0.0/8
FOLLOWINGFEED_SERVER_ADDR=:8080
FOLLOWINGFEED_SERVER_READ_TIMEOUT=10s
FOLLOWINGFEED_SERVER_READ_HEADER_TIMEOUT=5s
FOLLOWINGFEED_SERVER_WRITE_TIMEOUT=15s
FOLLOWINGFEED_SERVER_IDLE_TIMEOUT=60s
FOLLOWINGFEED_SERVER_SHUTDOWN_TIMEOUT=15s
FOLLOWINGFEED_MYSQL_HOST=mysql.internal
FOLLOWINGFEED_MYSQL_PORT=3306
FOLLOWINGFEED_MYSQL_USER=followingfeed
FOLLOWINGFEED_MYSQL_PASSWORD=<secret>
FOLLOWINGFEED_MYSQL_DATABASE=followingfeed
FOLLOWINGFEED_MYSQL_MAX_OPEN_CONNS=25
FOLLOWINGFEED_MYSQL_MAX_IDLE_CONNS=10
FOLLOWINGFEED_MYSQL_CONN_MAX_LIFETIME=30m
FOLLOWINGFEED_MYSQL_CONNECT_TIMEOUT=3s
FOLLOWINGFEED_MYSQL_READ_TIMEOUT=5s
FOLLOWINGFEED_MYSQL_WRITE_TIMEOUT=5s
FOLLOWINGFEED_MYSQL_QUERY_TIMEOUT=3s
FOLLOWINGFEED_REDIS_ADDR=redis.internal:6379
FOLLOWINGFEED_REDIS_USERNAME=<optional-acl-user>
FOLLOWINGFEED_REDIS_PASSWORD=<optional-secret>
FOLLOWINGFEED_REDIS_DB=0
FOLLOWINGFEED_REDIS_TLS=false
FOLLOWINGFEED_AUTH_SIGNUP_ENABLED=false
FOLLOWINGFEED_AUTH_PUBLISHING_ENABLED=false
FOLLOWINGFEED_AUTH_RATE_WINDOW=1m
FOLLOWINGFEED_AUTH_RATE_THRESHOLD=10
FOLLOWINGFEED_AUTH_MAX_CONCURRENT=1
FOLLOWINGFEED_RATE_LIMIT_WINDOW=1s
FOLLOWINGFEED_RATE_LIMIT_THRESHOLD=100
FOLLOWINGFEED_OBSERVABILITY_METRICS_ADDR=:8081
# 默认关闭交互式接口调试页面；确有内部联调需求时再开启。
FOLLOWINGFEED_SWAGGER_ENABLED=false
FOLLOWINGFEED_MIGRATION_TIMEOUT=30m
FOLLOWINGFEED_MIGRATION_LOCK_TIMEOUT=30
# 建议使用只在发布阶段注入、拥有 DDL 权限的独立账号。
FOLLOWINGFEED_MIGRATION_DSN=migrator:<secret>@tcp(mysql.internal:3306)/followingfeed?charset=utf8mb4&parseTime=true&loc=UTC&timeout=3s&readTimeout=5s&writeTimeout=5s
FOLLOWINGFEED_JWT_ACCESS_TOKEN_KEY=<random-32-plus-character-secret>
FOLLOWINGFEED_JWT_REFRESH_TOKEN_KEY=<different-random-32-plus-character-secret>
FOLLOWINGFEED_CORS_ALLOWED_ORIGINS=https://app.example.com
```

JWT 两个密钥必须独立、随机生成且至少 32 个字符。发生泄露时应立即轮换；
轮换后旧登录会话会失效。

## 3. 构建并迁移

```bash
docker build -t followingfeed-api:<version> .
```

前端构建时将 `NEXT_PUBLIC_API_BASE` 设为浏览器可访问的公网 API 地址；该值会写入构建产物，
不能只在容器启动后修改。Next.js 服务端渲染在运行时通过 `INTERNAL_API_BASE` 访问 API：

```text
NEXT_PUBLIC_API_BASE=https://api.example.com/api/v1
INTERNAL_API_BASE=http://followingfeed-api:8080/api/v1
```

如果 Next.js 运行环境能稳定访问公网 API，`INTERNAL_API_BASE` 可与公网地址相同；在容器网络中
应优先使用 API 服务名，不得指向 Next.js 容器自身的 `localhost`。

在启动新 API 副本前，以相同的数据库和迁移环境变量运行一次迁移命令。
迁移任务通过 MySQL advisory lock 防止多个发布任务并发修改结构；
超时时间应根据生产数据副本上的 DDL 预演结果设置：

```bash
# 容器化部署：把已构建镜像作为一次性 Job，并覆盖入口命令。
/app/migrate

# 仅在具备 Go 工具链的源码部署环境中使用：
go run ./cmd/migrate
```

生产环境应将 `/app/migrate` 作为发布过程中的一次性 Job 执行，由部署平台注入密钥并检查
退出码，而不是在每个 API 容器启动时执行。迁移命令只校验 MySQL 和迁移配置，不依赖
Redis、JWT 或 CORS；API 运行账号应只保留业务所需的 DML 权限，迁移账号才拥有 DDL 权限。

迁移器会保存每个 SQL 文件的 SHA-256 checksum，并在执行前把版本标记为 `dirty`。
这里的 `dirty` 表示“该迁移没有完整执行成功”。MySQL DDL 中途失败后，迁移器会停止，
不会提供跳过失败版本或直接修改迁移状态的参数。生产环境应在每次迁移前创建可恢复备份；
出现 dirty 状态时，回滚本次应用发布、恢复迁移前备份，然后重新执行：

```bash
go run ./cmd/migrate
```

禁止直接把 dirty 改为完成，否则可能在缺少字段或索引的数据库上启动应用。已经执行过的
迁移文件不可修改；任何变更都必须增加新的、版本号递增的 SQL 文件。

## 4. 启动与探针

API 进程在 `8080` 提供业务接口和健康检查，并在独立的 `8081` 管理端口提供 Prometheus
指标。负载均衡器只转发业务端口，并使用：

- 存活探针：`GET /health/live`
- 就绪探针：`GET /health/ready`

`/health/ready` 会检查 MySQL 和 Redis；未就绪时返回 HTTP 503。服务收到 `SIGTERM`
或 `SIGINT` 会停止接收新请求并等待在途请求结束。

## 5. 上线检查

- 生产 CORS 仅列出实际前端域名。
- `FOLLOWINGFEED_SERVER_TRUSTED_PROXIES` 仅包含实际负载均衡器或反向代理网段，避免客户端伪造转发 IP 绕过限流。
- MySQL、Redis 不暴露到公网。
- Prometheus 指标端口只允许监控网络访问，不暴露到公网，也不经过业务 JWT 和限流。
- Grafana 不直接暴露公网，使用 SSO 或强密码，并限制数据源编辑权限。
- Swagger UI 生产环境默认关闭；开启时只允许受控网络访问。`/openapi.yaml` 始终保留，
  可由网关或反向代理按部署要求限制访问。
- 网关应正确传递 HTTP 429，并区分限流 Redis 故障的 HTTP 500 与会话检查失败的 HTTP 503。
- HTTPS 由反向代理或云负载均衡器终止。
- 先执行迁移，再逐步发布 API；发布后检查 `/health/ready`、登录和发布文章流程。
