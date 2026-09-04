package ginx

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	ijwt "followingfeed/internal/handler/jwt"
	"followingfeed/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestWrapperWithToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		claims     any
		setClaims  bool
		handlerErr error
		wantStatus int
		wantCalled bool
	}{
		{
			name:       "claims 缺失",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "claims 类型错误",
			setClaims:  true,
			claims:     "invalid",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "调用业务 Handler",
			setClaims:  true,
			claims:     &ijwt.UserClaims{},
			wantStatus: http.StatusOK,
			wantCalled: true,
		},
		{
			name:       "业务 Handler 返回错误",
			setClaims:  true,
			claims:     &ijwt.UserClaims{},
			handlerErr: errors.New("service unavailable"),
			wantStatus: http.StatusInternalServerError,
			wantCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			server := gin.New()
			if tt.setClaims {
				server.Use(func(ctx *gin.Context) {
					ctx.Set("claims", tt.claims)
				})
			}
			server.GET("/profile", WrapperWithToken[*ijwt.UserClaims](
				&logger.NopLogger{},
				func(_ *gin.Context, _ *ijwt.UserClaims) (Result, error) {
					called = true
					return Result{Code: 0}, tt.handlerErr
				},
			))

			request := httptest.NewRequest(http.MethodGet, "/profile", nil)
			response := httptest.NewRecorder()
			server.ServeHTTP(response, request)

			assert.Equal(t, tt.wantStatus, response.Code)
			assert.Equal(t, tt.wantCalled, called)
		})
	}
}

func TestWrapperHTTPStatus(t *testing.T) {
	tests := []struct {
		name       string
		result     Result
		handlerErr error
		wantStatus int
	}{
		{name: "成功", result: Result{Code: 0}, wantStatus: http.StatusOK},
		{name: "参数错误", result: Result{Code: 4}, wantStatus: http.StatusBadRequest},
		{name: "业务层系统错误", result: Result{Code: 5}, wantStatus: http.StatusInternalServerError},
		{
			name:       "处理异常",
			result:     Result{Code: 0},
			handlerErr: errors.New("service unavailable"),
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "显式状态",
			result:     Result{Code: 4, HTTPStatus: http.StatusConflict},
			wantStatus: http.StatusConflict,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := gin.New()
			server.GET("/result", Wrapper(&logger.NopLogger{}, func(*gin.Context) (Result, error) {
				return tt.result, tt.handlerErr
			}))
			response := httptest.NewRecorder()
			server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/result", nil))
			assert.Equal(t, tt.wantStatus, response.Code)
		})
	}
}
