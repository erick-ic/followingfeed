package ioc

import (
	"context"
	"followingfeed/config"
	"followingfeed/internal/handler/article"
	ijwt "followingfeed/internal/handler/jwt"
	"followingfeed/internal/handler/middleware"
	"followingfeed/internal/handler/user"
	"followingfeed/pkg/ginx/middleware/ratelimit"
	"followingfeed/pkg/logger"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func InitGin(
	mdls []gin.HandlerFunc,
	userHdl *user.UserHandler,
	articleHdl *article.ArticleHandler,
	db *gorm.DB,
	redisClient redis.Cmdable,
) *gin.Engine {
	server := gin.Default()

	server.Use(mdls...)
	server.GET("/health/live", func(ctx *gin.Context) {
		ctx.Status(200)
	})
	server.GET("/health/ready", func(ctx *gin.Context) {
		checkCtx, cancel := context.WithTimeout(ctx.Request.Context(), 2*time.Second)
		defer cancel()
		sqlDB, err := db.DB()
		if err == nil {
			err = sqlDB.PingContext(checkCtx)
		}
		if err == nil {
			err = redisClient.Ping(checkCtx).Err()
		}
		if err != nil {
			ctx.Status(503)
			return
		}
		ctx.Status(200)
	})

	userHdl.RegisterUsersRouters(server)
	articleHdl.RegisterArticlesRouters(server)

	return server
}

func InitMiddlewares(
	cfg config.Config,
	redisClient redis.Cmdable,
	jwtHandler ijwt.JWTHandler,
	l logger.LoggerV1,
) []gin.HandlerFunc {
	return []gin.HandlerFunc{
		limitRequestBody(1 << 20),
		//跨域中间件
		handleCors(cfg.CORS),
		//路由中间件
		middleware.NewLoginJWTMiddlewareBuilder(jwtHandler).
			IgnorePaths("/health/live").
			IgnorePaths("/health/ready").
			IgnorePaths("/api/v1/users/signup").
			IgnorePaths("/api/v1/users/login").
			IgnorePaths("/api/v1/users/refreshToken").
			IgnorePaths("/api/v1/pub/list").
			IgnorePathPrefix("/api/v1/pub/detail/").
			Build(),
		//限流中间件
		ratelimit.NewBuilder(redisClient, time.Second, 100).Build(),
	}
}

func limitRequestBody(maxBytes int64) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, maxBytes)
		ctx.Next()
	}
}

func handleCors(cfg config.CORSConfig) gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOrigins:     cfg.AllowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With"},
		ExposeHeaders:    []string{"X-Total-Count", "X-JWT-Token", "x-refresh-token"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	})
}
