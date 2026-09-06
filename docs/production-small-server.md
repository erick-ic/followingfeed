# FollowingFeed 小服务器独立部署

本说明只涉及 FollowingFeed，不修改 iChengHub。`compose.production.yaml` 是独立配置，
不能与本地 Compose 叠加。尚未在用户 ECS 上部署，资源限额也不代表服务器已有足够余量。

## 本轮修复与行为变化

- 首页只查询一次，超范围跳到返回的最后一页；非法页码不访问 API。所有分页接口最多 1000 页，返回的 total 为真实总数，totalPages 为可访问页数。
- Web API 请求的 10 秒超时覆盖响应正文；提交超时不会自动重试。文章 404 与网络故障分别处理。
- Redis 从注销黑名单改为有效会话白名单。旧版本令牌升级后失效；Redis 重启、键丢失或被清理后要求重新登录。注销失败保留登录状态并显示错误，允许重试。
- 专用 Redis 禁用 RDB/AOF，用 tmpfs 避免意外恢复旧会话；不允许从旧快照恢复有效会话。若以后需要持久会话，必须先改成可靠的持久撤销/版本方案，不能直接打开快照。恢复旧 Redis 数据前必须轮换两把 JWT 密钥，让所有旧令牌失效。
- 登录与注册另有共享 IP 配额（默认 10 次/分钟）及 bcrypt 并发限制（默认 2，生产模板 1）。全局限流仍保留。Redis 满或不可用会拒绝依赖它的请求，必须监控并处理，不会绕过认证。
- 服务端注册/发布开关可关闭入口；注册页面读取 `/api/v1/site` 展示当前状态。生产模板默认关闭这两项，本地开发保持开放。
- API 支持 Redis 用户名、密码、DB 和 TLS；生产禁用仓库示例 JWT 密钥；CORS origin 在启动时校验格式。
- Web 镜像改用 standalone 和非 root 用户；增加 Web/API 健康检查。生产服务限制资源、轮转日志并自动重启。

## 构建与配置

在开发机或 CI 构建目标 Linux 架构镜像，确认 ECS 架构后选择平台。示例假定 Linux amd64：

```bash
docker buildx build --platform linux/amd64 --load -t followingfeed-api:<version> .
docker buildx build --platform linux/amd64 --load --build-arg NEXT_PUBLIC_API_BASE=/api/v1 -t followingfeed-web:<version> web
```

将镜像推送到服务器可访问的镜像仓库，或通过镜像归档传输；不要在 2 GB ECS 上运行构建。
准备服务器专用环境文件（权限 600，不提交 Git），至少设置：

```text
FOLLOWINGFEED_API_IMAGE=<versioned-api-image>
FOLLOWINGFEED_WEB_IMAGE=<versioned-web-image>
FOLLOWINGFEED_MYSQL_PASSWORD=<unique-random-password>
FOLLOWINGFEED_MIGRATION_DSN=<separate-migrator-dsn>
FOLLOWINGFEED_JWT_ACCESS_TOKEN_KEY=<random-32-plus-characters>
FOLLOWINGFEED_JWT_REFRESH_TOKEN_KEY=<different-random-32-plus-characters>
FOLLOWINGFEED_CORS_ALLOWED_ORIGINS=https://feed.ichenghub.cn
FOLLOWINGFEED_SERVER_TRUSTED_PROXIES=<actual-proxy-IP-or-CIDR>
FOLLOWINGFEED_AUTH_SIGNUP_ENABLED=false
FOLLOWINGFEED_AUTH_PUBLISHING_ENABLED=false
```

域名是建议值。可信代理必须根据真实容器网络与 Nginx 来源填写，不能直接复制宽泛网段。
现有模板仅在独立容器网络内访问无认证 Redis，不发布 Redis 端口；外部 Redis 需要通过自定义配置传入认证/TLS 项。

## 数据库初始化与发布

以下步骤由部署时执行，本轮没有执行。使用 `--env-file <server-env>` 选择配置，检查时使用
`config --quiet`，避免将展开的密钥打印到日志。

1. 检查 3100、18080 未被占用，并记录主站高峰期内存、CPU、响应时间；确认磁盘与备份位置。
2. 首次仅启动 `mysql redis`。用该 FollowingFeed MySQL 的管理员连接创建专用迁移账号，限制权限范围在 `followingfeed` 库；设置迁移 DSN。把应用 `followingfeed` 账号权限收敛为该库的 SELECT/INSERT/UPDATE/DELETE。镜像初始化给应用账号的默认权限更宽，不能忽略这一步。不要在主站 PostgreSQL 上执行这些操作。
3. 创建可恢复数据库备份。每次发布显式执行 `docker compose --env-file <server-env> -f compose.production.yaml run --rm migrate`，检查退出码；不能只依赖以前退出成功的 migrate 容器。
4. 执行 `docker compose --env-file <server-env> -f compose.production.yaml up -d api web`，确认所有健康检查通过。容器 unhealthy 不会被 Docker 自动重启，重启策略只处理进程退出；健康失败仍需告警与人工/外部监督处理。
5. 为社区域名新增独立 HTTPS 反向代理配置。`/api/v1/` 原样转发到 `127.0.0.1:18080`；其他路径转发到 `127.0.0.1:3100`。先检查 Nginx 配置再 reload，不修改主站 server 块。
6. 内部验证完整登录、刷新、注销、发布、撤回和关注流程。预置账号或临时开放注册/发布时，先在网关把社区访问限制在受控 IP；验收完成后关闭临时注册入口，不要在公网无保护地开放用于初始化的入口。
7. 验证容量余量、备份恢复和回退流程后，才决定是否开放注册/发布与添加主站卡片。

## 资源和隔离边界

常驻内存限额合计约 992 MiB（MySQL 512、Web 256、API 128、Redis 96），迁移期间另需 128 MiB；
这些是防失控的初始额度，不是实测占用或容量承诺。MySQL 或 Web 超限可能被终止，需根据观测调整。
常驻 CPU 配额合计 1.45 核；仍会共享磁盘、网络与内核，无法仅靠容器保证对主站零影响。
磁盘数据增长未被内存限额约束，需设置剩余空间与数据库增长告警。

默认只发布回环 3100/18080，不发布数据库、Redis 或 metrics。不使用主站端口、数据库、容器网络或数据卷。
两站共同出口为 3 Mbps，静态资源应压缩和缓存；不能缓存私人接口、登录或带 Cookie 的个性化响应。

SSR 当前仍按 Next.js 来源 IP 共用后端全局请求额度；本轮不信任未经验证的客户端转发头来绕过限制。
正式放量前在反向代理按访客 IP 控流，并测量 SSR 总额度。管理员下架、封禁、举报和完整账号恢复属于
后续开放社区功能，本轮未实现；在这些能力具备前宜保持受控账号模式。

回退应用只停止/替换 FollowingFeed 的 api/web，不停止主站、不删除 MySQL 卷。数据库迁移不能靠回退镜像撤销；
必须遵循兼容迁移和备份恢复流程。不要执行 `down -v`。

## 验证命令

```bash
make test
make vet
npm --prefix web test
npm --prefix web run lint
npm --prefix web run build
make test-integration
make test-e2e
docker compose config --quiet
docker compose --env-file <server-env> -f compose.production.yaml config --quiet
```

`test-integration` 和 `test-e2e` 均使用一次性容器。E2E 会验证真实 HTTP 与独立 MySQL/Redis，
不访问开发库或生产库。首次镜像下载需要可用网络。

## 本轮验证记录（2026-09-06）

Go 单元测试、Go Vet、8 项前端回归、ESLint、格式检查、MySQL 迁移/仓储集成、隔离 HTTP E2E 均通过。
E2E 包含真实 Redis 中注销后与会话键丢失后拒绝旧令牌。
API 和 Web 的 Linux Docker 镜像均构建成功。

使用随机 Compose 项目、一次性数据卷、测试凭据并取消宿主机端口发布，验证生产模板的资源限额、
独立迁移账号与仅 DML 应用账号：四个常驻服务均 healthy，SSR 首页、静态 JS、关闭注册的页面/API、
前端非 root 运行与非法页码重定向均通过。测试资源已清理。这是本机容器冒烟验证，不是 ECS 容量压测。

当前 Next.js 有 loading 边界时可能通过流式 HTML 的 meta 标签跳转，验收同时检查实际跳转标记，
而不将 HTTP 307 作为唯一通过条件。[Next.js redirect 官方说明](https://nextjs.org/docs/app/api-reference/functions/redirect)。
