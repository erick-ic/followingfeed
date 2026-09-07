# 部署指南

FollowingFeed 包含 API 和 Web 两个应用镜像，从同一 Git 提交构建，使用相同版本号发布。

- `docker-compose.yaml`：本地开发与运行，包含应用、数据库、缓存及可选监控组件。
- `compose.production.yaml`：生产部署，使用预构建镜像，配置资源限额、健康检查和服务端口。

两份 Compose 配置分别使用，不叠加执行。

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

以下以 Linux amd64 为例；按目标服务器架构调整平台，并将 `<version>` 替换为同一个发布版本号。

```bash
docker buildx build --platform linux/amd64 --load -t followingfeed-api:<version> .
docker buildx build --platform linux/amd64 --load --build-arg NEXT_PUBLIC_API_BASE=/api/v1 -t followingfeed-web:<version> web
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
出现 dirty 状态时停止发布，核对实际结构并制定恢复方案。只有明确授权恢复迁移前备份后，才能在恢复完成的数据库上重新执行：

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

## 6. 独立生产 Compose 配置

`compose.production.yaml` 的项目名为 `followingfeed-production`，使用预构建镜像。
API/Web 默认仅发布回环端口 18080/3100；MySQL、Redis 不发布宿主机端口。
迁移器与 API 共用 API 镜像，但迁移器使用独立 DDL 账号，应用账号仅保留业务库的
SELECT/INSERT/UPDATE/DELETE。反向代理将 `/api/v1/` 原样转发给 API，其他路径转发给 Web。
可信代理地址与 CORS 来源必须按实际网络配置，不能照搬其他环境。

模板内存限额为 MySQL 512 MiB、Web 256 MiB、API 128 MiB、Redis 96 MiB，
迁移器另需 128 MiB。这些是初始预算，不是容量保证；发布前需检查主机、同机服务和磁盘余量。
生产构建应在开发机或 CI 完成，不在资源紧张的应用服务器上编译。

注册与发布默认关闭，可通过环境变量按运营需求开启。专用 Redis 使用有效会话白名单，
关闭 RDB/AOF；重启将要求重新登录，不能恢复旧会话快照。容器健康检查失败需要人工或
外部监控处理，`unless-stopped` 不会仅因 unhealthy 自动重启容器。

## 7. 版本发布与回退要求

1. 完成相关测试并提交代码，从干净工作区构建。API/Web 使用相同版本标签和提交；镜像标签不可覆盖已发布版本。
2. 构建目标服务器架构的两个镜像，API 构建上下文为项目根目录，Web 为 `web/`；Web 的 `NEXT_PUBLIC_API_BASE=/api/v1` 在构建时传入。
3. 记录 Git 提交、镜像 ID、架构及构建参数。可以将两个镜像用 `docker image save` 合并导出，压缩后生成 SHA-256 校验清单。
4. 上传后先校验再加载镜像，保留旧镜像。加载镜像不等于更新运行容器。
5. 发布前保存配置；执行数据库迁移前保存当前数据库备份，必要时停止应用写入。压缩校验不能代替恢复演练。
6. 更新镜像配置并运行 `config --quiet`，避免输出展开后的密钥。迁移专用 DSN 的读取超时应覆盖预期 DDL 耗时，业务查询超时单独保持约束。
7. 明确执行 `run --rm --no-deps migrate`，确认成功及迁移记录无 dirty；随后使用 `up -d --no-deps api web` 更新应用。命令均需指定生产环境文件与生产 Compose 文件，且数据库和 Redis 已在运行。
8. 验收健康检查、登录、文章列表、Feed 分页、错误日志和资源占用；共用服务器时同时核对其他服务响应。
9. 若数据库兼容旧版本，可恢复发布前镜像引用并只重建 API/Web，保留数据及新增兼容索引。数据库恢复需明确恢复点及覆盖授权，不能自动执行。

不使用 `down -v` 发布，不重建数据卷，不因应用升级重启 Redis/MySQL。
迁移失败时先核查实际结构及登记状态，不重复执行 DDL 或手动清除 dirty 标记。
