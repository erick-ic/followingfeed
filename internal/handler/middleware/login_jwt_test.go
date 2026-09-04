package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	ijwt "followingfeed/internal/handler/jwt"
	jwtmocks "followingfeed/internal/handler/jwt/mocks"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestLoginJWTMiddlewareBuilderIgnoresOpenAPIPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := gin.New()
	server.Use(
		NewLoginJWTMiddlewareBuilder(nil).
			IgnoreRoute(http.MethodGet, "/openapi.yaml").
			Build(),
	)
	server.GET("/openapi.yaml", func(ctx *gin.Context) { ctx.Status(http.StatusNoContent) })
	req := httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil)
	resp := httptest.NewRecorder()
	server.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusNoContent, resp.Code)
}

func TestLoginJWTMiddlewareBuilderIgnorePublicBatchInteractions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := gin.New()
	server.Use(
		NewLoginJWTMiddlewareBuilder(nil).
			IgnoreRoute(http.MethodPost, "/api/v1/pub/articles/interactions").
			Build(),
	)
	server.POST("/api/v1/pub/articles/interactions", func(ctx *gin.Context) {
		ctx.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/pub/articles/interactions", nil)
	resp := httptest.NewRecorder()
	server.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusNoContent, resp.Code)
}

func TestLoginJWTMiddlewareBuilderAllowsAnonymousOptionalRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := gin.New()
	server.Use(
		NewLoginJWTMiddlewareBuilder(nil).
			OptionalRoute(http.MethodGet, "/api/v1/pub/articles/:id/interactions/status").
			Build(),
	)
	server.GET("/api/v1/pub/articles/:id/interactions/status", func(ctx *gin.Context) {
		ctx.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/pub/articles/2/interactions/status", nil)
	resp := httptest.NewRecorder()
	server.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusNoContent, resp.Code)
}

func TestLoginJWTMiddlewareBuilderMatchesExactRouteTemplate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	jwtHandler := jwtmocks.NewMockJWTHandler(ctrl)
	jwtHandler.EXPECT().ExtractToken(gomock.Any()).Return("")
	jwtHandler.EXPECT().ParseAccessToken("").Return(nil, errors.New("缺少访问令牌"))

	server := gin.New()
	server.Use(
		NewLoginJWTMiddlewareBuilder(jwtHandler).
			IgnoreRoute(http.MethodGet, "/api/v1/pub/articles/:id/interactions").
			Build(),
	)
	server.GET("/api/v1/private/articles/:id/interactions", func(ctx *gin.Context) {
		ctx.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/private/articles/2/interactions", nil)
	resp := httptest.NewRecorder()
	server.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestLoginJWTMiddlewareBuilderSessionCheck(t *testing.T) {
	type contextKey struct{}
	testKey := contextKey{}

	tests := []struct {
		name       string
		sessionErr error
		wantStatus int
		wantEvent  string
	}{
		{name: "会话有效", wantStatus: http.StatusNoContent},
		{
			name:       "会话已撤销",
			sessionErr: ijwt.ErrSessionRevoked,
			wantStatus: http.StatusUnauthorized,
			wantEvent:  "auth.jwt_check_failed",
		},
		{
			name:       "会话依赖异常",
			sessionErr: errors.New("redis unavailable"),
			wantStatus: http.StatusServiceUnavailable,
			wantEvent:  "auth.session_check_failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			jwtHandler := jwtmocks.NewMockJWTHandler(ctrl)
			jwtHandler.EXPECT().ExtractToken(gomock.Any()).Return("access-token")
			jwtHandler.EXPECT().ParseAccessToken("access-token").Return(&ijwt.UserClaims{
				Uid:  1,
				Ssid: "ssid-1",
			}, nil)
			jwtHandler.EXPECT().CheckSession(gomock.Any(), "ssid-1").
				DoAndReturn(func(ctx context.Context, _ string) error {
					assert.Equal(t, "request-value", ctx.Value(testKey))
					return tt.sessionErr
				})

			server := gin.New()
			var securityEvent string
			server.Use(func(ctx *gin.Context) {
				ctx.Next()
				securityEvent = ctx.GetString("security_event")
			})
			server.Use(NewLoginJWTMiddlewareBuilder(jwtHandler).Build())
			server.GET("/protected", func(ctx *gin.Context) {
				ctx.Status(http.StatusNoContent)
			})

			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			req = req.WithContext(context.WithValue(req.Context(), testKey, "request-value"))
			resp := httptest.NewRecorder()
			server.ServeHTTP(resp, req)

			assert.Equal(t, tt.wantStatus, resp.Code)
			assert.Equal(t, tt.wantEvent, securityEvent)
		})
	}
}
