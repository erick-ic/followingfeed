# FollowingFeed 接入 iChengHub 上线核查

> 后续修复说明：本文保留核查时的发现，不代表当前全部问题仍存在。本项目的本轮修复、行为变化与剩余部署条件见 [小服务器部署说明](production-small-server.md)。主站未修改。

核查日期：2026-09-05。范围：两个本地仓库、用户提供的首页截图和 ECS 配置、FollowingFeed 本地验证。没有登录 ECS，没有修改线上配置、数据或主站源码。仓库文档及脚本中的部署操作仅作为核查材料，没有当作用户指令执行。

**结论：适合作为 iChengHub 的独立技术社区模块；当前版本不宜直接公开上线。** 首页入口接入简单，业务与工程基础较完整，主要差距是生产部署、安全边界和容量验证。2 vCPU / 2 GB 可以作为受控小流量试运行的候选环境，但没有证据能保证与现有主站同机长期稳定运行。

## 1. 接入方式与产品契合度

推荐首页卡片链接到独立子域名，例如 `https://feed.ichenghub.cn`。这是建议地址，未验证 DNS、证书或当前占用。

```text
iChengHub 首页卡片
  → feed.ichenghub.cn → HTTPS 反向代理
      /api/v1/* → FollowingFeed Go API → MySQL + Redis
      其余路径  → FollowingFeed Next.js

ichenghub.cn → 原主站 Next.js → 原 PostgreSQL
```

主站 `platform/src/app/[locale]/page.tsx` 从 `ToolCard` 表读取 `status=1` 的卡片，按 `sortOrder` 升序展示；`platform/src/components/ToolCard.tsx` 的 HTTP(S) URL 分支直接打开新标签页。因此无需把两个 Next.js 项目合并，也不必先建设统一登录。

卡片建议：名称「技术社区 · FollowingFeed」；描述「分享技术实践，关注感兴趣的作者，建立你的技术阅读流。」；分类「自研工具」或「技术社区」；英文名称与描述填入已有双语字段；封面沿用 4:3 比例。创建卡片默认 `status=0`，验收后再启用。当前绝对链接会新开标签，如希望同页打开需调整主站卡片逻辑。

FollowingFeed 的浅灰背景、白色内容区和红色强调色（`web/app/globals.css`）与截图整体协调。需要补充「返回 iChengHub」入口、模块归属文案和一致的页脚。主站已有技术博客，应区分「站长精选博客」与「用户创作及关注社区」，避免入口文案重叠。FollowingFeed 目前只有中文界面，主站英文卡片不会自动使社区支持英文。

不建议首版直接使用 `/followingfeed` 子路径：当前 Next 配置无 `basePath`，导航采用根路径；主站有 `/zh`、`/en` 国际化跳转；刷新 Cookie 固定路径为 `/api/v1/users`。子路径方案需要共同处理页面与静态资源前缀、API 映射、Cookie Path、跳转以及主站国际化。仅改 Nginx 转发不够。[Next.js basePath 官方说明](https://nextjs.org/docs/app/api-reference/config/next-config-js/basePath)也说明前缀需要在构建时确定。

账号暂时保持独立：主站现有管理员 Cookie 与 FollowingFeed JWT 不是同一身份系统，不应共享管理员 Cookie 或签名密钥。

## 2. 公网开放前的阻塞项

### P0：主站后台鉴权可伪造

证据：`platform/src/middleware.ts:28` 仅检查 `admin_session` 是否存在；`platform/src/app/api/auth/check/route.ts:5` 同样只检查存在性；`platform/src/app/actions/authActions.ts` 登录后写入固定字符串。已检查的 `toolActions.ts` 创建、更新、删除等 Server Actions 内没有服务端身份校验。

风险：客户端自行构造该 Cookie 即可满足现有判断，HttpOnly 属性并不验证请求 Cookie 的真实性。若线上与本地一致且无额外网关保护，后台及卡片管理存在严重越权风险。本次没有对公网实施绕过测试。

处理：使用可验证、可过期、可撤销的服务端会话；每个敏感 Server Action 和 API 在服务端检查权限，不能只依赖页面或 middleware。先加固后台，再启用新卡片。

### P0：主站数据库凭据写在受 Git 跟踪的配置中

证据：`platform/ecosystem.config.js` 的 `DATABASE_URL` 包含明文凭据，且 `git ls-files` 确认该文件被跟踪。报告不记录凭据值。

处理：如果仍在使用，应轮换数据库密码，改为受控环境注入；核查 Git 历史、发布包和仓库可见范围。删除当前行不能使旧密码失效。是否已被第三方获取并未确认。

### P1：现有 Compose 不能原样用于主站服务器

证据：`docker-compose.yaml:156` 将浏览器 API 固定为 `http://localhost:8080/api/v1`，第 162 行绑定宿主机 3000；主站 PM2 配置也使用 3000。API CORS 固定为本地来源，Swagger 开启；MySQL 和 Redis 无自动重启策略；各服务未配置资源上限、日志轮转；API/Web 依赖仅等待进程启动，没有就绪检查。

风险：端口冲突；公网访问时浏览器请求访问者自己的 localhost；服务重启、日志增长和内存竞争缺乏控制。强密钥长度检查不会拒绝 `.env.example` 中那些足够长但公开的开发密钥。

处理：单独编制生产部署配置，注入真正随机且相互独立的密钥；新前端绑定例如 `127.0.0.1:3100`，保留主站 3000；构建时设置生产 API 地址；CORS、代理信任范围与 HTTPS 对齐；关闭 Swagger；补充重启、探针、日志与资源控制。

### P1：首页分页会放大请求，并吞掉重定向

证据：`web/app/page.tsx:15` 未限定有限安全整数或合理页码上限；第 23 行对空页逐页递减回查；第 32 行的 `redirect()` 在通用 `try/catch` 中。`web/lib/api.ts:80` 未设置显式请求超时。

本地隔离验证直接转译并执行实际页面组件，以空列表 API 和抛异常的 redirect 替身测试：访问 `page=25` 触发 **24 次 API 调用**，尝试跳转 `/?page=1`，最后却返回含 `NEXT_REDIRECT` 的错误页。该验证是组件逻辑复现，不是生产负载测试；真实 API 限流或错误可能提前中断回查。

处理：校验页码，只查询一次，根据返回总页数做一次有界跳转；将 redirect 放在 catch 外；前后端限制深分页并防止 offset 溢出；为 SSR fetch 设置超时。回归覆盖超大页码、空库、删除最后一页和非法数字。

### P1：Redis 丢失数据可能恢复已注销会话

证据：`internal/handler/jwt/redis_jwt.go:176` 将 SSID 注销标记保留 7 天；第 185 行以键是否存在判断撤销；Compose 未显式配置 Redis 持久卷和 AOF。不能依赖镜像匿名卷或默认快照来保证重建后的撤销状态。

风险：持有旧令牌的客户端，在注销黑名单丢失后可能重新通过检查，直至令牌过期。缓存可丢弃，注销记录不是普通缓存。

处理：定义会话持久性方案。最低限度配置命名卷、AOF、恢复验证，并禁止对注销键使用可淘汰缓存策略；AOF everysec 仍有短时间丢失窗口，必须安排丢失时的全局会话失效策略。更可靠的方案是持久会话/撤销版本，或分离可淘汰缓存与安全状态。不能只配置 `allkeys-lru` 后认为问题已解决。[Redis 持久化官方说明](https://redis.io/docs/latest/operate/oss_and_stack/management/persistence/)。

### P1：公开注册及发布缺少运营控制

证据：注册与登录公开；`internal/service/user.go:69` 对注册执行 bcrypt；默认统一限流为每 IP 每秒 100 次，未见登录失败/账号维度策略；发布接口只需普通登录，没有审核角色或邀请控制。当前路由与模型未提供举报处理、管理员下架和封禁工作流。

风险：批量注册会消耗 CPU；普通注册者能直接发布公开文章。作者自己的撤回/删除不能替代站长管理能力。

处理：首版采用受邀注册或只读展示，并在服务端或网关限制注册与写入，不能仅隐藏按钮；增加登录/注册专用速率与并发限制。若目标是开放社区，上线前补齐至少管理员下架、封禁和滥用处理入口。邮箱验证、找回密码、账号删除及隐私说明也需要安排。

### P1：系统与主站依赖安全维护落后

截至核查日期，Debian 官方已宣布 Debian 11 LTS 于 **2026-08-31 结束**。实例付费到期日 2027-05-21 不代表操作系统仍有官方安全维护。应迁移到受支持系统，或明确落实延长支持；容器不能替代宿主机内核维护。[Debian 官方公告](https://www.debian.org/News/2026/20260831)。

主站 `platform/package.json` 固定 Next.js 14.2.1，位于 CVE-2025-29927 的受影响范围。应升级到当前受维护并修复安全问题的版本，而不是把该历史漏洞的最低修复版当作今日的安全目标。此项不等于确认线上正在运行该版本。[Next.js 官方安全公告](https://github.com/vercel/next.js/security/advisories/GHSA-f82v-jwr5-mffw)。

系统升级时主站 Prisma 当前包含 `debian-openssl-1.1.x` 目标，需重新生成与目标 Linux/OpenSSL 匹配的客户端并回归数据库连接。

## 3. 2 vCPU / 2 GB / 3 Mbps 的容量判断

Go API 本身不是唯一成本。按主站配置推断，同机可能已有 Next.js + PostgreSQL；加入 FollowingFeed 后还会增加 Next.js + Go + MySQL + Redis。尚未读取服务器进程、真实空闲内存、磁盘空间或访问量。

下面只是规划用的粗略内存区间，**不是实测、不是容器上限、不能据此承诺并发量**：

| 分项 | 粗略规划区间 |
| --- | ---: |
| 系统、反向代理、现有主站与 PostgreSQL | 700–1100 MiB |
| 新增 MySQL，经小实例配置调优 | 350–600 MiB |
| 新增 FollowingFeed Next.js | 150–300 MiB |
| Go API | 50–150 MiB |
| Redis，小数据量 | 40–100 MiB |
| 合计 | 1290–2250 MiB |

备份、数据库维护、构建、进程重启重叠和流量峰值还会增加压力。上界已经超过 2 GB，因此当前配置只适合在实测通过后受控试运行。

建议：

1. 在开发机或 CI 构建目标 Linux 镜像/产物，ECS 只拉取与运行；避免在现有 2 GB 主机同时编译 Go 和 Next.js。Go 静态 Linux 二进制可独立构建；不要把 macOS 原生依赖直接当 Linux 产物。
2. 每个应用先运行一个实例，不启用本机 Prometheus/Grafana；保留指标供外部采集，并保留磁盘/内存/错误告警。
3. FollowingFeed MySQL 连接池可从 max-open=5–10、max-idle=2–3 起测，buffer pool 从 128–256 MiB 起测；这只是调优起点，不是完整 MySQL 内存限制。连接、排序和性能监控等也占内存。
4. 设置进程/容器资源预算与告警；Go 的 GOMEMLIMIT、Node 的 heap 限制都不等于总进程 RSS 上限。Redis 限制不能以淘汰注销记录为代价。
5. 若主站峰值下没有足够余量，选择扩容内存或迁出数据库/前端，先测再决定。仅因主站已有 PostgreSQL 就改造 FollowingFeed 存储不是一次配置替换：现有 MySQL SQL、迁移、索引及集成测试都需要适配。

3 Mbps 出口理论约为 **375 kB/s**，两站共享。1 MB 首次传输至少约 2.7 秒，5 MB 至少约 13.3 秒，不含 RTT、TLS 和并发竞争；200 Mbps 入带宽不会加快用户下载。应压缩 JS/CSS/JSON、缓存带哈希的静态资源、压缩卡片封面，必要时将静态资源分发到 CDN。不要对登录响应或私人文章/收藏/Feed 做公共缓存。

项目列表已经只查询 100 字摘要、限制每页最多 50 条和正文最多 60 KiB，适合文字型社区；正文外链图片由浏览器加载时不经过本站出口，但存在外链稳定性和读者隐私问题。当前不适合在同机扩展视频、原图上传或图片代理服务。

## 4. 其余需要改进或验证的项目

| 项目 | 已确认现状与建议 |
| --- | --- |
| SSR 限流 | `web/lib/api.ts` 服务端请求未透传受信任的访客 IP；Go 全局按 ClientIP 限流，因此多个访客的 SSR 请求会共享 Next.js 来源 IP 配额。设计边缘访客限流及内部服务额度，不要简单全量放行内部流量，也不要信任任意客户端 X-Forwarded-For。 |
| Redis 认证 | `config.RedisConfig` 和 `ioc/redis.go` 仅支持地址，没有账号、密码、DB、TLS 配置。只能接入匹配的隔离网络实例；若使用有认证的托管 Redis，需要先扩展代码。 |
| 深分页与计数 | 使用 offset/limit 和同步计数；现有索引、短 TTL 缓存是良好基础，但没有规模数据 EXPLAIN/负载报告。重点测试关注列表、Feed、热点文章同步互动写入。 |
| SEO 与错误页 | 未见文章动态 metadata、sitemap、robots；`web/app/articles/[id]/page.tsx:22` 将所有 API 错误显示为找不到文章，没有使用 notFound 区分真正 404 与依赖故障。建议补充文章标题、canonical、社交分享元数据及准确错误状态。 |
| Web 运行镜像 | Go 使用非 root distroless；Web 镜像仍以默认 root 运行，复制生产 node_modules，未启用 standalone。可以减少镜像体积并使用非 root 用户，但 standalone 不等于消除 SSR 内存成本。 |
| 重启与回滚 | 已有版本迁移、checksum、dirty 状态、数据库锁和优雅停机。仍需生产发布脚本、探针门禁、版本化镜像和真实备份恢复演练；应用回滚不能自动撤销数据库 DDL。 |
| 磁盘与备份 | 未提供系统盘大小、可用空间、增长率。两种数据库、MySQL binlog、Docker 层和日志会共同增长。分别备份两站数据库并异机保存；明确保留天数、日志轮转、恢复时间和可接受数据丢失量。 |
| HTTPS 与访问面 | 浏览器与 API 使用同一社区域名最简单；刷新 Cookie 已为 HttpOnly、生产 Secure、SameSite=Lax，路径与建议方案吻合。数据库、Redis、metrics 不应公开；健康/文档路径由内部监控访问。Nginx、安全组和证书实际状态未检查。 |
| 自动化验证 | 有 Makefile 和隔离测试脚本，但未发现已提交的 GitHub Actions 工作流。上线门禁需要实际执行，不能只依赖文档中列出的命令。 |

## 5. 推荐生产参数与上线顺序

独立子域方案可采用以下参数设计，**尚未配置或部署**：

| 位置 | 建议值或约束 |
| --- | --- |
| 主站入口 | `https://ichenghub.cn`，保持现有路由 |
| 社区入口 | `https://feed.ichenghub.cn`，确认 DNS/证书后使用 |
| 社区 Web 宿主机绑定 | `127.0.0.1:3100`，部署前确认未占用 |
| 社区 API | 容器内 8080，宿主机仅回环或仅私有容器网络 |
| 浏览器 API 构建值 | `NEXT_PUBLIC_API_BASE=/api/v1` |
| SSR API 运行值 | `INTERNAL_API_BASE=http://api:8080/api/v1`，服务名按实际部署调整 |
| CORS | `FOLLOWINGFEED_CORS_ALLOWED_ORIGINS=https://feed.ichenghub.cn` |
| 环境/Swagger | `FOLLOWINGFEED_ENV=production`、`FOLLOWINGFEED_SWAGGER_ENABLED=false` |
| 代理信任 | 仅实际到达 API 的 Nginx/代理来源，不能照搬 10.0.0.0/8 或盲填 127.0.0.1 |

反向代理将社区域名的 `/api/v1/` 原样转发给 Go，其余路径（包括 `/_next/`）交给社区 Web；社区流量不经过主站 next-intl。Nginx 在宿主机还是容器内会影响目标地址和来源 IP，需要根据真实拓扑确定。

执行顺序：修复 P0/P1 → 读取服务器基线并确认系统维护方案 → 准备生产配置和 Linux 产物 → 数据备份与迁移 → 内部试运行 → HTTPS 下全流程验证 → 小流量容量测试 → 启用首页卡片。

建议验收标准（待按实际访问目标调整）：

- 主站正常访问与社区同时运行时，没有 OOM、持续 swap 抖动或持续 CPU 饱和；峰值下至少保留约 20% 内存余量。不能只看空闲启动后的占用。
- 用 1/5/10 并发逐级测试阅读、登录、发布和关注，观察 p95、错误率、MySQL 连接等待、RSS、出口流量及主站响应变化；先在隔离环境测试，不能把恶意高页码压测打向线上。
- HTTPS 注册/登录/刷新/注销、作者权限、草稿不可公开、撤回后不可读、关注 Feed 与互动全部通过。
- 重建 Redis 后旧注销令牌仍被拒绝，或按已设计的丢失恢复策略使全部旧会话失效。
- 主机重启后数据库、Redis、API、Web 自动恢复；迁移失败禁止放量；验证一次真实备份恢复。
- 无 localhost 公网构建值、示例密钥、端口冲突、数据库公网暴露；日志不包含令牌和密码。

## 6. 本次验证结果与边界

| 检查 | 结果 |
| --- | --- |
| Go 单元测试 `go test ./...` | 通过；默认缓存目录受沙箱限制，改用临时 GOCACHE 后通过 |
| Go 静态检查 `go vet ./...` | 通过 |
| 一次性 MySQL 8.0 迁移与集成测试 | `scripts/test-integration.sh` 通过；测试容器由脚本清理 |
| Next.js 生产构建与 TypeScript | 通过 |
| 前端 ESLint / Prettier 检查 | 通过 |
| Compose 配置语法 | `docker compose config --quiet` 通过，不代表生产配置适用 |
| npm 生产依赖审计 | 使用 npm 官方注册表，报告 0 个已知漏洞；不等于不存在漏洞，也不覆盖主站、Go 模块、系统或镜像 |
| 首页分页组件逻辑复现 | 已确认 25 页输入放大为 24 次回查，且重定向被吞 |
| 完整 HTTP E2E、生产 Docker 镜像构建 | 本次未执行 |
| ECS 资源/端口、Nginx、证书、线上版本、备份、实机负载 | 未核实，不能据本地测试声称已具备线上承载能力 |

业务源码与生产环境均未改动；本次新增此核查文档。最小可行落地方式是：先完成阻塞项修复，使用独立子域和独立账号，在有内存余量的环境进行受控试运行，通过验收后发布首页卡片。
