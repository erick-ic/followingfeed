package ioc

import (
	"net/http"

	"followingfeed/config"
	"followingfeed/internal/handler"
	"followingfeed/pkg/ginx/middleware/ratelimit"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// authProtection 在执行 bcrypt 前按 IP 限流，并以进程级并发上限保护 CPU。
// 并发槽不排队；多实例部署还需在网关设置整体预算。
func authProtection(cfg config.AuthConfig, client redis.Cmdable) gin.HandlerFunc {
	limiter := ratelimit.NewBuilder(client, cfg.RateWindow, cfg.RateThreshold).
		Prefix("auth-limiter").Build()
	slots := make(chan struct{}, cfg.MaxConcurrent)
	return func(ctx *gin.Context) {
		if ctx.Request.Method != http.MethodPost {
			ctx.Next()
			return
		}
		path := ctx.FullPath()
		if path == "/api/v1/users/signup" && !cfg.SignupEnabled {
			ctx.AbortWithStatusJSON(http.StatusForbidden, handler.Result{Code: 4, Msg: "暂未开放注册"})
			return
		}
		if path == "/api/v1/articles/publish" && !cfg.PublishingEnabled {
			ctx.AbortWithStatusJSON(http.StatusForbidden, handler.Result{Code: 4, Msg: "暂未开放文章发布"})
			return
		}
		if path != "/api/v1/users/signup" && path != "/api/v1/users/login" {
			ctx.Next()
			return
		}
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
			limiter(ctx)
		default:
			ctx.Header("Retry-After", "1")
			ctx.AbortWithStatusJSON(http.StatusTooManyRequests, handler.Result{Code: 4, Msg: "登录服务繁忙，请稍后重试"})
		}
	}
}
