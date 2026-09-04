package ioc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"followingfeed/docs"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRegisterOpenAPISpec 验证原始接口契约不受 Swagger UI 开关影响。
func TestRegisterOpenAPISpec(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := gin.New()
	registerAPIDocs(server, false)

	req := httptest.NewRequest(http.MethodGet, openAPIPath, nil)
	resp := httptest.NewRecorder()
	server.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Header().Get("Content-Type"), "application/yaml")
	assert.True(t, strings.HasPrefix(resp.Body.String(), "openapi: 3.0.3"))
}

// TestRegisterAPIDocsEnablesSwaggerUI 验证开启配置后可以访问交互式接口文档。
func TestRegisterAPIDocsEnablesSwaggerUI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := gin.New()
	registerAPIDocs(server, true)

	req := httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil)
	resp := httptest.NewRecorder()
	server.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "Swagger UI")
}

// TestRegisterAPIDocsDisablesSwaggerUI 验证关闭配置时不会暴露交互式接口文档。
func TestRegisterAPIDocsDisablesSwaggerUI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := gin.New()
	registerAPIDocs(server, false)

	req := httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil)
	resp := httptest.NewRecorder()
	server.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusNotFound, resp.Code)
}

// TestOpenAPISpecIsValid 解析完整契约、解析内部引用并执行 OpenAPI 规范校验。
func TestOpenAPISpecIsValid(t *testing.T) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = false
	document, err := loader.LoadFromData(docs.OpenAPISpec)
	require.NoError(t, err)
	require.NoError(t, document.Validate(context.Background()))
}
