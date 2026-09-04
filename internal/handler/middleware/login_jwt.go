package middleware

import (
	"errors"
	"net/http"

	ijwt "followingfeed/internal/handler/jwt"
	"followingfeed/pkg/logger"

	"github.com/gin-gonic/gin"
)

type routeRule struct {
	method string // HTTP 方法，用于区分同一路径下的不同操作。
	path   string // Gin 路由模板，例如 /api/v1/articles/:id，而不是实际请求路径。
}

// LoginJWTMiddlewareBuilder 收集公开路由和可选认证路由，最终构造全局 JWT 中间件。
// 未出现在这两类规则中的路由默认必须登录，避免新增接口因遗漏配置而直接公开。
type LoginJWTMiddlewareBuilder struct {
	ignoredRoutes  map[routeRule]struct{} // 完全跳过登录校验的路由。
	optionalRoutes map[routeRule]struct{} // 允许匿名访问，但携带令牌时仍校验登录态的路由。
	jwtHandler     ijwt.JWTHandler        // 提供访问令牌解析和服务端会话校验能力。
}

// NewLoginJWTMiddlewareBuilder 创建尚未配置路由规则的 JWT 中间件构建器。
func NewLoginJWTMiddlewareBuilder(jwtHandler ijwt.JWTHandler) *LoginJWTMiddlewareBuilder {
	return &LoginJWTMiddlewareBuilder{
		ignoredRoutes:  make(map[routeRule]struct{}),
		optionalRoutes: make(map[routeRule]struct{}),
		jwtHandler:     jwtHandler,
	}
}

// IgnoreRoute 将指定 HTTP 方法和 Gin 路由模板标记为公开路由。
// 公开路由即使携带 Authorization 请求头也不会解析令牌。
func (ljb *LoginJWTMiddlewareBuilder) IgnoreRoute(method, path string) *LoginJWTMiddlewareBuilder {
	ljb.ignoredRoutes[routeRule{method: method, path: path}] = struct{}{}
	return ljb
}

// OptionalRoute 允许指定路由匿名访问；请求携带令牌时仍会校验并注入登录态。
func (ljb *LoginJWTMiddlewareBuilder) OptionalRoute(method, path string) *LoginJWTMiddlewareBuilder {
	ljb.optionalRoutes[routeRule{method: method, path: path}] = struct{}{}
	return ljb
}

// Build 根据已配置的路由规则创建 Gin 中间件。
// 调用 Build 完成后不应继续修改构建器，以免运行期间并发读写路由规则。
func (ljb *LoginJWTMiddlewareBuilder) Build() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// FullPath 返回注册时的 Gin 路由模板，使不同动态参数值能够命中同一条规则。
		route := routeRule{method: ctx.Request.Method, path: ctx.FullPath()}
		// 可选认证路由在没有令牌时直接匿名放行；有令牌时继续执行完整校验。
		_, optionalAuth := ljb.optionalRoutes[route]
		if optionalAuth && ctx.GetHeader("Authorization") == "" {
			return
		}

		// 完全公开路由始终跳过令牌解析。
		if _, ignored := ljb.ignoredRoutes[route]; ignored {
			return
		}

		// 第一层校验：提取并验证访问令牌的格式、签名和有效期。
		tokenStr := ljb.jwtHandler.ExtractToken(ctx)
		claims, err := ljb.jwtHandler.ParseAccessToken(tokenStr)
		if err != nil {
			ctx.Set("security_event", "auth.jwt_check_failed")
			ctx.Set("auth_failure_reason", "invalid_token")
			// 令牌缺失或无效时终止后续处理。
			ctx.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		// 第二层校验：查询 Redis 中的注销黑名单，确认服务端没有撤销当前会话。
		// 使用标准请求 Context，确保客户端断开或请求超时时能够取消 Redis 操作。
		err = ljb.jwtHandler.CheckSession(ctx.Request.Context(), claims.Ssid)
		if err != nil {
			if errors.Is(err, ijwt.ErrSessionRevoked) {
				ctx.Set("security_event", "auth.jwt_check_failed")
				ctx.Set("auth_failure_reason", "revoked")
				ctx.Set("auth_user_id", claims.Uid)
				ctx.AbortWithStatus(http.StatusUnauthorized)
				return
			}

			// Redis 等会话依赖异常不代表用户凭证无效，返回 503 供客户端稍后重试。
			_ = ctx.Error(err)
			ctx.Set("observability_level", "error")
			ctx.Set("security_event", "auth.session_check_failed")
			ctx.Set("auth_failure_reason", "dependency_error")
			ctx.Set("auth_user_id", claims.Uid)
			ctx.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}

		// 将解析后的身份写入 Gin Context，供需要登录态的 Handler 获取。
		ctx.Set("claims", claims)
		// 同时把用户 ID 写入标准请求 Context，使下游日志自动携带身份字段。
		ctx.Request = ctx.Request.WithContext(logger.ContextWithFields(
			ctx.Request.Context(),
			logger.Int64("user_id", claims.Uid),
		))
	}
}
