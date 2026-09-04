package ioc

import (
	"net/http"

	"followingfeed/docs"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

const openAPIPath = "/openapi.yaml"

// registerAPIDocs 始终提供内嵌的 OpenAPI 原始文档，并按配置决定是否注册 Swagger UI。
// 原始契约保持可用，便于网关、客户端生成器和自动化校验在不启用调试页面时继续访问。
func registerAPIDocs(server *gin.Engine, swaggerEnabled bool) {
	// 直接向传入的 Gin 引擎注册路由，因此只在应用启动装配阶段调用。
	// 原始 YAML 不受 Swagger UI 开关影响，始终作为机器可读的接口契约提供。
	server.GET(openAPIPath, func(ctx *gin.Context) {
		ctx.Data(http.StatusOK, "application/yaml; charset=utf-8", docs.OpenAPISpec)
	})
	// 控制 Swagger UI ，开关关闭时仅隐藏交互式页面，不影响上面的原始契约路由。
	if !swaggerEnabled {
		return
	}
	// Swagger UI 使用同源契约地址，避免额外维护一份接口定义。
	server.GET(
		"/swagger/*any",
		ginSwagger.WrapHandler(
			swaggerFiles.Handler,
			ginSwagger.URL(openAPIPath),
			ginSwagger.DocExpansion("list"),
		),
	)
}
