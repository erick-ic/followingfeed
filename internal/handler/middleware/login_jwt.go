package middleware

import (
	ijwt "followingfeed/internal/handler/jwt"
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
)

type LoginJWTMiddlewareBuilder struct {
	paths       []string //不需要校验的精确路由
	pathPrefixs []string //不需要校验的路由前缀
	ijwt.JWTHandler
}

func NewLoginJWTMiddlewareBuilder(jwtHandler ijwt.JWTHandler) *LoginJWTMiddlewareBuilder {
	return &LoginJWTMiddlewareBuilder{
		JWTHandler: jwtHandler,
	}
}

func (ljb *LoginJWTMiddlewareBuilder) IgnorePaths(path string) *LoginJWTMiddlewareBuilder {
	ljb.paths = append(ljb.paths, path)
	return ljb
}

func (ljb *LoginJWTMiddlewareBuilder) IgnorePathPrefix(prefix string) *LoginJWTMiddlewareBuilder {
	ljb.pathPrefixs = append(ljb.pathPrefixs, prefix)
	return ljb
}

func (ljb *LoginJWTMiddlewareBuilder) Build() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		//无需登录校验接口
		if slices.Contains(ljb.paths, ctx.Request.URL.Path) || hasPathPrefix(ctx.Request.URL.Path, ljb.pathPrefixs) {
			return
		}

		//使用JWT校验
		tokenStr := ljb.ExtractToken(ctx)
		claims, err := ljb.ParseAccessToken(tokenStr)
		if err != nil {
			//未登录
			ctx.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		//检验UserAgent
		if claims.UserAgent != ctx.Request.UserAgent() {
			//严重的安全问题，使用了不同的设备环境
			ctx.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		err = ljb.CheckSession(ctx, claims.Ssid)
		if err != nil {
			//Redis问题或已退出登录
			ctx.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		//可在ctx中传递数据，进行读写操作。
		ctx.Set("claims", claims)
	}
}

func hasPathPrefix(path string, prefixes []string) bool {
	return slices.ContainsFunc(prefixes, func(prefix string) bool {
		return strings.HasPrefix(path, prefix)
	})
}
