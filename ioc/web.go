package ioc

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"followingfeed/config"
	"followingfeed/internal/handler/article"
	"followingfeed/internal/handler/follow"
	"followingfeed/internal/handler/interactive"
	ijwt "followingfeed/internal/handler/jwt"
	"followingfeed/internal/handler/middleware"
	"followingfeed/internal/handler/user"
	"followingfeed/internal/observability"
	"followingfeed/pkg/ginx/middleware/ratelimit"
	"followingfeed/pkg/logger"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const (
	livePath               = "/health/live"  // 存活探针路径，只反映进程是否能够响应。
	readyPath              = "/health/ready" // 就绪探针路径，反映实例是否可以接收业务流量。
	dependencyCheckTimeout = 2 * time.Second // MySQL 和 Redis 共用的单次探针超时预算。
	maxRequestBodyBytes    = 1 << 20         // 全局请求体上限为 1 MiB。
)

// InitGin 组装全局中间件、运行探针、接口文档与各业务模块路由。
func InitGin(
	cfg config.Config,
	mdls []gin.HandlerFunc,
	userHdl *user.UserHandler,
	articleHdl *article.ArticleHandler,
	followHdl *follow.Handler,
	interactiveHdl *interactive.Handler,
	db *gorm.DB,
	redisClient redis.Cmdable,
) (*gin.Engine, error) {
	// 全局日志和异常恢复由 InitMiddlewares 统一提供，因此不使用带默认中间件的 gin.Default。
	server := gin.New()
	// 默认不信任任何代理请求头，避免客户端伪造 X-Forwarded-For 绕过 IP 限流。
	if err := server.SetTrustedProxies(cfg.Server.TrustedProxies); err != nil {
		return nil, fmt.Errorf("配置可信代理失败：%w", err)
	}

	// 先挂载全局中间件，确保随后注册的基础设施路由和业务路由使用同一条处理链。
	server.Use(mdls...)
	// 低频使用的基础设施路由由独立方法负责注册，保持主装配流程简洁。
	registerHealthRoutes(server, db, redisClient)
	registerAPIDocs(server, cfg.Swagger.Enabled)

	// 各业务 Handler 只负责所属领域的路由，IOC 层负责统一组装。
	userHdl.RegisterUsersRouters(server)
	articleHdl.RegisterArticlesRouters(server)
	followHdl.RegisterRouters(server)
	interactiveHdl.Register(server)

	return server, nil
}

// InitMiddlewares 按执行顺序构造全局中间件链。
// 请求关联与访问观测位于最外层，以覆盖恢复、请求限制、跨域、限流和认证的完整处理过程。
func InitMiddlewares(
	cfg config.Config,
	redisClient redis.Cmdable,
	jwtHandler ijwt.JWTHandler,
	l logger.LoggerV1,
	metrics *observability.Metrics,
) []gin.HandlerFunc {
	return []gin.HandlerFunc{
		// 最先生成或接收请求 ID，供后续日志和响应统一关联同一次请求。
		observability.RequestID(),
		// Observer 包裹后续完整处理链，统一采集耗时、状态码和错误信息。
		metrics.HTTPObserver(l),
		// Recovery 位于 Observer 内层，使 panic 转换后的 500 响应仍能被正常观测。
		metrics.Recovery(),
		// 在解析 JSON 或表单前限制读取量，避免大请求体持续占用服务资源。
		limitRequestBody(maxRequestBodyBytes),
		// 跨域校验在业务路由之前执行，允许浏览器正确处理预检请求。
		handleCors(cfg.CORS),
		// 按配置的滑动窗口和阈值限制客户端 IP；限流先于认证，避免无效令牌请求绕过流量保护。
		// 健康检查和文档路由不依赖 Redis 限流状态，防止基础设施诊断被依赖故障阻断。
		ratelimit.NewBuilder(redisClient, cfg.RateLimit.Window, cfg.RateLimit.Threshold).
			IgnorePaths(livePath, readyPath, openAPIPath).
			IgnorePathPrefix("/swagger/").
			Build(),
		// 使用 HTTP 方法和 Gin 路由模板精确声明公开边界，避免前后缀匹配误放行新路由。
		// OptionalRoute 允许匿名请求；如果请求主动携带令牌，仍会校验并注入登录态。
		middleware.NewLoginJWTMiddlewareBuilder(jwtHandler).
			IgnoreRoute(http.MethodGet, livePath).
			IgnoreRoute(http.MethodGet, readyPath).
			IgnoreRoute(http.MethodGet, openAPIPath).
			IgnoreRoute(http.MethodGet, "/swagger/*any").
			IgnoreRoute(http.MethodPost, "/api/v1/users/signup").
			IgnoreRoute(http.MethodPost, "/api/v1/users/login").
			IgnoreRoute(http.MethodPost, "/api/v1/users/refreshToken").
			IgnoreRoute(http.MethodGet, "/api/v1/pub/list").
			IgnoreRoute(http.MethodGet, "/api/v1/pub/detail/:id").
			IgnoreRoute(http.MethodGet, "/api/v1/pub/users/:id/profile").
			IgnoreRoute(http.MethodGet, "/api/v1/pub/users/:id/articles").
			IgnoreRoute(http.MethodPost, "/api/v1/pub/articles/interactions").
			IgnoreRoute(http.MethodGet, "/api/v1/pub/articles/:id/interactions").
			OptionalRoute(http.MethodGet, "/api/v1/pub/articles/:id/interactions/status").
			Build(),
	}
}

// limitRequestBody 限制请求体大小，防止超大载荷持续占用内存和连接资源。
func limitRequestBody(maxBytes int64) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// MaxBytesReader 在下游读取请求体时返回 *http.MaxBytesError，统一请求绑定层会将其转换为 413。
		ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, maxBytes)
		ctx.Next()
	}
}

// handleCors 根据配置创建跨域中间件，仅允许显式配置的来源访问业务接口。
func handleCors(cfg config.CORSConfig) gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOrigins: cfg.AllowedOrigins, // 不使用通配符，允许来源由不同环境的配置明确给出。
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			"Accept",
			"Authorization",
			"X-Requested-With",
			"X-Request-ID",
		},
		// 允许浏览器端读取分页、令牌和请求追踪相关的响应头。
		ExposeHeaders: []string{"X-Total-Count", "X-JWT-Token", "X-Request-ID"},
		// 刷新令牌通过 HttpOnly Cookie 传递；允许来源仍由 AllowedOrigins 精确限制。
		AllowCredentials: true,
		// 浏览器可缓存预检结果，减少重复的 OPTIONS 请求。
		MaxAge: 12 * time.Hour,
	})
}

// registerHealthRoutes 注册不依赖外部服务的存活探针，以及检查 MySQL、Redis 的就绪探针。
// 两项依赖共用一次超时预算，避免探针在依赖异常时长时间占用连接。
func registerHealthRoutes(server *gin.Engine, db *gorm.DB, redisClient redis.Cmdable) {
	// 存活探针不访问任何外部依赖，避免依赖故障导致平台反复重启仍可工作的进程。
	server.GET(livePath, func(ctx *gin.Context) {
		ctx.Status(http.StatusOK)
	})
	// 就绪探针检查提供业务服务所必需的依赖；失败时返回 503，让流量入口暂时摘除该实例。
	server.GET(readyPath, func(ctx *gin.Context) {
		checkCtx, cancel := context.WithTimeout(ctx.Request.Context(), dependencyCheckTimeout)
		defer cancel()

		sqlDB, err := db.DB()
		if err != nil {
			// 把底层原因附加到 Gin Context，供外层 Observer 记录，但不将内部细节返回给客户端。
			_ = ctx.Error(fmt.Errorf("获取 MySQL 连接池失败：%w", err))
			ctx.Status(http.StatusServiceUnavailable)
			return
		}
		if err = sqlDB.PingContext(checkCtx); err != nil {
			_ = ctx.Error(fmt.Errorf("连接 MySQL 失败：%w", err))
			ctx.Status(http.StatusServiceUnavailable)
			return
		}
		if err = redisClient.Ping(checkCtx).Err(); err != nil {
			_ = ctx.Error(fmt.Errorf("连接 Redis 失败：%w", err))
			ctx.Status(http.StatusServiceUnavailable)
			return
		}
		ctx.Status(http.StatusOK)
	})
}
